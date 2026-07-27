// The /ayuda relay — a static, trilingual page routing citizens with
// machinery or help toward the OFFICIAL emergency channels, born
// during the July 2026 fires. Its currency is freshness: every entry
// carries a verification date, so the page is served no-cache and the
// HTML itself warns when its stamps age past 48 h.
package main

import (
	_ "embed"
	"net/http"
)

//go:embed ayuda.html
var ayudaHTML []byte

func (s *server) handleAyuda(w http.ResponseWriter, r *http.Request) {
	s.counters.bump("page_ayuda")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(ayudaHTML)
}
