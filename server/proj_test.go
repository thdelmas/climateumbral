package main

import (
	"math"
	"testing"
)

// Ground truth from the EEA service itself: projecting the Barcelona
// demo bbox (2.14, 41.375, 2.19, 41.41) to EPSG:3035 returned the
// envelope (3662101.58, 2064166.36, 3666645.71, 2068710.50). The
// eastings are comparable corner-to-corner (sub-meter); the service
// pads northings to fit the requested pixel aspect, so y only gets a
// coarse bound.
func TestToLAEAAgainstEEA(t *testing.T) {
	eMin, _ := toLAEA(2.14, 41.375)
	eMax, _ := toLAEA(2.19, 41.41)
	_, nMax := toLAEA(2.14, 41.41) // ymax corner is nearest lon0
	if math.Abs(eMin-3662101.58) > 1 || math.Abs(eMax-3666645.71) > 1 {
		t.Fatalf("eastings (%.2f, %.2f), want (3662101.58, 3666645.71)",
			eMin, eMax)
	}
	if math.Abs(nMax-2068710.50) > 300 {
		t.Fatalf("northing %.2f, want within padding of 2068710.50",
			nMax)
	}
}

func TestLAEARoundTrip(t *testing.T) {
	pts := [][2]float64{
		{2.17, 41.39},  // Barcelona
		{10, 52},       // projection origin
		{-9.14, 38.71}, // Lisbon
		{25.28, 54.69}, // Vilnius
		{24.94, 60.17}, // Helsinki
	}
	for _, p := range pts {
		e, n := toLAEA(p[0], p[1])
		lon, lat := fromLAEA(e, n)
		if math.Abs(lon-p[0]) > 1e-6 || math.Abs(lat-p[1]) > 1e-6 {
			t.Fatalf("roundtrip %v -> (%.2f, %.2f) -> (%v, %v)",
				p, e, n, lon, lat)
		}
	}
}
