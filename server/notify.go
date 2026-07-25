// Feedback lands in files on a box nobody rereads daily; mail is
// the only channel that reaches the maintainer without polling.
// The notifier turns feedback into digests — counts only, never
// content: suggestion text stays behind the admin token, so a
// third-party mail hop never carries what someone pasted about
// themselves. Outbound goes through authenticated submission
// (587): port 25 is blocked on the hosting account.
package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type notifier struct {
	addr, host, user, pass, to string

	mu       sync.Mutex
	counts   map[string]int
	timer    *time.Timer
	lastSend time.Time
}

const (
	// first event waits this long so near-simultaneous feedback
	// shares one mail
	notifyCoalesce = time.Minute
	// and no two mails ever arrive closer than this — a flood of
	// suggestions costs one digest per interval, not one each
	notifyInterval = 15 * time.Minute
)

// newNotifier reads CLIMATEUMBRAL_SMTP_HOST/PORT/USER/PASS and
// CLIMATEUMBRAL_NOTIFY_TO. Any of them missing = notifications off,
// said once in the log; the nil receiver makes every event a no-op.
func newNotifier() *notifier {
	host := os.Getenv("CLIMATEUMBRAL_SMTP_HOST")
	user := os.Getenv("CLIMATEUMBRAL_SMTP_USER")
	pass := os.Getenv("CLIMATEUMBRAL_SMTP_PASS")
	to := os.Getenv("CLIMATEUMBRAL_NOTIFY_TO")
	if host == "" || user == "" || pass == "" || to == "" {
		log.Printf("notify: off (set CLIMATEUMBRAL_SMTP_HOST/USER/" +
			"PASS and CLIMATEUMBRAL_NOTIFY_TO to enable)")
		return nil
	}
	port := os.Getenv("CLIMATEUMBRAL_SMTP_PORT")
	if port == "" {
		port = "587"
	}
	log.Printf("notify: on, digests to %s via %s:%s", to, host, port)
	return &notifier{
		addr: host + ":" + port, host: host,
		user: user, pass: pass, to: to,
		counts: map[string]int{},
	}
}

// event records one piece of feedback; kind is a short plural
// ("suggestions", "reports"). Safe from any goroutine.
func (n *notifier) event(kind string) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.counts[kind]++
	if n.timer != nil {
		return // a digest is already scheduled; this event joins it
	}
	delay := notifyCoalesce
	if since := time.Since(n.lastSend); since < notifyInterval {
		delay = notifyInterval - since
	}
	n.timer = time.AfterFunc(delay, n.send)
}

func (n *notifier) send() {
	n.mu.Lock()
	counts := n.counts
	n.counts = map[string]int{}
	n.timer = nil
	n.lastSend = time.Now()
	n.mu.Unlock()

	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
	}
	summary := strings.Join(parts, ", ")

	msg := strings.Join([]string{
		"From: ClimateUmbral <" + n.user + ">",
		"To: " + n.to,
		"Subject: ClimateUmbral: " + summary,
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"New feedback on climateumbral.eu: " + summary + ".",
		"",
		"Read it: https://climateumbral.eu/admin",
		"",
	}, "\r\n")

	auth := smtp.PlainAuth("", n.user, n.pass, n.host)
	err := smtp.SendMail(n.addr, auth, n.user, []string{n.to},
		[]byte(msg))
	if err == nil {
		return
	}
	// failed sends put their counts back and retry next interval —
	// feedback is too rare to drop on a mail hiccup
	log.Printf("notify: send failed (will retry): %v", err)
	n.mu.Lock()
	for k, v := range counts {
		n.counts[k] += v
	}
	if n.timer == nil {
		n.timer = time.AfterFunc(notifyInterval, n.send)
	}
	n.mu.Unlock()
}
