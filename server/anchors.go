// Human-hour anchors from OpenStreetMap (Overpass): the places where
// bodies spend their hours — schools, playgrounds, hospitals,
// markets, bus stops. Level 1 of the presence layer (GAME_DESIGN):
// exposure-ranked candidates from open data, no tracking, no keys.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const overpassURL = "https://overpass-api.de/api/interpreter"
const overpassUA = "Tilewhip/0.1 (+https://github.com/thdelmas/Tilewhip)"

// anchorWeights: rough human-hours multipliers. Kids at school beat
// commuters at a bus stop; both beat an empty lot.
var anchorWeights = map[string]float64{
	"school": 3, "kindergarten": 3, "playground": 3,
	"hospital": 2, "marketplace": 2, "bus_stop": 1,
}

type anchor struct {
	Lon  float64 `json:"lon"`
	Lat  float64 `json:"lat"`
	Kind string  `json:"kind"`
	Name string  `json:"name,omitempty"`
	W    float64 `json:"w"`
}

type anchorClient struct {
	http  *http.Client
	mu    sync.Mutex
	cache map[string][]anchor
}

func newAnchors() *anchorClient {
	return &anchorClient{
		http:  &http.Client{Timeout: 30 * time.Second},
		cache: map[string][]anchor{},
	}
}

func (a *anchorClient) fetch(w, s, e, n float64) ([]anchor, error) {
	key := fmt.Sprintf("%.4f,%.4f,%.4f,%.4f", w, s, e, n)
	a.mu.Lock()
	if v, ok := a.cache[key]; ok {
		a.mu.Unlock()
		return v, nil
	}
	a.mu.Unlock()

	bb := fmt.Sprintf("(%f,%f,%f,%f)", s, w, n, e)
	amen := `["amenity"~"^(school|kindergarten|hospital|marketplace)$"]`
	q := `[out:json][timeout:10];(` +
		`node` + amen + bb + `;way` + amen + bb + `;` +
		`node["leisure"="playground"]` + bb + `;` +
		`way["leisure"="playground"]` + bb + `;` +
		`node["highway"="bus_stop"]` + bb + `;` +
		`);out center 150;`
	req, err := http.NewRequest(http.MethodPost, overpassURL,
		strings.NewReader(url.Values{"data": {q}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", overpassUA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass: %s", res.Status)
	}
	var body struct {
		Elements []struct {
			Lat    float64 `json:"lat"`
			Lon    float64 `json:"lon"`
			Center struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"center"`
			Tags map[string]string `json:"tags"`
		} `json:"elements"`
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	out := []anchor{}
	for _, el := range body.Elements {
		kind := el.Tags["amenity"]
		if kind == "" {
			kind = el.Tags["leisure"]
		}
		if kind == "" {
			kind = el.Tags["highway"]
		}
		wt := anchorWeights[kind]
		if wt == 0 {
			continue
		}
		lat, lon := el.Lat, el.Lon
		if lat == 0 && lon == 0 {
			lat, lon = el.Center.Lat, el.Center.Lon
		}
		out = append(out, anchor{
			Lon: lon, Lat: lat, Kind: kind,
			Name: el.Tags["name"], W: wt,
		})
	}
	a.mu.Lock()
	if len(a.cache) > 128 { // simple shed; anchors are cheap to refetch
		a.cache = map[string][]anchor{}
	}
	a.cache[key] = out
	a.mu.Unlock()
	return out, nil
}

// handleAnchors: GET /api/anchors?bbox=w,s,e,n (lon/lat degrees).
// Fails soft: the game plays fine without anchors, so errors return
// an empty list with a warning header rather than a 5xx.
func (s *server) handleAnchors(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Query().Get("bbox"), ",")
	if len(parts) != 4 {
		writeErr(w, http.StatusBadRequest, "need bbox=w,s,e,n")
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
	if b[2]-b[0] <= 0 || b[3]-b[1] <= 0 ||
		b[2]-b[0] > 0.12 || b[3]-b[1] > 0.08 {
		writeErr(w, http.StatusBadRequest, "bbox too large or empty")
		return
	}
	anchors, err := s.anchors.fetch(b[0], b[1], b[2], b[3])
	if err != nil {
		w.Header().Set("X-Anchors-Warning", err.Error())
		writeJSON(w, http.StatusOK, []anchor{})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	writeJSON(w, http.StatusOK, anchors)
}
