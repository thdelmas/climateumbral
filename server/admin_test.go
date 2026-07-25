package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// The whole email-link door, end to end: an unlisted address gets
// the same 202 and no mail; the listed one (case-insensitively)
// gets a link that mints a working session exactly once; asking
// again inside the cooldown mails nothing.
func TestEmailLinkSignIn(t *testing.T) {
	mails := make(chan string, 4)
	s := &server{
		adminToken:  "sesame",
		adminEmails: []string{"Op@Example.org"},
		baseURL:     "https://cu.test",
		mailer: func(to, subject, body string) error {
			mails <- body
			return nil
		},
	}
	login := func(email string) int {
		r := httptest.NewRequest("POST", "/api/admin/login",
			strings.NewReader(`{"email":"`+email+`"}`))
		w := httptest.NewRecorder()
		s.handleAdminLogin(w, r)
		return w.Code
	}
	noMail := func(why string) {
		t.Helper()
		select {
		case <-mails:
			t.Fatal(why)
		case <-time.After(50 * time.Millisecond):
		}
	}

	if got := login("stranger@example.org"); got != http.StatusAccepted {
		t.Fatalf("unlisted address: got %d, want 202", got)
	}
	noMail("mail sent to an unlisted address")

	if got := login("op@example.org"); got != http.StatusAccepted {
		t.Fatalf("listed address: got %d, want 202", got)
	}
	var body string
	select {
	case body = <-mails:
	case <-time.After(2 * time.Second):
		t.Fatal("no sign-in mail arrived")
	}
	const prefix = "https://cu.test/admin/session?t="
	i := strings.Index(body, prefix)
	if i < 0 {
		t.Fatalf("no link in mail body: %q", body)
	}
	tok := strings.Fields(body[i+len(prefix):])[0]

	exchange := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/admin/session?t="+tok, nil)
		w := httptest.NewRecorder()
		s.handleAdminSession(w, r)
		return w
	}
	w := exchange()
	if w.Code != http.StatusSeeOther ||
		w.Header().Get("Location") != "/admin" {
		t.Fatalf("exchange: got %d -> %q, want 303 -> /admin",
			w.Code, w.Header().Get("Location"))
	}
	var session string
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			session = c.Value
		}
	}
	if session == "" {
		t.Fatal("exchange set no session cookie")
	}
	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
	if !s.adminReq(r) {
		t.Fatal("minted session must pass adminReq")
	}

	if w := exchange(); w.Header().Get("Location") != "/admin?expired=1" {
		t.Fatalf("replayed link: got -> %q, want -> /admin?expired=1",
			w.Header().Get("Location"))
	}

	if got := login("op@example.org"); got != http.StatusAccepted {
		t.Fatalf("cooldown request: got %d, want 202", got)
	}
	noMail("cooldown ignored: second mail inside a minute")
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
