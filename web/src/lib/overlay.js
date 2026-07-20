// The game overlay: paints raster values (land) or modeled heat into
// a canvas at one canvas pixel per 10 m grid cell. Split from
// EuroMap.vue — pure painting, no map state.
import { colorFor, CANDIDATE_COLOR } from './grid.js'
import { heatColor } from './heat.js'

// heatColor(coef * S, coef) depends only on the clamped S, so a
// 256-step lookup replaces ~W*H ramp evaluations (and their array
// allocations) per heat repaint.
const HEAT_LUT = Array.from({ length: 256 },
  (_, k) => heatColor(k / 255, 1))

export function renderOverlay(raster, mode, canvas) {
  const { g, W, H, cands } = raster
  const S = mode === 'day' ? raster.Sday : raster.Snight
  canvas.width = W
  canvas.height = H
  const ctx = canvas.getContext('2d')
  const im = ctx.createImageData(W, H)
  const heat = mode !== 'land'
  for (let i = 0; i < g.length; i++) {
    let c = null
    let a = 0
    if (heat) {
      if (S[i] >= 0) {
        c = HEAT_LUT[Math.min(255, Math.round(S[i] * 255))]
        a = 210
      }
    } else if (cands.has(i)) {
      c = CANDIDATE_COLOR
      a = 255
    } else if (g[i] <= 100) {
      // the sealed-soil ramp: gray-green ground truth, the layer to
      // correlate with the heat views (sea/nodata stay transparent)
      c = colorFor(g[i])
      a = 235
    }
    if (c) {
      im.data[i * 4] = c[0]
      im.data[i * 4 + 1] = c[1]
      im.data[i * 4 + 2] = c[2]
      im.data[i * 4 + 3] = a
    }
  }
  if (!heat) {
    // halo: tint candidate neighbours so the front line pops
    for (const i of cands) {
      const x = i % W
      const y = Math.floor(i / W)
      for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
        const nx = x + dx
        const ny = y + dy
        if (nx < 0 || ny < 0 || nx >= W || ny >= H) continue
        const j = ny * W + nx
        if (cands.has(j) || g[j] > 100) continue
        im.data[j * 4] = (im.data[j * 4] + 255 * 2) / 3
        im.data[j * 4 + 1] = (im.data[j * 4 + 1] + 122 * 2) / 3
        im.data[j * 4 + 2] = (im.data[j * 4 + 2] + 26 * 2) / 3
        im.data[j * 4 + 3] = 235
      }
    }
  }
  ctx.putImageData(im, 0, 0)
  return canvas
}
