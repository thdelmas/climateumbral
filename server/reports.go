// Structured place reports — crowd-verification of the cool-places
// map, in the only form a heat-struck reporter can manage: one tap
// from a fixed set of issues. No free text, no identity, no
// coordinates of the reporter — just "this shelter was closed",
// counted. The counts are public: a city deserves to see what its
// residents report about its network, and so does everyone else.
package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// reportIssues: the closed vocabulary. "good" is deliberate — a
// confirmation is as much verification as a complaint.
var reportIssues = map[string]bool{
	"closed": true, "hours": true, "location": true, "good": true,
}

// targetOK: "refuge:src:lon,lat" or "place:node/123456". Printable
// ASCII, bounded — the target is a key, never rendered as HTML.
func targetOK(t string) bool {
	if len(t) > 140 ||
		(!strings.HasPrefix(t, "refuge:") &&
			!strings.HasPrefix(t, "place:")) {
		return false
	}
	for _, r := range t {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

type reportStore struct {
	mu    sync.Mutex
	n     map[string]int64
	dirty bool
	path  string
}

func newReports(path string) *reportStore {
	r := &reportStore{n: map[string]int64{}, path: path}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &r.n)
	}
	return r
}

func (r *reportStore) persistLoop() {
	for range time.Tick(time.Minute) {
		r.mu.Lock()
		if !r.dirty {
			r.mu.Unlock()
			continue
		}
		raw, err := json.Marshal(r.n)
		r.dirty = false
		r.mu.Unlock()
		if err == nil {
			_ = os.WriteFile(r.path, raw, 0o644)
		}
	}
}

func (s *server) handleReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
		Issue  string `json:"issue"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil ||
		!targetOK(body.Target) || !reportIssues[body.Issue] {
		writeJSON(w, http.StatusBadRequest,
			map[string]any{"error": "bad report"})
		return
	}
	s.reports.mu.Lock()
	s.reports.n[body.Target+"|"+body.Issue]++
	s.reports.dirty = true
	s.reports.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleReports(w http.ResponseWriter, _ *http.Request) {
	s.reports.mu.Lock()
	out := make(map[string]int64, len(s.reports.n))
	for k, v := range s.reports.n {
		out[k] = v
	}
	s.reports.mu.Unlock()
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, out)
}
