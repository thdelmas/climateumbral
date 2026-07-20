<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch }
  from 'vue'
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
import { nearestRefuge, hoursLabel } from '../lib/refuges.js'
import { renderOverlay } from '../lib/overlay.js'
import { fetchCoolPlacesBBox, coolPlacesGeojson }
  from '../lib/coolplaces.js'
import {
  EMPTY_FC,
  baseStyle,
  basemapMood,
  addLedgerLayers,
  addCoolPlaceLayers,
  pinAt,
  openRefugePopup,
  openCoolPlacePopup,
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

// vestibular safety: continent-crossing camera flights become jumps
// for users who asked the OS for reduced motion
const REDUCED_MOTION =
  window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ??
  false

function glide(opts) {
  if (REDUCED_MOTION) {
    map.jumpTo({ center: opts.center, zoom: opts.zoom })
  } else {
    map.flyTo(opts)
  }
}

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
let hoverEvt = null // latest mousemove, processed once per frame
let hoverRaf = 0

const claimAt = computed(() => {
  const m = new Map()
  for (const c of props.claims) m.set(`${c.pe},${c.pn}`, c)
  return m
})

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
  // a programmatic fetch (frontline, seed flight) is in progress —
  // don't stack a viewport fetch on top of it
  if (loading.value) return
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

// claimsFingerprint captures the claims that fall inside the loaded
// raster — the only ledger state recompute() actually reads. Ledger
// events elsewhere in Europe leave it unchanged, so the version
// watcher can skip the full re-detection for them.
function claimsFingerprint() {
  const parts = []
  for (const c of props.claims) {
    const i = localIdx(c.pe, c.pn)
    if (i >= 0) parts.push(`${i}:${c.kind}:${c.status}`)
  }
  return parts.sort().join('|')
}

// Re-run detection + repaint from cached raster (claims changed, mode
// changed) without refetching values.
function recompute() {
  if (!raster) return
  raster.claimsFp = claimsFingerprint()
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

// The overlay rides a canvas source, not an image source: MapLibre
// reads the pixels straight off the canvas, where updateImage would
// PNG-encode (toDataURL) and decode 262k pixels on every repaint.
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
    src.setCoordinates(quad)
    // animate:false, so one play/pause uploads the repainted canvas:
    // pause() re-reads the canvas while still "playing", and a source
    // without tiles yet reads it on first draw anyway
    src.play?.()
    src.pause?.()
  } else {
    map.addSource('game', {
      type: 'canvas',
      canvas: overlay,
      coordinates: quad,
      animate: false,
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
      paint: {
        'raster-resampling': 'nearest',
        // no crossfade: a repainted overlay should swap, not flash
        'raster-fade-duration': 0,
        // translucent: the street map must stay readable under the
        // grid, or the player can't tell where their square is
        'raster-opacity': gameOpacity(),
      } },
    before,
  )
}

// heat reads better a touch denser than the land grid
function gameOpacity() {
  return props.mode === 'land' ? 0.62 : 0.8
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

// grantedPosition: the device fix, but only when the user already
// granted geolocation (to this site or via the 📍 control) — a
// "find" click must never spring a permission dialog. The fix moves
// the map and nothing else; it never leaves the browser.
async function grantedPosition() {
  try {
    const st =
      await navigator.permissions.query({ name: 'geolocation' })
    if (st.state !== 'granted') return null
    return await new Promise((resolve) => {
      navigator.geolocation.getCurrentPosition(
        (p) => resolve(p),
        () => resolve(null),
        { timeout: 4000, maximumAge: 300000 },
      )
    })
  } catch {
    return null
  }
}

// looksUrban: is a meaningful share of this raster's land built?
// The arbitrary map-center search must not seat a first-timer at a
// farm shed in open country — offline check, no extra fetch.
function looksUrban(r) {
  let built = 0
  let land = 0
  for (let i = 0; i < r.g.length; i++) {
    if (r.g[i] > 100) continue
    land++
    if (r.g[i] >= 30) built++
  }
  return land > 0 && built / land >= 0.12
}

const SEED = [2.165, 41.39] // Barcelona — the seed city

async function frontline() {
  if (raster?.cands?.size) {
    pickAndGo()
    return
  }
  const half = (MAX_DIM * 10) / 2 - 10
  // near the player first: their granted location beats wherever
  // the map happens to be pointing
  const pos = await grantedPosition()
  if (pos) {
    const [E, N] =
      toLAEA(pos.coords.longitude, pos.coords.latitude)
    if (inEurope(Math.floor(E / 10), Math.floor(N / 10))) {
      hint.value = 'searching the front line around you…'
      const ok = await fetchRasterBbox({
        e0: E - half, n0: N - half, e1: E + half, n1: N + half,
      })
      if (ok && raster.cands.size) {
        pickAndGo()
        return
      }
    }
  }
  // then around the current center — but only once the user has
  // actually aimed the map at somewhere lived-in; at continental
  // zoom the center is just the middle of the default view, and a
  // 512×512 fetch of countryside is a wasted five seconds
  const c = map.getCenter()
  const [E, N] = toLAEA(c.lng, c.lat)
  if (
    map.getZoom() >= 9 &&
    inEurope(Math.floor(E / 10), Math.floor(N / 10))
  ) {
    hint.value = 'searching the front line around you…'
    const ok = await fetchRasterBbox({
      e0: E - half, n0: N - half, e1: E + half, n1: N + half,
    })
    if (ok && raster.cands.size && looksUrban(raster)) {
      pickAndGo()
      return
    }
  }
  hint.value = 'no front line nearby — flying to the seed city'
  pendingFrontline = true
  // the seed raster downloads while the camera is still in the
  // air: the flight and the EEA fetch run concurrently, and
  // recompute picks the square the moment the data lands
  const [sE, sN] = toLAEA(SEED[0], SEED[1])
  const seedFetch = fetchRasterBbox({
    e0: sE - half, n0: sN - half, e1: sE + half, n1: sN + half,
  })
  glide({ center: SEED, zoom: 15.5, speed: 2.4 })
  await seedFetch
}

function pickAndGo() {
  const i = pickByExposure(raster, [...raster.cands])
  const pe = raster.pe0 + (i % raster.W)
  const pn = raster.pn0 + (raster.H - 1 - Math.floor(i / raster.W))
  glide({ center: pixelCenter(pe, pn), zoom: 16.5, speed: 2.4 })
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

// The not-official ring follows the viewport at city zoom. One
// fetch per settled view (quantized key skips repeats, in-flight
// guard skips pile-ups); a failed lookup leaves the last data
// rather than painting false emptiness. Wide views are clamped to
// a centered window inside the server's tile budget — a viewport
// slightly too broad must load its middle, not silently nothing
// (the first live report was exactly that silence).
const RING_ZOOM = PLAY_ZOOM - 1
const RING_MAX_W = 0.24 // ° lon — stays within the tile budget
const RING_MAX_H = 0.15 // ° lat
let coolPlacesKey = ''
let coolPlacesBusy = false
async function refreshCoolPlaces() {
  if (!map || map.getZoom() < RING_ZOOM || coolPlacesBusy) {
    return
  }
  const b = map.getBounds()
  let w = b.getWest()
  let e = b.getEast()
  let s = b.getSouth()
  let n = b.getNorth()
  if (e - w > RING_MAX_W) {
    const cx = (w + e) / 2
    w = cx - RING_MAX_W / 2
    e = cx + RING_MAX_W / 2
  }
  if (n - s > RING_MAX_H) {
    const cy = (s + n) / 2
    s = cy - RING_MAX_H / 2
    n = cy + RING_MAX_H / 2
  }
  const key = [w, s, e, n].map((v) => v.toFixed(3)).join(',')
  if (key === coolPlacesKey) return
  coolPlacesBusy = true
  let ok = false
  try {
    const res = await fetchCoolPlacesBBox(w, s, e, n)
    if (res) {
      ok = true
      coolPlacesKey = key
      map.getSource('osmcool')?.setData(coolPlacesGeojson(res.places))
    }
  } finally {
    coolPlacesBusy = false
  }
  // A fetch can outlive the view that asked for it (fresh tiles take
  // seconds); the moveends that arrived meanwhile were swallowed by
  // the busy guard. Catch up once — exits immediately when the view
  // hasn't moved, and a failed fetch does NOT loop (the next real
  // moveend retries instead).
  if (ok && map) refreshCoolPlaces()
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
        'Barcelona, Paris, Vienna and Lyon so far; more cities welcome'
    return
  }
  const c = map.getCenter()
  const best = nearestRefuge(refugeList, [c.lng, c.lat])
  glide({ center: [best.lon, best.lat], zoom: 15.5, speed: 2.4 })
  openRefugePopup(map, {
    properties: {
      name: best.name, addr: best.addr ?? '', web: best.web ?? '',
      hours: hoursLabel(best),
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
  // server-side location, matching the ledger's data stance. Coarse
  // fixes are plenty for 100 m² squares and spare GPS battery/CPU
  // while tracking.
  map.addControl(new maplibregl.GeolocateControl({
    trackUserLocation: true,
  }))
  map.on('error', (e) => console.error('map:', e.error ?? e))
  // ?nolive is the headless-driving mode (no SSE); expose the map
  // handle there so drivers can query rendered truth instead of
  // guessing from pixels. Never set on the live path.
  if (new URLSearchParams(location.search).has('nolive')) {
    window.__map = map
  }
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
  map.on('moveend', refreshCoolPlaces)
  map.on('click', (e) => {
    // refuge pins show at any zoom, well before the game raster
    const pin = pinAt(map, e.point)
    if (pin?.layer.id === 'refuge-clusters') {
      const to = {
        center: pin.geometry.coordinates,
        zoom: map.getZoom() + 2.5,
      }
      if (REDUCED_MOTION) map.jumpTo(to)
      else map.easeTo(to)
      return
    }
    if (pin?.layer.id === 'refuges') {
      openRefugePopup(map, pin)
      return
    }
    if (pin?.layer.id === 'osmcool') {
      openCoolPlacePopup(map, pin)
      return
    }
    if (map.getZoom() < PLAY_ZOOM) return
    const [E, N] = toLAEA(e.lngLat.lng, e.lngLat.lat)
    emit('select', { pe: Math.floor(E / 10), pn: Math.floor(N / 10) })
  })
  // hover work (queryRenderedFeatures + tooltip) runs at most once
  // per frame — mousemove can fire far faster than the display paints
  map.on('mousemove', (e) => {
    hoverEvt = e
    if (!hoverRaf) {
      hoverRaf = requestAnimationFrame(() => {
        hoverRaf = 0
        hover(hoverEvt)
      })
    }
  })
})

function hover(e) {
  if (!map) return
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
  const text = tipTextAt(raster, claimAt.value, props.mode,
    Math.floor(E / 10), Math.floor(N / 10))
  // an already-hidden tooltip stays hidden without touching the ref
  if (text) {
    tip.value = { show: true, x: e.point.x + 14, y: e.point.y + 14,
      text }
  } else {
    tip.value.show = false
  }
}
onBeforeUnmount(() => {
  cancelAnimationFrame(hoverRaf)
  map?.remove()
})

watch(() => props.version, () => {
  syncLedger()
  // most ledger events happen outside the loaded raster — repaint
  // the vector layers above, but skip the raster re-detection
  if (raster && raster.claimsFp === claimsFingerprint()) return
  recompute()
})
watch(() => props.mode, () => {
  basemapMood(map, props.mode !== 'land')
  paintOverlay()
  if (map?.getLayer('game')) {
    map.setPaintProperty('game', 'raster-opacity', gameOpacity())
  }
  coolSpotsVisible()
  updateHint()
})
watch(() => props.selected, () => map?.getSource('selection') &&
  syncLedger())

defineExpose({ frontline, goTo, shelterTonight })
</script>

<template>
  <div class="euromap">
    <div
      ref="el"
      class="map"
      role="region"
      aria-label="map of Europe — sealed ground, heat views, and
        cooling acts"
    />
    <div
      v-if="tip.show"
      class="tooltip"
      aria-hidden="true"
      :style="{ left: tip.x + 'px', top: tip.y + 'px' }"
    >
      {{ tip.text }}
    </div>
    <div v-if="loading" class="loading" role="status">
      loading the front line…
    </div>
    <p class="hint" aria-live="polite">
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
  /* svh: stable under mobile browser chrome; vh line is the
     fallback for engines without it */
  height: min(72vh, 640px);
  height: min(72svh, 640px);
  min-height: 320px;
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
