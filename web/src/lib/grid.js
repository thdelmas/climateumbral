// Grid semantics shared with tools/fetch_grid.py and server/main.go:
// 0-100 = % sealed (Copernicus IMD 2018, 10 m), 254 = water
// (WAW permanent water or sea, merged server-side), 255 = nodata.
export const SEA = 254
export const NODATA = 255
export const HARD_SEALED = 90
export const GREEN_MAX = 10
export const MIN_GREENS = 3

export const isGreen = (v) => v <= GREEN_MAX
export const isSealed = (v) => v >= HARD_SEALED && v < SEA

// A candidate is a hard-sealed pixel touching >=3 green-or-claimed neighbours.
// Claimed pixels count as green: that is the cascade — every claim can open
// the sealed pixels around it. Mirrors server/main.go's candidate().
export function computeCandidates(grid, w, h, claimed) {
  const cands = new Set()
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = y * w + x
      if (!isSealed(grid[i]) || claimed.has(i)) continue
      let greens = 0
      for (let dy = -1; dy <= 1; dy++) {
        for (let dx = -1; dx <= 1; dx++) {
          if (!dx && !dy) continue
          const nx = x + dx
          const ny = y + dy
          if (nx < 0 || ny < 0 || nx >= w || ny >= h) continue
          const ni = ny * w + nx
          if (claimed.has(ni) || isGreen(grid[ni])) greens++
        }
      }
      if (greens >= MIN_GREENS) cands.add(i)
    }
  }
  return cands
}

export function colorFor(v) {
  if (v === SEA) return [72, 118, 160]
  if (v === NODATA) return [157, 191, 216]
  const t = v / 100
  return hsl(125, 48 * (1 - t), 46 - t * 14) // green -> dark gray
}

export const FLIPPED_COLOR = [125, 200, 110] // brighter than natural green
export const PLEDGED_COLOR = [235, 179, 66] // amber: promised, not yet done
export const WATCHED_COLOR = [150, 118, 220] // violet: petition forming
export const CANDIDATE_COLOR = [255, 122, 26]

function hsl(h, s, l) {
  s /= 100
  l /= 100
  const a = s * Math.min(l, 1 - l)
  const f = (n) => {
    const k = (n + h / 30) % 12
    return Math.round(255 * (l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1))))
  }
  return [f(0), f(8), f(4)]
}
