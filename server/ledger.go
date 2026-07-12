package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"time"
)

const claimM2 = 100 // one 10 m pixel

// A claim is a pledge to depave one pixel. It either gets flipped
// (photo proof) before its deadline or expires and the pixel returns
// to the pool. A watch is a co-signature on a pixel the watcher can't
// flip themselves — public land, someone else's pledge.
//
// GDPR discipline (design caveat): the ledger stores only what the
// public board shows — pixel, pseudonym, timestamps — plus a bearer
// token per act so its author can flip or erase it. No IPs, no
// accounts. Erasure removes the record entirely.
type claim struct {
	Pe       int        `json:"pe"`
	Pn       int        `json:"pn"`
	Kind     string     `json:"kind"` // depave | tree | coolroof
	Name     string     `json:"name,omitempty"`
	TS       time.Time  `json:"ts"`
	Deadline time.Time  `json:"deadline"`
	Flipped  *time.Time `json:"flipped,omitempty"`
	Photo    string     `json:"photo,omitempty"`
	Token    string     `json:"token,omitempty"` // never in GET output
}

type watch struct {
	Pe    int       `json:"pe"`
	Pn    int       `json:"pn"`
	Name  string    `json:"name,omitempty"`
	TS    time.Time `json:"ts"`
	Token string    `json:"token,omitempty"` // never in GET output
}

type ledger struct {
	Claims  []claim `json:"claims"`
	Watches []watch `json:"watches"`
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
	}
	return l, nil
}

func (l *ledger) persist(path string) error {
	raw, err := json.MarshalIndent(l, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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

func (l *ledger) watchesAt(pe, pn int) []watch {
	var out []watch
	for _, w := range l.Watches {
		if w.Pe == pe && w.Pn == pn {
			out = append(out, w)
		}
	}
	return out
}

type rank struct {
	Name      string `json:"name"`
	PledgedM2 int    `json:"pledged_m2"`
	FlippedM2 int    `json:"flipped_m2"`
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
		} else {
			r.PledgedM2 += claimM2
		}
	}
	out := make([]rank, 0, len(byName))
	for _, r := range byName {
		out = append(out, *r)
	}
	sort.Slice(out, func(a, b int) bool {
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
