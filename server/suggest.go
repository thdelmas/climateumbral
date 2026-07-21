// The suggestion box — free text, so the rules differ from every
// other store here: suggestions are NOT public (people paste
// anything into text boxes, including things about themselves),
// reading them requires the admin token, and nothing about the
// sender is recorded beyond the moment it arrived.
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var suggestMu sync.Mutex

func (s *server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&body) != nil {
		writeJSON(w, http.StatusBadRequest,
			map[string]any{"error": "bad request"})
		return
	}
	text := strings.TrimSpace(body.Text)
	if len(text) < 3 || len(text) > 2000 {
		writeJSON(w, http.StatusBadRequest,
			map[string]any{"error": "3–2000 characters"})
		return
	}
	line, _ := json.Marshal(map[string]any{
		"ts":   time.Now().UTC().Format(time.RFC3339),
		"text": text,
	})
	suggestMu.Lock()
	defer suggestMu.Unlock()
	f, err := os.OpenFile(s.suggestPath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]any{"error": "could not save"})
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
	w.WriteHeader(http.StatusNoContent)
}

// handleSuggestions: admin-only. With no admin token configured the
// box is write-only from the network — readable on the box itself.
func (s *server) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	if s.adminToken == "" ||
		r.Header.Get(tokenHeader) != s.adminToken {
		writeJSON(w, http.StatusForbidden,
			map[string]any{"error": "admin token required"})
		return
	}
	raw, err := os.ReadFile(s.suggestPath)
	if err != nil {
		raw = nil
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	_, _ = w.Write(raw)
}
