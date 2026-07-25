// Minimal HS256 JWT — sign and verify, nothing else, no deps.
// Sessions for the back office: the admin token is exchanged once
// at login for a short-lived signed session, so the long-lived
// credential stops traveling with every request. The signing key
// is derived from the admin token itself: rotate the token and
// every outstanding session dies with it — one secret to manage,
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

const jwtTTL = 12 * time.Hour

var b64 = base64.RawURLEncoding

func jwtKey(adminToken string) []byte {
	// domain-separated derivation; bump the tag to invalidate all
	// sessions without rotating the admin token
	sum := sha256.Sum256([]byte("climateumbral-jwt-v1|" + adminToken))
	return sum[:]
}

type jwtClaims struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

func jwtSign(key []byte, now time.Time) string {
	head := b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, _ := json.Marshal(jwtClaims{
		Sub: "admin",
		Iat: now.Unix(),
		Exp: now.Add(jwtTTL).Unix(),
	})
	signing := head + "." + b64.EncodeToString(body)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signing))
	return signing + "." + b64.EncodeToString(mac.Sum(nil))
}

// jwtVerify accepts only tokens this server could have minted:
// exact three-part shape, HS256, our sub, unexpired. The signature
// check runs before the payload is even parsed.
func jwtVerify(key []byte, token string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	want := mac.Sum(nil)
	got, err := b64.DecodeString(parts[2])
	if err != nil || !hmac.Equal(got, want) {
		return false
	}
	head, err := b64.DecodeString(parts[0])
	if err != nil ||
		!strings.Contains(string(head), `"alg":"HS256"`) {
		return false
	}
	raw, err := b64.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var c jwtClaims
	if json.Unmarshal(raw, &c) != nil {
		return false
	}
	return c.Sub == "admin" && now.Unix() < c.Exp
}
