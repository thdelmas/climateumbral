// Map bootstrap: base style and the static layer stacks, split out of
// EuroMap.vue so the component stays about behavior. Layer discipline
// lives here too: blue refuge pins (a city's promise) and green cool
// spots (model output) are separate sources that never blend.
import maplibregl from 'maplibre-gl'
import { fetchRefuges, refugesGeojson } from './refuges.js'

export const EMPTY_FC = { type: 'FeatureCollection', features: [] }

const EEA_PNG =
  'https://image.discomap.eea.europa.eu/arcgis/rest/services' +
  '/GioLandPublic/HRL_ImperviousnessDensity_2018/ImageServer' +
  '/exportImage?bbox={bbox-epsg-3857}&bboxSR=3857&imageSR=3857' +
  '&size=256,256&format=png&f=image'

// baseStyle: OSM under the EU-wide imperviousness layer.
export function baseStyle() {
  return {
    version: 8,
    sources: {
      osm: {
        type: 'raster',
        tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
        tileSize: 256,
        maxzoom: 19,
        attribution: '© OpenStreetMap contributors',
      },
      imd: {
        type: 'raster',
        tiles: [EEA_PNG],
        tileSize: 256,
        attribution: '© European Union, Copernicus / EEA',
      },
    },
    layers: [
      { id: 'osm', type: 'raster', source: 'osm' },
      { id: 'imd', type: 'raster', source: 'imd',
        paint: { 'raster-opacity': 0.5 } },
    ],
  }
}

// Heat modes lean the sealed layer warmer at continental zoom.
export function basemapMood(map, heat) {
  if (!map?.getLayer('imd')) return
  map.setPaintProperty('imd', 'raster-opacity', heat ? 0.85 : 0.5)
  map.setPaintProperty('imd', 'raster-saturation', heat ? 0.5 : 0)
  map.setPaintProperty('imd', 'raster-contrast', heat ? 0.15 : 0)
}

// addLedgerLayers: claims, petition blocks and the selection ring.
export function addLedgerLayers(map, data) {
  map.addSource('claims', { type: 'geojson', data: data.claims })
  map.addSource('blocks', { type: 'geojson', data: data.blocks })
  map.addLayer({
    id: 'blocks',
    type: 'line',
    source: 'blocks',
    paint: {
      'line-color': 'rgb(150,118,220)',
      'line-width': 2,
      'line-dasharray': [2, 1],
    },
  })
  map.addSource('selection', {
    type: 'geojson',
    data: data.selection,
  })
  map.addLayer({
    id: 'claims-fill',
    type: 'fill',
    source: 'claims',
    paint: {
      'fill-color': [
        'match', ['get', 'kind'],
        'flipped', 'rgb(125,200,110)',
        'rgb(235,179,66)', // pledged
      ],
      'fill-opacity': 0.9,
    },
  })
  map.addLayer({
    id: 'claims-mine',
    type: 'line',
    source: 'claims',
    filter: ['==', ['get', 'mine'], true],
    paint: { 'line-color': '#ffffff', 'line-width': 1.5 },
  })
  map.addLayer({
    id: 'selection',
    type: 'line',
    source: 'selection',
    paint: { 'line-color': '#ffffff', 'line-width': 2.5 },
  })
}

// addCoolPlaceLayers: two tiers, never blended (see refuges.js /
// coolspots.js). Cool islands only show in the heat views — they are
// a reading of that model and would masquerade as ground truth on
// the land map. Shelters cluster, with no minzoom: a city's network
// must be findable from continental zoom, not only once you already
// know where it is. onData reports adapter status + the pin list.
export function addCoolPlaceLayers(map, minCoolZoom, landMode,
  onData) {
  map.addSource('refuges', {
    type: 'geojson',
    data: EMPTY_FC,
    cluster: true,
    clusterMaxZoom: 13,
    clusterRadius: 40,
  })
  map.addLayer({
    id: 'refuge-clusters',
    type: 'circle',
    source: 'refuges',
    filter: ['has', 'point_count'],
    paint: {
      'circle-color': 'rgb(43, 108, 196)',
      'circle-radius': [
        'interpolate', ['linear'], ['get', 'point_count'],
        2, 9, 50, 14, 300, 18,
      ],
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 2,
      'circle-opacity': 0.9,
    },
  })
  map.addLayer({
    id: 'refuges',
    type: 'circle',
    source: 'refuges',
    filter: ['!', ['has', 'point_count']],
    paint: {
      'circle-color': 'rgb(43, 108, 196)',
      'circle-radius': [
        'interpolate', ['linear'], ['zoom'], 4, 3, 16, 8,
      ],
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 1.5,
    },
  })
  map.addSource('coolspots', { type: 'geojson', data: EMPTY_FC })
  map.addLayer({
    id: 'coolspots',
    type: 'circle',
    source: 'coolspots',
    minzoom: minCoolZoom,
    layout: {
      visibility: landMode ? 'none' : 'visible',
    },
    paint: {
      'circle-color': 'rgb(58, 122, 84)',
      'circle-radius': [
        'interpolate', ['linear'], ['zoom'], 13, 5, 16, 9,
      ],
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 1.5,
    },
  })
  fetchRefuges().then(({ sources, refuges }) => {
    onData(sources, refuges)
    map?.getSource('refuges')?.setData(refugesGeojson(refuges))
  })
}

// pinAt: topmost cool-place feature under the cursor, if any.
export function pinAt(map, point) {
  const layers = ['refuges', 'refuge-clusters', 'coolspots']
    .filter((l) => map.getLayer(l))
  if (!layers.length) return null
  return map.queryRenderedFeatures(point, { layers })[0] ?? null
}

// The popup builds DOM nodes, not HTML: dataset strings stay text.
export function openRefugePopup(map, f) {
  const p = f.properties
  const el = document.createElement('div')
  el.className = 'refuge-pop'
  const name = document.createElement('strong')
  name.textContent = p.name
  el.appendChild(name)
  if (p.addr) {
    const addr = document.createElement('div')
    addr.textContent = p.addr
    el.appendChild(addr)
  }
  if (p.hours) {
    const hours = document.createElement('div')
    hours.textContent = `🕐 ${p.hours}`
    el.appendChild(hours)
  }
  const note = document.createElement('div')
  note.className = 'note'
  note.textContent =
    'official climate shelter — check hours before you go'
  el.appendChild(note)
  if (p.web) {
    const a = document.createElement('a')
    a.href = p.web
    a.target = '_blank'
    a.rel = 'noopener noreferrer'
    a.textContent = 'official page ↗'
    el.appendChild(a)
  }
  new maplibregl.Popup({ maxWidth: '280px' })
    .setLngLat(f.geometry.coordinates)
    .setDOMContent(el)
    .addTo(map)
}
