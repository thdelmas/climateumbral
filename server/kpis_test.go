package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSnapshot(t *testing.T, dir, date string, m map[string]int64) {
	t.Helper()
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(
		filepath.Join(dir, date+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A day's usage is the difference between two snapshots of lifetime
// totals: gaps widen the base, resets clamp to zero, the horizon
// cuts old days, and the oldest snapshot is only ever a base.
func TestDailySeries(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	writeSnapshot(t, dir, "2026-06-01",
		map[string]int64{"cool_route": 5}) // outside the horizon
	writeSnapshot(t, dir, "2026-07-22",
		map[string]int64{"cool_route": 10, "cool_open": 40})
	// no file for the 23rd: the 24th's delta spans the gap
	writeSnapshot(t, dir, "2026-07-24",
		map[string]int64{"cool_route": 13, "cool_open": 40})
	writeSnapshot(t, dir, "2026-07-25",
		map[string]int64{"cool_route": 2, "cool_open": 41}) // reset

	got := dailySeries(dir, 30, now)
	if got["2026-07-22"]["cool_route"] != 5 {
		t.Errorf("22nd: got %v, want cool_route 5 over the old base",
			got["2026-07-22"])
	}
	if got["2026-07-24"]["cool_route"] != 3 {
		t.Errorf("24th: got %v, want cool_route 3", got["2026-07-24"])
	}
	if _, ok := got["2026-07-24"]["cool_open"]; ok {
		t.Error("24th: zero-delta events must not appear")
	}
	if d := got["2026-07-25"]; d["cool_route"] != 0 || d["cool_open"] != 1 {
		t.Errorf("25th: got %v, want reset clamped, cool_open 1", d)
	}
	if _, ok := got["2026-06-01"]; ok {
		t.Error("a day outside the horizon must not appear")
	}
	if empty := dailySeries(t.TempDir(), 30, now); len(empty) != 0 {
		t.Errorf("empty dir: got %v, want nothing", empty)
	}
}

func TestSnapshotStats(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "stats")
	s := &server{counters: &counters{n: map[string]int64{
		"cool_route": 7,
	}}}
	now := time.Date(2026, 7, 25, 23, 0, 0, 0, time.UTC)
	if err := s.snapshotStats(dir, now); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "2026-07-25.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]int64{}
	if json.Unmarshal(raw, &m) != nil || m["cool_route"] != 7 {
		t.Fatalf("snapshot content: %s", raw)
	}
}

// The panel keeps the board's discipline: flipped and pledged never
// sum, expired counts only in the flip-rate denominator, and spread
// ignores expired pixels.
func TestHandleKPIs(t *testing.T) {
	now := time.Now()
	flip := func(x, y int, kind string, v int) claim {
		c := pledge(x, y, "a", now.Add(-time.Hour))
		c.Kind, c.V = kind, v
		f := now.Add(-time.Minute)
		c.Flipped = &f
		return c
	}
	dead := pledge(50, 50, "b", now.Add(-2*expiry))
	dead.Deadline = now.Add(-expiry)
	s := &server{
		counters: &counters{n: map[string]int64{}},
		reports: &reportStore{n: map[string]int64{
			"p1|closed": 2, "p2|full": 1,
		}},
		statsDir: t.TempDir(),
		ledger: &ledger{
			Claims: []claim{
				flip(1, 1, "tree", 95),     // block 0,0
				flip(2, 2, "coolroof", 95), // block 0,0
				pledge(100, 100, "c", now), // block 6,6, open
				dead,                       // resolved, lost
			},
			Joins: []join{
				{Be: 0, Bn: 0, TS: now}, {Be: 0, Bn: 0, TS: now},
				{Be: 9, Bn: 9, TS: now},
			},
		},
	}
	w := httptest.NewRecorder()
	s.handleKPIs(w, httptest.NewRequest("GET", "/api/kpis", nil))
	var out kpiOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Acts.Flipped != 2 || out.Acts.Pledged != 1 ||
		out.Acts.Expired != 1 {
		t.Fatalf("acts: %+v", out.Acts)
	}
	if out.Acts.FlipRate != 2.0/3.0 {
		t.Errorf("flip rate: got %v, want 2/3 of resolved", out.Acts.FlipRate)
	}
	if out.Acts.FlippedM2 != 2*claimM2 {
		t.Errorf("flipped m2: got %d", out.Acts.FlippedM2)
	}
	if out.Acts.NightMC <= 0 {
		t.Errorf("night m degC: got %v, want > 0", out.Acts.NightMC)
	}
	if out.Spread.BlocksWithActs != 2 { // 0,0 and 6,6; expired 3,3 out
		t.Errorf("blocks with acts: got %d, want 2",
			out.Spread.BlocksWithActs)
	}
	if out.Spread.Joins != 3 || out.Spread.BlocksJoined != 2 {
		t.Errorf("joins: %+v", out.Spread)
	}
	if out.ReportsOpen != 3 {
		t.Errorf("reports open: got %d, want 3", out.ReportsOpen)
	}
}
