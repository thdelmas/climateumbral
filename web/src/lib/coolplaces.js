// Other cool public places, client side — the not-official ring.
// Crowd knowledge from OpenStreetMap (air_conditioning=yes + malls),
// served by /api/coolplaces. Tier discipline: these are never mixed
// into the refuge source, and every surface that shows them says
// "not an official shelter".

export const KIND_ICON = {
  mall: '🛍️', department_store: '🏬', supermarket: '🛒',
  chemist: '💊', pharmacy: '💊', library: '📚',
  community_centre: '🏠', cinema: '🎬', place_of_worship: '⛪',
  cafe: '☕', restaurant: '🍽️', fast_food: '🍔', townhall: '🏛️',
  arts_centre: '🎭', museum: '🏛️', gallery: '🖼️',
}

// fetchCoolPlacesBBox: viewport lookup. null means "couldn't ask"
// (unavailable, out of Europe, too zoomed out) — callers must not
// render that as an empty layer being the truth.
export async function fetchCoolPlacesBBox(w, s, e, n) {
  try {
    const res = await fetch(
      `/api/coolplaces?w=${w.toFixed(4)}&s=${s.toFixed(4)}` +
      `&e=${e.toFixed(4)}&n=${n.toFixed(4)}`)
    if (!res.ok) return null
    const { places, partial } = await res.json()
    return { places: places ?? [], partial: !!partial }
  } catch {
    return null
  }
}

export function coolPlacesGeojson(places) {
  return {
    type: 'FeatureCollection',
    features: places.map((p) => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [p.lon, p.lat] },
      properties: {
        name: p.name,
        kind: p.kind,
        icon: KIND_ICON[p.kind] ?? '🏢',
        hours: p.hours ?? '',
        tip: `${p.name} — air-conditioned, not an official shelter`,
      },
    })),
  }
}
