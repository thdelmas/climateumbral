// Minimal HS256 JWT — sign and verify, nothing else, no deps.
// Two token kinds, one key: 12h "admin" sessions for the back
// office, and 15-minute single-use "login" links that travel by
// mail and exchange for a session. The signing key is derived from
// the admin token itself: rotate the token and every outstanding
// session and unclicked link dies with it — one secret to manage,
// the correct blast radius.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

const (
	jwtTTL = 12 * time.Hour
	// a sign-in link lives long enough to survive a slow mail hop,
	// short enough that a leaked inbox is a 15-minute problem
	loginTTL = 15 * time.Minute
)

var b64 = base64.RawURLEncoding

func jwtKey(adminToken string) []byte {
	// domain-separated derivation; bump the tag to invalidate all
	// sessions without rotating the admin token
	sum := sha256.Sum256([]byte("climateumbral-jwt-v1|" + adminToken))
	return sum[:]
}

type jwtClaims struct {
	Sub string `json:"sub"`
	Jti string `json:"jti,omitempty"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

func jwtMint(key []byte, c jwtClaims) string {
	head := b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, _ := json.Marshal(c)
	signing := head + "." + b64.EncodeToString(body)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signing))
	return signing + "." + b64.EncodeToString(mac.Sum(nil))
}

// jwtParse accepts only tokens this server could have minted: exact
// three-part shape, HS256, unexpired. The signature check runs
// before the payload is even parsed. Sub is the caller's problem.
func jwtParse(key []byte, token string, now time.Time) (jwtClaims, bool) {
	var c jwtClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	want := mac.Sum(nil)
	got, err := b64.DecodeString(parts[2])
	if err != nil || !hmac.Equal(got, want) {
		return c, false
	}
	head, err := b64.DecodeString(parts[0])
	if err != nil ||
		!strings.Contains(string(head), `"alg":"HS256"`) {
		return c, false
	}
	raw, err := b64.DecodeString(parts[1])
	if err != nil {
		return c, false
	}
	if json.Unmarshal(raw, &c) != nil {
		return c, false
	}
	return c, now.Unix() < c.Exp
}

func jwtSign(key []byte, now time.Time) string {
	return jwtMint(key, jwtClaims{
		Sub: "admin",
		Iat: now.Unix(),
		Exp: now.Add(jwtTTL).Unix(),
	})
}

func jwtVerify(key []byte, token string, now time.Time) bool {
	c, ok := jwtParse(key, token, now)
	return ok && c.Sub == "admin"
}

// Sign-in links carry their own sub so a mailed link can never pass
// as a session, and a stolen session can never mint fresh links.
// The jti is the single-use handle the server burns on exchange.
func jwtSignLogin(key []byte, jti string, now time.Time) string {
	return jwtMint(key, jwtClaims{
		Sub: "login",
		Jti: jti,
		Iat: now.Unix(),
		Exp: now.Add(loginTTL).Unix(),
	})
}

func jwtVerifyLogin(key []byte, token string, now time.Time) (string, bool) {
	c, ok := jwtParse(key, token, now)
	if !ok || c.Sub != "login" || c.Jti == "" {
		return "", false
	}
	return c.Jti, true
}
