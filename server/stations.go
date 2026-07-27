// Measured air temperature — the ground-truth tier. A station pin is
// a real thermometer on a mast, its hourly record published by NOAA
// (GHCN-hourly, the ISD successor). These numbers exist to anchor the
// modeled heat layers to reality, and the rule they enforce is design
// rule 1 at its strictest: measured and modeled degrees NEVER mix. A
// station reading never colors the heat overlay; the model never
// touches a pin. The client says "measured" on every surface a
// station number appears.
//
// Coverage is a curated list: only stations verified live in the
// current GHCNh by-year feed belong here — a stale anchor is worse
// than none, so the response carries the observation time and the
// client shows the lag rather than implying "now". (ISD-lite, the
// previous NOAA product, silently stopped for Spain in 2025 — that
// trap is why the list is verified, not derived.)
package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type stationDef struct {
	ID   string // GHCNh station id
	Name string // display name
	Lat  float64
	Lon  float64
	Elev float64 // meters — part of reading the number honestly
}

// Verified live in the 2026 by-year feed on 2026-07-26. Other
// Barcelona-area GHCNh ids (airport SPN00000076, Fabra SPUP00000D5,
// Raval-adjacent SPUPXX0201B…) have no current-year file — do not add
// a station without probing its by-year URL first.
var stationDefs = []stationDef{{
	ID:   "SPM00008180",
	Name: "Barcelona city (WMO 08180)",
	Lat:  41.417, Lon: 2.133, Elev: 176,
}, {
	ID:   "SPUP00000Y9",
	Name: "Port de Barcelona — ZAL Prat",
	Lat:  41.32, Lon: 2.13, Elev: 4,
}}

const (
	ghcnhURL = "https://www.ncei.noaa.gov/oa/global-historical-" +
		"climatology-network/hourly/access/by-year/%d/psv/GHCNh_%s_%d.psv"
	// The by-year file is append-only; the tail holds weeks of
	// hourly rows. A ranged read spares NCEI the megabytes.
	stationTail  = 256 * 1024
	stationTTL   = 6 * time.Hour
	stationRetry = 10 * time.Minute
)

type stationObs struct {
	t time.Time // UTC (GHCNh DATE column)
	c float64   // degC
}

// parseGHCNh reads temperature rows from a psv fragment. The first
// line is dropped (a ranged read lands mid-line; on a full read it is
// the header). Pipe-separated, 1-indexed: field 3 = ISO date (UTC),
// field 12 = air temperature in degC. Rows with missing or absurd
// temperatures are skipped, not zeroed.
func parseGHCNh(data []byte) []stationObs {
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 {
		lines = lines[1:]
	}
	obs := make([]stationObs, 0, len(lines))
	for _, ln := range lines {
		f := strings.Split(ln, "|")
		if len(f) < 12 {
			continue
		}
		t, err := time.Parse("2006-01-02T15:04:05", f[2])
		if err != nil {
			continue
		}
		c, err := strconv.ParseFloat(strings.TrimSpace(f[11]), 64)
		if err != nil || c < -60 || c > 60 {
			continue
		}
		obs = append(obs, stationObs{t: t.UTC(), c: c})
	}
	sort.Slice(obs, func(i, j int) bool { return obs[i].t.Before(obs[j].t) })
	return obs
}

// summarize: the latest observation plus min/max over the 24 h that
// precede it — "the last measured day", whatever the feed's lag is.
func summarize(obs []stationObs) (latest stationObs, min24, max24 float64, ok bool) {
	if len(obs) == 0 {
		return stationObs{}, 0, 0, false
	}
	latest = obs[len(obs)-1]
	cut := latest.t.Add(-24 * time.Hour)
	min24, max24 = math.Inf(1), math.Inf(-1)
	for _, o := range obs {
		if o.t.Before(cut) {
			continue
		}
		min24 = math.Min(min24, o.c)
		max24 = math.Max(max24, o.c)
	}
	return latest, min24, max24, true
}

type stationEntry struct {
	obs     []stationObs
	fetched time.Time
	tried   time.Time
	err     error
}

type stationClient struct {
	http *http.Client
	now  func() time.Time

	mu    sync.Mutex
	cache map[string]stationEntry
}

func newStations() *stationClient {
	return &stationClient{
		http:  &http.Client{Timeout: 30 * time.Second},
		now:   time.Now,
		cache: map[string]stationEntry{},
	}
}

// get mirrors refugeClient.get: refetch past the TTL, serve stale on
// upstream failure, back a failing upstream off for stationRetry.
func (c *stationClient) get(id string) ([]stationObs, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.cache[id]
	now := c.now()
	fresh := !e.fetched.IsZero() && now.Sub(e.fetched) < stationTTL
	backoff := e.err != nil && now.Sub(e.tried) < stationRetry
	if fresh || backoff {
		return e.obs, e.err
	}
	e.tried = now
	obs, err := c.fetch(id, now)
	if err != nil {
		e.err = err
		c.cache[id] = e
		return e.obs, err // stale beats nothing
	}
	c.cache[id] = stationEntry{obs: obs, fetched: now, tried: now}
	return obs, nil
}

// fetch pulls the tail of the station's by-year file, falling back to
// the previous year around New Year (the new file appears with the
// first published day).
func (c *stationClient) fetch(id string, now time.Time) ([]stationObs, error) {
	var lastErr error
	for _, year := range []int{now.UTC().Year(), now.UTC().Year() - 1} {
		obs, err := c.fetchYear(id, year)
		if err == nil && len(obs) > 0 {
			return obs, nil
		}
		if err == nil {
			err = fmt.Errorf("no parseable rows for %d", year)
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *stationClient) fetchYear(id string, year int) ([]stationObs, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(ghcnhURL, year, id, year), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=-%d", stationTail))
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("ghcnh %s %d: HTTP %d", id, year, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, stationTail+1))
	if err != nil {
		return nil, err
	}
	// The bucket answers some misses with an XML error document.
	if len(body) > 0 && body[0] == '<' {
		return nil, fmt.Errorf("ghcnh %s %d: error document", id, year)
	}
	return parseGHCNh(body), nil
}

type stationView struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	ElevM     float64 `json:"elev_m"`
	OK        bool    `json:"ok"`
	LatestC   float64 `json:"latest_c,omitempty"`
	LatestUTC string  `json:"latest_utc,omitempty"` // RFC3339, Z
	Min24C    float64 `json:"min24_c,omitempty"`
	Max24C    float64 `json:"max24_c,omitempty"`
}

// handleStations: GET /api/stations — measured air temperature
// anchors. Stations are reported even when their feed fails
// (OK: false): "thermometer data unavailable" must never read as
// "no thermometers exist".
func (s *server) handleStations(w http.ResponseWriter, _ *http.Request) {
	views := make([]stationView, 0, len(stationDefs))
	for _, d := range stationDefs {
		v := stationView{
			ID: d.ID, Name: d.Name, Lat: d.Lat, Lon: d.Lon, ElevM: d.Elev,
		}
		obs, err := s.stations.get(d.ID)
		if err != nil {
			log.Printf("stations %s: %v", d.ID, err)
		}
		if latest, min24, max24, ok := summarize(obs); ok {
			v.OK = true
			v.LatestC = latest.c
			v.LatestUTC = latest.t.Format(time.RFC3339)
			v.Min24C = min24
			v.Max24C = max24
		}
		views = append(views, v)
	}
	w.Header().Set("Cache-Control", "public, max-age=1800")
	writeJSON(w, http.StatusOK, map[string]any{
		"stations":    views,
		"attribution": "NOAA NCEI, Global Historical Climatology Network hourly",
	})
}
