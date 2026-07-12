// Live ledger sync (SSE) and a small per-IP rate limiter for the
// mutation endpoints.
package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// hub fans a "ledger changed" tick out to every open event stream, so
// two open maps see each other's claims without reloading.
type hub struct {
	mu   sync.Mutex
	subs map[chan struct{}]bool
}

func newHub() *hub {
	return &hub{subs: map[chan struct{}]bool{}}
}

func (h *hub) notify() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- struct{}{}:
		default: // slow subscriber already has a tick pending
		}
	}
}

func (h *hub) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.subs[ch] = true
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan struct{}) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

// handleEvents streams "ledger" events until the client goes away.
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "no streaming")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)
	fmt.Fprint(w, "event: hello\ndata: {}\n\n")
	fl.Flush()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprint(w, "event: ledger\ndata: {}\n\n")
			fl.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			fl.Flush()
		}
	}
}

// limiter: token bucket per client IP. Acts are physical-world events
// (a pledge is a promise, a flip took a crowbar); nobody honest needs
// more than a burst of a few per minute.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(rate, burst float64) *limiter {
	return &limiter{
		buckets: map[string]*bucket{},
		rate:    rate,
		burst:   burst,
	}
}

func (l *limiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 10_000 { // abuse backstop: shed stale state
		for k, b := range l.buckets {
			if now.Sub(b.last) > time.Hour {
				delete(l.buckets, k)
			}
		}
	}
	b := l.buckets[ip]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[ip] = b
	}
	b.tokens = min(l.burst,
		b.tokens+l.rate*now.Sub(b.last).Seconds())
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// limit wraps a mutation handler with the per-IP rate limiter.
func (s *server) limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !s.limiter.allow(ip, time.Now()) {
			writeErr(w, http.StatusTooManyRequests,
				"easy — the ledger takes a few acts per minute at most")
			return
		}
		next(w, r)
	}
}
