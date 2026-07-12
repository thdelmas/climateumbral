// GeoJSON builders for the ledger layers on the MapLibre map.
import { pixelRing } from './proj.js'

const key = (pe, pn) => `${pe},${pn}`

// Claims as fill polygons; watched-only pixels as their own features.
export function ledgerGeojson(claims, watches, mineKeys) {
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
  const seen = new Set(feats.map((f) => f.properties.key))
  for (const w of watches) {
    const k = key(w.pe, w.pn)
    if (seen.has(k)) continue
    seen.add(k)
    feats.push({
      type: 'Feature',
      properties: { kind: 'watched', key: k, mine: mineKeys.has(k) },
      geometry: {
        type: 'Polygon',
        coordinates: [pixelRing(w.pe, w.pn)],
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
