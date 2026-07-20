// HTTP handlers: the game's API surface. Rule of the house: no
// network I/O while holding s.mu — upstream fetches happen first,
// the lock only guards ledger reads and writes.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ---- views: what GET endpoints expose (never tokens) ----

type claimView struct {
	Pe       int        `json:"pe"`
	Pn       int        `json:"pn"`
	Kind     string     `json:"kind"`
	V        int        `json:"v"`
	Name     string     `json:"name,omitempty"`
	TS       time.Time  `json:"ts"`
	Deadline time.Time  `json:"deadline"`
	Status   string     `json:"status"`
	Flipped  *time.Time `json:"flipped,omitempty"`
	Photo    string     `json:"photo,omitempty"`
}

type joinView struct {
	Be   int       `json:"be"`
	Bn   int       `json:"bn"`
	Name string    `json:"name,omitempty"`
	TS   time.Time `json:"ts"`
}

func viewOf(c *claim, now time.Time) claimView {
	return claimView{
		Pe: c.Pe, Pn: c.Pn, Kind: c.Kind, V: c.V, Name: c.Name,
		TS: c.TS, Deadline: c.Deadline, Status: c.status(now),
		Flipped: c.Flipped, Photo: c.Photo,
	}
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func pathPePn(r *http.Request) (int, int, error) {
	pe, errE := strconv.Atoi(r.PathValue("pe"))
	pn, errN := strconv.Atoi(r.PathValue("pn"))
	if errE != nil || errN != nil {
		return 0, 0, errors.New("bad pixel coordinates in path")
	}
	return pe, pn, nil
}

// validPhotoURL: parsed, absolute, http(s) with a host — never a
// javascript:/data: payload replayed to every open map.
func validPhotoURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Host != "" &&
		(u.Scheme == "http" || u.Scheme == "https")
}

// parseBbox reads "a,b,c,d" as finite floats within sane magnitude.
func parseBbox(raw string) ([4]float64, error) {
	var b [4]float64
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return b, errors.New("need bbox=a,b,c,d")
	}
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) ||
			math.Abs(v) > 1e8 {
			return b, errors.New("bad bbox number")
		}
		b[i] = v
	}
	return b, nil
}

// ---- raster ----

// handleRaster proxies a viewport of raw sealed-% values in native
// EPSG:3035, 10 m per pixel, bbox snapped to the pixel grid — so a
// client raster index IS a continent pixel: pe = pe0+col,
// pn = pn0+(h-1-row) (row 0 = north). This exactness is what makes
// client-side candidates agree with server-side validation.
// GET /api/raster?bbox=e0,n0,e1,n1  ->  w*h U8 bytes + X-Raster-*
// headers (which carry the snapped bbox actually served — snapping
// can also clamp an oversized request down to maxRaster).
func (s *server) handleRaster(w http.ResponseWriter, r *http.Request) {
	b, err := parseBbox(r.URL.Query().Get("bbox"))
	if err != nil {
		writeErr(w, http.StatusBadRequest,
			err.Error()+" (bbox=e0,n0,e1,n1 in 3035)")
		return
	}
	e0 := int(math.Floor(b[0]/10)) * 10
	n0 := int(math.Floor(b[1]/10)) * 10
	e1 := int(math.Ceil(b[2]/10)) * 10
	n1 := int(math.Ceil(b[3]/10)) * 10
	wd, ht := (e1-e0)/10, (n1-n0)/10
	// snapping widens by up to 2 px; a client at exactly the cap
	// must not 400 for it, so clamp instead of rejecting
	if wd > maxRaster {
		wd, e1 = maxRaster, e0+maxRaster*10
	}
	if ht > maxRaster {
		ht, n1 = maxRaster, n0+maxRaster*10
	}
	if wd < 1 || ht < 1 {
		writeErr(w, http.StatusBadRequest, "empty bbox")
		return
	}
	bbox := fmt.Sprintf("%d,%d,%d,%d", e0, n0, e1, n1)
	img, err := s.eea.values("3035", bbox, wd, ht)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Raster-Bbox", bbox)
	w.Header().Set("X-Raster-Size", fmt.Sprintf("%d,%d", wd, ht))
	w.Header().Set("Access-Control-Expose-Headers",
		"X-Raster-Bbox, X-Raster-Size")
	w.Write(img)
}

// ---- ledger ----

func (s *server) handleGetLedger(w http.ResponseWriter,
	_ *http.Request) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	claims := make([]claimView, 0, len(s.ledger.Claims))
	pledged, flipped := 0, 0
	cooling := 0.0
	for i := range s.ledger.Claims {
		v := viewOf(&s.ledger.Claims[i], now)
		claims = append(claims, v)
		switch v.Status {
		case statusPledged:
			pledged += claimM2
		case statusFlipped:
			flipped += claimM2
			cooling += nightCooling(&s.ledger.Claims[i])
		}
	}
	joins := make([]joinView, 0, len(s.ledger.Joins))
	for _, j := range s.ledger.Joins {
		joins = append(joins,
			joinView{Be: j.Be, Bn: j.Bn, Name: j.Name, TS: j.TS})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"claims":      claims,
		"joins":       joins,
		"pledged_m2":  pledged,
		"flipped_m2":  flipped,
		"night_mdegc": cooling,
	})
}

func (s *server) handlePledge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pe   int    `json:"pe"`
		Pn   int    `json:"pn"`
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad JSON body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if len(name) > maxNameLen {
		writeErr(w, http.StatusBadRequest, "name too long")
		return
	}
	if req.Kind == "" {
		req.Kind = "depave"
	}
	if !actKinds[req.Kind] {
		writeErr(w, http.StatusBadRequest, "unknown act kind")
		return
	}
	if !inEurope(req.Pe, req.Pn) {
		writeErr(w, http.StatusConflict, "outside the European grid")
		return
	}
	// upstream first, lock second: this fetch can take seconds and
	// must never stall every other request behind s.mu
	nb, err := s.eea.neighborhood(req.Pe, req.Pn)
	if err != nil {
		writeErr(w, http.StatusBadGateway,
			"upstream data unavailable: "+err.Error())
		return
	}
	if len(nb) != 9 {
		writeErr(w, http.StatusBadGateway, "upstream data malformed")
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ledger.Claims) >= maxClaims {
		writeErr(w, http.StatusServiceUnavailable,
			"claim ledger is full")
		return
	}
	if err := s.pledgeable(req.Pe, req.Pn, req.Kind, nb, now); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	c := claim{
		Pe: req.Pe, Pn: req.Pn, Kind: req.Kind, V: int(nb[4]),
		Name: name, TS: now, Deadline: now.Add(s.expiry),
		Token: newToken(),
	}
	s.ledger.Claims = append(s.ledger.Claims, c)
	if err := s.persist(); err != nil {
		s.ledger.Claims = s.ledger.Claims[:len(s.ledger.Claims)-1]
		writeErr(w, http.StatusServiceUnavailable,
			"ledger storage failed; act not recorded")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"claim": viewOf(&c, now),
		"token": c.Token,
	})
}

func (s *server) handleFlip(w http.ResponseWriter, r *http.Request) {
	pe, pn, err := pathPePn(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		Token string `json:"token"`
		Photo string `json:"photo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad JSON body")
		return
	}
	photo := strings.TrimSpace(req.Photo)
	if photo != "" &&
		(len(photo) > maxPhotoLen || !validPhotoURL(photo)) {
		writeErr(w, http.StatusBadRequest,
			"photo must be an http(s) URL")
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.ledger.activeAt(pe, pn, now)
	if c == nil {
		writeErr(w, http.StatusNotFound, "no live pledge on this pixel")
		return
	}
	if c.status(now) == statusFlipped {
		writeErr(w, http.StatusConflict, "already flipped")
		return
	}
	if !tokenMatch(req.Token, c.Token) {
		writeErr(w, http.StatusForbidden, "wrong or missing token")
		return
	}
	prevFlipped, prevPhoto := c.Flipped, c.Photo
	c.Flipped = &now
	c.Photo = photo
	if err := s.persist(); err != nil {
		c.Flipped, c.Photo = prevFlipped, prevPhoto
		writeErr(w, http.StatusServiceUnavailable,
			"ledger storage failed; flip not recorded")
		return
	}
	writeJSON(w, http.StatusOK, viewOf(c, now))
}

// handleAbandon erases a claim entirely — both "abandon my pledge"
// and the GDPR right to erasure are this one act.
func (s *server) handleAbandon(w http.ResponseWriter, r *http.Request) {
	pe, pn, err := pathPePn(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	token := r.Header.Get(tokenHeader)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ledger.Claims {
		c := s.ledger.Claims[i] // copy: survives the delete below
		if c.Pe != pe || c.Pn != pn ||
			(!tokenMatch(token, c.Token) && !s.isAdmin(token)) {
			continue
		}
		s.ledger.Claims = slices.Delete(s.ledger.Claims, i, i+1)
		if err := s.persist(); err != nil {
			s.ledger.Claims = slices.Insert(s.ledger.Claims, i, c)
			writeErr(w, http.StatusServiceUnavailable,
				"ledger storage failed; nothing erased")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeErr(w, http.StatusForbidden, "wrong or missing token")
}

// handleJoin signs the standing petition for a 150 m block. Joining
// is non-exclusive and has no deadline: it is a signature, not a
// promise of labor. But a petition sheet is finite: one name signs a
// block once, and a block holds maxJoinsPerBlock signatures.
func (s *server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Be   int    `json:"be"`
		Bn   int    `json:"bn"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad JSON body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if len(name) > maxNameLen {
		writeErr(w, http.StatusBadRequest, "name too long")
		return
	}
	if !inEurope(req.Be*blockPx, req.Bn*blockPx) {
		writeErr(w, http.StatusConflict, "outside the European grid")
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ledger.Joins) >= maxJoins {
		writeErr(w, http.StatusServiceUnavailable,
			"join ledger is full")
		return
	}
	inBlock := 0
	for i := range s.ledger.Joins {
		j := &s.ledger.Joins[i]
		if j.Be != req.Be || j.Bn != req.Bn {
			continue
		}
		inBlock++
		if name != "" && j.Name == name {
			writeErr(w, http.StatusConflict,
				"this name already signed this block")
			return
		}
	}
	if inBlock >= maxJoinsPerBlock {
		writeErr(w, http.StatusConflict,
			"this block's petition sheet is full")
		return
	}
	j := join{Be: req.Be, Bn: req.Bn, Name: name, TS: now,
		Token: newToken()}
	s.ledger.Joins = append(s.ledger.Joins, j)
	if err := s.persist(); err != nil {
		s.ledger.Joins = s.ledger.Joins[:len(s.ledger.Joins)-1]
		writeErr(w, http.StatusServiceUnavailable,
			"ledger storage failed; signature not recorded")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"join":  joinView{Be: j.Be, Bn: j.Bn, Name: j.Name, TS: j.TS},
		"token": j.Token,
	})
}

// handleLeave erases a signature — GDPR erasure included.
func (s *server) handleLeave(w http.ResponseWriter, r *http.Request) {
	be, errB := strconv.Atoi(r.PathValue("be"))
	bn, errN := strconv.Atoi(r.PathValue("bn"))
	if errB != nil || errN != nil {
		writeErr(w, http.StatusBadRequest, "bad block coordinates")
		return
	}
	token := r.Header.Get(tokenHeader)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ledger.Joins {
		j := s.ledger.Joins[i] // copy: survives the delete below
		if j.Be != be || j.Bn != bn ||
			(!tokenMatch(token, j.Token) && !s.isAdmin(token)) {
			continue
		}
		s.ledger.Joins = slices.Delete(s.ledger.Joins, i, i+1)
		if err := s.persist(); err != nil {
			s.ledger.Joins = slices.Insert(s.ledger.Joins, i, j)
			writeErr(w, http.StatusServiceUnavailable,
				"ledger storage failed; nothing erased")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeErr(w, http.StatusForbidden, "wrong or missing token")
}

func (s *server) handleLeaderboard(w http.ResponseWriter,
	_ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK,
		s.ledger.leaderboard(time.Now().UTC(), 20))
}

// spaHandler serves dist with an index.html fallback for client-side
// routes.
func spaHandler(dist string) http.Handler {
	fs := http.FileServer(http.Dir(dist))
	return http.HandlerFunc(func(w http.ResponseWriter,
		r *http.Request) {
		p := filepath.Join(dist, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(p); err != nil || info.IsDir() {
			r.URL.Path = "/"
		}
		fs.ServeHTTP(w, r)
	})
}
