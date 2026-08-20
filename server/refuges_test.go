package main

import (
	"strings"
	"testing"
	"unicode/utf16"
)

// utf16le encodes a string the way Open Data BCN serves its CSVs:
// UTF-16LE with a leading BOM — and, faithfully to the live feed,
// every data line carries its own stray BOM too.
func utf16le(s string) []byte {
	u := utf16.Encode([]rune("\ufeff" + s))
	b := make([]byte, 0, len(u)*2)
	for _, v := range u {
		b = append(b, byte(v), byte(v>>8))
	}
	return b
}

// Fabricated fixture — shaped like the real feed, no real records.
const bcnFixture = `register_id,name,addresses_road_name,` +
	`addresses_start_street_number,addresses_district_name,` +
	`values_attribute_name,values_value,` +
	`geo_epgs_4326_lat,geo_epgs_4326_lon
` + "\ufeff" + `100001,Biblioteca de Prova,Carrer Imaginari,12,` +
	`Districte Zero,Web,http://example.org/prova,41.40,2.15
` + "\ufeff" + `100002,Parc Fictici,,,,,,41.41,2.16
` + "\ufeff" + `100001,Biblioteca de Prova,Carrer Imaginari,12,` +
	`Districte Zero,Web,http://example.org/prova,41.40,2.15
` + "\ufeff" + `100003,Sense Coordenades,,,,,,,
`

func TestParseBCNRefuges(t *testing.T) {
	got, err := parseBCNRefuges(utf16le(bcnFixture))
	if err != nil {
		t.Fatal(err)
	}
	// duplicate register_id and the coordinate-less row both drop
	if len(got) != 2 {
		t.Fatalf("want 2 refuges, got %d: %+v", len(got), got)
	}
	r := got[0]
	if r.Name != "Biblioteca de Prova" {
		t.Errorf("name = %q (BOM not stripped?)", r.Name)
	}
	if r.Addr != "Carrer Imaginari 12 · Districte Zero" {
		t.Errorf("addr = %q", r.Addr)
	}
	if r.Web != "http://example.org/prova" {
		t.Errorf("web = %q", r.Web)
	}
	if r.Lat != 41.40 || r.Lon != 2.15 {
		t.Errorf("coords = %v,%v", r.Lat, r.Lon)
	}
	if r.Src != "bcn" {
		t.Errorf("src = %q", r.Src)
	}
	if got[1].Addr != "" || got[1].Web != "" {
		t.Errorf("optional fields should stay empty: %+v", got[1])
	}
}

// Timetable fixtures — each the shape of a real feed variant, no
// real records. The feed serves &nbsp; as a literal entity and
// marks the applicable seasonal period with a `current` class.
const bcnTTSimple = `<table class="timetable-table">` +
	`<tr class="timetable-header"><th class="weekdays">Dies</th>` +
	`<th class="hours">Hores</th></tr>` +
	`<tr><td class="timetable-day" rowspan=1><div>De dilluns a ` +
	`divendres</div></td><td class="timetable-hour" rowspan=1>` +
	`<div>de 10.30&nbsp;h a 20.30&nbsp;h</div></td></tr>` +
	`<tr><td class="timetable-day" rowspan=1><div>Dissabte</div>` +
	`</td><td class="timetable-hour" rowspan=1><div>de ` +
	`09.30&nbsp;h a 13.30&nbsp;h i de 15.30&nbsp;h a ` +
	`20.00&nbsp;h</div></td></tr>` +
	`<tr><td class="timetable-day" rowspan=1><div>Diumenge i ` +
	`festius</div></td><td class="timetable-hour" rowspan=1>` +
	`<div>Tancat</div></td></tr></table>`

const bcnTTPeriods = `<table class="timetable-table">` +
	`<tr class="timetable-header"><th class="period">Període</th>` +
	`<th class="weekdays">Dies</th><th class="hours">Hores</th></tr>` +
	`<tr><td class="timetable-period" rowspan=1><div>De l'1 juny ` +
	`al 31 juliol</div></td><td class="timetable-day" rowspan=1>` +
	`<div>Dissabte</div></td><td class="timetable-hour" rowspan=1>` +
	`<div>de 10.30&nbsp;h a 13.00&nbsp;h</div></td></tr>` +
	`<tr><td class="timetable-period current" rowspan=2><div>De ` +
	`l'1 agost al 31 agost</div></td><td class="timetable-day" ` +
	`rowspan=2><div>Tots els dies</div></td>` +
	`<td class="timetable-hour" rowspan=1><div>de 08.00&nbsp;h a ` +
	`14.00&nbsp;h</div></td></tr>` +
	`<tr><td class="timetable-hour" rowspan=1><div>de ` +
	`16.00&nbsp;h a 20.00&nbsp;h</div></td></tr>` +
	`<tr><td class="timetable-period" rowspan=1><div>De l'1 ` +
	`setembre al 12 setembre</div></td><td class="timetable-day" ` +
	`rowspan=1><div>Dimarts 23 de juny</div></td>` +
	`<td class="timetable-hour" rowspan=1><div>de 10.30&nbsp;h a ` +
	`13.00&nbsp;h</div></td></tr></table>`

func TestParseBCNTimetableSimple(t *testing.T) {
	w := parseBCNTimetable(bcnTTSimple)
	if w == nil {
		t.Fatal("want a week from the simple table")
	}
	for d := 0; d < 5; d++ {
		if w[d] != "10:30-20:30" {
			t.Errorf("day %d = %q", d, w[d])
		}
	}
	if w[5] != "09:30-13:30, 15:30-20:00" {
		t.Errorf("split saturday = %q", w[5])
	}
	if w[6] != "tancat" {
		t.Errorf("closed sunday = %q", w[6])
	}
}

func TestParseBCNTimetablePeriods(t *testing.T) {
	w := parseBCNTimetable(bcnTTPeriods)
	if w == nil {
		t.Fatal("want a week from the current period")
	}
	// only the `current` period's rows count; its day cell spans
	// two hour rows (rowspan) — the feed's row-by-row spelling of
	// a split day, so the two ranges accumulate; the dated
	// exception row after it drops
	for d := 0; d < 7; d++ {
		if w[d] != "08:00-14:00, 16:00-20:00" {
			t.Errorf("day %d = %q", d, w[d])
		}
	}
}

func TestParseBCNTimetableNoClaim(t *testing.T) {
	// seasonal table, no period marked current -> no claim
	noCurrent := strings.Replace(bcnTTPeriods,
		"timetable-period current", "timetable-period", 1)
	if w := parseBCNTimetable(noCurrent); w != nil {
		t.Errorf("out-of-season table must claim nothing: %+v", w)
	}
	if w := parseBCNTimetable(""); w != nil {
		t.Errorf("empty cell must claim nothing: %+v", w)
	}
	// a dated exception row alone must not produce a week
	dated := `<table class="timetable-table"><tr><td ` +
		`class="timetable-day" rowspan=1><div>Dimarts 23 de juny` +
		`</div></td><td class="timetable-hour" rowspan=1><div>de ` +
		`10.00&nbsp;h a 14.00&nbsp;h</div></td></tr></table>`
	if w := parseBCNTimetable(dated); w != nil {
		t.Errorf("dated exception must claim nothing: %+v", w)
	}
}

func TestParseBCNTimetableAllClosed(t *testing.T) {
	// "Tots els dies tancat" lives in the day cell, hour cell empty
	closed := `<table class="timetable-table"><tr><td ` +
		`class="timetable-day" rowspan=1><div>Tots els dies tancat` +
		`</div></td><td class="timetable-hour" rowspan=1><div></div>` +
		`</td></tr></table>`
	w := parseBCNTimetable(closed)
	if w == nil {
		t.Fatal("want an all-closed week")
	}
	for d := 0; d < 7; d++ {
		if w[d] != "tancat" {
			t.Errorf("day %d = %q", d, w[d])
		}
	}
}

func TestParseBCNRefugesTimetableColumn(t *testing.T) {
	// the timetable column is optional — pins survive without it
	// (bcnFixture has none) and fill Week when it is present
	csv := "register_id,name,timetable," +
		"geo_epgs_4326_lat,geo_epgs_4326_lon\n" +
		`200001,Casal de Prova,"` +
		strings.ReplaceAll(bcnTTSimple, `"`, `""`) +
		`",41.40,2.15` + "\n"
	got, err := parseBCNRefuges([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Week == nil {
		t.Fatalf("want 1 refuge with a week: %+v", got)
	}
	if got[0].Week[0] != "10:30-20:30" {
		t.Errorf("monday = %q", got[0].Week[0])
	}
}

func TestParseBCNRefugesUTF8Fallback(t *testing.T) {
	// same content served without UTF-16 must still parse
	got, err := parseBCNRefuges([]byte(bcnFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 refuges, got %d", len(got))
	}
}

func TestParseBCNRefugesMissingColumn(t *testing.T) {
	broken := strings.Replace(bcnFixture,
		"geo_epgs_4326_lat", "renamed_by_the_city", 1)
	if _, err := parseBCNRefuges([]byte(broken)); err == nil {
		t.Fatal("want an error when a needed column vanishes")
	}
}

// Fabricated fixture — shaped like the Paris Data JSON export, no
// real records. One keeper of each interesting kind, plus every
// drop reason live in the real feed: an outdoor type, a duplicated
// identifiant, a missing geo_point_2d, and a type the city hasn't
// invented yet (whitelist must default it out).
const parisFixture = `[
  {"identifiant": "BI01", "nom": "Bibliothèque Imaginaire",
   "type": "Bibliothèque", "payant": "Non",
   "adresse": "3  RUE  INVENTEE", "arrondissement": "75099",
   "geo_point_2d": {"lon": 2.35, "lat": 48.85}},
  {"identifiant": "PI01", "nom": "Piscine Fictive",
   "type": "Piscine", "payant": "Oui",
   "adresse": "9, rue Imaginaire", "arrondissement": "75098",
   "horaires_lundi": "10h00 - 20h00", "horaires_mardi": "Fermé",
   "horaires_mercredi": "-",
   "geo_point_2d": {"lon": 2.36, "lat": 48.86}},
  {"identifiant": "BR01", "nom": "Brumisateur Quelconque",
   "type": "Brumisateur", "payant": "Non",
   "adresse": "PLACE FICTIVE", "arrondissement": "75097",
   "geo_point_2d": {"lon": 2.37, "lat": 48.87}},
  {"identifiant": "BI01", "nom": "Bibliothèque Imaginaire",
   "type": "Bibliothèque", "payant": "Non",
   "adresse": "3  RUE  INVENTEE", "arrondissement": "75099",
   "geo_point_2d": {"lon": 2.35, "lat": 48.85}},
  {"identifiant": "MU01", "nom": "Musée Sans Position",
   "type": "Musée", "payant": "Non",
   "adresse": "1 RUE PERDUE", "arrondissement": "75096",
   "geo_point_2d": null},
  {"identifiant": "XX01", "nom": "Type Futur Inconnu",
   "type": "Grotte municipale", "payant": "Non",
   "adresse": "2 RUE NOUVELLE", "arrondissement": "75095",
   "geo_point_2d": {"lon": 2.38, "lat": 48.88}}
]`

func TestParseParisRefuges(t *testing.T) {
	got, err := parseParisRefuges([]byte(parisFixture))
	if err != nil {
		t.Fatal(err)
	}
	// outdoor type, duplicate id, missing geo, unknown type all drop
	if len(got) != 2 {
		t.Fatalf("want 2 refuges, got %d: %+v", len(got), got)
	}
	r := got[0]
	if r.Name != "Bibliothèque Imaginaire" {
		t.Errorf("name = %q", r.Name)
	}
	if r.Addr != "3 RUE INVENTEE · 75099" {
		t.Errorf("addr = %q (doubled spaces not collapsed?)", r.Addr)
	}
	if r.Lat != 48.85 || r.Lon != 2.35 {
		t.Errorf("coords = %v,%v", r.Lat, r.Lon)
	}
	if r.Src != "paris" {
		t.Errorf("src = %q", r.Src)
	}
	if r.Web != "" {
		t.Errorf("paris pins have no per-site link, got %q", r.Web)
	}
	if got[1].Addr != "9, rue Imaginaire · 75098 · entrée payante" {
		t.Errorf("paid pool addr = %q", got[1].Addr)
	}
	// the library has no hour columns at all -> no week claimed;
	// the pool's "-" day records "no information", not closed
	if got[0].Week != nil {
		t.Errorf("hourless refuge must carry no week: %+v", got[0].Week)
	}
	w := got[1].Week
	if w == nil || w[0] != "10h00 - 20h00" || w[1] != "Fermé" ||
		w[2] != "" || w[6] != "" {
		t.Errorf("pool week = %+v", w)
	}
}

func TestParseParisRefugesGarbage(t *testing.T) {
	if _, err := parseParisRefuges([]byte(`{"not":"a list"}`)); err == nil {
		t.Fatal("want an error on a non-array export")
	}
	if _, err := parseParisRefuges([]byte(`[]`)); err == nil {
		t.Fatal("want an error on an empty export")
	}
}

// Fabricated fixture — shaped like the Coole Zonen WFS GeoJSON, no
// real records. A keeper with hours and a newline-tailed weblink
// (faithful to the live feed), a duplicate OBJECTID, and a feature
// with no geometry coordinates.
const wienFixture = `{"type":"FeatureCollection","features":[
 {"type":"Feature","geometry":{"type":"Point",
   "coordinates":[16.35,48.20]},
  "properties":{"OBJECTID":1,"BEZEICHNUNG":"Erfundene Bibliothek",
   "ADRESSE":"9., Erfundene Gasse 1",
   "OEFFNUNGSZEIT":"MO-FR, 09-17 Uhr",
   "WEBLINK1":"https://example.org/kuehl\n"}},
 {"type":"Feature","geometry":{"type":"Point",
   "coordinates":[16.36,48.21]},
  "properties":{"OBJECTID":1,"BEZEICHNUNG":"Doppelt",
   "ADRESSE":"1., Anderswo 2","OEFFNUNGSZEIT":null,"WEBLINK1":null}},
 {"type":"Feature","geometry":{"type":"Point","coordinates":[]},
  "properties":{"OBJECTID":2,"BEZEICHNUNG":"Ohne Ort",
   "ADRESSE":"2., Nirgendwo 3","OEFFNUNGSZEIT":null,"WEBLINK1":null}}
]}`

func TestParseWienRefuges(t *testing.T) {
	got, err := parseWienRefuges([]byte(wienFixture))
	if err != nil {
		t.Fatal(err)
	}
	// duplicate OBJECTID and the geometry-less feature both drop
	if len(got) != 1 {
		t.Fatalf("want 1 refuge, got %d: %+v", len(got), got)
	}
	r := got[0]
	if r.Name != "Erfundene Bibliothek" || r.Src != "wien" {
		t.Errorf("name/src = %q/%q", r.Name, r.Src)
	}
	if r.Addr != "9., Erfundene Gasse 1" {
		t.Errorf("addr = %q", r.Addr)
	}
	if r.Hours != "MO-FR, 09-17 Uhr" {
		t.Errorf("hours = %q", r.Hours)
	}
	if r.Web != "https://example.org/kuehl" {
		t.Errorf("web = %q (trailing newline not trimmed?)", r.Web)
	}
}

func TestParseWienRefugesAxisFlip(t *testing.T) {
	flipped := strings.Replace(wienFixture,
		"[16.35,48.20]", "[48.20,16.35]", 1)
	if _, err := parseWienRefuges([]byte(flipped)); err == nil {
		t.Fatal("want a loud error when lon/lat arrive swapped")
	}
}

// Fabricated fixture — shaped like the Grand Lyon WFS GeoJSON, no
// real records. Keepers: a typed indoor site with the live feed's
// trailing \r in the address, and an untyped row whose theme marks
// it indoor (uid null throughout, faithful to the live feed — gid
// is the identity). Drops: an outdoor park, an untyped row with an
// unknown theme, and a duplicate gid.
const lyonFixture = `{"type":"FeatureCollection","features":[
 {"type":"Feature","geometry":{"type":"Point",
   "coordinates":[4.85,45.76]},
  "properties":{"gid":1,"uid":null,"theme":"Equipement culturel",
   "type":"Musée","nom":"Musée Imaginaire",
   "adresse":"1 Rue Inventée\r ","commune":"Lyon",
   "web":"https://example.org/musee"}},
 {"type":"Feature","geometry":{"type":"Point",
   "coordinates":[4.86,45.77]},
  "properties":{"gid":2,"uid":null,"theme":"Equipement cultuel",
   "type":null,"nom":"Basilique Fictive",
   "adresse":"2 Place Inventée","commune":"Lyon","web":""}},
 {"type":"Feature","geometry":{"type":"Point",
   "coordinates":[4.87,45.78]},
  "properties":{"gid":3,"uid":null,"theme":"Parcs et jardins",
   "type":"Parc public","nom":"Parc Fictif",
   "adresse":"3 Allée Inventée","commune":"Lyon","web":""}},
 {"type":"Feature","geometry":{"type":"Point",
   "coordinates":[4.88,45.79]},
  "properties":{"gid":4,"uid":null,"theme":"Thème Futur Inconnu",
   "type":null,"nom":"Objet Mystère",
   "adresse":"4 Voie Inventée","commune":"Lyon","web":""}},
 {"type":"Feature","geometry":{"type":"Point",
   "coordinates":[4.85,45.76]},
  "properties":{"gid":1,"uid":null,"theme":"Equipement culturel",
   "type":"Musée","nom":"Musée Imaginaire",
   "adresse":"1 Rue Inventée","commune":"Lyon","web":""}}
]}`

func TestParseLyonRefuges(t *testing.T) {
	got, err := parseLyonRefuges([]byte(lyonFixture))
	if err != nil {
		t.Fatal(err)
	}
	// the park, the unknown theme and the duplicate gid all drop
	if len(got) != 2 {
		t.Fatalf("want 2 refuges, got %d: %+v", len(got), got)
	}
	r := got[0]
	if r.Name != "Musée Imaginaire" || r.Src != "lyon" {
		t.Errorf("name/src = %q/%q", r.Name, r.Src)
	}
	if r.Addr != "1 Rue Inventée · Lyon" {
		t.Errorf("addr = %q (stray \\r not collapsed?)", r.Addr)
	}
	if r.Web != "https://example.org/musee" {
		t.Errorf("web = %q", r.Web)
	}
	if got[1].Name != "Basilique Fictive" {
		t.Errorf("untyped indoor-theme row should survive: %+v", got[1])
	}
}

func TestParseLyonRefugesAxisFlip(t *testing.T) {
	flipped := strings.Replace(lyonFixture,
		"[4.85,45.76]", "[45.76,4.85]", 1)
	if _, err := parseLyonRefuges([]byte(flipped)); err == nil {
		t.Fatal("want a loud error when lon/lat arrive swapped")
	}
}

// Fabricated fixture, shaped like the live Socrata rows including the
// feed's coordinate mangling: stripped decimal points, longitudes
// missing their leading zero, municipality codes missing the zero of
// "08" and sometimes the check digit.
const catFixture = `[
 {"nom":"Biblioteca de Prova","indicador_refugi":"170792|BIB_PROVA",
  "municipi":"Vilaprova","codi_municipi":"170792",
  "latitud":"41.97","longitud":"2.82",
  "adreca":"C. Imaginari,  12","informacio_web":"https://example.org/b",
  "tipologia":"Biblioteca"},
 {"nom":"Casal Sense Punt","indicador_refugi":"80018|CASAL",
  "municipi":"Vilapunt","codi_municipi":"80018",
  "latitud":"41521503","longitud":"1901389",
  "adreca":"","informacio_web":"www.example.org/casal"},
 {"nom":"Piscina Zero Perdut","indicador_refugi":"250046|PISCINA",
  "municipi":"Vilazero","codi_municipi":"250046",
  "latitud":"4162300","longitud":"621424",
  "informacio_web":"De dilluns a divendres de 9 a 14 h"},
 {"nom":"Parc Codi Curt","indicador_refugi":"8180|PARC",
  "municipi":"Vilacurta","codi_municipi":"8180",
  "latitud":"41.504","longitud":"2.155"},
 {"nom":"Refugi Repetit","indicador_refugi":"8180|PARC",
  "municipi":"Vilacurta","codi_municipi":"8180",
  "latitud":"41.504","longitud":"2.155"},
 {"nom":"Bib. de la Ciutat","indicador_refugi":"080193|BIB_BCN",
  "municipi":"Barcelona","codi_municipi":"080193",
  "latitud":"41.40","longitud":"2.15"},
 {"nom":"Casal Mal Geocodificat","indicador_refugi":"81878|MAL",
  "municipi":"Vilalluny","codi_municipi":"81878",
  "latitud":"41.6564146","longitud":"0.7986808"},
 {"nom":"Sala Codi Estrany","indicador_refugi":"99999|SALA",
  "municipi":"Vilaestranya","codi_municipi":"99999",
  "latitud":"41.50","longitud":"2.10"}
]`

func TestParseCatRefuges(t *testing.T) {
	got, err := parseCatRefuges([]byte(catFixture))
	if err != nil {
		t.Fatal(err)
	}
	// Barcelona, the duplicate id, the wrong-province longitude and
	// the unknown municipality code all drop
	if len(got) != 4 {
		t.Fatalf("want 4 refuges, got %d: %+v", len(got), got)
	}
	r := got[0]
	if r.Name != "Biblioteca de Prova" || r.Src != "cat" {
		t.Errorf("name/src = %q/%q", r.Name, r.Src)
	}
	if r.Addr != "C. Imaginari, 12 · Vilaprova" {
		t.Errorf("addr = %q (doubled spaces not collapsed?)", r.Addr)
	}
	if r.Lat != 41.97 || r.Lon != 2.82 {
		t.Errorf("clean coords = %v,%v", r.Lat, r.Lon)
	}
	// stripped decimal points re-pointed
	if got[1].Lat != 41.521503 || got[1].Lon != 1.901389 {
		t.Errorf("re-pointed coords = %v,%v", got[1].Lat, got[1].Lon)
	}
	if got[1].Web != "https://www.example.org/casal" {
		t.Errorf("bare www. host should become a link: %q", got[1].Web)
	}
	if got[1].Addr != "Vilapunt" {
		t.Errorf("empty adreca should leave municipality: %q", got[1].Addr)
	}
	// longitude that lost its leading zero lands below 1°E, and the
	// free text riding in informacio_web never becomes a link
	if got[2].Lat != 41.623 || got[2].Lon != 0.621424 {
		t.Errorf("zero-lost coords = %v,%v", got[2].Lat, got[2].Lon)
	}
	if got[2].Web != "" {
		t.Errorf("free-text web should stay empty: %q", got[2].Web)
	}
	// a 4-digit municipality code still names Barcelona province
	if got[3].Name != "Parc Codi Curt" {
		t.Errorf("short-code row should survive: %+v", got[3])
	}
}

func TestParseCatRefugesGarbage(t *testing.T) {
	if _, err := parseCatRefuges([]byte("not json")); err == nil {
		t.Fatal("want an error on garbage")
	}
	if _, err := parseCatRefuges([]byte("[]")); err == nil {
		t.Fatal("want an error on an empty feed, not a silent empty layer")
	}
}
