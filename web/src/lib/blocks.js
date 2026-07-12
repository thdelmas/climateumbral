// 150 m petition blocks — the join system, client side.
// A block is the night-window-scale cell you choose to stand behind;
// its signatures and delta trajectory are the live petition.
import { fromLAEA } from './proj.js'

export const BLOCK_PX = 15 // pixels per block edge = 150 m

export const blockOf = (pe, pn) => [
  Math.floor(pe / BLOCK_PX),
  Math.floor(pn / BLOCK_PX),
]
export const blockKey = (be, bn) => `${be},${bn}`

export function blockRing(be, bn) {
  const e = be * BLOCK_PX * 10
  const n = bn * BLOCK_PX * 10
  const d = BLOCK_PX * 10
  return [
    fromLAEA(e, n),
    fromLAEA(e + d, n),
    fromLAEA(e + d, n + d),
    fromLAEA(e, n + d),
    fromLAEA(e, n),
  ]
}

// blocksGeojson: one feature per petitioned block, with signature
// count — the governance view.
export function blocksGeojson(joins) {
  const byBlock = new Map()
  for (const j of joins) {
    const k = blockKey(j.be, j.bn)
    byBlock.set(k, [...(byBlock.get(k) ?? []), j])
  }
  return {
    type: 'FeatureCollection',
    features: [...byBlock.entries()].map(([k, list]) => ({
      type: 'Feature',
      properties: { key: k, joiners: list.length },
      geometry: {
        type: 'Polygon',
        coordinates: [blockRing(list[0].be, list[0].bn)],
      },
    })),
  }
}

// Mirror of server nightCooling: modeled m°C an act delivers.
const ACT_NIGHT = { depave: 0, tree: 1, coolroof: 0.9 }
export function actNightMC(c) {
  const f = ACT_NIGHT[c.kind] ?? 0
  return ((4.0 * (c.v ?? 90)) / 100) * (1 - f) / (31 * 31) * 1000
}

// blockCoolingSince: modeled night cooling delivered inside a block
// by acts done since a timestamp. Mirrors server ledger.blockCooling.
export function blockCoolingSince(claims, be, bn, since) {
  let sum = 0
  for (const c of claims) {
    if (c.status !== 'flipped' || !c.flipped) continue
    if (new Date(c.flipped) < new Date(since)) continue
    const [cbe, cbn] = blockOf(c.pe, c.pn)
    if (cbe === be && cbn === bn) sum += actNightMC(c)
  }
  return sum
}
