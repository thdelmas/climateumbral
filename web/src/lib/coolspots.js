// Modeled cool islands — where the night model says a body can cool
// down: green ground whose banked-heat neighborhood stays coolest
// after dark. Works for any loaded viewport, so it covers the whole
// board — every European city gets cool islands the moment its
// raster loads, no dataset required.
//
// Tier discipline (the other half of refuges.js): these are model
// output, labeled modeled everywhere, and never mix with official
// shelters. A cool island is a shaded park block, not a promise of
// a roof or opening hours.
import { NIGHT_COEF } from './heat.js'
import { GREEN_MAX } from './grid.js'
import { pixelCenter } from './proj.js'

export const COOL_MIN_DELTA = 1 // °C below the viewport mean to count
const SEP_PX = 30 // >= 300 m apart, or one park eats the whole list
const MAX_SPOTS = 6

// coolSpots picks the coolest well-separated green pixels of a loaded
// raster, each at least COOL_MIN_DELTA modeled °C below the viewport's
// mean night penalty. Empty when nothing qualifies — a sealed-solid
// viewport honestly has no cool island to offer.
export function coolSpots(raster) {
  const { g, Snight, W, H, pe0, pn0 } = raster
  let sum = 0
  let n = 0
  for (let i = 0; i < Snight.length; i++) {
    if (Snight[i] >= 0) {
      sum += Snight[i]
      n++
    }
  }
  if (!n) return []
  const mean = (NIGHT_COEF * sum) / n
  const greens = []
  for (let i = 0; i < g.length; i++) {
    if (g[i] <= GREEN_MAX && Snight[i] >= 0) greens.push(i)
  }
  greens.sort((a, b) => Snight[a] - Snight[b])
  const spots = []
  for (const i of greens) {
    const night = NIGHT_COEF * Snight[i]
    if (mean - night < COOL_MIN_DELTA) break // sorted: rest are warmer
    const x = i % W
    const y = Math.floor(i / W)
    if (spots.some((s) => Math.hypot(s.x - x, s.y - y) < SEP_PX)) {
      continue
    }
    spots.push({
      x,
      y,
      pe: pe0 + x,
      pn: pn0 + (H - 1 - y),
      night,
      delta: mean - night,
    })
    if (spots.length >= MAX_SPOTS) break
  }
  return spots
}

export function coolSpotsGeojson(spots) {
  return {
    type: 'FeatureCollection',
    features: spots.map((s) => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: pixelCenter(s.pe, s.pn) },
      properties: {
        pe: s.pe,
        pn: s.pn,
        tip:
          `modeled cool island — ≈${s.delta.toFixed(1)} °C cooler at ` +
          `night than this view's average`,
      },
    })),
  }
}
