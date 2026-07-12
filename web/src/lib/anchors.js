// Human-hour anchors, client side: fetch for a loaded raster, weight
// candidates by nearby hours, name the nearest anchor for a square.
// Level 1 of the presence layer — open data, no tracking.
import { toLAEA, fromLAEA } from './proj.js'

export const EXPOSURE_R = 20 // pixels = 200 m

const KIND = {
  school: 'school',
  kindergarten: 'kindergarten',
  playground: 'playground',
  hospital: 'hospital',
  marketplace: 'market',
  bus_stop: 'bus stop',
}

// fetchAnchors returns the raster's anchors in raster-local pixel
// coordinates. Fails soft (empty list): the game plays without them.
export async function fetchAnchors(r) {
  const e0 = r.pe0 * 10
  const n0 = r.pn0 * 10
  const corners = [
    fromLAEA(e0, n0),
    fromLAEA(e0 + r.W * 10, n0),
    fromLAEA(e0, n0 + r.H * 10),
    fromLAEA(e0 + r.W * 10, n0 + r.H * 10),
  ]
  const w = Math.min(...corners.map((c) => c[0]))
  const e = Math.max(...corners.map((c) => c[0]))
  const s = Math.min(...corners.map((c) => c[1]))
  const n = Math.max(...corners.map((c) => c[1]))
  try {
    const res = await fetch(`/api/anchors?bbox=${w},${s},${e},${n}`)
    const list = await res.json()
    return list.map((a) => {
      const [E, N] = toLAEA(a.lon, a.lat)
      return {
        ...a,
        px: E / 10 - r.pe0,
        py: r.H - 1 - (N / 10 - r.pn0),
      }
    })
  } catch {
    return []
  }
}

// exposureAt: human-hours weight of a raster pixel, from nearby
// anchors with linear falloff to 200 m.
export function exposureAt(r, i) {
  const x = i % r.W
  const y = Math.floor(i / r.W)
  let sum = 0
  for (const a of r.anchors ?? []) {
    const d = Math.hypot(a.px - x, a.py - y)
    if (d < EXPOSURE_R) sum += a.w * (1 - d / EXPOSURE_R)
  }
  return sum
}

// nearestAnchor: human-readable context for a square, or null.
export function nearestAnchor(r, i, maxPx = 25) {
  if (!r?.anchors?.length || i < 0) return null
  const x = i % r.W
  const y = Math.floor(i / r.W)
  let best = null
  let bestD = maxPx
  for (const a of r.anchors) {
    const d = Math.hypot(a.px - x, a.py - y)
    if (d < bestD) {
      bestD = d
      best = a
    }
  }
  if (!best) return null
  const name = best.name
    ? `${best.name} (${KIND[best.kind]})`
    : `a ${KIND[best.kind]}`
  return `≈ ${Math.round(bestD * 10)} m from ${name}`
}

// pickByExposure: weighted-random candidate pick — prefer squares
// where the hours are, with a floor so unanchored candidates still
// surface sometimes.
export function pickByExposure(r, arr) {
  const weights = arr.map((i) => 0.25 + exposureAt(r, i))
  let pick = Math.random() * weights.reduce((a, b) => a + b, 0)
  for (let k = 0; k < arr.length; k++) {
    pick -= weights[k]
    if (pick <= 0) return arr[k]
  }
  return arr[arr.length - 1]
}
