// The back office. Auth is standard email-link sign-in: POST an
// address, and if the operator listed it (CLIMATEUMBRAL_ADMIN_EMAILS)
// a single-use 15-minute link travels by mail and exchanges for a
// 12h HS256 session in an httpOnly Secure SameSite=Lax cookie
// (Lax, not Strict: arriving from a mail client is a cross-site
// navigation, and Strict would eat the fresh session on landing;
// mutations stay safe because Lax withholds the cookie from
// everything but top-level GETs, and those never mutate). Nothing
// to type means nothing to phish, and the response never says
// whether an address was listed. The admin token no longer travels
// through a browser at all — it stays the signing seed and the curl
// header (X-ClimateUmbral-Token) for scripted moderation; rotate it
// and every session, link and script credential dies together.
// Login and exchange share the act rate-limiter, so guessing costs
// a visitor the same budget as spamming pledges. With no admin
// token configured the page doesn't exist.
package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookie = "cu_admin"
	// one sign-in mail per address per interval: a stranger poking
	// the form can't fill the operator's inbox
	loginMailCooldown = time.Minute
)

//go:embed admin_login.html
var adminLoginHTML string

//go:embed admin.html
var adminHTML string

// adminReq accepts either credential: the X-ClimateUmbral-Token
// header (curl, scripts) or a valid session cookie (the browser,
// after following a mailed link).
func (s *server) adminReq(r *http.Request) bool {
	if s.isAdmin(r.Header.Get(tokenHeader)) {
		return true
	}
	if s.adminToken == "" {
		return false
	}
	c, err := r.Cookie(sessionCookie)
	return err == nil &&
		jwtVerify(jwtKey(s.adminToken), c.Value, time.Now())
}

// listedEmail reports whether addr is on the operator allowlist.
func (s *server) listedEmail(addr string) bool {
	for _, e := range s.adminEmails {
		if strings.EqualFold(e, addr) {
			return true
		}
	}
	return false
}

// consumeLogin burns a link's jti: true exactly once. The set lives
// in memory, so a restart forgets burned links and would accept a
// replay for the rest of the 15 minutes — with one operator that
// window buys not carrying a database.
func (s *server) consumeLogin(jti string, now time.Time) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if s.loginUsed == nil {
		s.loginUsed = map[string]time.Time{}
	}
	for j, exp := range s.loginUsed { // a handful of entries at most
		if now.After(exp) {
			delete(s.loginUsed, j)
		}
	}
	if _, dup := s.loginUsed[jti]; dup {
		return false
	}
	s.loginUsed[jti] = now.Add(loginTTL)
	return true
}

// mayMail enforces the per-address cooldown; asking again inside it
// silently sends nothing.
func (s *server) mayMail(email string, now time.Time) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if s.loginLast == nil {
		s.loginLast = map[string]time.Time{}
	}
	if last, ok := s.loginLast[email]; ok &&
		now.Sub(last) < loginMailCooldown {
		return false
	}
	s.loginLast[email] = now
	return true
}

// handleAdminLogin takes an address and answers 202 no matter what:
// whether it was listed, cooling down, or unknown is not for the
// asker to learn. The mail goes out on its own goroutine so the
// response time says nothing either.
func (s *server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if s.adminToken == "" {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).
		Decode(&body) != nil {
		writeErr(w, http.StatusBadRequest, "bad JSON body")
		return
	}
	email := strings.TrimSpace(body.Email)
	if email == "" || len(email) > 254 ||
		!strings.Contains(email, "@") {
		writeErr(w, http.StatusBadRequest, "not an email address")
		return
	}
	now := time.Now()
	if s.listedEmail(email) && s.mailer != nil &&
		s.mayMail(strings.ToLower(email), now) {
		var jti [16]byte
		if _, err := rand.Read(jti[:]); err != nil {
			writeErr(w, http.StatusInternalServerError,
				"entropy unavailable")
			return
		}
		link := s.baseURL + "/admin/session?t=" +
			jwtSignLogin(jwtKey(s.adminToken),
				hex.EncodeToString(jti[:]), now)
		go func() {
			err := s.mailer(email,
				"ClimateUmbral back office: your sign-in link",
				"Sign in to the back office — the link works once "+
					"and dies in 15 minutes:\n\n"+link+"\n\n"+
					"Not you? Ignore this mail; nothing happens "+
					"without the link.\n")
			if err != nil {
				log.Printf("admin sign-in mail: %v", err)
			}
		}()
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleAdminSession exchanges a mailed link for a session cookie.
// Burned or expired links bounce back to the sign-in page instead
// of a dead end — the operator just asks for a fresh one.
func (s *server) handleAdminSession(w http.ResponseWriter, r *http.Request) {
	if s.adminToken == "" {
		http.NotFound(w, r)
		return
	}
	now := time.Now()
	jti, ok := jwtVerifyLogin(jwtKey(s.adminToken),
		r.URL.Query().Get("t"), now)
	if !ok || !s.consumeLogin(jti, now) {
		http.Redirect(w, r, "/admin?expired=1", http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: jwtSign(jwtKey(s.adminToken), now),
		Path: "/", MaxAge: int(jwtTTL.Seconds()),
		HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *server) handleAdminLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if s.adminToken == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.Header().Set("Cache-Control", "no-store")
	if !s.adminReq(r) {
		_, _ = w.Write([]byte(adminLoginHTML))
		return
	}
	_, _ = w.Write([]byte(adminHTML))
}
