// Live ledger sync (SSE) and a small per-IP rate limiter for the
// mutation and read endpoints.
package main

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// hub fans a "ledger changed" tick out to every open event stream, so
// two open maps see each other's claims without reloading. It also
// counts open streams per client so one IP cannot hold the server's
// whole connection budget.
type hub struct {
	mu    sync.Mutex
	subs  map[chan struct{}]bool
	perIP map[string]int
	total int
}

const (
	maxStreams      = 512 // open event streams, server-wide
	maxStreamsPerIP = 8
)

func newHub() *hub {
	return &hub{
		subs:  map[chan struct{}]bool{},
		perIP: map[string]int{},
	}
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

// acquire reserves an event-stream slot for ip; release returns it.
func (h *hub) acquire(ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.total >= maxStreams || h.perIP[ip] >= maxStreamsPerIP {
		return false
	}
	h.total++
	h.perIP[ip]++
	return true
}

func (h *hub) release(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.total--
	if h.perIP[ip]--; h.perIP[ip] <= 0 {
		delete(h.perIP, ip)
	}
}

// handleEvents streams "ledger" events until the client goes away.
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "no streaming")
		return
	}
	ip := clientIP(r, s.trustProxy)
	if !s.hub.acquire(ip) {
		writeErr(w, http.StatusTooManyRequests,
			"too many open event streams")
		return
	}
	defer s.hub.release(ip)
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

// limiter: token bucket per client key. Acts are physical-world
// events (a pledge is a promise, a flip took a crowbar); nobody
// honest needs more than a burst of a few per minute.
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
		idle := time.Hour
		if len(l.buckets) > 100_000 { // under active flooding, sooner
			idle = 5 * time.Minute
		}
		for k, b := range l.buckets {
			if now.Sub(b.last) > idle {
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

// clientIP is the rate-limit key. Directly exposed we use RemoteAddr
// (spoofable headers are ignored); behind a reverse proxy,
// -trust-proxy switches to the rightmost X-Forwarded-For hop — the
// one our own proxy appended. IPv6 keys collapse to their /64: one
// household is one bucket, not 2^64 of them.
func clientIP(r *http.Request, trustProxy bool) string {
	host := r.RemoteAddr
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			host = strings.TrimSpace(parts[len(parts)-1])
		}
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	if addr.Is4() || addr.Is4In6() {
		return addr.String()
	}
	return netip.PrefixFrom(addr, 64).Masked().Addr().String()
}

const maxBody = 8 << 10 // mutation bodies are small JSON documents

// limit wraps a mutation handler: per-IP rate limit, JSON-only POSTs
// (a cross-site form can't send application/json without a CORS
// preflight, which we never grant), and bounded request bodies.
func (s *server) limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.allow(clientIP(r, s.trustProxy), time.Now()) {
			writeErr(w, http.StatusTooManyRequests,
				"easy — the ledger takes a few acts per minute at most")
			return
		}
		if r.Method == http.MethodPost {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				writeErr(w, http.StatusUnsupportedMediaType,
					"send application/json")
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		next(w, r)
	}
}

// rlimit wraps read endpoints with the larger read budget: cheap when
// cached, but raster misses fan out to upstream fetches and the
// leaderboard walks the whole ledger.
func (s *server) rlimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, s.trustProxy)
		if !s.readLimiter.allow(ip, time.Now()) {
			writeErr(w, http.StatusTooManyRequests, "slow down")
			return
		}
		next(w, r)
	}
}
