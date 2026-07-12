package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	t0     = time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	expiry = 90 * 24 * time.Hour
)

func pledge(x, y int, name string, ts time.Time) claim {
	return claim{
		Pe: x, Pn: y, Kind: "depave", Name: name, TS: ts,
		Deadline: ts.Add(expiry), Token: newToken(),
	}
}

func TestNightCooling(t *testing.T) {
	d := pledge(1, 1, "a", t0)
	d.V = 95
	if got := nightCooling(&d); got < 3.9 || got > 4.0 {
		t.Fatalf("depave at 95%%: got %.3f m degC, want ~3.95", got)
	}
	tr := pledge(2, 2, "b", t0)
	tr.Kind = "tree"
	tr.V = 95
	if got := nightCooling(&tr); got != 0 {
		t.Fatalf("trees cool the day, not the night: got %.3f", got)
	}
	cr := pledge(3, 3, "c", t0)
	cr.Kind = "coolroof"
	cr.V = 100
	if got := nightCooling(&cr); got < 0.3 || got > 0.5 {
		t.Fatalf("coolroof: got %.3f m degC, want ~0.42", got)
	}
}

func TestGreenSetExcludesCoolroofs(t *testing.T) {
	tree := pledge(1, 1, "a", t0)
	tree.Kind = "tree"
	roof := pledge(2, 2, "b", t0)
	roof.Kind = "coolroof"
	l := &ledger{Claims: []claim{tree, roof}}
	set := l.greenSet(t0)
	if !set[[2]int{1, 1}] {
		t.Fatal("trees extend the living network")
	}
	if set[[2]int{2, 2}] {
		t.Fatal("cool surfaces are still sealed")
	}
}

func TestLegacyClaimsBecomeDepaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claims.json")
	legacy := `{"claims":[{"pe":1,"pn":1,"ts":"2026-07-12T11:00:00Z",` +
		`"deadline":"2026-10-10T11:00:00Z"}],"watches":[]}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := loadLedger(path, expiry)
	if err != nil {
		t.Fatal(err)
	}
	if l.Claims[0].Kind != "depave" {
		t.Fatalf("kindless claim should migrate to depave, got %q",
			l.Claims[0].Kind)
	}
}

func TestClaimLifecycle(t *testing.T) {
	c := pledge(1, 2, "mia", t0)
	if got := c.status(t0); got != statusPledged {
		t.Fatalf("fresh claim: got %q, want pledged", got)
	}
	if got := c.status(t0.Add(expiry + time.Hour)); got != statusExpired {
		t.Fatalf("past deadline: got %q, want expired", got)
	}
	flipTS := t0.Add(time.Hour)
	c.Flipped = &flipTS
	if got := c.status(t0.Add(expiry + time.Hour)); got != statusFlipped {
		t.Fatalf("flipped claims never expire: got %q", got)
	}
}

func TestExpiryFreesThePixel(t *testing.T) {
	l := &ledger{Claims: []claim{pledge(5, 5, "a", t0)}}
	if l.activeAt(5, 5, t0) == nil {
		t.Fatal("live pledge should hold its pixel")
	}
	late := t0.Add(expiry + time.Hour)
	if l.activeAt(5, 5, late) != nil {
		t.Fatal("expired pledge should release its pixel")
	}
	if l.greenSet(late)[[2]int{5, 5}] {
		t.Fatal("expired pledge should not count as green")
	}
}

func TestLeaderboardSeparatesColumns(t *testing.T) {
	flipTS := t0.Add(time.Hour)
	l := &ledger{Claims: []claim{
		pledge(1, 1, "mia", t0),
		pledge(2, 2, "mia", t0),
		pledge(3, 3, "", t0),
	}}
	l.Claims[1].Flipped = &flipTS
	ranks := l.leaderboard(t0.Add(2*time.Hour), 20)
	if len(ranks) != 2 {
		t.Fatalf("got %d ranks, want 2", len(ranks))
	}
	if ranks[0].Name != "mia" || ranks[0].FlippedM2 != claimM2 ||
		ranks[0].PledgedM2 != claimM2 {
		t.Fatalf("mia rank wrong: %+v", ranks[0])
	}
	if ranks[1].Name != "anonymous" || ranks[1].PledgedM2 != claimM2 {
		t.Fatalf("anonymous rank wrong: %+v", ranks[1])
	}
	// expired pledges count for nothing
	ranks = l.leaderboard(t0.Add(expiry+time.Hour), 20)
	if len(ranks) != 1 || ranks[0].PledgedM2 != 0 {
		t.Fatalf("after expiry only the flip should remain: %+v", ranks)
	}
}

func TestLoadLegacyFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claims.json")
	legacy := `[{"pe":26,"pn":21,"name":"mia","ts":"2026-07-12T11:00:00Z"}]`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := loadLedger(path, expiry)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Claims) != 1 {
		t.Fatalf("got %d claims, want 1", len(l.Claims))
	}
	c := l.Claims[0]
	if c.Deadline != c.TS.Add(expiry) {
		t.Fatalf("legacy claim should gain deadline, got %v", c.Deadline)
	}
	if c.status(c.TS) != statusPledged {
		t.Fatalf("legacy claim should be a live pledge")
	}
}

func TestBlockCooling(t *testing.T) {
	flipTS := t0.Add(time.Hour)
	inside := pledge(16, 16, "a", t0) // block (1,1)
	inside.V = 95
	inside.Flipped = &flipTS
	outside := pledge(50, 50, "a", t0) // block (3,3)
	outside.V = 95
	outside.Flipped = &flipTS
	l := &ledger{Claims: []claim{inside, outside}}
	got := l.blockCooling(1, 1, t0)
	if got < 3.9 || got > 4.0 {
		t.Fatalf("block (1,1) cooling: got %.3f, want ~3.95", got)
	}
	if l.blockCooling(1, 1, t0.Add(2*time.Hour)) != 0 {
		t.Fatal("acts before the signature don't count")
	}
}

func TestLeaderboardBlockColumn(t *testing.T) {
	flipTS := t0.Add(2 * time.Hour)
	act := pledge(16, 16, "worker", t0)
	act.V = 95
	act.Flipped = &flipTS
	l := &ledger{
		Claims: []claim{act},
		Joins:  []join{{Be: 1, Bn: 1, Name: "signer", TS: t0}},
	}
	ranks := l.leaderboard(t0.Add(3*time.Hour), 20)
	if ranks[0].Name != "signer" || ranks[0].BlockMC < 3.9 {
		t.Fatalf("signer should lead on block delta: %+v", ranks)
	}
}

func TestPersistRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claims.json")
	l := &ledger{
		Claims: []claim{pledge(1, 1, "mia", t0)},
		Joins:  []join{{Be: 2, Bn: 2, Name: "ana", TS: t0, Token: "t"}},
	}
	if err := l.persist(path); err != nil {
		t.Fatal(err)
	}
	got, err := loadLedger(path, expiry)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Claims) != 1 || len(got.Joins) != 1 {
		t.Fatalf("round trip lost records: %+v", got)
	}
	if got.Claims[0].Token != l.Claims[0].Token {
		t.Fatal("token must survive persistence (it is the only key)")
	}
}
