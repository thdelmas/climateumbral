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
	coolCellDeg   = 0.02            // cache cell ≈ 1.5 km
	coolRadiusM   = 2500            // query radius from cell center
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
	q := fmt.Sprintf(`[out:json][timeout:15];(`+
		`nwr["air_conditioning"="yes"]["name"](around:%d,%f,%f);`+
		`nwr["shop"="mall"]["name"](around:%d,%f,%f);`+
		`);out center 80;`,
		coolRadiusM, lat, lon, coolRadiusM, lat, lon)
	var lastErr error
	for _, host := range overpassHosts {
		req, err := http.NewRequest(http.MethodPost, host,
			strings.NewReader("data="+url.QueryEscape(q)))
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

// handleCoolPlaces: GET /api/coolplaces?lon=&lat= — the not-official
// ring around one point. 502 on upstream failure so the client can
// say "unavailable" instead of the false all-clear of an empty list.
func (s *server) handleCoolPlaces(w http.ResponseWriter, r *http.Request) {
	lon, errLon := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	lat, errLat := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	if errLon != nil || errLat != nil ||
		lon < -25 || lon > 45 || lat < 34 || lat > 72 {
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
