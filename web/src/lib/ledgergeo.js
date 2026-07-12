// GeoJSON builders for the ledger layers on the MapLibre map.
import { pixelRing } from './proj.js'

const key = (pe, pn) => `${pe},${pn}`

// Claims (acts) as fill polygons.
export function ledgerGeojson(claims, mineKeys) {
  const feats = []
  for (const c of claims) {
    feats.push({
      type: 'Feature',
      properties: {
        kind: c.status,
        key: key(c.pe, c.pn),
        mine: mineKeys.has(key(c.pe, c.pn)),
      },
      geometry: {
        type: 'Polygon',
        coordinates: [pixelRing(c.pe, c.pn)],
      },
    })
  }
  return { type: 'FeatureCollection', features: feats }
}

export function selectionGeojson(selected) {
  if (!selected) return { type: 'FeatureCollection', features: [] }
  return {
    type: 'FeatureCollection',
    features: [{
      type: 'Feature',
      properties: {},
      geometry: {
        type: 'Polygon',
        coordinates: [pixelRing(selected.pe, selected.pn)],
      },
    }],
  }
}
