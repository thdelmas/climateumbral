// The Renfort waitlist — consent-based signups to be notified when
// the citizen-resource registry opens in an area. Same privacy rules
// as the suggestion box: never public, admin-only reads, and nothing
// stored beyond what the person typed plus the moment it arrived.
// This is NOT emergency intake: the only promise made is one future
// email.
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var waitlistMu sync.Mutex

var waitlistStatuses = map[string]bool{
	"official": true, "company": true, "farmer": true, "civilian": true,
}

var waitlistEmailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func clipField(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) > n {
		v = v[:n]
	}
	return v
}

func (s *server) handleWaitlistJoin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email   string `json:"email"`
		Status  string `json:"status"`
		Country string `json:"country"`
		Region  string `json:"region"`
		City    string `json:"city"`
		Lang    string `json:"lang"`
		Website string `json:"website"` // honeypot: humans never see it
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body) != nil {
		writeJSON(w, http.StatusBadRequest,
			map[string]any{"error": "bad request"})
		return
	}
	if body.Website != "" { // bot: accept silently, store nothing
		w.WriteHeader(http.StatusNoContent)
		return
	}
	email := strings.ToLower(clipField(body.Email, 200))
	if !waitlistEmailRe.MatchString(email) {
		writeJSON(w, http.StatusBadRequest,
			map[string]any{"error": "invalid email"})
		return
	}
	if !waitlistStatuses[body.Status] {
		writeJSON(w, http.StatusBadRequest,
			map[string]any{"error": "invalid status"})
		return
	}
	country := clipField(body.Country, 60)
	if country == "" {
		writeJSON(w, http.StatusBadRequest,
			map[string]any{"error": "country required"})
		return
	}
	line, _ := json.Marshal(map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339),
		"email":   email,
		"status":  body.Status,
		"country": country,
		"region":  clipField(body.Region, 80),
		"city":    clipField(body.City, 80),
		"lang":    clipField(body.Lang, 5),
	})
	waitlistMu.Lock()
	defer waitlistMu.Unlock()
	f, err := os.OpenFile(s.waitlistPath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]any{"error": "could not save"})
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
	s.notify.event("waitlist")
	w.WriteHeader(http.StatusNoContent)
}

// handleWaitlist: admin-only NDJSON dump, one line per signup as it
// arrived (dedup by email is the reader's judgment call: last signup
// carries the freshest zone).
func (s *server) handleWaitlist(w http.ResponseWriter, r *http.Request) {
	if !s.adminReq(r) {
		writeJSON(w, http.StatusForbidden,
			map[string]any{"error": "admin token required"})
		return
	}
	raw, err := os.ReadFile(s.waitlistPath)
	if err != nil {
		raw = nil
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	_, _ = w.Write(raw)
}
