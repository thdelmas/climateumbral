// Usage counters — the minimum a distribution effort needs, and not
// one bit more. Whole-number tallies per named event, nothing else:
// no IPs, no sessions, no timestamps per hit, no user agents. The
// counts are public (GET /api/stats) — a project that stores only
// what its board shows has nothing to hide about its own traffic.
package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
)

// countEvents: the closed set a client may report. Anything else is
// dropped without note — the whitelist is the privacy boundary.
var countEvents = map[string]bool{
	"cool_open":   true, // panic screen opened
	"cool_locate": true, // find-near-me tapped
	"cool_route":  true, // a route button tapped
	"cool_share":  true, // share tapped
}

type counters struct {
	mu    sync.Mutex
	n     map[string]int64
	dirty bool
	path  string
}

func newCounters(path string) *counters {
	c := &counters{n: map[string]int64{}, path: path}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &c.n)
	}
	return c
}

func (c *counters) bump(event string) {
	c.mu.Lock()
	c.n[event]++
	c.dirty = true
	c.mu.Unlock()
}

// persistLoop writes at most once a minute, and only when moved —
// counters are approximate by design; losing a minute on a crash
// is fine, wearing the disk per tap is not.
func (c *counters) persistLoop() {
	for range time.Tick(time.Minute) {
		c.mu.Lock()
		if !c.dirty {
			c.mu.Unlock()
			continue
		}
		raw, err := json.Marshal(c.n)
		c.dirty = false
		c.mu.Unlock()
		if err == nil {
			_ = os.WriteFile(c.path, raw, 0o644)
		}
	}
}

func (s *server) handlePing(w http.ResponseWriter, r *http.Request) {
	var body struct {
		E string `json:"e"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil ||
		!countEvents[body.E] {
		w.WriteHeader(http.StatusNoContent) // never worth an error
		return
	}
	s.counters.bump(body.E)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleStats(w http.ResponseWriter, _ *http.Request) {
	s.counters.mu.Lock()
	out := make(map[string]int64, len(s.counters.n))
	for k, v := range s.counters.n {
		out[k] = v
	}
	s.counters.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}
