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

// nearestRefuge: the closest pin to [lon, lat], plus its distance in
// km (equirectangular — city scale, not navigation).
export function nearestRefuge(refuges, here) {
  let best = null
  let bestD = Infinity
  for (const r of refuges) {
    const dx = (here[0] - r.lon) *
      Math.cos(((here[1] + r.lat) / 2) * Math.PI / 180)
    const d = Math.hypot(dx, here[1] - r.lat) * 111.32
    if (d < bestD) {
      bestD = d
      best = r
    }
  }
  return best && { ...best, km: bestD }
}

// hoursLabel: one display line. Freeform hours as published; from
// structured per-day hours (week, Monday-first), today's entry.
export function hoursLabel(r) {
  if (r.hours) return r.hours
  if (!r.week) return ''
  const today = r.week[(new Date().getDay() + 6) % 7]
  return today ? `today ${today}` : ''
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
        hours: hoursLabel(r),
        src: r.src,
        tip: `${r.name} — official climate shelter`,
      },
    })),
  }
}
