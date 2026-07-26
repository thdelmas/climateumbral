// Measured air temperature, client side — the ground-truth tier.
// A station pin is a real thermometer on a mast whose hourly record
// NOAA publishes (GHCN-hourly). The pins exist to anchor the modeled
// heat layers to reality, under design rule 1 at its strictest:
// measured and modeled degrees NEVER mix. A station reading never
// colors the heat overlay, the model never touches a pin, and every
// surface a station number appears on says "measured".
//
// The feed lags a few days; honesty means showing the observation
// time, never implying "now".
export async function fetchStations() {
  try {
    const res = await fetch('/api/stations')
    if (!res.ok) throw new Error()
    const { stations } = await res.json()
    return stations
  } catch {
    return []
  }
}

// obsLabel: "30.5 °C on 23 Jul, 16:00" — the reading is nothing
// without its date; the feed lags and "now" would be a lie.
export function obsLabel(latestC, latestUtc) {
  const d = new Date(latestUtc)
  const when = d.toLocaleString(undefined, {
    day: 'numeric', month: 'short',
    hour: '2-digit', minute: '2-digit',
  })
  return `${latestC.toFixed(1)} °C on ${when}`
}

export function lagDays(latestUtc) {
  return Math.floor((Date.now() - new Date(latestUtc)) / 86400000)
}

export function stationsGeojson(stations) {
  return {
    type: 'FeatureCollection',
    features: stations.filter((s) => s.ok).map((s) => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
      properties: {
        id: s.id,
        name: s.name,
        elev: s.elev_m,
        latestC: s.latest_c,
        latestUtc: s.latest_utc,
        min24: s.min24_c,
        max24: s.max24_c,
        tip: `measured ${obsLabel(s.latest_c, s.latest_utc)} — ` +
          'real thermometer, not the model',
      },
    })),
  }
}
