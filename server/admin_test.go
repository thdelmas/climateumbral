package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// The moderation token erases any act through the same DELETE
// endpoints players use; with no token configured the door is shut.
func TestAdminTokenErasesAnyAct(t *testing.T) {
	s := &server{
		hub:        newHub(),
		adminToken: "sesame",
		ledgerPath: filepath.Join(t.TempDir(), "claims.json"),
		expiry:     expiry,
		ledger:     &ledger{Claims: []claim{pledge(3, 4, "eve", t0)}},
	}
	del := func(token string) int {
		r := httptest.NewRequest("DELETE", "/api/claims/3/4", nil)
		r.SetPathValue("pe", "3")
		r.SetPathValue("pn", "4")
		if token != "" {
			r.Header.Set(tokenHeader, token)
		}
		w := httptest.NewRecorder()
		s.handleAbandon(w, r)
		return w.Code
	}
	if got := del("guess"); got != http.StatusForbidden {
		t.Fatalf("wrong token: got %d, want 403", got)
	}
	if got := del("sesame"); got != http.StatusNoContent {
		t.Fatalf("admin token: got %d, want 204", got)
	}
	if len(s.ledger.Claims) != 0 {
		t.Fatal("admin delete should erase the claim")
	}

	s.adminToken = "" // moderation off
	s.ledger.Claims = []claim{pledge(3, 4, "eve", t0)}
	if got := del(""); got != http.StatusForbidden {
		t.Fatalf("no admin token set: got %d, want 403", got)
	}
	if len(s.ledger.Claims) != 1 {
		t.Fatal("claim must survive when moderation is off")
	}
}

func TestIsAdmin(t *testing.T) {
	s := &server{}
	if s.isAdmin("") || s.isAdmin("anything") {
		t.Fatal("unset admin token must never match")
	}
	s.adminToken = "sesame"
	if s.isAdmin("") || s.isAdmin("Sesame") {
		t.Fatal("near-misses must not match")
	}
	if !s.isAdmin("sesame") {
		t.Fatal("exact token must match")
	}
}
