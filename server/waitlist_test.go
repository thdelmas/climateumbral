package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func waitlistServer(t *testing.T) *server {
	t.Helper()
	return &server{
		waitlistPath: filepath.Join(t.TempDir(), "waitlist.jsonl"),
		notify:       newNotifier(),
	}
}

func postWaitlist(s *server, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/waitlist", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleWaitlistJoin(w, r)
	return w
}

func TestWaitlistStoresValidSignup(t *testing.T) {
	s := waitlistServer(t)
	w := postWaitlist(s, `{"email":"Ana@Example.ORG","status":"farmer",
		"country":"ES","region":"Priorat","city":"Falset","lang":"es"}`)
	if w.Code != 204 {
		t.Fatalf("want 204, got %d: %s", w.Code, w.Body)
	}
	raw, _ := os.ReadFile(s.waitlistPath)
	if !strings.Contains(string(raw), `"ana@example.org"`) {
		t.Fatalf("email must be stored lowercased: %s", raw)
	}
	if !strings.Contains(string(raw), `"Priorat"`) {
		t.Fatalf("region must be stored: %s", raw)
	}
}

func TestWaitlistHoneypotStoresNothing(t *testing.T) {
	s := waitlistServer(t)
	w := postWaitlist(s, `{"email":"bot@spam.io","status":"farmer",
		"country":"ES","website":"gotcha"}`)
	if w.Code != 204 {
		t.Fatalf("honeypot must answer like success, got %d", w.Code)
	}
	if _, err := os.Stat(s.waitlistPath); !os.IsNotExist(err) {
		t.Fatal("honeypot submission must not touch the file")
	}
}

func TestWaitlistValidation(t *testing.T) {
	for name, body := range map[string]string{
		"bad email":       `{"email":"nope","status":"farmer","country":"ES"}`,
		"bad status":      `{"email":"a@b.co","status":"alien","country":"ES"}`,
		"missing country": `{"email":"a@b.co","status":"farmer"}`,
		"bad country":     `{"email":"a@b.co","status":"farmer","country":"Narnia"}`,
		"html email":      `{"email":"x@y.z<svg>","status":"farmer","country":"ES"}`,
	} {
		s := waitlistServer(t)
		if w := postWaitlist(s, body); w.Code != 400 {
			t.Fatalf("%s: want 400, got %d", name, w.Code)
		}
	}
}

func TestWaitlistReadIsAdminOnly(t *testing.T) {
	s := waitlistServer(t)
	r := httptest.NewRequest("GET", "/api/waitlist", nil)
	w := httptest.NewRecorder()
	s.handleWaitlist(w, r)
	if w.Code != 403 {
		t.Fatalf("unauthenticated read: want 403, got %d", w.Code)
	}
}
