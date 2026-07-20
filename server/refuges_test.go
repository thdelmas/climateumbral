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
