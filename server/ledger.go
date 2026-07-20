package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const claimM2 = 100 // one 10 m pixel

// A claim is a pledge to depave one pixel. It either gets flipped
// (photo proof) before its deadline or expires and the pixel returns
// to the pool. A join is a standing signature on a block — the
// petition local governance can see.
//
// GDPR discipline (design caveat): the ledger stores only what the
// public board shows — pixel, pseudonym, timestamps — plus a bearer
// token per act so its author can flip or erase it. No IPs, no
// accounts. Erasure removes the record entirely.
type claim struct {
	Pe       int        `json:"pe"`
	Pn       int        `json:"pn"`
	Kind     string     `json:"kind"` // depave | tree | coolroof
	V        int        `json:"v"`    // sealed % at pledge time
	Name     string     `json:"name,omitempty"`
	TS       time.Time  `json:"ts"`
	Deadline time.Time  `json:"deadline"`
	Flipped  *time.Time `json:"flipped,omitempty"`
	Photo    string     `json:"photo,omitempty"`
	Token    string     `json:"token,omitempty"` // never in GET output
}

// A join is a standing signature on a 150 m block — the petition a
// local government can see: who stands behind cooling this place,
// and how the block's modeled delta moved since they signed.
type join struct {
	Be    int       `json:"be"` // block easting index (pe / 15)
	Bn    int       `json:"bn"`
	Name  string    `json:"name,omitempty"`
	TS    time.Time `json:"ts"`
	Token string    `json:"token,omitempty"` // never in GET output
}

type ledger struct {
	Claims []claim `json:"claims"`
	Joins  []join  `json:"joins"`
}

const blockPx = 15 // 150 m blocks: the night-window scale

func blockOf(pe, pn int) (int, int) {
	return pe / blockPx, pn / blockPx
}

// blockCooling: modeled night cooling delivered inside a block by
// acts done since t.
func (l *ledger) blockCooling(be, bn int, since time.Time) float64 {
	sum := 0.0
	for i := range l.Claims {
		c := &l.Claims[i]
		if c.Flipped == nil || c.Flipped.Before(since) {
			continue
		}
		if cbe, cbn := blockOf(c.Pe, c.Pn); cbe == be && cbn == bn {
			sum += nightCooling(c)
		}
	}
	return sum
}

const (
	statusPledged = "pledged"
	statusFlipped = "flipped"
	statusExpired = "expired"
)

func (c *claim) status(now time.Time) string {
	if c.Flipped != nil {
		return statusFlipped
	}
	if now.After(c.Deadline) {
		return statusExpired
	}
	return statusPledged
}

// loadLedger reads the ledger, accepting the pre-lifecycle format
// (a bare claims array) and giving those claims a deadline.
func loadLedger(path string, expiry time.Duration) (*ledger, error) {
	l := &ledger{}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
		err = json.Unmarshal(raw, &l.Claims)
	} else {
		err = json.Unmarshal(raw, l)
	}
	if err != nil {
		return nil, err
	}
	for i := range l.Claims {
		if l.Claims[i].Deadline.IsZero() {
			l.Claims[i].Deadline = l.Claims[i].TS.Add(expiry)
		}
		if l.Claims[i].Kind == "" { // pre-acts ledgers were all depaves
			l.Claims[i].Kind = "depave"
		}
		if l.Claims[i].V == 0 { // pre-snapshot claims: hard-sealed floor
			l.Claims[i].V = hardSealed
		}
	}
	return l, nil
}

// persist writes the ledger atomically and durably: tmp file, fsync,
// rename, directory fsync. 0600 — the file holds the bearer tokens
// that are the only proof of authorship; no other local user gets to
// read them.
func (l *ledger) persist(path string) error {
	raw, err := json.MarshalIndent(l, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil { // survive power loss, not crash
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		d.Sync() // the rename is durable only after a dir sync
		d.Close()
	}
	return nil
}

// activeAt returns the claim currently holding (pe, pn): a live pledge
// or a flip. Expired claims stay in the ledger as history but hold
// nothing.
func (l *ledger) activeAt(pe, pn int, now time.Time) *claim {
	for i := range l.Claims {
		c := &l.Claims[i]
		if c.Pe == pe && c.Pn == pn && c.status(now) != statusExpired {
			return c
		}
	}
	return nil
}

// greenSet is the set of pixels that count as green for the cascade:
// live-or-flipped depaves and trees (both extend the living network;
// a cool surface is still sealed).
func (l *ledger) greenSet(now time.Time) map[[2]int]bool {
	set := map[[2]int]bool{}
	for i := range l.Claims {
		c := &l.Claims[i]
		if c.status(now) != statusExpired && c.Kind != "coolroof" {
			set[[2]int{c.Pe, c.Pn}] = true
		}
	}
	return set
}

type rank struct {
	Name      string  `json:"name"`
	PledgedM2 int     `json:"pledged_m2"`
	FlippedM2 int     `json:"flipped_m2"`
	NightMC   float64 `json:"night_mdegc"` // own done acts
	BlockMC   float64 `json:"block_mdegc"` // avg block delta since join
	Blocks    int     `json:"blocks"`      // blocks petitioned
}

// nightCooling scores one done act in modeled milli-degrees of
// block-average night cooling. Mirrors web/src/lib/heat.js: night
// coefficient 4 degC over a 150 m window (31x31 pixels; interior
// window assumed), act night factors depave 0, tree 1, coolroof 0.9.
// Small numbers are honest numbers: a depave is worth ~4 m degC to
// its block's nights.
func nightCooling(c *claim) float64 {
	factor := map[string]float64{
		"depave": 0, "tree": 1, "coolroof": 0.9,
	}[c.Kind]
	return 4.0 * float64(c.V) / 100 * (1 - factor) / (31 * 31) * 1000
}

// leaderboard keeps claimed and measured-ish apart (design rule 1):
// flipped m² and still-pledged m² are separate columns, and expired
// pledges count for nothing.
func (l *ledger) leaderboard(now time.Time, top int) []rank {
	byName := map[string]*rank{}
	for i := range l.Claims {
		c := &l.Claims[i]
		st := c.status(now)
		if st == statusExpired {
			continue
		}
		name := c.Name
		if name == "" {
			name = "anonymous"
		}
		r := byName[name]
		if r == nil {
			r = &rank{Name: name}
			byName[name] = r
		}
		if st == statusFlipped {
			r.FlippedM2 += claimM2
			r.NightMC += nightCooling(c)
		} else {
			r.PledgedM2 += claimM2
		}
	}
	// communal column: for each signatory, the average modeled night
	// cooling of their blocks since they signed — the outcome score
	byJoiner := map[string][]join{}
	for _, j := range l.Joins {
		n := j.Name
		if n == "" {
			n = "anonymous"
		}
		byJoiner[n] = append(byJoiner[n], j)
	}
	for n, joins := range byJoiner {
		r := byName[n]
		if r == nil {
			r = &rank{Name: n}
			byName[n] = r
		}
		sum := 0.0
		for _, j := range joins {
			sum += l.blockCooling(j.Be, j.Bn, j.TS)
		}
		r.BlockMC = sum / float64(len(joins))
		r.Blocks = len(joins)
	}
	out := make([]rank, 0, len(byName))
	for _, r := range byName {
		out = append(out, *r)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].BlockMC != out[b].BlockMC {
			return out[a].BlockMC > out[b].BlockMC
		}
		if out[a].NightMC != out[b].NightMC {
			return out[a].NightMC > out[b].NightMC
		}
		if out[a].FlippedM2 != out[b].FlippedM2 {
			return out[a].FlippedM2 > out[b].FlippedM2
		}
		if out[a].PledgedM2 != out[b].PledgedM2 {
			return out[a].PledgedM2 > out[b].PledgedM2
		}
		return out[a].Name < out[b].Name
	})
	if len(out) > top {
		out = out[:top]
	}
	return out
}

func newToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the OS entropy source is gone; nothing sane to do
	}
	return hex.EncodeToString(b)
}

// tokenMatch compares a presented token against the stored one in
// constant time; empty tokens never match.
func tokenMatch(got, want string) bool {
	return got != "" &&
		subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
