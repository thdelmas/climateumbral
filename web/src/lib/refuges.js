// Official climate shelters, client side — the adaptation layer.
// A pin is a room a city promised (roof, cool air, hours), straight
// from that city's open data. Tier discipline: these never come from
// the model, and modeled cool islands never appear here.
//
// Coverage is per-city adapters (there is no EU-wide dataset), so
// `sources` carries the truth about absence: sources === null means
// "couldn't ask", a source with ok === false means "network exists
// but its data is down" — neither may read as "no shelters".
export async function fetchRefuges() {
  try {
    const res = await fetch('/api/refuges')
    if (!res.ok) throw new Error()
    const { sources, refuges } = await res.json()
    return { sources, refuges }
  } catch {
    return { sources: null, refuges: [] }
  }
}

export function refugesGeojson(refuges) {
  return {
    type: 'FeatureCollection',
    features: refuges.map((r) => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [r.lon, r.lat] },
      properties: {
        name: r.name,
        addr: r.addr ?? '',
        web: r.web ?? '',
        src: r.src,
        tip: `${r.name} — official climate shelter`,
      },
    })),
  }
}
