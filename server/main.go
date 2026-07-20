// Tilewhip API, V3: the whole of Europe is the board.
//
// There is no local grid anymore. The visual map streams straight
// from the EEA image service; this server owns the game: viewport
// value rasters (proxied + cached), and the claims ledger keyed to
// the continent-wide EPSG:3035 10 m pixel grid — pixel (pe, pn) =
// floor(easting/10), floor(northing/10).
//
// The cascade rule is enforced against live upstream data: a pixel is
// pledgeable only if it is hard-sealed (>=90%) and touches >=2
// green-or-actively-claimed neighbours; live pledges and flips count
// as green. Expired pledges release their pixel.
package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

const (
	hardSealed  = 90 // >= this % imperviousness is claimable
	greenMax    = 10 // <= this % imperviousness counts as green
	minGreens   = 2  // neighbours needed to be a candidate
	maxNameLen  = 40
	maxPhotoLen = 500
	maxRaster   = 512 // max viewport raster dimension

	// the ledger is one JSON file rewritten per act and walked per
	// leaderboard: it must stay bounded no matter who is posting
	maxClaims        = 50_000
	maxJoins         = 50_000
	maxJoinsPerBlock = 200

	tokenHeader = "X-Tilewhip-Token"
)

type server struct {
	eea         *eeaClient
	anchors     *anchorClient
	refuges     *refugeClient
	hub         *hub
	limiter     *limiter
	readLimiter *limiter
	trustProxy  bool

	mu         sync.Mutex
	ledger     *ledger
	ledgerPath string
	expiry     time.Duration
}

var actKinds = map[string]bool{
	"depave": true, "tree": true, "coolroof": true,
}

// pledgeable reports whether continent pixel (pe, pn) can take a
// pledge of the given kind, judged against nb — its 3x3 water-merged
// neighbourhood, fetched by the caller before taking s.mu so no
// network I/O ever happens under the lock. All acts need hard-sealed
// ground; only depaves need the front line (>=2 green-or-claimed
// neighbours) — a tree pit breaks the middle of a parking lot
// precisely where no front line reaches. Callers hold s.mu.
func (s *server) pledgeable(
	pe, pn int, kind string, nb []byte, now time.Time,
) error {
	if c := s.ledger.activeAt(pe, pn, now); c != nil {
		return errors.New("already " + c.status(now))
	}
	if v := nb[4]; v < hardSealed || v > 100 {
		return errors.New("not hard-sealed (needs >=90% imperviousness)")
	}
	if kind != "depave" {
		return nil
	}
	green := s.ledger.greenSet(now)
	greens := 0
	for dy := -1; dy <= 1; dy++ { // dy = +1 is north
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			v := nb[(1-dy)*3+(1+dx)] // row 0 = north
			if v <= greenMax || green[[2]int{pe + dx, pn + dy}] {
				greens++
			}
		}
	}
	if greens < minGreens {
		return errors.New(
			"not a candidate: needs >=2 green or claimed neighbours")
	}
	return nil
}

// persist writes the ledger to disk; only a durable write notifies
// the live streams. Callers hold s.mu and roll their mutation back
// when this fails — a 2xx must mean "on disk".
func (s *server) persist() error {
	if err := s.ledger.persist(s.ledgerPath); err != nil {
		log.Printf("persist ledger: %v", err)
		return err
	}
	s.hub.notify()
	return nil
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dataDir := flag.String("data", "./data",
		"directory holding claims.json")
	dist := flag.String("dist", "",
		"built frontend to serve at / (empty = API only)")
	expiryDays := flag.Int("expiry-days", 90,
		"days before an unflipped pledge returns to the pool")
	trustProxy := flag.Bool("trust-proxy", false,
		"key rate limits on the rightmost X-Forwarded-For hop; "+
			"set only behind a reverse proxy that always sets it")
	flag.Parse()

	expiry := time.Duration(*expiryDays) * 24 * time.Hour
	s := &server{
		eea:     newEEA(),
		anchors: newAnchors(),
		refuges: newRefuges(),
		hub:     newHub(),
		// ~12 acts/min after a burst of 5; reads get a budget an
		// honest map never exhausts but a tight loop does
		limiter:     newLimiter(0.2, 5),
		readLimiter: newLimiter(10, 50),
		trustProxy:  *trustProxy,
		ledgerPath:  filepath.Join(*dataDir, "claims.json"),
		expiry:      expiry,
	}
	var err error
	s.ledger, err = loadLedger(s.ledgerPath, expiry)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("europe is the board: %d acts / %d signatures",
		len(s.ledger.Claims), len(s.ledger.Joins))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/raster", s.rlimit(s.handleRaster))
	mux.HandleFunc("GET /api/anchors", s.rlimit(s.handleAnchors))
	mux.HandleFunc("GET /api/refuges", s.rlimit(s.handleRefuges))
	mux.HandleFunc("GET /api/claims", s.rlimit(s.handleGetLedger))
	mux.HandleFunc("POST /api/claims", s.limit(s.handlePledge))
	mux.HandleFunc("POST /api/claims/{pe}/{pn}/flip",
		s.limit(s.handleFlip))
	mux.HandleFunc("DELETE /api/claims/{pe}/{pn}",
		s.limit(s.handleAbandon))
	mux.HandleFunc("POST /api/joins", s.limit(s.handleJoin))
	mux.HandleFunc("DELETE /api/joins/{be}/{bn}",
		s.limit(s.handleLeave))
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/leaderboard",
		s.rlimit(s.handleLeaderboard))
	health := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	mux.HandleFunc("GET /api/health", health)
	if *dist != "" {
		mux.Handle("/", spaHandler(*dist))
	}

	// no blanket Read/WriteTimeout: /api/events is a long-lived
	// stream. The header timeout is what stops slowloris openers.
	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	log.Printf("listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}
