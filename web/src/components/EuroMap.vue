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
import { viewport3035, rasterContains, MAX_DIM }
  from '../lib/viewport.js'
import { tipTextAt } from '../lib/tiptext.js'
import { computeCandidates } from '../lib/grid.js'
import { sealedStats } from '../lib/heat.js'
import { coolSpots, coolSpotsGeojson } from '../lib/coolspots.js'
import { nearestRefuge } from '../lib/refuges.js'
import { renderOverlay } from '../lib/overlay.js'
import {
  EMPTY_FC,
  baseStyle,
  basemapMood,
  addLedgerLayers,
  addCoolPlaceLayers,
  pinAt,
  openRefugePopup,
} from '../lib/maplayers.js'

const props = defineProps({
  claims: Array, // active claimViews
  joins: Array, // joinViews — the standing petitions
  mineKeys: Set, // "pe,pn" keys whose tokens this browser holds
  selected: Object, // {pe, pn} or null
  mode: String, // 'land' | 'day' | 'night'
  version: Number, // ledger refresh counter
})
const emit = defineEmits(['select', 'raster', 'refuges'])

const PLAY_ZOOM = 13.2

const el = ref(null)
const tip = ref({ show: false, x: 0, y: 0, text: '' })
const hint = ref('zoom into a city to load the front line')
const loading = ref(false)

let map = null
let raster = null // {g, W, H, pe0, pn0, S, C, cands:Set(local idx)}
let overlay = null // offscreen canvas painted with candidates / heat
let pendingFrontline = false
let pendingGoTo = null
let refugeList = [] // loaded shelter pins, for "nearest shelter"
let refugeSources = null // adapter status, for honest absence

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
    const res =
      await fetch(`/api/raster?bbox=${e0},${n0},${e1},${n1}`)
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

// loadAnchors fills the current raster's human-hour anchors after
// the fact — the front line paints first, exposure ranking follows.
async function loadAnchors() {
  const r = raster
  if (!r) return
  const anchors = await fetchAnchors(r)
  if (raster !== r) return // a newer raster landed meanwhile
  r.anchors = anchors
  emit('raster', r) // re-emit: nearestAnchor labels can render now
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
  map.getSource('coolspots')
    ?.setData(coolSpotsGeojson(coolSpots(raster)))
  paintOverlay()
  emit('raster', raster)
  if (pendingFrontline) {
    pendingFrontline = false
    frontline()
  }
}

function paintOverlay() {
  if (!raster) return
  if (!overlay) overlay = document.createElement('canvas')
  renderOverlay(raster, props.mode, overlay)
  const e0 = raster.pe0 * 10
  const n0 = raster.pn0 * 10
  const e1 = e0 + raster.W * 10
  const n1 = n0 + raster.H * 10
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
  const before = map.getLayer('claims-fill') ? 'claims-fill'
    : undefined
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
    map.setLayoutProperty('game', 'visibility',
      on ? 'visible' : 'none')
  }
}

function coolSpotsVisible() {
  if (map?.getLayer('coolspots')) {
    map.setLayoutProperty('coolspots', 'visibility',
      props.mode === 'land' ? 'none' : 'visible')
  }
}

// ---- claims / blocks / selection as vector layers ----

function syncLedger() {
  map.getSource('claims')?.setData(
    ledgerGeojson(props.claims, props.mineKeys))
  map.getSource('blocks')?.setData(blocksGeojson(props.joins))
  map.getSource('selection')
    ?.setData(selectionGeojson(props.selected))
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

// shelterTonight flies to the closest official shelter from where
// the user is looking (tap 📍 first and it is "closest to me").
// Absence stays honest: no network here is said, not shown as an
// empty map.
function shelterTonight() {
  if (!map) return
  if (!refugeList.length) {
    hint.value = refugeSources === null
      ? 'shelter data unavailable right now — try again in a minute'
      : 'no shelter network published here yet — ' +
        'Barcelona is the first adapter; more cities welcome'
    return
  }
  const c = map.getCenter()
  const best = nearestRefuge(refugeList, [c.lng, c.lat])
  map.flyTo({ center: [best.lon, best.lat], zoom: 15.5, speed: 2.4 })
  openRefugePopup(map, {
    properties: {
      name: best.name, addr: best.addr ?? '', web: best.web ?? '',
    },
    geometry: { coordinates: [best.lon, best.lat] },
  })
  const km = Math.round(best.km)
  hint.value = best.km > 50
    ? `nearest published shelter network is ~${km} km away — ` +
      'your city may not publish one yet'
    : `nearest official shelter: ${best.name}` +
      (best.km >= 1 ? ` — ~${km} km` : '')
}

onMounted(() => {
  map = new maplibregl.Map({
    container: el.value,
    center: [10, 51],
    zoom: 4,
    style: baseStyle(),
    attributionControl: { compact: true },
  })
  map.addControl(
    new maplibregl.NavigationControl({ showCompass: false }))
  // "around me": the fix never leaves the browser — no accounts, no
  // server-side location, matching the ledger's data stance
  map.addControl(new maplibregl.GeolocateControl({
    positionOptions: { enableHighAccuracy: true },
    trackUserLocation: true,
  }))
  map.on('error', (e) => console.error('map:', e.error ?? e))
  map.on('load', () => {
    addLedgerLayers(map, {
      claims: ledgerGeojson(props.claims, props.mineKeys),
      blocks: blocksGeojson(props.joins),
      selection: selectionGeojson(props.selected),
    })
    addCoolPlaceLayers(map, PLAY_ZOOM - 0.5,
      props.mode === 'land', (sources, refuges) => {
        refugeSources = sources
        refugeList = refuges
        emit('refuges', sources)
      })
    // a pre-load paint may have added the game layer unanchored;
    // restore the intended order now that the claim layers exist
    if (map.getLayer('game')) map.moveLayer('game', 'claims-fill')
    basemapMood(map, props.mode !== 'land')
    refreshRaster()
    if (pendingGoTo) {
      goTo(...pendingGoTo)
      pendingGoTo = null
    }
  })
  map.on('moveend', refreshRaster)
  map.on('click', (e) => {
    // refuge pins show at any zoom, well before the game raster
    const pin = pinAt(map, e.point)
    if (pin?.layer.id === 'refuge-clusters') {
      map.easeTo({
        center: pin.geometry.coordinates,
        zoom: map.getZoom() + 2.5,
      })
      return
    }
    if (pin?.layer.id === 'refuges') {
      openRefugePopup(map, pin)
      return
    }
    if (map.getZoom() < PLAY_ZOOM) return
    const [E, N] = toLAEA(e.lngLat.lng, e.lngLat.lat)
    emit('select', { pe: Math.floor(E / 10), pn: Math.floor(N / 10) })
  })
  map.on('mousemove', (e) => {
    const pin = pinAt(map, e.point)
    if (pin) {
      const n = pin.properties.point_count
      tip.value = {
        show: true,
        x: e.point.x + 14,
        y: e.point.y + 14,
        text: n
          ? `${n} official climate shelters — click to zoom`
          : pin.properties.tip,
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
  basemapMood(map, props.mode !== 'land')
  paintOverlay()
  coolSpotsVisible()
  updateHint()
})
watch(() => props.selected, () => map?.getSource('selection') &&
  syncLedger())

defineExpose({ frontline, goTo, shelterTonight })
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
