package main

import "testing"

// Fabricated fixture — shaped like an Overpass out-center response,
// no real places. A named a/c café (node), a mall (way, coords under
// center), an a/c office (walk-in kind missing -> drops), a nameless
// supermarket (drops), and the same café twice (ways often twin
// their entrance nodes -> dedup).
const overpassFixture = `{"elements":[
 {"type":"node","lat":48.85,"lon":2.35,
  "tags":{"name":"Café Fictif","amenity":"cafe",
   "air_conditioning":"yes","opening_hours":"Mo-Su 08:00-22:00"}},
 {"type":"way","center":{"lat":48.86,"lon":2.36},
  "tags":{"name":"Centre Commercial Imaginaire","shop":"mall"}},
 {"type":"node","lat":48.87,"lon":2.37,
  "tags":{"name":"Bureau Privé","office":"company",
   "air_conditioning":"yes"}},
 {"type":"node","lat":48.88,"lon":2.38,
  "tags":{"shop":"supermarket","air_conditioning":"yes"}},
 {"type":"node","lat":48.85,"lon":2.35,
  "tags":{"name":"Café Fictif","amenity":"cafe",
   "air_conditioning":"yes"}}
]}`

func TestParseOverpassCoolPlaces(t *testing.T) {
	got, err := parseOverpassCoolPlaces([]byte(overpassFixture))
	if err != nil {
		t.Fatal(err)
	}
	// office kind, nameless row and the duplicate all drop
	if len(got) != 2 {
		t.Fatalf("want 2 places, got %d: %+v", len(got), got)
	}
	if got[0].Name != "Café Fictif" || got[0].Kind != "cafe" ||
		got[0].Hours != "Mo-Su 08:00-22:00" {
		t.Errorf("cafe = %+v", got[0])
	}
	if got[1].Kind != "mall" || got[1].Lat != 48.86 {
		t.Errorf("mall (center coords) = %+v", got[1])
	}
}

func TestParseOverpassCoolPlacesGarbage(t *testing.T) {
	if _, err := parseOverpassCoolPlaces([]byte(`<html>504`)); err == nil {
		t.Fatal("want an error on a non-JSON body")
	}
	got, err := parseOverpassCoolPlaces([]byte(`{"elements":[]}`))
	if err != nil || len(got) != 0 {
		t.Fatalf("empty elements is a real answer: %v %v", got, err)
	}
}
