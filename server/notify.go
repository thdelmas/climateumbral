// Feedback lands in files on a box nobody rereads daily; mail is
// the only channel that reaches the maintainer without polling.
// The notifier turns feedback into digests — counts only, never
// content: suggestion text stays behind the admin token, so a
// third-party mail hop never carries what someone pasted about
// themselves. Outbound goes through authenticated submission
// (587): port 25 is blocked on the hosting account.
package main

import (
	"errors"
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
	addr, host, user, pass, from, to string

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
	// relay logins are not addresses (Resend's user is the literal
	// string "resend"); the envelope sender gets its own knob
	from := os.Getenv("CLIMATEUMBRAL_MAIL_FROM")
	if from == "" {
		from = user
	}
	log.Printf("notify: on, digests to %s via %s:%s", to, host, port)
	return &notifier{
		addr: host + ":" + port, host: host,
		user: user, pass: pass, from: from, to: to,
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

	err := n.mail(n.to, "ClimateUmbral: "+summary,
		"New feedback on climateumbral.eu: "+summary+".\n\n"+
			"Read it: https://climateumbral.eu/admin\n")
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

// mail sends one plain-text message through the configured relay;
// digests and back-office sign-in links both go through here. The
// nil receiver means mail is off and says so as an error.
func (n *notifier) mail(to, subject, body string) error {
	if n == nil {
		return errors.New("mail off (SMTP not configured)")
	}
	msg := strings.Join([]string{
		"From: ClimateUmbral <" + n.from + ">",
		"To: " + to,
		"Subject: " + subject,
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		strings.ReplaceAll(body, "\n", "\r\n"),
	}, "\r\n")
	auth := smtp.PlainAuth("", n.user, n.pass, n.host)
	return smtp.SendMail(n.addr, auth, n.from, []string{to},
		[]byte(msg))
}
