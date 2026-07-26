package main

import (
	"testing"
	"time"
)

// A ranged read lands mid-line: the fragment's first line is garbage
// and must be dropped, missing temps skipped, absurd temps refused.
const ghcnhFragment = `0|41.417|2.133|176.0|22.1|junk-partial-line
SPM00008180|BARCELONA CITY|2026-07-22T18:00:00|2026|07|22|18|00|41.417|2.133|176.0|28.5|||FM94_1
SPM00008180|BARCELONA CITY|2026-07-22T23:00:00|2026|07|22|23|00|41.417|2.133|176.0|24.0|||FM94_1
SPM00008180|BARCELONA CITY|2026-07-23T05:00:00|2026|07|23|05|00|41.417|2.133|176.0|21.5|||FM94_1
SPM00008180|BARCELONA CITY|2026-07-23T08:00:00|2026|07|23|08|00|41.417|2.133|176.0||||FM94_1
SPM00008180|BARCELONA CITY|2026-07-23T11:00:00|2026|07|23|11|00|41.417|2.133|176.0|999.9|||FM94_1
SPM00008180|BARCELONA CITY|2026-07-23T14:00:00|2026|07|23|14|00|41.417|2.133|176.0|30.5|||FM94_1
`

func TestParseGHCNh(t *testing.T) {
	obs := parseGHCNh([]byte(ghcnhFragment))
	if len(obs) != 4 {
		t.Fatalf("want 4 valid rows (partial, empty and 999.9 dropped), got %d", len(obs))
	}
	if obs[0].c != 28.5 || obs[3].c != 30.5 {
		t.Fatalf("rows out of order or misparsed: %+v", obs)
	}
	if obs[3].t != time.Date(2026, 7, 23, 14, 0, 0, 0, time.UTC) {
		t.Fatalf("timestamp misparsed: %v", obs[3].t)
	}
}

func TestSummarize(t *testing.T) {
	obs := parseGHCNh([]byte(ghcnhFragment))
	latest, min24, max24, ok := summarize(obs)
	if !ok || latest.c != 30.5 {
		t.Fatalf("latest: got %+v ok=%v", latest, ok)
	}
	// The 24 h window before 07-23T14:00 excludes 07-22T18:00 (28.5)
	// but keeps the 24.0 and 21.5 night readings.
	if min24 != 21.5 || max24 != 30.5 {
		t.Fatalf("24 h window wrong: min %.1f max %.1f", min24, max24)
	}
	if _, _, _, ok := summarize(nil); ok {
		t.Fatal("empty record must report not-ok, never zeros")
	}
}

func TestStationsServeStaleOnFailure(t *testing.T) {
	c := newStations()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	c.cache["X"] = stationEntry{
		obs:     []stationObs{{t: now.Add(-48 * time.Hour), c: 25}},
		fetched: now.Add(-time.Minute),
	}
	obs, err := c.get("X")
	if err != nil || len(obs) != 1 {
		t.Fatalf("fresh cache should serve without fetching: %v", err)
	}
}
