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
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
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
	// Hours is a display string, as published ("Juni-Aug, MO-FR,
	// 09-17 Uhr"). Week is structured per-day hours, Monday first,
	// "" where unknown — only sources whose published hours reduce
	// to unambiguous per-day entries fill it (Paris's day columns,
	// Barcelona's timetable tables), and only Week is honest enough
	// to compute an open-now claim from. A guessed "open" sends a
	// body to a locked door; freeform text stays display-only.
	Hours string     `json:"hours,omitempty"`
	Week  *[7]string `json:"week,omitempty"`
	Src   string     `json:"src"`
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

// Refugis climàtics dels municipis de Catalunya — Generalitat de
// Catalunya (Departament d'Interior), CC BY 4.0. Every municipality's
// declared refuges in one dataset: the first adapter that is a region,
// not a city. Socrata API; $limit lifts the default 1000-row page well
// above the ~2600 live rows, $order pins a stable read order.
const catRefugesJSON = "https://analisi.transparenciacatalunya.cat" +
	"/resource/9gu7-iwci.json?$limit=10000&$order=indicador_refugi"

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
}, {
	ID:          "cat",
	Name:        "Catalunya — Refugis climàtics municipals",
	Attribution: "Generalitat de Catalunya, Dades Obertes (CC BY 4.0)",
	URL:         catRefugesJSON,
	parse:       parseCatRefuges,
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
			Week: parseBCNTimetable(get(row, "timetable")),
		})
	}
	if len(out) == 0 {
		return nil, errors.New("bcn refuges: no rows parsed")
	}
	return out, nil
}

// The BCN CSV's `timetable` column is an HTML table meant for the
// city's own pages (the Ajuntament confirmed this shape, 2026-08).
// Its cells are regular enough to reduce to per-day hours: day
// phrases are a small Catalan vocabulary, hour cells read
// "de 10.00 h a 14.00 h" with " i " joining split days, "Tancat"
// marks closure. Seasonal tables carry one row set per period and
// the feed marks the period that applies now with a `current`
// class — that mark is upstream's, regenerated on their weekly
// cadence, so honoring it at parse time matches the data's own
// freshness.
var (
	bcnTimeTR   = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	bcnTimeTD   = regexp.MustCompile(`(?s)<td class="timetable-(\w+)([^"]*)"(?:\s+rowspan=(\d+))?[^>]*>(.*?)</td>`)
	bcnTimeTag  = regexp.MustCompile(`<[^>]+>`)
	bcnHourSpan = regexp.MustCompile(`(\d{1,2})(?:[.:](\d{2}))?\s*h?\s*a\s*(\d{1,2})(?:[.:](\d{2}))?`)
	bcnAllDays  = regexp.MustCompile(`tots?\s+els?\s+dies|cada\s+dia`)
	bcnDayRange = regexp.MustCompile(`de\s+(\p{L}+)\s+a\s+(\p{L}+)`)
	bcnDayWord  = regexp.MustCompile(`\p{L}+`)
	bcnAnyDigit = regexp.MustCompile(`\d`)
)

var bcnDayIdx = map[string]int{
	"dilluns": 0, "dimarts": 1, "dimecres": 2, "dijous": 3,
	"divendres": 4, "dissabte": 5, "dissabtes": 5,
	"diumenge": 6, "diumenges": 6,
}

// bcnCellText flattens one cell to plain text: tags out, entities
// decoded (&nbsp; arrives literally in the feed), spaces collapsed.
func bcnCellText(cell string) string {
	s := html.UnescapeString(bcnTimeTag.ReplaceAllString(cell, " "))
	return strings.Join(strings.Fields(s), " ")
}

// bcnDays maps a day phrase to weekday indexes (Monday 0). A phrase
// with a digit is a dated exception ("Dimarts 23 de juny") — those
// single-day overrides are skipped rather than smeared over every
// week of the season. "festius" has no weekday to land on and is
// ignored. closed reports a "tancat" living in the day cell itself
// ("Tots els dies tancat"), where the hour cell may be empty.
func bcnDays(phrase string) (days []int, closed bool) {
	p := strings.ToLower(phrase)
	if bcnAnyDigit.MatchString(p) {
		return nil, false
	}
	closed = strings.Contains(p, "tancat") ||
		strings.Contains(p, "tancada")
	if bcnAllDays.MatchString(p) {
		return []int{0, 1, 2, 3, 4, 5, 6}, closed
	}
	if m := bcnDayRange.FindStringSubmatch(p); m != nil {
		a, okA := bcnDayIdx[m[1]]
		b, okB := bcnDayIdx[m[2]]
		if okA && okB && a <= b {
			for d := a; d <= b; d++ {
				days = append(days, d)
			}
			return days, closed
		}
	}
	seen := [7]bool{}
	for _, w := range bcnDayWord.FindAllString(p, -1) {
		if d, ok := bcnDayIdx[w]; ok && !seen[d] {
			seen[d] = true
			days = append(days, d)
		}
	}
	return days, closed
}

// bcnHours normalizes one hour cell to "HH:MM-HH:MM" ranges joined
// by ", " — the shape the client computes open-now from. "" means
// the cell said nothing usable, which must stay "unknown", never
// "closed".
func bcnHours(phrase string) string {
	p := strings.ToLower(phrase)
	if strings.Contains(p, "tancat") || strings.Contains(p, "tancada") {
		return "tancat"
	}
	spans := []string{}
	for _, m := range bcnHourSpan.FindAllStringSubmatch(p, -1) {
		fh, _ := strconv.Atoi(m[1])
		th, _ := strconv.Atoi(m[3])
		fm, tm := m[2], m[4]
		if fm == "" {
			fm = "00"
		}
		if tm == "" {
			tm = "00"
		}
		if fh > 24 || th > 24 {
			continue
		}
		spans = append(spans,
			fmt.Sprintf("%02d:%s-%02d:%s", fh, fm, th, tm))
	}
	return strings.Join(spans, ", ")
}

// parseBCNTimetable reduces one timetable HTML table to a week, or
// nil when nothing unambiguous survives. Seasonal tables without a
// `current` mark (their periods are all out of season today) yield
// nil — no claim beats a wrong one. rowspan carries day and period
// cells down across their following rows, faithful to the feed.
func parseBCNTimetable(tbl string) *[7]string {
	if !strings.Contains(tbl, "timetable-") {
		return nil
	}
	hasPeriod := strings.Contains(tbl, "timetable-period")
	if hasPeriod && !strings.Contains(tbl, "timetable-period current") {
		return nil
	}
	type carry struct {
		left    int
		text    string
		current bool
	}
	carried := map[string]*carry{}
	var week [7]string
	got := false
	for _, trm := range bcnTimeTR.FindAllStringSubmatch(tbl, -1) {
		tr := trm[1]
		if strings.Contains(tr, "timetable-header") {
			continue
		}
		cells := map[string]string{}
		current := map[string]bool{}
		dayCarried := false
		for _, m := range bcnTimeTD.FindAllStringSubmatch(tr, -1) {
			cls, extra, body := m[1], m[2], bcnCellText(m[4])
			cells[cls] = body
			current[cls] = strings.Contains(extra, "current")
			if n, err := strconv.Atoi(m[3]); err == nil && n > 1 {
				carried[cls] = &carry{
					left: n - 1, text: body,
					current: strings.Contains(extra, "current"),
				}
			}
		}
		for _, cls := range []string{"period", "day", "hour"} {
			c := carried[cls]
			if _, here := cells[cls]; !here && c != nil && c.left > 0 {
				cells[cls] = c.text
				current[cls] = c.current
				c.left--
				if cls == "day" {
					dayCarried = true
				}
			}
		}
		if hasPeriod && !current["period"] {
			continue
		}
		days, closed := bcnDays(cells["day"])
		if len(days) == 0 {
			continue
		}
		h := "tancat"
		if !closed {
			h = bcnHours(cells["hour"])
			if h == "" {
				continue
			}
		}
		for _, d := range days {
			// A day cell spanning several hour rows is the feed's
			// row-by-row way of writing a split day — those ranges
			// accumulate. A fresh day cell naming an already-set
			// day is a refinement ("Dissabte" after "De dilluns a
			// dissabte") — that one replaces.
			if dayCarried && week[d] != "" &&
				week[d] != "tancat" && h != "tancat" {
				week[d] += ", " + h
			} else {
				week[d] = h
			}
		}
		got = true
	}
	if !got {
		return nil
	}
	return &week
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
		Mon    string `json:"horaires_lundi"`
		Tue    string `json:"horaires_mardi"`
		Wed    string `json:"horaires_mercredi"`
		Thu    string `json:"horaires_jeudi"`
		Fri    string `json:"horaires_vendredi"`
		Sat    string `json:"horaires_samedi"`
		Sun    string `json:"horaires_dimanche"`
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
		// Per-day columns, Monday first. "-" is the feed's "no
		// information" — recorded as "", never as closed.
		var week *[7]string
		days := [7]string{r.Mon, r.Tue, r.Wed, r.Thu, r.Fri,
			r.Sat, r.Sun}
		for i, d := range days {
			d = strings.TrimSpace(d)
			if d == "-" {
				d = ""
			}
			days[i] = d
			if d != "" {
				week = &days
			}
		}
		out = append(out, refuge{
			Lon: r.Geo.Lon, Lat: r.Geo.Lat, Name: r.Name,
			Addr: addr, Week: week, Src: "paris",
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
		web := strings.TrimSpace(p.Web)
		if !strings.HasPrefix(web, "http") {
			web = ""
		}
		out = append(out, refuge{
			Lon: lon, Lat: lat, Name: p.Name,
			Addr:  strings.Join(strings.Fields(p.Addr), " "),
			Hours: strings.TrimSpace(p.Hours),
			Web:   web, Src: "wien",
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
			Hours   string `json:"openinghours"`
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
		// "NR" is the feed's "not reported" — not hours
		hours := strings.TrimSpace(p.Hours)
		if hours == "NR" {
			hours = ""
		}
		out = append(out, refuge{
			Lon: lon, Lat: lat, Name: p.Name, Addr: addr,
			Hours: hours, Web: web, Src: "lyon",
		})
	}
	if len(out) == 0 {
		return nil, errors.New("lyon refuges: no rows parsed")
	}
	return out, nil
}

// Catalunya's register is declared data: each row exists because a
// municipality (or the metropolitan area) filed that place as a
// climate refuge, so — like Barcelona's own network and unlike the
// Paris/Lyon feeds — every row belongs to the tier and no type
// whitelist applies. What the feed lacks is hygiene: it aggregates
// hand-entered municipal submissions, and roughly half the
// coordinates arrive with the decimal point stripped ("41521503"
// for 41.521503) at whatever precision the town hall typed. The
// same disease eats leading zeros — longitudes below 1°E lose
// their "0." entirely, municipality codes lose the zero of "08"
// and sometimes their check digit too. Recovery below; a row that
// stays ambiguous is dropped and counted, because a missing pin
// beats a misplaced one.

// All of Catalunya lies in 40.4–43.0°N, and latitudes always keep
// two integer digits (40–42), so a stripped latitude re-points
// unambiguously. Longitudes need the per-province windows.
const (
	catLatMin = 40.4
	catLatMax = 43.0
)

// catProvLon: each province's longitude window, loose enough to hold
// every municipality in it, tight enough that at most one of a
// mangled longitude's two candidate readings fits (they differ by
// ~10×). Verified over the full live feed 2026-08-20: 0 ambiguous,
// 3 dropped (rows whose clean-looking longitude sits in the wrong
// province — real upstream geocoding errors the window catches).
var catProvLon = map[string][2]float64{
	"08": {1.30, 2.90}, // Barcelona
	"17": {1.60, 3.35}, // Girona
	"25": {0.30, 1.90}, // Lleida
	"43": {0.20, 1.70}, // Tarragona
}

// catProvince: the INE municipality code's first two digits name the
// province, but the feed's codes lose leading zeros and sometimes
// the trailing check digit ("080193" arrives as "80193", even
// "8180"). Only Barcelona's "08" has a zero to lose, so any code
// leading with an 8 is Barcelona province.
func catProvince(code string) string {
	for _, p := range []string{"17", "25", "43"} {
		if strings.HasPrefix(code, p) {
			return p
		}
	}
	if strings.HasPrefix(code, "08") || strings.HasPrefix(code, "8") {
		return "08"
	}
	return ""
}

// catLat accepts a sane decimal in range, or re-points a stripped
// digit run after its two integer digits.
func catLat(s string) (float64, bool) {
	if !strings.Contains(s, ".") && len(s) >= 3 {
		s = s[:2] + "." + s[2:]
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil && v >= catLatMin && v <= catLatMax
}

// catLon re-points a stripped longitude. Two readings are possible —
// the point went after the first digit ("1901389" → 1.901389) or a
// leading zero was lost too ("621424" → 0.621424) — and the province
// window admits at most one. Neither fitting reads as unrecoverable,
// and the n==1 guard stays even though both-fit is impossible by
// window construction.
func catLon(s string, win [2]float64) (float64, bool) {
	if strings.Contains(s, ".") {
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil && v >= win[0] && v <= win[1]
	}
	if len(s) < 2 {
		return 0, false
	}
	got, n := 0.0, 0
	for _, c := range []string{s[:1] + "." + s[1:], "0." + s} {
		if v, err := strconv.ParseFloat(c, 64); err == nil &&
			v >= win[0] && v <= win[1] {
			got, n = v, n+1
		}
	}
	return got, n == 1
}

// parseCatRefuges reads the Socrata JSON rows. Beyond the coordinate
// recovery above: Barcelona files its network here too (546 rows),
// but the city's own dataset is adapter #1 with richer data
// (timetables → open-now), so the region adapter yields to it rather
// than double-pin every shelter; `indicador_refugi` is the identity
// (one metropolitan park spanning two municipalities appears twice);
// `informacio_web` carries URLs, bare "www." hosts, and free text
// (opening hours, phone numbers) in one column — only link-shaped
// values become links, and the free text is NOT surfaced as hours
// because nothing marks which rows are hours at all.
func parseCatRefuges(raw []byte) ([]refuge, error) {
	var rows []struct {
		ID   string `json:"indicador_refugi"`
		Name string `json:"nom"`
		Mun  string `json:"municipi"`
		Code string `json:"codi_municipi"`
		Lat  string `json:"latitud"`
		Lon  string `json:"longitud"`
		Addr string `json:"adreca"`
		Web  string `json:"informacio_web"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("cat refuges: %w", err)
	}
	seen := map[string]bool{}
	out := []refuge{}
	dropped := 0
	for _, r := range rows {
		if r.Mun == "Barcelona" {
			continue // the city's own adapter owns these pins
		}
		if r.ID == "" || seen[r.ID] || r.Name == "" {
			continue
		}
		win, okProv := catProvLon[catProvince(r.Code)]
		if !okProv {
			dropped++
			continue
		}
		lat, okLat := catLat(r.Lat)
		lon, okLon := catLon(r.Lon, win)
		if !okLat || !okLon {
			dropped++
			continue
		}
		seen[r.ID] = true
		addr := strings.Join(strings.Fields(r.Addr), " ")
		if r.Mun != "" {
			if addr != "" {
				addr += " · "
			}
			addr += r.Mun
		}
		web := strings.TrimSpace(r.Web)
		switch {
		case strings.HasPrefix(web, "http"):
		case strings.HasPrefix(strings.ToLower(web), "www."):
			web = "https://" + web
		default:
			web = ""
		}
		out = append(out, refuge{
			Lon: lon, Lat: lat, Name: r.Name, Addr: addr,
			Web: web, Src: "cat",
		})
	}
	if len(out) == 0 {
		return nil, errors.New("cat refuges: no rows parsed")
	}
	if dropped > 0 {
		log.Printf("cat refuges: %d rows dropped (unrecoverable coordinates)",
			dropped)
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
