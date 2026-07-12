// Modeled heat penalty, v0 — degrees above the unsealed state.
//
// The layer answers: how much warmer is this place than it would be
// unsealed — day and night — for a body, not a thermometer. It is a
// MODEL and is labeled modeled everywhere it appears (design rule 1
// extended: modeled and measured degrees never mix).
//
// v0 is a linear penalty on the neighborhood sealed fraction, the
// first-order driver of the surface urban heat island:
//
//   penalty(pixel) = COEF * mean(sealed fraction within 150 m)
//
// DAY_COEF 6 °C: daytime surface penalty — shade and evapotranspiration
// lost. NIGHT_COEF 4 °C: the day's sun stored in thermal mass and
// released until dawn — smaller, but it hits a sleeping body whose
// livable optimum is lower; night is the layer that kills. Magnitudes
// sit in the range European SUHI literature reports; calibration
// against real LST (Sentinel-3 / MODIS, day and night passes)
// replaces them in v1. Flipped pixels count as unsealed, so every
// flip cools its block in the model.
export const DAY_COEF = 6
export const NIGHT_COEF = 4
export const RADIUS_PX = 15 // 150 m at 10 m per pixel

// sealedStats computes, per land pixel, the mean sealed fraction of
// land pixels within RADIUS_PX (box window, integral images) and the
// count of land pixels in the window. Sea/nodata pixels get S = -1.
export function sealedStats(grid, w, h, flipped) {
  const iw = w + 1
  const sumS = new Float64Array(iw * (h + 1))
  const sumN = new Float64Array(iw * (h + 1))
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = y * w + x
      const land = grid[i] <= 100
      const s = !land || flipped.has(i) ? 0 : grid[i] / 100
      const j = (y + 1) * iw + (x + 1)
      sumS[j] = s + sumS[j - 1] + sumS[j - iw] - sumS[j - iw - 1]
      sumN[j] =
        (land ? 1 : 0) + sumN[j - 1] + sumN[j - iw] - sumN[j - iw - 1]
    }
  }
  const rect = (a, x0, y0, x1, y1) =>
    a[(y1 + 1) * iw + x1 + 1] -
    a[y0 * iw + x1 + 1] -
    a[(y1 + 1) * iw + x0] +
    a[y0 * iw + x0]
  const S = new Float32Array(w * h)
  const C = new Int32Array(w * h)
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = y * w + x
      if (grid[i] > 100) {
        S[i] = -1
        continue
      }
      const x0 = Math.max(0, x - RADIUS_PX)
      const y0 = Math.max(0, y - RADIUS_PX)
      const x1 = Math.min(w - 1, x + RADIUS_PX)
      const y1 = Math.min(h - 1, y + RADIUS_PX)
      const n = rect(sumN, x0, y0, x1, y1)
      C[i] = n
      S[i] = n ? rect(sumS, x0, y0, x1, y1) / n : 0
    }
  }
  return { S, C }
}

// meanPenalty: average modeled penalty over land pixels, in °C.
export function meanPenalty(S, coef) {
  let sum = 0
  let n = 0
  for (let i = 0; i < S.length; i++) {
    if (S[i] >= 0) {
      sum += S[i]
      n++
    }
  }
  return n ? (coef * sum) / n : 0
}

// flipsPerDegree: how many flips like this one the block needs for
// one modeled degree of night cooling.
export function flipsPerDegree(grid, i, C) {
  if (grid[i] > 100 || !C[i]) return 0
  const perFlip = (NIGHT_COEF * (grid[i] / 100)) / C[i]
  return perFlip > 0 ? Math.ceil(1 / perFlip) : 0
}

const STOPS = [
  [0.0, [58, 122, 84]], // livable green
  [0.45, [235, 179, 66]], // amber
  [0.75, [214, 82, 38]], // red
  [1.0, [126, 24, 60]], // the nights that kill
]

export function heatColor(delta, max) {
  const t = Math.min(1, Math.max(0, delta / max))
  for (let k = 1; k < STOPS.length; k++) {
    if (t <= STOPS[k][0]) {
      const [t0, c0] = STOPS[k - 1]
      const [t1, c1] = STOPS[k]
      const f = (t - t0) / (t1 - t0)
      return c0.map((v, j) => Math.round(v + (c1[j] - v) * f))
    }
  }
  return STOPS[STOPS.length - 1][1]
}

export const HEAT_GRADIENT_CSS =
  'linear-gradient(90deg, rgb(58,122,84), rgb(235,179,66) 45%, ' +
  'rgb(214,82,38) 75%, rgb(126,24,60))'
