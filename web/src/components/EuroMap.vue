<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import {
  toLAEA,
  fromLAEA,
  pixelCenter,
  inEurope,
} from '../lib/proj.js'
import { ledgerGeojson, selectionGeojson } from '../lib/ledgergeo.js'
import { blocksGeojson } from '../lib/blocks.js'
import { fetchAnchors, pickByExposure } from '../lib/anchors.js'
import { viewport3035, rasterContains, MAX_DIM } from '../lib/viewport.js'
import { tipTextAt } from '../lib/tiptext.js'
import {
  computeCandidates,
  colorFor,
  CANDIDATE_COLOR,
} from '../lib/grid.js'
import {
  sealedStats,
  heatColor,
  DAY_COEF,
  NIGHT_COEF,
} from '../lib/heat.js'
import { fetchRefuges, refugesGeojson } from '../lib/refuges.js'
import { coolSpots, coolSpotsGeojson } from '../lib/coolspots.js'

const props = defineProps({
  claims: Array, // active claimViews
  joins: Array, // joinViews — the standing petitions
  mineKeys: Set, // "pe,pn" keys whose tokens this browser holds
  selected: Object, // {pe, pn} or null
  mode: String, // 'land' | 'day' | 'night'
  version: Number, // ledger refresh counter
})
const emit = defineEmits(['select', 'raster', 'refuges'])

const EMPTY_FC = { type: 'FeatureCollection', features: [] }

const PLAY_ZOOM = 13.2
const EEA_PNG =
  'https://image.discomap.eea.europa.eu/arcgis/rest/services' +
  '/GioLandPublic/HRL_ImperviousnessDensity_2018/ImageServer' +
  '/exportImage?bbox={bbox-epsg-3857}&bboxSR=3857&imageSR=3857' +
  '&size=256,256&format=png&f=image'

const el = ref(null)
const tip = ref({ show: false, x: 0, y: 0, text: '' })
const hint = ref('zoom into a city to load the front line')
const loading = ref(false)

let map = null
let raster = null // {g, W, H, pe0, pn0, S, C, cands:Set(local idx)}
let overlay = null // offscreen canvas painted with candidates / heat
let pendingFrontline = false
let pendingGoTo = null

const localIdx = (pe, pn) => {
  if (!raster) return -1
  const col = pe - raster.pe0
  const row = raster.H - 1 - (pn - raster.pn0)
  const out = col < 0 || row < 0 || col >= raster.W || row >= raster.H
  return out ? -1 : row * raster.W + col
}

// ---- game raster: fetch viewport values, detect, paint ----

function updateHint() {
  const heat = props.mode !== 'land'
  hint.value = raster
    ? (heat
      ? 'hover for modeled °C — click a square for details'
      : 'click a square — orange is the front line · drag to pan')
    : (heat
      ? 'modeled °C appears at street level — zoom into a city'
      : 'zoom into a city to load the front line')
}

// Heat modes lean the sealed layer warmer at continental zoom.
function basemapMood() {
  if (!map?.getLayer('imd')) return
  const heat = props.mode !== 'land'
  map.setPaintProperty('imd', 'raster-opacity', heat ? 0.85 : 0.5)
  map.setPaintProperty('imd', 'raster-saturation', heat ? 0.5 : 0)
  map.setPaintProperty('imd', 'raster-contrast', heat ? 0.15 : 0)
}

async function refreshRaster() {
  if (!map) return
  if (map.getZoom() < PLAY_ZOOM) {
    raster = null
    updateHint()
    setOverlayVisible(false)
    map.getSource('coolspots')?.setData(EMPTY_FC)
    emit('raster', null)
    return
  }
  if (rasterContains(raster, map.getBounds())) {
    updateHint()
    return
  }
  const vp = viewport3035(map.getBounds())
  if (!vp) {
    hint.value = 'zoom in a little more to load the front line'
    setOverlayVisible(false)
    return
  }
  await fetchRasterBbox(vp)
}

// fetchRasterBbox loads one game raster and recomputes everything on
// it. Returns true on success.
async function fetchRasterBbox({ e0, n0, e1, n1 }) {
  loading.value = true
  try {
    const res = await fetch(`/api/raster?bbox=${e0},${n0},${e1},${n1}`)
    if (!res.ok) throw new Error((await res.json()).error)
    const [W, H] = res.headers
      .get('X-Raster-Size')
      .split(',')
      .map(Number)
    const [be0, bn0] = res.headers
      .get('X-Raster-Bbox')
      .split(',')
      .map(Number)
    const g = new Uint8Array(await res.arrayBuffer())
    raster = { g, W, H, pe0: be0 / 10, pn0: bn0 / 10, anchors: [] }
    recompute()
    updateHint()
    loadAnchors()
    return true
  } catch (err) {
    hint.value = `front line unavailable: ${err.message}`
    return false
  } finally {
    loading.value = false
  }
}

// Re-run detection + repaint from cached raster (claims changed, mode
// changed) without refetching values.
function recompute() {
  if (!raster) return
  const { g, W, H } = raster
  const claimedGreen = new Set() // depaves + trees extend the network
  const flippedActs = new Map() // only flipped acts cool the model
  for (const c of props.claims) {
    const i = localIdx(c.pe, c.pn)
    if (i < 0) continue
    if (c.kind !== 'coolroof') claimedGreen.add(i)
    if (c.status === 'flipped') flippedActs.set(i, c.kind)
  }
  raster.cands = computeCandidates(g, W, H, claimedGreen)
  const { Sday, Snight, C } = sealedStats(g, W, H, flippedActs)
  raster.Sday = Sday
  raster.Snight = Snight
  raster.C = C
  map.getSource('coolspots')?.setData(coolSpotsGeojson(coolSpots(raster)))
  paintOverlay()
  emit('raster', raster)
  if (pendingFrontline) {
    pendingFrontline = false
    frontline()
  }
}

function paintOverlay() {
  if (!raster) return
  const { g, W, H, cands } = raster
  const S = props.mode === 'day' ? raster.Sday : raster.Snight
  if (!overlay) overlay = document.createElement('canvas')
  overlay.width = W
  overlay.height = H
  const ctx = overlay.getContext('2d')
  const im = ctx.createImageData(W, H)
  const heat = props.mode !== 'land'
  const coef = props.mode === 'day' ? DAY_COEF : NIGHT_COEF
  for (let i = 0; i < g.length; i++) {
    let c = null
    let a = 0
    if (heat) {
      if (S[i] >= 0) {
        c = heatColor(coef * S[i], coef)
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
  const e0 = raster.pe0 * 10
  const n0 = raster.pn0 * 10
  const e1 = e0 + W * 10
  const n1 = n0 + H * 10
  const src = map.getSource('game')
  const quad = quadOf(e0, n0, e1, n1)
  if (src) {
    src.updateImage({ url: overlay.toDataURL(), coordinates: quad })
  } else {
    map.addSource('game', {
      type: 'image',
      url: overlay.toDataURL(),
      coordinates: quad,
    })
  }
  ensureGameLayer()
  setOverlayVisible(true)
}

// Pre-load paints anchor on whatever layer exists (see 'load').
function ensureGameLayer() {
  if (map.getLayer('game')) return
  const before = map.getLayer('claims-fill') ? 'claims-fill' : undefined
  map.addLayer(
    { id: 'game', type: 'raster', source: 'game',
      paint: { 'raster-resampling': 'nearest' } },
    before,
  )
}

function quadOf(e0, n0, e1, n1) {
  // TL, TR, BR, BL
  return [[e0, n1], [e1, n1], [e1, n0], [e0, n0]]
    .map(([e, n]) => fromLAEA(e, n))
}

function setOverlayVisible(on) {
  if (map?.getLayer('game')) {
    map.setLayoutProperty('game', 'visibility', on ? 'visible' : 'none')
  }
}

// ---- cool places: official shelters + modeled cool islands ----

// Two tiers, never blended (see refuges.js / coolspots.js): blue pins
// are rooms a city promised, green pins are model output. Cool
// islands only show in the heat views — they are a reading of that
// model and would masquerade as ground truth on the land map.
function addCoolPlaceLayers() {
  map.addSource('refuges', { type: 'geojson', data: EMPTY_FC })
  map.addLayer({
    id: 'refuges',
    type: 'circle',
    source: 'refuges',
    minzoom: 10,
    paint: {
      'circle-color': 'rgb(43, 108, 196)',
      'circle-radius': [
        'interpolate', ['linear'], ['zoom'], 10, 3.5, 16, 8,
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
    minzoom: PLAY_ZOOM - 0.5,
    layout: {
      visibility: props.mode === 'land' ? 'none' : 'visible',
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
    emit('refuges', sources)
    map?.getSource('refuges')?.setData(refugesGeojson(refuges))
  })
}

function coolSpotsVisible() {
  if (map?.getLayer('coolspots')) {
    map.setLayoutProperty('coolspots', 'visibility',
      props.mode === 'land' ? 'none' : 'visible')
  }
}

// pinAt: topmost cool-place feature under the cursor, if any.
function pinAt(point) {
  const layers = ['refuges', 'coolspots'].filter((l) => map.getLayer(l))
  if (!layers.length) return null
  return map.queryRenderedFeatures(point, { layers })[0] ?? null
}

// The popup builds DOM nodes, not HTML: dataset strings stay text.
function openRefugePopup(f) {
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
  const note = document.createElement('div')
  note.className = 'note'
  note.textContent =
    'official climate shelter — check hours before you go'
  el.appendChild(note)
  if (p.web) {
    const a = document.createElement('a')
    a.href = p.web
    a.target = '_blank'
    a.rel = 'noopener'
    a.textContent = 'official page ↗'
    el.appendChild(a)
  }
  new maplibregl.Popup({ maxWidth: '280px' })
    .setLngLat(f.geometry.coordinates)
    .setDOMContent(el)
    .addTo(map)
}

// ---- claims / blocks / selection as vector layers ----

function syncLedger() {
  map.getSource('claims')?.setData(
    ledgerGeojson(props.claims, props.mineKeys))
  map.getSource('blocks')?.setData(blocksGeojson(props.joins))
  map.getSource('selection')?.setData(selectionGeojson(props.selected))
}

// ---- interactions ----

async function frontline() {
  if (raster?.cands?.size) {
    pickAndGo()
    return
  }
  // search around the current center before teleporting anywhere
  const c = map.getCenter()
  const [E, N] = toLAEA(c.lng, c.lat)
  if (inEurope(Math.floor(E / 10), Math.floor(N / 10))) {
    hint.value = 'searching the front line around you…'
    const half = (MAX_DIM * 10) / 2 - 10
    const ok = await fetchRasterBbox({
      e0: E - half, n0: N - half, e1: E + half, n1: N + half,
    })
    if (ok && raster.cands.size) {
      pickAndGo()
      return
    }
  }
  hint.value = 'no front line nearby — flying to the seed city'
  pendingFrontline = true
  map.flyTo({ center: [2.165, 41.39], zoom: 15.5, speed: 2.4 })
}

function pickAndGo() {
  const i = pickByExposure(raster, [...raster.cands])
  const pe = raster.pe0 + (i % raster.W)
  const pn = raster.pn0 + (raster.H - 1 - Math.floor(i / raster.W))
  map.flyTo({ center: pixelCenter(pe, pn), zoom: 16.5, speed: 2.4 })
  emit('select', { pe, pn })
}

// Instant, not animated: permalinks and "go to my pledge" should
// arrive, not tour the continent.
function goTo(pe, pn) {
  if (!inEurope(pe, pn)) return
  if (!map) {
    pendingGoTo = [pe, pn]
    return
  }
  map.jumpTo({ center: pixelCenter(pe, pn), zoom: 16.5 })
}

onMounted(() => {
  map = new maplibregl.Map({
    container: el.value,
    center: [10, 51],
    zoom: 4,
    style: {
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
    },
    attributionControl: { compact: true },
  })
  map.addControl(new maplibregl.NavigationControl({ showCompass: false }))
  // "around me": the fix never leaves the browser — no accounts, no
  // server-side location, matching the ledger's data stance
  map.addControl(new maplibregl.GeolocateControl({
    positionOptions: { enableHighAccuracy: true },
    trackUserLocation: true,
  }))
  map.on('error', (e) => console.error('map:', e.error ?? e))
  map.on('load', () => {
    map.addSource('claims', {
      type: 'geojson',
      data: ledgerGeojson(props.claims, props.mineKeys),
    })
    map.addSource('blocks', {
      type: 'geojson',
      data: blocksGeojson(props.joins),
    })
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
      data: selectionGeojson(props.selected),
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
    addCoolPlaceLayers()
    // a pre-load paint may have added the game layer unanchored;
    // restore the intended order now that the claim layers exist
    if (map.getLayer('game')) map.moveLayer('game', 'claims-fill')
    basemapMood()
    refreshRaster()
    if (pendingGoTo) {
      goTo(...pendingGoTo)
      pendingGoTo = null
    }
  })
  map.on('moveend', refreshRaster)
  map.on('click', (e) => {
    // refuge pins show from city zoom, well before the game raster
    const pin = pinAt(e.point)
    if (pin?.layer.id === 'refuges') {
      openRefugePopup(pin)
      return
    }
    if (map.getZoom() < PLAY_ZOOM) return
    const [E, N] = toLAEA(e.lngLat.lng, e.lngLat.lat)
    emit('select', { pe: Math.floor(E / 10), pn: Math.floor(N / 10) })
  })
  map.on('mousemove', (e) => {
    const pin = pinAt(e.point)
    if (pin) {
      tip.value = {
        show: true,
        x: e.point.x + 14,
        y: e.point.y + 14,
        text: pin.properties.tip,
      }
      return
    }
    if (!raster) {
      tip.value.show = false
      return
    }
    const [E, N] = toLAEA(e.lngLat.lng, e.lngLat.lat)
    const text = tipTextAt(raster, props.claims, props.mode,
      Math.floor(E / 10), Math.floor(N / 10))
    tip.value = text
      ? { show: true, x: e.point.x + 14, y: e.point.y + 14, text }
      : { show: false, x: 0, y: 0, text: '' }
  })
})
onBeforeUnmount(() => map?.remove())

watch(() => props.version, () => {
  syncLedger()
  recompute()
})
watch(() => props.mode, () => {
  basemapMood()
  paintOverlay()
  coolSpotsVisible()
  updateHint()
})
watch(() => props.selected, () => map?.getSource('selection') &&
  syncLedger())

defineExpose({ frontline, goTo })
</script>

<template>
  <div class="euromap">
    <div ref="el" class="map" />
    <div
      v-if="tip.show"
      class="tooltip"
      :style="{ left: tip.x + 'px', top: tip.y + 'px' }"
    >
      {{ tip.text }}
    </div>
    <div v-if="loading" class="loading">loading the front line…</div>
    <p class="hint">
      {{ loading ? 'loading the front line…' : hint }}
    </p>
  </div>
</template>

<style scoped>
.euromap {
  position: relative;
}
.map {
  width: 100%;
  height: min(72vh, 640px);
  border-radius: 6px;
  border: 1px solid var(--line);
  overflow: hidden;
}
.tooltip {
  position: absolute;
  pointer-events: none;
  background: var(--ink);
  color: var(--bg);
  font-size: 12.5px;
  padding: 3px 9px;
  border-radius: 6px;
  white-space: nowrap;
  z-index: 3;
}
.loading {
  position: absolute;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--ink);
  color: var(--bg);
  font-size: 13px;
  font-weight: 600;
  padding: 6px 14px;
  border-radius: 999px;
  z-index: 3;
  pointer-events: none;
}
.hint {
  margin-top: 8px;
  font-size: 12.5px;
  color: var(--ink-3);
  text-align: center;
  min-height: 1.4em;
}
.map :deep(.refuge-pop) {
  font-size: 13px;
  line-height: 1.45;
  color: #1c1c22;
}
.map :deep(.refuge-pop strong) {
  display: block;
  margin-bottom: 2px;
}
.map :deep(.refuge-pop .note) {
  color: #6b6b74;
  font-size: 12px;
  margin: 4px 0;
}
.map :deep(.refuge-pop a) {
  color: rgb(43, 108, 196);
}
</style>
