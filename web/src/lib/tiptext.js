// tipText: what the cursor is over, for the map tooltip.
import { DAY_COEF, NIGHT_COEF } from './heat.js'

const SEA = 254
const NODATA = 255

export function tipTextAt(raster, claims, mode, pe, pn) {
  const k = `${pe},${pn}`
  const claim = claims.find((c) => `${c.pe},${c.pn}` === k)
  const col = pe - raster.pe0
  const row = raster.H - 1 - (pn - raster.pn0)
  const inR =
    col >= 0 && row >= 0 && col < raster.W && row < raster.H
  const i = inR ? row * raster.W + col : -1
  const v = i >= 0 ? raster.g[i] : null
  const S = mode === 'day' ? raster.Sday : raster.Snight
  if (mode !== 'land' && i >= 0 && S[i] >= 0) {
    const coef = mode === 'day' ? DAY_COEF : NIGHT_COEF
    const tag = raster.cands?.has(i) ? ' · candidate' : ''
    return `+${(coef * S[i]).toFixed(1)} °C ${mode} (modeled)${tag}`
  }
  if (claim?.status === 'flipped') return 'done — cooler for real'
  if (claim) return 'pledged — click for details'
  if (i >= 0 && raster.cands?.has(i)) {
    return 'candidate — click: pledge it or join its block'
  }
  if (v === null) return null
  if (v === SEA) return 'water'
  if (v === NODATA) return 'no data'
  return `${v}% sealed`
}
