// EPSG:3035 — ETRS89 / LAEA Europe. Same math as server/proj.go;
// the two must agree: continent pixel (pe, pn) = floor(E/10),
// floor(N/10) is the key every claim lives under.
const A = 6378137.0
const F = 1 / 298.257222101
const E2 = F * (2 - F)
const E = Math.sqrt(E2)
const LAT0 = (52 * Math.PI) / 180
const LON0 = (10 * Math.PI) / 180
const FE = 4321000.0
const FN = 3210000.0

function q(phi) {
  const s = Math.sin(phi)
  return (
    (1 - E2) *
    (s / (1 - E2 * s * s) -
      (1 / (2 * E)) * Math.log((1 - E * s) / (1 + E * s)))
  )
}
const QP = q(Math.PI / 2)
const B1 = Math.asin(q(LAT0) / QP)
const RQ = A * Math.sqrt(QP / 2)
const D =
  (A * Math.cos(LAT0)) /
  Math.sqrt(1 - E2 * Math.sin(LAT0) ** 2) /
  (RQ * Math.cos(B1))

export function toLAEA(lon, lat) {
  const lam = (lon * Math.PI) / 180 - LON0
  const phi = (lat * Math.PI) / 180
  const beta = Math.asin(q(phi) / QP)
  const b =
    RQ *
    Math.sqrt(
      2 /
        (1 +
          Math.sin(B1) * Math.sin(beta) +
          Math.cos(B1) * Math.cos(beta) * Math.cos(lam)),
    )
  return [
    FE + b * D * Math.cos(beta) * Math.sin(lam),
    FN +
      (b / D) *
        (Math.cos(B1) * Math.sin(beta) -
          Math.sin(B1) * Math.cos(beta) * Math.cos(lam)),
  ]
}

export function fromLAEA(e, n) {
  const xp = (e - FE) / D
  const yp = D * (n - FN)
  const rho = Math.hypot(xp, yp)
  if (rho === 0) return [(LON0 * 180) / Math.PI, (LAT0 * 180) / Math.PI]
  const ce = 2 * Math.asin(rho / (2 * RQ))
  const qv =
    QP *
    (Math.cos(ce) * Math.sin(B1) +
      (yp * Math.sin(ce) * Math.cos(B1)) / rho)
  const lam =
    LON0 +
    Math.atan2(
      (e - FE) * Math.sin(ce),
      D * rho * Math.cos(B1) * Math.cos(ce) -
        D * D * (n - FN) * Math.sin(B1) * Math.sin(ce),
    )
  let phi = Math.asin(qv / 2)
  for (let i = 0; i < 8; i++) {
    const s = Math.sin(phi)
    phi +=
      (((1 - E2 * s * s) ** 2 / (2 * Math.cos(phi))) *
        (qv / (1 - E2) -
          s / (1 - E2 * s * s) +
          (1 / (2 * E)) * Math.log((1 - E * s) / (1 + E * s))))
  }
  return [(lam * 180) / Math.PI, (phi * 180) / Math.PI]
}

// Center of a continent pixel, as lon/lat.
export function pixelCenter(pe, pn) {
  return fromLAEA(pe * 10 + 5, pn * 10 + 5)
}

// Corners of a continent pixel as a lon/lat polygon ring (closed).
export function pixelRing(pe, pn) {
  const e = pe * 10
  const n = pn * 10
  return [
    fromLAEA(e, n),
    fromLAEA(e + 10, n),
    fromLAEA(e + 10, n + 10),
    fromLAEA(e, n + 10),
    fromLAEA(e, n),
  ]
}
