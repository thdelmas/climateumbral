package main

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := newLimiter(0.2, 5)
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if !l.allow("1.2.3.4", now) {
			t.Fatalf("burst request %d should pass", i)
		}
	}
	if l.allow("1.2.3.4", now) {
		t.Fatal("6th immediate request should be limited")
	}
	if !l.allow("5.6.7.8", now) {
		t.Fatal("other IPs are unaffected")
	}
	if !l.allow("1.2.3.4", now.Add(6*time.Second)) {
		t.Fatal("tokens refill over time")
	}
}

func TestHubNotify(t *testing.T) {
	h := newHub()
	ch := h.subscribe()
	h.notify()
	select {
	case <-ch:
	default:
		t.Fatal("subscriber should receive a tick")
	}
	h.notify() // pending tick present: must not block
	h.unsubscribe(ch)
	h.notify() // no subscribers: must not panic
}
