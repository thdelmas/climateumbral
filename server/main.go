// Tilewhip API: serves the sealed-% grid and keeps the claims ledger.
//
// A claim is a pledge to depave one 10x10 m pixel. The cascade rule is
// enforced here, not just in the UI: a pixel is claimable only if it is
// hard-sealed (>=90%) and touches >=3 green-or-claimed neighbours — the
// same "gray touching green" detector the map shows, with claimed pixels
// counting as green so every claim can open its neighbours.
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
)

type claim struct {
	X    int       `json:"x"`
	Y    int       `json:"y"`
	Name string    `json:"name,omitempty"`
	TS   time.Time `json:"ts"`
}

type server struct {
	grid []byte
	w, h int
	meta map[string]any

	mu         sync.Mutex
	claims     []claim
	claimed    map[int]bool
	claimsPath string
}

func load(dataDir, name string) (*server, error) {
	metaRaw, err := os.ReadFile(filepath.Join(dataDir, name+".json"))
	if err != nil {
		return nil, fmt.Errorf("read metadata (run `make fetch` first?): %w", err)
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
		claimed:    map[int]bool{},
		claimsPath: filepath.Join(dataDir, "claims.json"),
	}
	if raw, err := os.ReadFile(s.claimsPath); err == nil {
		if err := json.Unmarshal(raw, &s.claims); err != nil {
			return nil, fmt.Errorf("parse claims.json: %w", err)
		}
		for _, c := range s.claims {
			s.claimed[c.Y*s.w+c.X] = true
		}
	}
	return s, nil
}

// candidate reports whether (x, y) is claimable now. Callers hold s.mu.
func (s *server) candidate(x, y int) error {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return errors.New("out of bounds")
	}
	i := y*s.w + x
	if s.claimed[i] {
		return errors.New("already claimed")
	}
	if v := s.grid[i]; v < hardSealed || v >= seaValue {
		return errors.New("not hard-sealed (needs >=90% imperviousness)")
	}
	greens := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			if (dx == 0 && dy == 0) || nx < 0 || ny < 0 || nx >= s.w || ny >= s.h {
				continue
			}
			if s.claimed[ny*s.w+nx] || s.grid[ny*s.w+nx] <= greenMax {
				greens++
			}
		}
	}
	if greens < minGreens {
		return errors.New("not a candidate: needs >=3 green or claimed neighbours")
	}
	return nil
}

func (s *server) persistClaims() error {
	raw, err := json.MarshalIndent(s.claims, "", " ")
	if err != nil {
		return err
	}
	tmp := s.claimsPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.claimsPath)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.meta)
}

func (s *server) handleGridRaw(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(s.grid)
}

func (s *server) handleGetClaims(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"claims": s.claims,
		"m2":     len(s.claims) * 100, // one 10 m pixel = 100 m²
	})
}

func (s *server) handlePostClaim(w http.ResponseWriter, r *http.Request) {
	var req struct {
		X    int    `json:"x"`
		Y    int    `json:"y"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "bad JSON body"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.candidate(req.X, req.Y); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	c := claim{
		X: req.X, Y: req.Y,
		Name: strings.TrimSpace(req.Name),
		TS:   time.Now().UTC(),
	}
	s.claims = append(s.claims, c)
	s.claimed[req.Y*s.w+req.X] = true
	if err := s.persistClaims(); err != nil {
		log.Printf("persist claims: %v", err)
	}
	writeJSON(w, http.StatusCreated, c)
}

// spaHandler serves dist with an index.html fallback for client-side routes.
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
	flag.Parse()

	s, err := load(*dataDir, *gridName)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("grid %s: %dx%d, %d claims on the ledger",
		*gridName, s.w, s.h, len(s.claims))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/grid", s.handleMeta)
	mux.HandleFunc("GET /api/grid.raw", s.handleGridRaw)
	mux.HandleFunc("GET /api/claims", s.handleGetClaims)
	mux.HandleFunc("POST /api/claims", s.handlePostClaim)
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
