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
import { fetchAnchors, pickByExposure } from '../lib/anchors.js'
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

const props = defineProps({
  claims: Array, // active claimViews
  watches: Array, // watchViews
  mineKeys: Set, // "pe,pn" keys whose tokens this browser holds
  selected: Object, // {pe, pn} or null
  mode: String, // 'land' | 'day' | 'night'
  version: Number, // ledger refresh counter
})
const emit = defineEmits(['select', 'mode', 'raster'])

const PLAY_ZOOM = 13.2
const MAX_DIM = 512
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

const key = (pe, pn) => `${pe},${pn}`
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
  if (!raster) {
    hint.value = heat
      ? 'modeled °C appears at street level — zoom into a city or ' +
        'hit "find me a square"'
      : 'zoom into a city to load the front line'
  } else {
    hint.value = heat
      ? 'hover for modeled °C — click a square for details'
      : 'click a square — orange is the front line · drag to pan'
  }
}

// Heat modes lean the continental sealed layer warmer and denser —
// the toggle answers even before street-level data loads.
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
    emit('raster', null)
    return
  }
  const bo = map.getBounds()
  const corners = [
    [bo.getWest(), bo.getSouth()],
    [bo.getEast(), bo.getSouth()],
    [bo.getWest(), bo.getNorth()],
    [bo.getEast(), bo.getNorth()],
  ].map(([lo, la]) => toLAEA(lo, la))
  let e0 = Math.min(...corners.map((c) => c[0]))
  let e1 = Math.max(...corners.map((c) => c[0]))
  let n0 = Math.min(...corners.map((c) => c[1]))
  let n1 = Math.max(...corners.map((c) => c[1]))
  if ((e1 - e0) / 10 > MAX_DIM || (n1 - n0) / 10 > MAX_DIM) {
    hint.value = 'zoom in a little more to load the front line'
    setOverlayVisible(false)
    return
  }
  // Panning inside the last fetch shouldn't refetch: if the viewport
  // still fits in the loaded raster, everything is already on screen.
  const r = raster
  if (
    r &&
    e0 >= r.pe0 * 10 && n0 >= r.pn0 * 10 &&
    e1 <= (r.pe0 + r.W) * 10 && n1 <= (r.pn0 + r.H) * 10
  ) {
    updateHint()
    return
  }
  // Fetch with margin so small pans and the zoom-in after "find me a
  // square" reuse the same raster instead of a new EEA round-trip.
  const pad = (hi, lo) =>
    Math.min((hi - lo) * 0.3, (MAX_DIM * 10 - (hi - lo)) / 2)
  const padE = Math.max(0, pad(e1, e0))
  const padN = Math.max(0, pad(n1, n0))
  e0 -= padE
  e1 += padE
  n0 -= padN
  n1 += padN
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
  } catch (err) {
    hint.value = `front line unavailable: ${err.message}`
  } finally {
    loading.value = false
  }
}

async function loadAnchors() {
  const r = raster
  r.anchors = await fetchAnchors(r)
  if (raster === r) emit('raster', r) // superseded otherwise
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

// A pre-load paint can't insert before claims-fill (not added yet);
// anchor on whatever exists — the load handler restores order.
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

// ---- claims / watches / selection as vector layers ----

function syncLedger() {
  map.getSource('claims')?.setData(
    ledgerGeojson(props.claims, props.watches, props.mineKeys))
  map.getSource('selection')?.setData(selectionGeojson(props.selected))
}

// ---- interactions ----

function frontline() {
  if (!raster || !raster.cands?.size) {
    pendingFrontline = true
    map.flyTo({ center: [2.165, 41.39], zoom: 15.5, speed: 2.4 })
    return
  }
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

function tipTextAt(pe, pn) {
  const k = key(pe, pn)
  const claim = props.claims.find((c) => key(c.pe, c.pn) === k)
  const i = localIdx(pe, pn)
  const v = i >= 0 ? raster.g[i] : null
  const S = props.mode === 'day' ? raster.Sday : raster.Snight
  if (props.mode !== 'land' && i >= 0 && S[i] >= 0) {
    const coef = props.mode === 'day' ? DAY_COEF : NIGHT_COEF
    const tag = raster.cands?.has(i) ? ' · candidate' : ''
    return `+${(coef * S[i]).toFixed(1)} °C ` +
      `${props.mode} (modeled)${tag}`
  }
  if (claim?.status === 'flipped') return 'flipped — soil again'
  if (claim) return 'pledged — click for details'
  if (i >= 0 && raster.cands?.has(i)) {
    return 'candidate — click: pledge it or watch it'
  }
  if (v === null) return null
  if (v > 100) return 'no data'
  return `${v}% sealed`
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
  map.on('error', (e) => console.error('map:', e.error ?? e))
  map.on('load', () => {
    map.addSource('claims', {
      type: 'geojson',
      data: ledgerGeojson(props.claims, props.watches, props.mineKeys),
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
          'pledged', 'rgb(235,179,66)',
          'rgb(150,118,220)', // watched
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
    if (map.getZoom() < PLAY_ZOOM) return
    const [E, N] = toLAEA(e.lngLat.lng, e.lngLat.lat)
    emit('select', { pe: Math.floor(E / 10), pn: Math.floor(N / 10) })
  })
  map.on('mousemove', (e) => {
    if (!raster) {
      tip.value.show = false
      return
    }
    const [E, N] = toLAEA(e.lngLat.lng, e.lngLat.lat)
    const text = tipTextAt(Math.floor(E / 10), Math.floor(N / 10))
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
</style>
