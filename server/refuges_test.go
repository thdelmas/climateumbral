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
