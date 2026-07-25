// KPIs — the instrument reads its own dials. Everything here is an
// aggregate the public board already implies (docs/KPIS.md defines
// the panel): act counts and modeled cooling from the ledger, and a
// time axis for the usage counters, which are lifetime totals and so
// can't answer "how many found a cool place during the heatwave".
// The time axis is a daily snapshot file of the totals; a day's
// activity is the difference between two snapshots. Approximate by
// design, like the counters themselves: the last write of a day
// stands in for its end, and a lost hour is noise, not a wound.
package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	statsSnapshotEvery = time.Hour
	kpiDays            = 30 // daily series horizon served by /api/kpis
)

// snapshotStats writes the current counter totals to
// <dir>/YYYY-MM-DD.json for now's UTC day. Same-day writes overwrite,
// so each file converges on that day's closing totals.
func (s *server) snapshotStats(dir string, now time.Time) error {
	s.counters.mu.Lock()
	raw, err := json.Marshal(s.counters.n)
	s.counters.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := now.UTC().Format("2006-01-02") + ".json"
	return os.WriteFile(filepath.Join(dir, name), raw, 0o644)
}

// statsSnapshotLoop keeps the day files fresh; one write at startup
// so a young deploy still has today on disk.
func (s *server) statsSnapshotLoop(dir string) {
	for {
		_ = s.snapshotStats(dir, time.Now())
		time.Sleep(statsSnapshotEvery)
	}
}

// dailySeries turns snapshot totals back into per-day deltas for the
// last `days` days before now. A day's delta is its totals minus the
// previous available snapshot's; gaps just widen the base, and a
// negative delta (a totals reset after a lost counters file) clamps
// to zero rather than inventing negative usage.
func dailySeries(dir string, days int, now time.Time) map[string]map[string]int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return map[string]map[string]int64{}
	}
	var dates []string
	for _, e := range entries {
		if n := e.Name(); len(n) == len("2006-01-02.json") &&
			filepath.Ext(n) == ".json" {
			dates = append(dates, n[:len("2006-01-02")])
		}
	}
	sort.Strings(dates)
	read := func(date string) map[string]int64 {
		raw, err := os.ReadFile(filepath.Join(dir, date+".json"))
		if err != nil {
			return nil
		}
		m := map[string]int64{}
		if json.Unmarshal(raw, &m) != nil {
			return nil
		}
		return m
	}
	cutoff := now.UTC().AddDate(0, 0, -days).Format("2006-01-02")
	out := map[string]map[string]int64{}
	var prev map[string]int64
	for _, date := range dates {
		cur := read(date)
		if cur == nil {
			continue
		}
		if date > cutoff && prev != nil {
			delta := map[string]int64{}
			for k, v := range cur {
				if d := v - prev[k]; d > 0 {
					delta[k] = d
				}
			}
			if len(delta) > 0 {
				out[date] = delta
			}
		}
		prev = cur
	}
	return out
}

type kpiActs struct {
	Pledged   int     `json:"pledged"`
	Flipped   int     `json:"flipped"`
	Expired   int     `json:"expired"`
	FlipRate  float64 `json:"flip_rate"` // flipped / resolved
	FlippedM2 int     `json:"flipped_m2"`
	NightMC   float64 `json:"night_mdegc"`
}

type kpiSpread struct {
	BlocksWithActs int `json:"blocks_with_acts"`
	Joins          int `json:"joins"`
	BlocksJoined   int `json:"blocks_joined"`
}

type kpiOut struct {
	Acts         kpiActs                     `json:"acts"`
	Spread       kpiSpread                   `json:"spread"`
	ReportsOpen  int64                       `json:"reports_open"`
	Daily        map[string]map[string]int64 `json:"daily"`
	GeneratedUTC string                      `json:"generated_utc"`
}

// handleKPIs serves the panel defined in docs/KPIS.md. Same
// discipline as the leaderboard: flipped and pledged never sum, and
// expired pledges count for nothing except the flip-rate denominator
// — a promise that died is signal about the funnel, not impact.
func (s *server) handleKPIs(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	var out kpiOut

	s.mu.Lock()
	actBlocks := map[[2]int]bool{}
	for i := range s.ledger.Claims {
		c := &s.ledger.Claims[i]
		switch c.status(now) {
		case statusFlipped:
			out.Acts.Flipped++
			out.Acts.FlippedM2 += claimM2
			out.Acts.NightMC += nightCooling(c)
		case statusExpired:
			out.Acts.Expired++
		default:
			out.Acts.Pledged++
		}
		if c.status(now) != statusExpired {
			be, bn := blockOf(c.Pe, c.Pn)
			actBlocks[[2]int{be, bn}] = true
		}
	}
	joinBlocks := map[[2]int]bool{}
	for _, j := range s.ledger.Joins {
		joinBlocks[[2]int{j.Be, j.Bn}] = true
	}
	out.Spread = kpiSpread{
		BlocksWithActs: len(actBlocks),
		Joins:          len(s.ledger.Joins),
		BlocksJoined:   len(joinBlocks),
	}
	s.mu.Unlock()

	if resolved := out.Acts.Flipped + out.Acts.Expired; resolved > 0 {
		out.Acts.FlipRate =
			float64(out.Acts.Flipped) / float64(resolved)
	}
	s.reports.mu.Lock()
	for _, n := range s.reports.n {
		out.ReportsOpen += n
	}
	s.reports.mu.Unlock()

	out.Daily = dailySeries(s.statsDir, kpiDays, now)
	out.GeneratedUTC = now.UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusOK, out)
}
