// Official climate shelters — the adaptation layer. A refuge pin is
// a room a city promised: roof, cool air, opening hours, published as
// open data by the municipality that runs the network. That promise
// is the whole tier: pins here NEVER come from the model, and modeled
// cool islands (client-side) never appear in this list — a wrong
// "shelter" sends a body somewhere that won't cool it.
//
// Europe has no continent-wide shelter dataset; refuge networks are
// municipal programs. So coverage is a list of per-city adapters, and
// the response says which networks it carries — an empty map must
// read "no network published here", never "no shelters exist".
package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

type refuge struct {
	Lon  float64 `json:"lon"`
	Lat  float64 `json:"lat"`
	Name string  `json:"name"`
	Addr string  `json:"addr,omitempty"`
	Web  string  `json:"web,omitempty"`
	Src  string  `json:"src"`
}

// refugeSource is one city's published network. Adding a city is
// adding one entry here plus a parser for its format.
type refugeSource struct {
	ID          string
	Name        string
	Attribution string
	URL         string
	parse       func([]byte) ([]refuge, error)
}

// Xarxa de refugis climàtics — Ajuntament de Barcelona, CC BY 4.0,
// updated weekly upstream. The CSV resource (the JSON twin is 40 MB).
const bcnRefugesCSV = "https://opendata-ajuntament.barcelona.cat" +
	"/data/dataset/8f9da263-ff41-4765-ab0d-61b97d7a00b2" +
	"/resource/7ecae024-6cb2-427d-b2d0-e170500e2a38/download"

// Îlots de fraîcheur, équipements & activités — Ville de Paris,
// ODbL, refreshed daily upstream. The JSON export (~400 KB); the
// records API pages at 100 rows.
const parisRefugesJSON = "https://opendata.paris.fr" +
	"/api/explore/v2.1/catalog/datasets" +
	"/ilots-de-fraicheur-equipements-activites/exports/json"

// Coole Zonen — Stadt Wien, CC BY 4.0. The city WFS serves local
// Gauß-Krüger coordinates unless EPSG:4326 is asked for by name.
const wienRefugesWFS = "https://data.wien.gv.at/daten/geo" +
	"?service=WFS&version=1.1.0&request=GetFeature" +
	"&typeName=ogdwien:COOLEZONEOGD" +
	"&outputFormat=json&srsName=EPSG:4326"

// Équipements publics climatisés — Métropole de Lyon, Licence
// Ouverte. The WFS layer behind the metropole's cool-places map.
const lyonRefugesWFS = "https://download.data.grandlyon.com" +
	"/wfs/grandlyon?SERVICE=WFS&VERSION=2.0.0&REQUEST=GetFeature" +
	"&typename=metropole-de-lyon:" +
	"com_donnees_communales.equipementspublicsclimatises" +
	"&outputFormat=application/json&SRSNAME=EPSG:4326"

var refugeSources = []refugeSource{{
	ID:          "bcn",
	Name:        "Barcelona — Xarxa de refugis climàtics",
	Attribution: "Ajuntament de Barcelona, Open Data BCN (CC BY 4.0)",
	URL:         bcnRefugesCSV,
	parse:       parseBCNRefuges,
}, {
	ID:          "paris",
	Name:        "Paris — Îlots de fraîcheur (équipements)",
	Attribution: "Ville de Paris, Paris Data (ODbL)",
	URL:         parisRefugesJSON,
	parse:       parseParisRefuges,
}, {
	ID:          "wien",
	Name:        "Vienna — Coole Zonen",
	Attribution: "Stadt Wien, data.wien.gv.at (CC BY 4.0)",
	URL:         wienRefugesWFS,
	parse:       parseWienRefuges,
}, {
	ID:          "lyon",
	Name:        "Grand Lyon — Équipements publics climatisés",
	Attribution: "Métropole de Lyon, data.grandlyon.com (Licence Ouverte)",
	URL:         lyonRefugesWFS,
	parse:       parseLyonRefuges,
}}

const (
	refugeTTL   = 7 * 24 * time.Hour // upstream cadence is weekly
	refugeRetry = 10 * time.Minute   // back off a failing upstream
)

type refugeEntry struct {
	refuges []refuge
	fetched time.Time // last success
	tried   time.Time // last attempt
	err     error     // last failure, for the backoff window
}

type refugeClient struct {
	http *http.Client

	mu    sync.Mutex
	cache map[string]refugeEntry
}

func newRefuges() *refugeClient {
	return &refugeClient{
		http:  &http.Client{Timeout: 30 * time.Second},
		cache: map[string]refugeEntry{},
	}
}

// get returns a source's refuges, refetching past the TTL. A failed
// refetch serves the stale list rather than nothing: last week's
// shelter network beats an empty map on a hot night. Failures are
// cached too (refugeRetry): a dead portal must not turn every
// request into a 30 s upstream hang.
func (c *refugeClient) get(src refugeSource) ([]refuge, error) {
	c.mu.Lock()
	ent, ok := c.cache[src.ID]
	c.mu.Unlock()
	if ok && ent.refuges != nil &&
		time.Since(ent.fetched) < refugeTTL {
		return ent.refuges, nil
	}
	if ok && time.Since(ent.tried) < refugeRetry {
		if ent.refuges != nil { // stale beats nothing
			return ent.refuges, nil
		}
		return nil, ent.err
	}
	list, err := c.fetch(src)
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.cache[src.ID] = refugeEntry{
			refuges: ent.refuges, fetched: ent.fetched,
			tried: now, err: err,
		}
		if ent.refuges != nil {
			log.Printf("refuges %s: serving stale: %v", src.ID, err)
			return ent.refuges, nil
		}
		return nil, err
	}
	c.cache[src.ID] = refugeEntry{
		refuges: list, fetched: now, tried: now,
	}
	return list, nil
}

func (c *refugeClient) fetch(src refugeSource) ([]refuge, error) {
	res, err := c.http.Get(src.URL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refuges %s: %s", src.ID, res.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return src.parse(raw)
}

// decodeMaybeUTF16 turns a BOM-led UTF-16 byte stream into a string;
// anything else is passed through as UTF-8.
func decodeMaybeUTF16(b []byte) string {
	if len(b) < 2 || !((b[0] == 0xFF && b[1] == 0xFE) ||
		(b[0] == 0xFE && b[1] == 0xFF)) {
		return string(b)
	}
	le := b[0] == 0xFF
	u := make([]uint16, 0, len(b)/2)
	for i := 2; i+1 < len(b); i += 2 {
		if le {
			u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
		} else {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		}
	}
	return string(utf16.Decode(u))
}

// parseBCNRefuges reads Open Data BCN's CSV. Traps (all live in the
// real feed): UTF-16LE with a BOM per line, not just one; columns
// addressed by header name because the city reorders them; one row
// per refuge today but register_id deduped in case the values_*
// columns ever fan out to row-per-attribute like sibling datasets.
func parseBCNRefuges(raw []byte) ([]refuge, error) {
	text := strings.ReplaceAll(decodeMaybeUTF16(raw), "\ufeff", "")
	rd := csv.NewReader(strings.NewReader(text))
	rd.FieldsPerRecord = -1
	rows, err := rd.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("bcn refuges: %w", err)
	}
	if len(rows) < 2 {
		return nil, errors.New("bcn refuges: empty csv")
	}
	col := map[string]int{}
	for i, name := range rows[0] {
		col[strings.TrimSpace(name)] = i
	}
	for _, n := range []string{"register_id", "name",
		"geo_epgs_4326_lat", "geo_epgs_4326_lon"} {
		if _, ok := col[n]; !ok {
			return nil, fmt.Errorf("bcn refuges: column %q missing", n)
		}
	}
	get := func(row []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	seen := map[string]bool{}
	out := []refuge{}
	for _, row := range rows[1:] {
		id := get(row, "register_id")
		name := get(row, "name")
		lat, errLat := strconv.ParseFloat(
			get(row, "geo_epgs_4326_lat"), 64)
		lon, errLon := strconv.ParseFloat(
			get(row, "geo_epgs_4326_lon"), 64)
		if id == "" || seen[id] || name == "" ||
			errLat != nil || errLon != nil {
			continue
		}
		seen[id] = true
		addr := strings.TrimSpace(get(row, "addresses_road_name") +
			" " + get(row, "addresses_start_street_number"))
		if d := get(row, "addresses_district_name"); d != "" {
			if addr != "" {
				addr += " · "
			}
			addr += d
		}
		web := ""
		if get(row, "values_attribute_name") == "Web" {
			if v := get(row, "values_value"); strings.HasPrefix(v, "http") {
				web = v
			}
		}
		out = append(out, refuge{
			Lon: lon, Lat: lat, Name: name, Addr: addr,
			Web: web, Src: "bcn",
		})
	}
	if len(out) == 0 {
		return nil, errors.New("bcn refuges: no rows parsed")
	}
	return out, nil
}

// parisIndoorTypes: the tier promise is a ROOM — roof, cool air. The
// Paris feed mixes those with outdoor street furniture (misters,
// shade sails, pétanque grounds), which belongs to the modeled
// cool-island tier, not here. Whitelist, not blacklist: a type the
// city invents next summer stays off the map until a human reads
// what it is.
var parisIndoorTypes = map[string]bool{
	"Bibliothèque":            true,
	"Musée":                   true,
	"Mairie d'arrondissement": true, // hosts the plan-canicule cooled rooms
	"Lieux de culte":          true,
	"Bains-douches":           true,
	"Piscine":                 true,
}

// parseParisRefuges reads the Paris Data JSON export. Traps (live in
// the real feed): indoor and outdoor site types share one dataset —
// filter by type or a mister pin masquerades as a shelter; a couple
// of identifiants are duplicated; addresses carry doubled spaces;
// paris.fr venue pages need a slug we don't have, so pins get no web
// link. Museums and pools can charge — the addr says so, because a
// paywall at the door is part of whether a body gets cooled.
func parseParisRefuges(raw []byte) ([]refuge, error) {
	var rows []struct {
		ID     string `json:"identifiant"`
		Name   string `json:"nom"`
		Type   string `json:"type"`
		Paying string `json:"payant"`
		Addr   string `json:"adresse"`
		Arr    string `json:"arrondissement"`
		Geo    *struct {
			Lon float64 `json:"lon"`
			Lat float64 `json:"lat"`
		} `json:"geo_point_2d"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("paris refuges: %w", err)
	}
	seen := map[string]bool{}
	out := []refuge{}
	for _, r := range rows {
		if r.ID == "" || seen[r.ID] || r.Name == "" ||
			r.Geo == nil || !parisIndoorTypes[r.Type] {
			continue
		}
		seen[r.ID] = true
		addr := strings.Join(strings.Fields(r.Addr), " ")
		if r.Arr != "" {
			if addr != "" {
				addr += " · "
			}
			addr += r.Arr
		}
		if r.Paying == "Oui" {
			addr += " · entrée payante"
		}
		out = append(out, refuge{
			Lon: r.Geo.Lon, Lat: r.Geo.Lat, Name: r.Name,
			Addr: addr, Src: "paris",
		})
	}
	if len(out) == 0 {
		return nil, errors.New("paris refuges: no rows parsed")
	}
	return out, nil
}

// geojsonFC is the shared shape of the WFS GeoJSON feeds (Vienna,
// Lyon): point features carrying a per-city property bag.
type geojsonFC struct {
	Features []struct {
		Geometry struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties json.RawMessage `json:"properties"`
	} `json:"features"`
}

// inBounds guards against the classic WFS axis-order flip: asking a
// 1.1.0 server for EPSG:4326 is exactly where lon/lat swaps happen,
// and a swapped feed would silently pin every shelter in the wrong
// hemisphere. A city's refuges outside its own loose bounding box
// must fail loudly so the client serves stale truth instead.
func inBounds(lon, lat, lonMin, lonMax, latMin, latMax float64) bool {
	return lon >= lonMin && lon <= lonMax &&
		lat >= latMin && lat <= latMax
}

// parseWienRefuges reads the Coole Zonen WFS layer: free indoor cool
// rooms (20–24 °C), every feature belongs to the tier — no type
// filter needed. Traps: the server answers in Gauß-Krüger unless
// srsName=EPSG:4326 is in the URL, and WEBLINK1 carries a trailing
// newline in the live feed.
func parseWienRefuges(raw []byte) ([]refuge, error) {
	var fc geojsonFC
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("wien refuges: %w", err)
	}
	seen := map[int64]bool{}
	out := []refuge{}
	for _, f := range fc.Features {
		var p struct {
			ID    int64  `json:"OBJECTID"`
			Name  string `json:"BEZEICHNUNG"`
			Addr  string `json:"ADRESSE"`
			Hours string `json:"OEFFNUNGSZEIT"`
			Web   string `json:"WEBLINK1"`
		}
		if err := json.Unmarshal(f.Properties, &p); err != nil {
			continue
		}
		if f.Geometry.Type != "Point" ||
			len(f.Geometry.Coordinates) < 2 ||
			p.ID == 0 || seen[p.ID] || p.Name == "" {
			continue
		}
		lon, lat := f.Geometry.Coordinates[0], f.Geometry.Coordinates[1]
		if !inBounds(lon, lat, 16.0, 16.7, 48.0, 48.4) {
			return nil, fmt.Errorf(
				"wien refuges: %.2f,%.2f outside Vienna — axis flip "+
					"or wrong CRS upstream", lon, lat)
		}
		seen[p.ID] = true
		addr := strings.Join(strings.Fields(p.Addr), " ")
		if h := strings.TrimSpace(p.Hours); h != "" {
			if addr != "" {
				addr += " · "
			}
			addr += h
		}
		web := strings.TrimSpace(p.Web)
		if !strings.HasPrefix(web, "http") {
			web = ""
		}
		out = append(out, refuge{
			Lon: lon, Lat: lat, Name: p.Name, Addr: addr,
			Web: web, Src: "wien",
		})
	}
	if len(out) == 0 {
		return nil, errors.New("wien refuges: no rows parsed")
	}
	return out, nil
}

// Lyon's dataset is already curated as cooled public facilities, but
// a handful of outdoor sites ride along (parks, an open-air sports
// complex, a cemetery) and a third of the rows carry no `type` at
// all — those are churches, libraries and covered market halls,
// recognizable by `theme`. Same whitelist discipline as Paris: an
// unrecognized type or theme defaults out until a human reads it.
var lyonIndoorTypes = map[string]bool{
	"Bassin de natation":                       true,
	"Bibliothèque":                             true,
	"Centre social":                            true,
	"Eglise catholique":                        true,
	"Equipement pour personnes âgées":          true,
	"Hôtel de ville ; Mairie":                  true,
	"Musée":                                    true,
	"Médiathèque":                              true,
	"Résidence service":                        true,
	"Site d'activités aquatiques et nautiques": true,
}

var lyonIndoorThemes = map[string]bool{ // only for untyped rows
	"Equipement cultuel":            true,
	"Equipement culturel":           true,
	"Autre service à la population": true, // covered market halls, malls
}

// parseLyonRefuges reads the Grand Lyon WFS layer. Traps: `uid` is
// null on 80 of 90 live rows (only one commune fills it) — the row
// identity is `gid`; addresses end in a literal \r in the live feed;
// the `climatise` flag is false even for sites the comment calls
// cooled, so it must not be used as the filter — type/theme is.
func parseLyonRefuges(raw []byte) ([]refuge, error) {
	var fc geojsonFC
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("lyon refuges: %w", err)
	}
	seen := map[int64]bool{}
	out := []refuge{}
	for _, f := range fc.Features {
		var p struct {
			GID     int64  `json:"gid"`
			Theme   string `json:"theme"`
			Type    string `json:"type"`
			Name    string `json:"nom"`
			Addr    string `json:"adresse"`
			Commune string `json:"commune"`
			Web     string `json:"web"`
		}
		if err := json.Unmarshal(f.Properties, &p); err != nil {
			continue
		}
		indoor := lyonIndoorTypes[p.Type] ||
			(p.Type == "" && lyonIndoorThemes[p.Theme])
		if f.Geometry.Type != "Point" ||
			len(f.Geometry.Coordinates) < 2 ||
			p.GID == 0 || seen[p.GID] || p.Name == "" || !indoor {
			continue
		}
		lon, lat := f.Geometry.Coordinates[0], f.Geometry.Coordinates[1]
		if !inBounds(lon, lat, 4.4, 5.3, 45.4, 46.0) {
			return nil, fmt.Errorf(
				"lyon refuges: %.2f,%.2f outside Grand Lyon — axis "+
					"flip or wrong CRS upstream", lon, lat)
		}
		seen[p.GID] = true
		addr := strings.Join(strings.Fields(p.Addr), " ")
		if p.Commune != "" {
			if addr != "" {
				addr += " · "
			}
			addr += p.Commune
		}
		web := strings.TrimSpace(p.Web)
		if !strings.HasPrefix(web, "http") {
			web = ""
		}
		out = append(out, refuge{
			Lon: lon, Lat: lat, Name: p.Name, Addr: addr,
			Web: web, Src: "lyon",
		})
	}
	if len(out) == 0 {
		return nil, errors.New("lyon refuges: no rows parsed")
	}
	return out, nil
}

type refugeSourceView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Attribution string `json:"attribution"`
	OK          bool   `json:"ok"`
	Count       int    `json:"count"`
}

// handleRefuges: GET /api/refuges — every adapter's pins plus a
// per-source status line. Sources are reported even when they fail
// (OK: false), so the client can say "shelter data unavailable"
// instead of the false all-clear of an empty layer.
func (s *server) handleRefuges(w http.ResponseWriter, _ *http.Request) {
	views := make([]refugeSourceView, 0, len(refugeSources))
	all := []refuge{}
	for _, src := range refugeSources {
		list, err := s.refuges.get(src)
		if err != nil {
			log.Printf("refuges %s: %v", src.ID, err)
		}
		views = append(views, refugeSourceView{
			ID: src.ID, Name: src.Name, Attribution: src.Attribution,
			OK: err == nil, Count: len(list),
		})
		all = append(all, list...)
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, map[string]any{
		"sources": views,
		"refuges": all,
	})
}
