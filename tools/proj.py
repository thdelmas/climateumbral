"""ETRS89 coordinate conversions, stdlib only (GRS80 ellipsoid).

- utm_to_geo: ETRS89 / UTM zone N (EPSG:258NN) -> lon, lat. Krüger series to
  6th order (Karney 2011), millimetre-accurate far outside the zone, which
  matters because Spain ships its whole territory in zone 30.
- geo_to_laea: lon, lat -> ETRS89-LAEA (EPSG:3035), ellipsoidal (Snyder 1987).
- laea_to_geo: the inverse, for checks.
"""
import math

A = 6378137.0
F = 1 / 298.257222101
E2 = F * (2 - F)
E = math.sqrt(E2)

# ---------- Transverse Mercator (Krüger series) ----------
_N = F / (2 - F)
_n = _N
_A = A / (1 + _n) * (1 + _n ** 2 / 4 + _n ** 4 / 64 + _n ** 6 / 256)
_BETA = [
    _n / 2 - 2 * _n ** 2 / 3 + 37 * _n ** 3 / 96 - _n ** 4 / 360 - 81 * _n ** 5 / 512 + 96199 * _n ** 6 / 604800,
    _n ** 2 / 48 + _n ** 3 / 15 - 437 * _n ** 4 / 1440 + 46 * _n ** 5 / 105 - 1118711 * _n ** 6 / 3870720,
    17 * _n ** 3 / 480 - 37 * _n ** 4 / 840 - 209 * _n ** 5 / 4480 + 5569 * _n ** 6 / 90720,
    4397 * _n ** 4 / 161280 - 11 * _n ** 5 / 504 - 830251 * _n ** 6 / 7257600,
    4583 * _n ** 5 / 161280 - 108847 * _n ** 6 / 3991680,
    20648693 * _n ** 6 / 638668800,
]
_DELTA = [
    2 * _n - 2 * _n ** 2 / 3 - 2 * _n ** 3 + 116 * _n ** 4 / 45 + 26 * _n ** 5 / 45 - 2854 * _n ** 6 / 675,
    7 * _n ** 2 / 3 - 8 * _n ** 3 / 5 - 227 * _n ** 4 / 45 + 2704 * _n ** 5 / 315 + 2323 * _n ** 6 / 945,
    56 * _n ** 3 / 15 - 136 * _n ** 4 / 35 - 1262 * _n ** 5 / 105 + 73814 * _n ** 6 / 2835,
    4279 * _n ** 4 / 630 - 332 * _n ** 5 / 35 - 399572 * _n ** 6 / 14175,
    4174 * _n ** 5 / 315 - 144838 * _n ** 6 / 6237,
    601676 * _n ** 6 / 22275,
]


def utm_to_geo(x, y, zone=30):
    k0, fe = 0.9996, 500000.0
    xi = y / (k0 * _A)
    eta = (x - fe) / (k0 * _A)
    xi2, eta2 = xi, eta
    for j, b in enumerate(_BETA, 1):
        xi2 -= b * math.sin(2 * j * xi) * math.cosh(2 * j * eta)
        eta2 -= b * math.cos(2 * j * xi) * math.sinh(2 * j * eta)
    chi = math.asin(math.sin(xi2) / math.cosh(eta2))
    lat = chi
    for j, d in enumerate(_DELTA, 1):
        lat += d * math.sin(2 * j * chi)
    lon0 = math.radians(zone * 6 - 183)
    lon = lon0 + math.atan2(math.sinh(eta2), math.cos(xi2))
    return math.degrees(lon), math.degrees(lat)


# ---------- Lambert Azimuthal Equal Area, EPSG:3035 ----------
_LAT0, _LON0 = math.radians(52), math.radians(10)
_FE, _FN = 4321000.0, 3210000.0


def _q(phi):
    s = math.sin(phi)
    return (1 - E2) * (s / (1 - E2 * s * s) - (1 / (2 * E)) * math.log((1 - E * s) / (1 + E * s)))


_QP = _q(math.pi / 2)
_RQ = A * math.sqrt(_QP / 2)
_B1 = math.asin(_q(_LAT0) / _QP)
_M1 = math.cos(_LAT0) / math.sqrt(1 - E2 * math.sin(_LAT0) ** 2)
_D = A * _M1 / (_RQ * math.cos(_B1))


def geo_to_laea(lon, lat):
    phi, lam = math.radians(lat), math.radians(lon)
    beta = math.asin(_q(phi) / _QP)
    B = _RQ * math.sqrt(2 / (1 + math.sin(_B1) * math.sin(beta)
                             + math.cos(_B1) * math.cos(beta) * math.cos(lam - _LON0)))
    x = _FE + B * _D * math.cos(beta) * math.sin(lam - _LON0)
    y = _FN + (B / _D) * (math.cos(_B1) * math.sin(beta)
                          - math.sin(_B1) * math.cos(beta) * math.cos(lam - _LON0))
    return x, y


def laea_to_geo(x, y):
    x, y = x - _FE, y - _FN
    rho = math.hypot(x / _D, _D * y)
    ce = 2 * math.asin(rho / (2 * _RQ))
    q = _QP * (math.cos(ce) * math.sin(_B1) + _D * y * math.sin(ce) * math.cos(_B1) / rho)
    lam = _LON0 + math.atan2(x * math.sin(ce),
                             _D * rho * math.cos(_B1) * math.cos(ce) - _D * _D * y * math.sin(_B1) * math.sin(ce))
    phi = math.asin(q / 2)
    for _ in range(10):  # Snyder 3-16
        s = math.sin(phi)
        phi += ((1 - E2 * s * s) ** 2 / (2 * math.cos(phi))) * (
            q / (1 - E2) - s / (1 - E2 * s * s) + (1 / (2 * E)) * math.log((1 - E * s) / (1 + E * s)))
    return math.degrees(lam), math.degrees(phi)


def utm30_to_laea(x, y):
    return geo_to_laea(*utm_to_geo(x, y, 30))
