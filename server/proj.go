// EPSG:3035 — ETRS89-extended / LAEA Europe (Lambert azimuthal
// equal-area on GRS80, lat0 52, lon0 10, false origin 4321000 /
// 3210000). Snyder, Map Projections: A Working Manual, pp. 187-190.
//
// This is the continent-wide 10 m pixel grid every claim is keyed to:
// pixel (pe, pn) = floor(easting/10), floor(northing/10). The same
// math exists in web/src/lib/proj.js; the two must agree.
package main

import "math"

const (
	grsA  = 6378137.0
	grsF  = 1 / 298.257222101
	laeE2 = grsF * (2 - grsF)
	lat0  = 52.0 * math.Pi / 180
	lon0  = 10.0 * math.Pi / 180
	falsE = 4321000.0
	falsN = 3210000.0
)

var (
	laeE  = math.Sqrt(laeE2)
	laeQp = authalicQ(math.Pi / 2)
	beta1 = math.Asin(authalicQ(lat0) / laeQp)
	laeRq = grsA * math.Sqrt(laeQp/2)
	laeD  = grsA * math.Cos(lat0) /
		math.Sqrt(1-laeE2*math.Sin(lat0)*math.Sin(lat0)) /
		(laeRq * math.Cos(beta1))
)

// inEurope guards against garbage pixel keys (e.g. pre-V3 permalink
// coordinates): the EPSG:3035 domain the imperviousness layer covers.
func inEurope(pe, pn int) bool {
	return pe >= 100_000 && pe <= 750_000 &&
		pn >= 90_000 && pn <= 550_000
}

func authalicQ(phi float64) float64 {
	s := math.Sin(phi)
	return (1 - laeE2) * (s/(1-laeE2*s*s) -
		1/(2*laeE)*math.Log((1-laeE*s)/(1+laeE*s)))
}

// toLAEA converts lon/lat degrees to EPSG:3035 easting/northing.
func toLAEA(lonDeg, latDeg float64) (float64, float64) {
	lam := lonDeg*math.Pi/180 - lon0
	phi := latDeg * math.Pi / 180
	beta := math.Asin(authalicQ(phi) / laeQp)
	b := laeRq * math.Sqrt(2/(1+math.Sin(beta1)*math.Sin(beta)+
		math.Cos(beta1)*math.Cos(beta)*math.Cos(lam)))
	e := falsE + b*laeD*math.Cos(beta)*math.Sin(lam)
	n := falsN + b/laeD*(math.Cos(beta1)*math.Sin(beta)-
		math.Sin(beta1)*math.Cos(beta)*math.Cos(lam))
	return e, n
}

// fromLAEA converts EPSG:3035 easting/northing to lon/lat degrees.
func fromLAEA(e, n float64) (float64, float64) {
	xp := (e - falsE) / laeD
	yp := laeD * (n - falsN)
	rho := math.Hypot(xp, yp)
	if rho == 0 {
		return lon0 * 180 / math.Pi, lat0 * 180 / math.Pi
	}
	ce := 2 * math.Asin(rho/(2*laeRq))
	q := laeQp * (math.Cos(ce)*math.Sin(beta1) +
		yp*math.Sin(ce)*math.Cos(beta1)/rho)
	lam := lon0 + math.Atan2((e-falsE)*math.Sin(ce),
		laeD*rho*math.Cos(beta1)*math.Cos(ce)-
			laeD*laeD*(n-falsN)*math.Sin(beta1)*math.Sin(ce))
	phi := math.Asin(q / 2)
	for i := 0; i < 8; i++ {
		s := math.Sin(phi)
		phi += (1 - laeE2*s*s) * (1 - laeE2*s*s) / (2 * math.Cos(phi)) *
			(q/(1-laeE2) - s/(1-laeE2*s*s) +
				1/(2*laeE)*math.Log((1-laeE*s)/(1+laeE*s)))
	}
	return lam * 180 / math.Pi, phi * 180 / math.Pi
}
