// Tilewhip API, V3: the whole of Europe is the board.
//
// There is no local grid anymore. The visual map streams straight
// from the EEA image service; this server owns the game: viewport
// value rasters (proxied + cached), and the claims ledger keyed to
// the continent-wide EPSG:3035 10 m pixel grid — pixel (pe, pn) =
// floor(easting/10), floor(northing/10).
//
// The cascade rule is enforced against live upstream data: a pixel is
// pledgeable only if it is hard-sealed (>=90%) and touches >=3
// green-or-actively-claimed neighbours; live pledges and flips count
// as green. Expired pledges release their pixel.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hardSealed  = 90 // >= this % imperviousness is claimable
	greenMax    = 10 // <= this % imperviousness counts as green
	minGreens   = 3  // neighbours needed to be a candidate
	maxNameLen  = 40
	maxPhotoLen = 500
	maxRaster   = 512 // max viewport raster dimension
	tokenHeader = "X-Tilewhip-Token"
)

type server struct {
	eea     *eeaClient
	hub     *hub
	limiter *limiter

	mu         sync.Mutex
	ledger     *ledger
	ledgerPath string
	expiry     time.Duration
}

// pledgeable reports whether continent pixel (pe, pn) can be pledged.
// The 3x3 neighbourhood comes live from the EEA service; callers hold
// s.mu.
func (s *server) pledgeable(pe, pn int, now time.Time) error {
	if !inEurope(pe, pn) {
		return errors.New("outside the European grid")
	}
	if c := s.ledger.activeAt(pe, pn, now); c != nil {
		return errors.New("already " + c.status(now))
	}
	nb, err := s.eea.neighborhood(pe, pn)
	if err != nil {
		return fmt.Errorf("upstream data unavailable: %w", err)
	}
	if v := nb[4]; v < hardSealed || v > 100 {
		return errors.New("not hard-sealed (needs >=90% imperviousness)")
	}
	active := s.ledger.activeSet(now)
	greens := 0
	for dy := -1; dy <= 1; dy++ { // dy = +1 is north
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			v := nb[(1-dy)*3+(1+dx)] // row 0 = north
			if v <= greenMax || active[[2]int{pe + dx, pn + dy}] {
				greens++
			}
		}
	}
	if greens < minGreens {
		return errors.New(
			"not a candidate: needs >=3 green or claimed neighbours")
	}
	return nil
}

func (s *server) persist() {
	if err := s.ledger.persist(s.ledgerPath); err != nil {
		log.Printf("persist ledger: %v", err)
	}
	s.hub.notify()
}

// ---- views: what GET endpoints expose (never tokens) ----

type claimView struct {
	Pe       int        `json:"pe"`
	Pn       int        `json:"pn"`
	Name     string     `json:"name,omitempty"`
	TS       time.Time  `json:"ts"`
	Deadline time.Time  `json:"deadline"`
	Status   string     `json:"status"`
	Flipped  *time.Time `json:"flipped,omitempty"`
	Photo    string     `json:"photo,omitempty"`
}

type watchView struct {
	Pe   int       `json:"pe"`
	Pn   int       `json:"pn"`
	Name string    `json:"name,omitempty"`
	TS   time.Time `json:"ts"`
}

func viewOf(c *claim, now time.Time) claimView {
	return claimView{
		Pe: c.Pe, Pn: c.Pn, Name: c.Name, TS: c.TS,
		Deadline: c.Deadline, Status: c.status(now),
		Flipped: c.Flipped, Photo: c.Photo,
	}
}

// ---- handlers ----

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

// handleRaster proxies a viewport of raw sealed-% values in native
// EPSG:3035, 10 m per pixel, bbox snapped to the pixel grid — so a
// client raster index IS a continent pixel: pe = pe0+col,
// pn = pn0+(h-1-row) (row 0 = north). This exactness is what makes
// client-side candidates agree with server-side validation.
// GET /api/raster?bbox=e0,n0,e1,n1  ->  w*h U8 bytes + X-Raster-*
// headers. Cached upstream-side by the eeaClient.
func (s *server) handleRaster(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Query().Get("bbox"), ",")
	if len(parts) != 4 {
		writeErr(w, http.StatusBadRequest, "need bbox=e0,n0,e1,n1 (3035)")
		return
	}
	var b [4]float64
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "bad bbox number")
			return
		}
		b[i] = v
	}
	e0 := int(math.Floor(b[0]/10)) * 10
	n0 := int(math.Floor(b[1]/10)) * 10
	e1 := int(math.Ceil(b[2]/10)) * 10
	n1 := int(math.Ceil(b[3]/10)) * 10
	wd, ht := (e1-e0)/10, (n1-n0)/10
	if wd < 1 || ht < 1 || wd > maxRaster || ht > maxRaster {
		writeErr(w, http.StatusBadRequest,
			fmt.Sprintf("snapped size %dx%d not in 1..%d",
				wd, ht, maxRaster))
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

func (s *server) handleGetLedger(w http.ResponseWriter, _ *http.Request) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	claims := make([]claimView, 0, len(s.ledger.Claims))
	pledged, flipped := 0, 0
	for i := range s.ledger.Claims {
		v := viewOf(&s.ledger.Claims[i], now)
		claims = append(claims, v)
		switch v.Status {
		case statusPledged:
			pledged += claimM2
		case statusFlipped:
			flipped += claimM2
		}
	}
	watches := make([]watchView, 0, len(s.ledger.Watches))
	for _, wa := range s.ledger.Watches {
		watches = append(watches,
			watchView{Pe: wa.Pe, Pn: wa.Pn, Name: wa.Name, TS: wa.TS})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"claims":     claims,
		"watches":    watches,
		"pledged_m2": pledged,
		"flipped_m2": flipped,
	})
}

func (s *server) handlePledge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pe   int    `json:"pe"`
		Pn   int    `json:"pn"`
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
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.pledgeable(req.Pe, req.Pn, now); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	c := claim{
		Pe: req.Pe, Pn: req.Pn, Name: name, TS: now,
		Deadline: now.Add(s.expiry), Token: newToken(),
	}
	s.ledger.Claims = append(s.ledger.Claims, c)
	s.persist()
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
	if len(photo) > maxPhotoLen ||
		(photo != "" && !strings.HasPrefix(photo, "http")) {
		writeErr(w, http.StatusBadRequest, "photo must be an http(s) URL")
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
	if req.Token == "" || req.Token != c.Token {
		writeErr(w, http.StatusForbidden, "wrong or missing token")
		return
	}
	c.Flipped = &now
	c.Photo = photo
	s.persist()
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
		c := &s.ledger.Claims[i]
		if c.Pe == pe && c.Pn == pn && token != "" && c.Token == token {
			s.ledger.Claims = append(
				s.ledger.Claims[:i], s.ledger.Claims[i+1:]...)
			s.persist()
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeErr(w, http.StatusForbidden, "wrong or missing token")
}

func (s *server) handleWatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pe   int    `json:"pe"`
		Pn   int    `json:"pn"`
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
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !inEurope(req.Pe, req.Pn) {
		writeErr(w, http.StatusConflict, "outside the European grid")
		return
	}
	nb, err := s.eea.neighborhood(req.Pe, req.Pn)
	if err != nil {
		writeErr(w, http.StatusBadGateway,
			"upstream data unavailable: "+err.Error())
		return
	}
	if v := nb[4]; v < hardSealed || v > 100 {
		writeErr(w, http.StatusConflict,
			"only sealed pixels need watching")
		return
	}
	if c := s.ledger.activeAt(req.Pe, req.Pn, now); c != nil &&
		c.status(now) == statusFlipped {
		writeErr(w, http.StatusConflict, "already flipped")
		return
	}
	wa := watch{Pe: req.Pe, Pn: req.Pn, Name: name, TS: now,
		Token: newToken()}
	s.ledger.Watches = append(s.ledger.Watches, wa)
	s.persist()
	writeJSON(w, http.StatusCreated, map[string]any{
		"watch": watchView{Pe: wa.Pe, Pn: wa.Pn, Name: wa.Name,
			TS: wa.TS},
		"token": wa.Token,
	})
}

func (s *server) handleUnwatch(w http.ResponseWriter, r *http.Request) {
	pe, pn, err := pathPePn(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	token := r.Header.Get(tokenHeader)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ledger.Watches {
		wa := &s.ledger.Watches[i]
		if wa.Pe == pe && wa.Pn == pn && token != "" &&
			wa.Token == token {
			s.ledger.Watches = append(
				s.ledger.Watches[:i], s.ledger.Watches[i+1:]...)
			s.persist()
			w.WriteHeader(http.StatusNoContent)
			return
		}
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dist, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(p); err != nil || info.IsDir() {
			r.URL.Path = "/"
		}
		fs.ServeHTTP(w, r)
	})
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dataDir := flag.String("data", "./data",
		"directory holding claims.json")
	dist := flag.String("dist", "",
		"built frontend to serve at / (empty = API only)")
	expiryDays := flag.Int("expiry-days", 90,
		"days before an unflipped pledge returns to the pool")
	flag.Parse()

	expiry := time.Duration(*expiryDays) * 24 * time.Hour
	s := &server{
		eea:        newEEA(),
		hub:        newHub(),
		limiter:    newLimiter(0.2, 5), // ~12 acts/min after a burst of 5
		ledgerPath: filepath.Join(*dataDir, "claims.json"),
		expiry:     expiry,
	}
	var err error
	s.ledger, err = loadLedger(s.ledgerPath, expiry)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("europe is the board: %d claims / %d watches",
		len(s.ledger.Claims), len(s.ledger.Watches))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/raster", s.handleRaster)
	mux.HandleFunc("GET /api/claims", s.handleGetLedger)
	mux.HandleFunc("POST /api/claims", s.limit(s.handlePledge))
	mux.HandleFunc("POST /api/claims/{pe}/{pn}/flip", s.limit(s.handleFlip))
	mux.HandleFunc("DELETE /api/claims/{pe}/{pn}", s.limit(s.handleAbandon))
	mux.HandleFunc("POST /api/watches", s.limit(s.handleWatch))
	mux.HandleFunc("DELETE /api/watches/{pe}/{pn}",
		s.limit(s.handleUnwatch))
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/leaderboard", s.handleLeaderboard)
	health := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	mux.HandleFunc("GET /api/health", health)
	if *dist != "" {
		mux.Handle("/", spaHandler(*dist))
	}

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
