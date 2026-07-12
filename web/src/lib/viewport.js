// Viewport → EPSG:3035 bbox math for the game raster.
import { toLAEA } from './proj.js'

export const MAX_DIM = 512 // server-side raster cap, pixels

// viewport3035: the 3035 bbox covering a MapLibre bounds object,
// padded 30% (so small pans and the post-frontline zoom reuse one
// raster) but capped to the server's max size. Returns null when the
// viewport is too large to play at 10 m.
export function viewport3035(bounds) {
  const corners = [
    [bounds.getWest(), bounds.getSouth()],
    [bounds.getEast(), bounds.getSouth()],
    [bounds.getWest(), bounds.getNorth()],
    [bounds.getEast(), bounds.getNorth()],
  ].map(([lo, la]) => toLAEA(lo, la))
  let e0 = Math.min(...corners.map((c) => c[0]))
  let e1 = Math.max(...corners.map((c) => c[0]))
  let n0 = Math.min(...corners.map((c) => c[1]))
  let n1 = Math.max(...corners.map((c) => c[1]))
  if ((e1 - e0) / 10 > MAX_DIM || (n1 - n0) / 10 > MAX_DIM) return null
  const pad = (hi, lo) =>
    Math.max(
      0,
      Math.min((hi - lo) * 0.3, (MAX_DIM * 10 - (hi - lo)) / 2),
    )
  const padE = pad(e1, e0)
  const padN = pad(n1, n0)
  return { e0: e0 - padE, n0: n0 - padN, e1: e1 + padE, n1: n1 + padN }
}

// contains: is the bbox already inside the loaded raster?
export function rasterContains(r, bounds) {
  if (!r) return false
  const corners = [
    [bounds.getWest(), bounds.getSouth()],
    [bounds.getEast(), bounds.getNorth()],
  ].map(([lo, la]) => toLAEA(lo, la))
  const e0 = Math.min(corners[0][0], corners[1][0])
  const e1 = Math.max(corners[0][0], corners[1][0])
  const n0 = Math.min(corners[0][1], corners[1][1])
  const n1 = Math.max(corners[0][1], corners[1][1])
  return (
    e0 >= r.pe0 * 10 &&
    n0 >= r.pn0 * 10 &&
    e1 <= (r.pe0 + r.W) * 10 &&
    n1 <= (r.pn0 + r.H) * 10
  )
}
