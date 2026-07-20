// Public air-conditioned places — the fallback ring, NOT the shelter
// tier. An official refuge is a city's promise; these are crowd
// knowledge from OpenStreetMap: buildings anyone may walk into that
// mappers tagged air_conditioning=yes, plus shopping malls (climate-
// controlled by construction). They exist everywhere, including the
// cities that publish no shelter list — which is exactly when a hot
// body needs an option. The client must label them as not-official;
// this endpoint keeps them in a separate response so the two tiers
// can never blend by accident.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type coolPlace struct {
	Lon   float64 `json:"lon"`
	Lat   float64 `json:"lat"`
	Name  string  `json:"name"`
	Kind  string  `json:"kind"`
	Hours string  `json:"hours,omitempty"`
}

// Public instances, tried in order. Overpass 504s under summer load
// (observed live 2026-07-20) — failover plus the cell cache plus
// stale-serving is the difference between "degraded" and "down".
var overpassHosts = []string{
	"https://overpass-api.de/api/interpreter",
	"https://overpass.openstreetmap.fr/api/interpreter",
	"https://lz4.overpass-api.de/api/interpreter",
	"https://overpass.kumi.systems/api/interpreter",
	"https://overpass.private.coffee/api/interpreter",
}

// Kinds a stranger may walk into. air_conditioning=yes on an office
// or hotel room helps nobody passing by; the whitelist keeps the
// list walkable-into. Malls are included without the tag — enclosed
// shopping centres are climate-controlled by construction.
var coolPlaceKinds = map[string]bool{
	// shop
	"mall": true, "department_store": true, "supermarket": true,
	"chemist": true,
	// amenity
	"library": true, "community_centre": true, "cinema": true,
	"place_of_worship": true, "pharmacy": true, "cafe": true,
	"restaurant": true, "fast_food": true, "townhall": true,
	"arts_centre": true,
	// tourism
	"museum": true, "gallery": true,
}

const (
	coolCellDeg   = 0.02            // point-mode cache cell ≈ 1.5 km
	coolRadiusM   = 2500            // point-mode radius from cell center
	coolTileDeg   = 0.1             // map-mode tile ≈ 8×11 km
	coolMaxTiles  = 12              // per request — forces city zoom
	coolPlacesTTL = 24 * time.Hour  // OSM edits are slow-moving
	coolPlacesErr = 5 * time.Minute // back off failing instances
)

type coolPlacesEntry struct {
	places  []coolPlace
	fetched time.Time
	tried   time.Time
	err     error
}

type coolPlacesClient struct {
	http *http.Client

	mu    sync.Mutex
	cache map[string]coolPlacesEntry
}

func newCoolPlaces() *coolPlacesClient {
	return &coolPlacesClient{
		http:  &http.Client{Timeout: 25 * time.Second},
		cache: map[string]coolPlacesEntry{},
	}
}

// get returns the places for the cell containing (lon, lat); same
// stale-beats-nothing contract as the refuge client.
func (c *coolPlacesClient) get(lon, lat float64) ([]coolPlace, error) {
	ce := math.Round(lon/coolCellDeg) * coolCellDeg
	cn := math.Round(lat/coolCellDeg) * coolCellDeg
	key := fmt.Sprintf("%.2f,%.2f", ce, cn)

	c.mu.Lock()
	ent, ok := c.cache[key]
	c.mu.Unlock()
	if ok && ent.places != nil &&
		time.Since(ent.fetched) < coolPlacesTTL {
		return ent.places, nil
	}
	if ok && time.Since(ent.tried) < coolPlacesErr {
		if ent.places != nil {
			return ent.places, nil
		}
		return nil, ent.err
	}
	places, err := c.fetch(ce, cn)
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.cache[key] = coolPlacesEntry{
			places: ent.places, fetched: ent.fetched,
			tried: now, err: err,
		}
		if ent.places != nil {
			return ent.places, nil
		}
		return nil, err
	}
	c.cache[key] = coolPlacesEntry{
		places: places, fetched: now, tried: now,
	}
	return places, nil
}

func (c *coolPlacesClient) fetch(lon, lat float64) ([]coolPlace, error) {
	dLat := coolRadiusM / 111320.0
	dLon := dLat / math.Cos(lat*math.Pi/180)
	return c.fetchBBox(lat-dLat, lon-dLon, lat+dLat, lon+dLon)
}

// getTile: one map tile's places, cached like the point cells.
func (c *coolPlacesClient) getTile(ti, tj int) ([]coolPlace, error) {
	key := fmt.Sprintf("t:%d,%d", ti, tj)
	c.mu.Lock()
	ent, ok := c.cache[key]
	c.mu.Unlock()
	if ok && ent.places != nil &&
		time.Since(ent.fetched) < coolPlacesTTL {
		return ent.places, nil
	}
	if ok && time.Since(ent.tried) < coolPlacesErr {
		if ent.places != nil {
			return ent.places, nil
		}
		return nil, ent.err
	}
	s := float64(tj) * coolTileDeg
	w := float64(ti) * coolTileDeg
	places, err := c.fetchBBox(s, w, s+coolTileDeg, w+coolTileDeg)
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.cache[key] = coolPlacesEntry{
			places: ent.places, fetched: ent.fetched,
			tried: now, err: err,
		}
		if ent.places != nil {
			return ent.places, nil
		}
		return nil, err
	}
	c.cache[key] = coolPlacesEntry{
		places: places, fetched: now, tried: now,
	}
	return places, nil
}

// getBBox: a viewport's places, assembled from cached tiles fetched
// concurrently. Partial truth is reported as partial (ok=false per
// failed tile counted), never silently passed off as complete.
func (c *coolPlacesClient) getBBox(w, s, e, n float64) (
	[]coolPlace, bool, error) {
	i0 := int(math.Floor(w / coolTileDeg))
	i1 := int(math.Floor(e / coolTileDeg))
	j0 := int(math.Floor(s / coolTileDeg))
	j1 := int(math.Floor(n / coolTileDeg))
	if (i1-i0+1)*(j1-j0+1) > coolMaxTiles {
		return nil, false, errBBoxTooBig
	}
	type result struct {
		places []coolPlace
		err    error
	}
	var wg sync.WaitGroup
	results := make([]result, 0, coolMaxTiles)
	for i := i0; i <= i1; i++ {
		for j := j0; j <= j1; j++ {
			results = append(results, result{})
			r := &results[len(results)-1]
			wg.Add(1)
			go func(ti, tj int) {
				defer wg.Done()
				r.places, r.err = c.getTile(ti, tj)
			}(i, j)
		}
	}
	wg.Wait()
	out := []coolPlace{}
	seen := map[string]bool{}
	failed := 0
	for _, r := range results {
		if r.err != nil {
			failed++
			continue
		}
		for _, p := range r.places {
			key := fmt.Sprintf("%s@%.3f,%.3f", p.Name, p.Lon, p.Lat)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, p)
		}
	}
	if failed == len(results) {
		return nil, false, errors.New("coolplaces: every tile failed")
	}
	return out, failed > 0, nil
}

var errBBoxTooBig = errors.New("bbox too big — zoom in")

func (c *coolPlacesClient) fetchBBox(s, w, n, e float64) (
	[]coolPlace, error) {
	// A global [bbox:...] prefilter, NOT (around:...): around on a
	// planet-wide tag forces a tag-first scan that timed out on
	// every instance (32 s and dead); the same search bbox-first
	// answered in 2 s. Verified live on a loaded instance.
	q := fmt.Sprintf(`[out:json][timeout:8][bbox:%f,%f,%f,%f];(`+
		`nwr["air_conditioning"="yes"]["name"];`+
		`nwr["shop"="mall"]["name"];`+
		`);out center 150;`,
		s, w, n, e)
	// One overall deadline across every instance: a panicking phone
	// is waiting on this response, and four hung mirrors must cost
	// seconds, not two minutes. (Observed live: 4×25 s of hangs.)
	ctx, cancel := context.WithTimeout(context.Background(),
		20*time.Second)
	defer cancel()
	var lastErr error
	for _, host := range overpassHosts {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			host, strings.NewReader("data="+url.QueryEscape(q)))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type",
			"application/x-www-form-urlencoded")
		// OSM policy asks for an identifying UA; overpass-api.de
		// 406s some defaults (seen live)
		req.Header.Set("User-Agent",
			"ClimateUmbral/1.0 (+https://climateumbral.eu)")
		res, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("coolplaces %s: %s", host, res.Status)
			continue
		}
		places, err := parseOverpassCoolPlaces(raw)
		if err != nil {
			lastErr = err
			continue
		}
		return places, nil
	}
	if lastErr == nil {
		lastErr = errors.New("coolplaces: no instance reachable")
	}
	return nil, lastErr
}

func parseOverpassCoolPlaces(raw []byte) ([]coolPlace, error) {
	var doc struct {
		// Overpass reports its own failures as HTTP 200 with a
		// remark ("runtime error: Query timed out…") and empty
		// elements — that is a failed instance, not "nothing
		// tagged here". Seen live: two city centres "empty".
		Remark   string `json:"remark"`
		Elements []struct {
			Lon    float64 `json:"lon"`
			Lat    float64 `json:"lat"`
			Center *struct {
				Lon float64 `json:"lon"`
				Lat float64 `json:"lat"`
			} `json:"center"`
			Tags map[string]string `json:"tags"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("coolplaces: %w", err)
	}
	if strings.Contains(strings.ToLower(doc.Remark), "error") {
		return nil, fmt.Errorf("coolplaces: remark: %s", doc.Remark)
	}
	seen := map[string]bool{} // name+rounded coords: ways often twin nodes
	out := []coolPlace{}
	for _, e := range doc.Elements {
		lon, lat := e.Lon, e.Lat
		if e.Center != nil {
			lon, lat = e.Center.Lon, e.Center.Lat
		}
		name := e.Tags["name"]
		kind := ""
		for _, k := range []string{"shop", "amenity", "tourism"} {
			if v := e.Tags[k]; coolPlaceKinds[v] {
				kind = v
				break
			}
		}
		if name == "" || kind == "" || (lon == 0 && lat == 0) {
			continue
		}
		key := fmt.Sprintf("%s@%.3f,%.3f", name, lon, lat)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, coolPlace{
			Lon: lon, Lat: lat, Name: name, Kind: kind,
			Hours: strings.TrimSpace(e.Tags["opening_hours"]),
		})
	}
	return out, nil // empty is a real answer: nothing tagged here
}

// handleCoolPlaces: GET /api/coolplaces — the not-official ring.
// Point mode (?lon=&lat=) serves the panic finder; bbox mode
// (?w=&s=&e=&n=) serves the map viewport, capped to city-zoom tile
// counts. 502 on upstream failure so the client can say
// "unavailable" instead of the false all-clear of an empty list;
// a partially-fetched viewport says so.
func (s *server) handleCoolPlaces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	inEU := func(lon, lat float64) bool {
		return lon >= -25 && lon <= 45 && lat >= 34 && lat <= 72
	}
	if q.Has("w") {
		bw, errW := strconv.ParseFloat(q.Get("w"), 64)
		bs, errS := strconv.ParseFloat(q.Get("s"), 64)
		be, errE := strconv.ParseFloat(q.Get("e"), 64)
		bn, errN := strconv.ParseFloat(q.Get("n"), 64)
		if errW != nil || errS != nil || errE != nil || errN != nil ||
			bw >= be || bs >= bn ||
			!inEU(bw, bs) || !inEU(be, bn) {
			writeJSON(w, http.StatusBadRequest,
				map[string]any{"error": "bad bbox"})
			return
		}
		places, partial, err := s.coolPlaces.getBBox(bw, bs, be, bn)
		if errors.Is(err, errBBoxTooBig) {
			writeJSON(w, http.StatusBadRequest,
				map[string]any{"error": err.Error()})
			return
		}
		if err != nil {
			log.Printf("coolplaces bbox: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error": "cool places lookup unavailable"})
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		writeJSON(w, http.StatusOK, map[string]any{
			"places":      places,
			"partial":     partial,
			"attribution": "© OpenStreetMap contributors (ODbL)",
		})
		return
	}
	lon, errLon := strconv.ParseFloat(q.Get("lon"), 64)
	lat, errLat := strconv.ParseFloat(q.Get("lat"), 64)
	if errLon != nil || errLat != nil || !inEU(lon, lat) {
		writeJSON(w, http.StatusBadRequest,
			map[string]any{"error": "lon/lat outside Europe"})
		return
	}
	places, err := s.coolPlaces.get(lon, lat)
	if err != nil {
		log.Printf("coolplaces: %v", err)
		writeJSON(w, http.StatusBadGateway,
			map[string]any{"error": "cool places lookup unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, map[string]any{
		"places":      places,
		"attribution": "© OpenStreetMap contributors (ODbL)",
	})
}
