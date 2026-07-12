// Tilewhip API: serves the sealed-% grid and keeps the game ledger.
//
// The cascade rule is enforced here, not just in the UI: a pixel is
// pledgeable only if it is hard-sealed (>=90%) and touches >=3
// green-or-actively-claimed neighbours — the same "gray touching
// green" detector the map shows, with live pledges and flips counting
// as green so every claim can open its neighbours. Expired pledges
// stop counting and their pixel returns to the pool.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	seaValue    = 254
	nodataValue = 255
	hardSealed  = 90 // >= this % imperviousness is claimable
	greenMax    = 10 // <= this % imperviousness counts as green
	minGreens   = 3  // neighbours needed to be a candidate
	maxNameLen  = 40
	maxPhotoLen = 500
	tokenHeader = "X-Tilewhip-Token"
)

type server struct {
	grid []byte
	w, h int
	meta map[string]any

	mu         sync.Mutex
	ledger     *ledger
	ledgerPath string
	expiry     time.Duration
}

func load(dataDir, name string, expiry time.Duration) (*server, error) {
	metaRaw, err := os.ReadFile(filepath.Join(dataDir, name+".json"))
	if err != nil {
		return nil, fmt.Errorf(
			"read metadata (run `make fetch` first?): %w", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return nil, fmt.Errorf("parse %s.json: %w", name, err)
	}
	// fetch_grid.py embeds the grid here; we serve it as binary instead.
	delete(meta, "b64")
	w, _ := meta["width"].(float64)
	h, _ := meta["height"].(float64)
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%s.json has no width/height", name)
	}
	grid, err := os.ReadFile(filepath.Join(dataDir, name+".raw"))
	if err != nil {
		return nil, err
	}
	if len(grid) != int(w)*int(h) {
		return nil, fmt.Errorf("%s.raw is %d bytes, expected %dx%d",
			name, len(grid), int(w), int(h))
	}

	s := &server{
		grid: grid, w: int(w), h: int(h), meta: meta,
		ledgerPath: filepath.Join(dataDir, "claims.json"),
		expiry:     expiry,
	}
	s.ledger, err = loadLedger(s.ledgerPath, expiry)
	if err != nil {
		return nil, fmt.Errorf("load ledger: %w", err)
	}
	return s, nil
}

func (s *server) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < s.w && y < s.h
}

func (s *server) sealed(x, y int) bool {
	v := s.grid[y*s.w+x]
	return v >= hardSealed && v < seaValue
}

// pledgeable reports whether (x, y) can be pledged now. Callers hold
// s.mu.
func (s *server) pledgeable(x, y int, now time.Time) error {
	if !s.inBounds(x, y) {
		return errors.New("out of bounds")
	}
	if !s.sealed(x, y) {
		return errors.New("not hard-sealed (needs >=90% imperviousness)")
	}
	if c := s.ledger.activeAt(x, y, now); c != nil {
		return errors.New("already " + c.status(now))
	}
	active := s.ledger.activeSet(now)
	greens := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			if (dx == 0 && dy == 0) || !s.inBounds(nx, ny) {
				continue
			}
			if active[[2]int{nx, ny}] || s.grid[ny*s.w+nx] <= greenMax {
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
}

// ---- views: what GET endpoints expose (never tokens) ----

type claimView struct {
	X        int        `json:"x"`
	Y        int        `json:"y"`
	Name     string     `json:"name,omitempty"`
	TS       time.Time  `json:"ts"`
	Deadline time.Time  `json:"deadline"`
	Status   string     `json:"status"`
	Flipped  *time.Time `json:"flipped,omitempty"`
	Photo    string     `json:"photo,omitempty"`
}

type watchView struct {
	X    int       `json:"x"`
	Y    int       `json:"y"`
	Name string    `json:"name,omitempty"`
	TS   time.Time `json:"ts"`
}

func viewOf(c *claim, now time.Time) claimView {
	return claimView{
		X: c.X, Y: c.Y, Name: c.Name, TS: c.TS,
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

func pathXY(r *http.Request) (int, int, error) {
	x, errX := strconv.Atoi(r.PathValue("x"))
	y, errY := strconv.Atoi(r.PathValue("y"))
	if errX != nil || errY != nil {
		return 0, 0, errors.New("bad pixel coordinates in path")
	}
	return x, y, nil
}

func (s *server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.meta)
}

func (s *server) handleGridRaw(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(s.grid)
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
			watchView{X: wa.X, Y: wa.Y, Name: wa.Name, TS: wa.TS})
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
		X    int    `json:"x"`
		Y    int    `json:"y"`
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
	if err := s.pledgeable(req.X, req.Y, now); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	c := claim{
		X: req.X, Y: req.Y, Name: name, TS: now,
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
	x, y, err := pathXY(r)
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
	c := s.ledger.activeAt(x, y, now)
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
	x, y, err := pathXY(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	token := r.Header.Get(tokenHeader)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ledger.Claims {
		c := &s.ledger.Claims[i]
		if c.X == x && c.Y == y && token != "" && c.Token == token {
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
		X    int    `json:"x"`
		Y    int    `json:"y"`
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
	if !s.inBounds(req.X, req.Y) {
		writeErr(w, http.StatusConflict, "out of bounds")
		return
	}
	if !s.sealed(req.X, req.Y) {
		writeErr(w, http.StatusConflict, "only sealed pixels need watching")
		return
	}
	if c := s.ledger.activeAt(req.X, req.Y, now); c != nil &&
		c.status(now) == statusFlipped {
		writeErr(w, http.StatusConflict, "already flipped")
		return
	}
	wa := watch{X: req.X, Y: req.Y, Name: name, TS: now, Token: newToken()}
	s.ledger.Watches = append(s.ledger.Watches, wa)
	s.persist()
	writeJSON(w, http.StatusCreated, map[string]any{
		"watch": watchView{X: wa.X, Y: wa.Y, Name: wa.Name, TS: wa.TS},
		"token": wa.Token,
	})
}

func (s *server) handleUnwatch(w http.ResponseWriter, r *http.Request) {
	x, y, err := pathXY(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	token := r.Header.Get(tokenHeader)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ledger.Watches {
		wa := &s.ledger.Watches[i]
		if wa.X == x && wa.Y == y && token != "" && wa.Token == token {
			s.ledger.Watches = append(
				s.ledger.Watches[:i], s.ledger.Watches[i+1:]...)
			s.persist()
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeErr(w, http.StatusForbidden, "wrong or missing token")
}

func (s *server) handleLeaderboard(w http.ResponseWriter, _ *http.Request) {
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
		"directory with <grid>.raw/.json; claims.json lives here")
	gridName := flag.String("grid", "bcn", "grid basename inside the data dir")
	dist := flag.String("dist", "",
		"built frontend to serve at / (empty = API only)")
	expiryDays := flag.Int("expiry-days", 90,
		"days before an unflipped pledge returns to the pool")
	flag.Parse()

	expiry := time.Duration(*expiryDays) * 24 * time.Hour
	s, err := load(*dataDir, *gridName, expiry)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("grid %s: %dx%d, %d claims / %d watches on the ledger",
		*gridName, s.w, s.h, len(s.ledger.Claims), len(s.ledger.Watches))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/grid", s.handleMeta)
	mux.HandleFunc("GET /api/grid.raw", s.handleGridRaw)
	mux.HandleFunc("GET /api/claims", s.handleGetLedger)
	mux.HandleFunc("POST /api/claims", s.handlePledge)
	mux.HandleFunc("POST /api/claims/{x}/{y}/flip", s.handleFlip)
	mux.HandleFunc("DELETE /api/claims/{x}/{y}", s.handleAbandon)
	mux.HandleFunc("POST /api/watches", s.handleWatch)
	mux.HandleFunc("DELETE /api/watches/{x}/{y}", s.handleUnwatch)
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
