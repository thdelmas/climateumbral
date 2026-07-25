// The session gate, table-tested: a token is accepted only if this
// server could have minted it. Every forgery here is one a real
// attacker can produce for free — wrong key, wrong alg, wrong sub,
// expired, tampered, malformed — so every row must stay red.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// forge builds a token with an arbitrary header and claims, signed
// with the given key — the shapes jwtSign refuses to produce.
func forge(key []byte, header string, c jwtClaims) string {
	body, _ := json.Marshal(c)
	signing := b64.EncodeToString([]byte(header)) + "." +
		b64.EncodeToString(body)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signing))
	return signing + "." + b64.EncodeToString(mac.Sum(nil))
}

func TestJWTVerify(t *testing.T) {
	key := jwtKey("test-admin-token")
	now := time.Unix(1_753_400_000, 0)
	good := jwtSign(key, now)
	claims := func(sub string, iat time.Time) jwtClaims {
		return jwtClaims{
			Sub: sub,
			Iat: iat.Unix(),
			Exp: iat.Add(jwtTTL).Unix(),
		}
	}
	hs256 := `{"alg":"HS256","typ":"JWT"}`

	cases := []struct {
		name  string
		token string
		at    time.Time
		want  bool
	}{
		{"fresh token verifies", good, now, true},
		{"still valid one second before expiry",
			good, now.Add(jwtTTL - time.Second), true},
		{"dead exactly at expiry", good, now.Add(jwtTTL), false},
		{"dead after expiry", good, now.Add(jwtTTL + time.Hour), false},
		{"wrong key",
			jwtSign(jwtKey("other-token"), now), now, false},
		{"tampered signature",
			good[:len(good)-2] + "xx", now, false},
		{"tampered payload keeps old signature", func() string {
			p := strings.Split(good, ".")
			body, _ := json.Marshal(claims("admin",
				now.Add(-2*jwtTTL).Add(3*jwtTTL)))
			return p[0] + "." + b64.EncodeToString(body) +
				"." + p[2]
		}(), now, false},
		{"alg none refused even with valid mac",
			forge(key, `{"alg":"none","typ":"JWT"}`,
				claims("admin", now)), now, false},
		{"wrong sub refused",
			forge(key, hs256, claims("user", now)), now, false},
		{"garbage payload refused", func() string {
			h := b64.EncodeToString([]byte(hs256))
			signing := h + "." + b64.EncodeToString(
				[]byte("not json"))
			mac := hmac.New(sha256.New, key)
			mac.Write([]byte(signing))
			return signing + "." +
				b64.EncodeToString(mac.Sum(nil))
		}(), now, false},
		{"two parts", "aaaa.bbbb", now, false},
		{"four parts", good + ".extra", now, false},
		{"empty", "", now, false},
		{"invalid base64 signature", func() string {
			p := strings.Split(good, ".")
			return p[0] + "." + p[1] + ".!!!"
		}(), now, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := jwtVerify(key, c.token, c.at); got != c.want {
				t.Errorf("jwtVerify = %v, want %v", got, c.want)
			}
		})
	}
}

// The two token kinds share a key but must never cross: a mailed
// link is not a session, a stolen session mints no links.
func TestLoginTokens(t *testing.T) {
	key := jwtKey("test-admin-token")
	now := time.Unix(1_753_400_000, 0)
	tok := jwtSignLogin(key, "abc123", now)

	jti, ok := jwtVerifyLogin(key, tok, now)
	if !ok || jti != "abc123" {
		t.Fatalf("fresh link: got (%q, %v), want (abc123, true)", jti, ok)
	}
	if _, ok := jwtVerifyLogin(key, tok, now.Add(loginTTL)); ok {
		t.Fatal("link must die at loginTTL")
	}
	if jwtVerify(key, tok, now) {
		t.Fatal("a login link must not pass as a session")
	}
	if _, ok := jwtVerifyLogin(key, jwtSign(key, now), now); ok {
		t.Fatal("a session must not pass as a login link")
	}
	noJti := forge(key, `{"alg":"HS256","typ":"JWT"}`, jwtClaims{
		Sub: "login", Iat: now.Unix(),
		Exp: now.Add(loginTTL).Unix(),
	})
	if _, ok := jwtVerifyLogin(key, noJti, now); ok {
		t.Fatal("a login link without a jti has nothing to burn")
	}
}

func TestJWTKeyDerivation(t *testing.T) {
	// rotate the admin token -> every session key changes
	if string(jwtKey("a")) == string(jwtKey("b")) {
		t.Fatal("different admin tokens derived the same key")
	}
	if string(jwtKey("a")) != string(jwtKey("a")) {
		t.Fatal("key derivation is not deterministic")
	}
}
