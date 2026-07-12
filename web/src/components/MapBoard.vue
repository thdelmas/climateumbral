<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  colorFor,
  FLIPPED_COLOR,
  PLEDGED_COLOR,
  WATCHED_COLOR,
  CANDIDATE_COLOR,
  SEA,
  NODATA,
} from '../lib/grid.js'

const props = defineProps({
  grid: Uint8Array,
  meta: Object,
  pledged: Set,
  flipped: Set,
  watched: Set,
  candidates: Set,
  mine: Set, // pixels whose claim token this browser holds
  selected: Object, // {x, y, i} or null
  version: Number, // bumped by the parent to trigger a redraw
})
const emit = defineEmits(['select'])

const wrap = ref(null)
const canvas = ref(null)
const tip = ref({ show: false, x: 0, y: 0, text: '' })
const zoomPct = ref(100)

const MIN_ZOOM = 1
const MAX_ZOOM = 24
let off = null // offscreen canvas holding the full grid, 1px per pixel
let cssW = 640
let cssH = 640
const view = { zoom: 1, cx: 0, cy: 0 } // cx/cy: grid coords at center
let dragging = false
let moved = 0
let last = null

const scale = () => (cssW / props.meta.width) * view.zoom

function paintOffscreen() {
  const { grid, meta, pledged, flipped, watched, candidates } = props
  const ctx = off.getContext('2d')
  const im = ctx.createImageData(meta.width, meta.height)
  for (let i = 0; i < grid.length; i++) {
    let c
    if (flipped.has(i)) c = FLIPPED_COLOR
    else if (pledged.has(i)) c = PLEDGED_COLOR
    else if (candidates.has(i)) c = CANDIDATE_COLOR
    else if (watched.has(i)) c = WATCHED_COLOR
    else c = colorFor(grid[i])
    im.data[i * 4] = c[0]
    im.data[i * 4 + 1] = c[1]
    im.data[i * 4 + 2] = c[2]
    im.data[i * 4 + 3] = 255
  }
  ctx.putImageData(im, 0, 0)
}

function clampView() {
  const { width, height } = props.meta
  view.zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, view.zoom))
  view.cx = Math.min(width, Math.max(0, view.cx))
  view.cy = Math.min(height, Math.max(0, view.cy))
  zoomPct.value = Math.round(view.zoom * 100)
}

function draw() {
  if (!canvas.value || !off) return
  const dpr = window.devicePixelRatio || 1
  canvas.value.width = cssW * dpr
  canvas.value.height = cssH * dpr
  const ctx = canvas.value.getContext('2d')
  ctx.imageSmoothingEnabled = false
  const s = scale()
  ctx.setTransform(
    dpr * s, 0, 0, dpr * s,
    dpr * (cssW / 2 - view.cx * s),
    dpr * (cssH / 2 - view.cy * s),
  )
  ctx.drawImage(off, 0, 0)
  for (const i of props.mine ?? []) {
    const x = i % props.meta.width
    const y = Math.floor(i / props.meta.width)
    ctx.lineWidth = 1.6 / s
    ctx.strokeStyle = 'rgba(0,0,0,.85)'
    ctx.strokeRect(x + 0.08, y + 0.08, 0.84, 0.84)
    ctx.lineWidth = 0.8 / s
    ctx.strokeStyle = '#ffffff'
    ctx.strokeRect(x + 0.08, y + 0.08, 0.84, 0.84)
  }
  if (props.selected) {
    const { x, y } = props.selected
    ctx.lineWidth = 4 / s
    ctx.strokeStyle = 'rgba(0,0,0,.8)'
    ctx.strokeRect(x - 0.5, y - 0.5, 2, 2)
    ctx.lineWidth = 2 / s
    ctx.strokeStyle = '#ffffff'
    ctx.strokeRect(x - 0.5, y - 0.5, 2, 2)
  }
}

function toGrid(e) {
  const r = canvas.value.getBoundingClientRect()
  const s = scale()
  return {
    gx: view.cx + (e.clientX - r.left - cssW / 2) / s,
    gy: view.cy + (e.clientY - r.top - cssH / 2) / s,
  }
}

function pixelAt(e) {
  const { gx, gy } = toGrid(e)
  const x = Math.floor(gx)
  const y = Math.floor(gy)
  const { width, height } = props.meta
  if (x < 0 || y < 0 || x >= width || y >= height) return null
  return { x, y, i: y * width + x }
}

function tipText(p) {
  const v = props.grid[p.i]
  const yours = props.mine?.has(p.i) ? 'your ' : ''
  if (props.flipped.has(p.i)) return `${yours}flip — soil again`
  if (props.pledged.has(p.i)) return `${yours}pledge — click for details`
  if (props.candidates.has(p.i)) return 'candidate! click to claim it'
  if (props.watched.has(p.i)) return 'watched — click to join'
  if (v === SEA) return 'the sea'
  if (v === NODATA) return 'no data'
  return `${v}% sealed`
}

function onPointerDown(e) {
  dragging = true
  moved = 0
  last = { x: e.clientX, y: e.clientY }
  canvas.value.setPointerCapture(e.pointerId)
}

function onPointerMove(e) {
  if (dragging && last) {
    const dx = e.clientX - last.x
    const dy = e.clientY - last.y
    moved += Math.abs(dx) + Math.abs(dy)
    if (moved > 4) {
      const s = scale()
      view.cx -= dx / s
      view.cy -= dy / s
      clampView()
      draw()
      tip.value.show = false
    }
    last = { x: e.clientX, y: e.clientY }
    if (moved > 4) return
  }
  const p = pixelAt(e)
  if (!p) {
    tip.value.show = false
    return
  }
  const r = wrap.value.getBoundingClientRect()
  tip.value = {
    show: true,
    x: e.clientX - r.left + 14,
    y: e.clientY - r.top + 14,
    text: tipText(p),
  }
}

function onPointerUp(e) {
  dragging = false
  if (moved <= 4) {
    const p = pixelAt(e)
    if (p) emit('select', p)
  }
}

function onWheel(e) {
  e.preventDefault()
  const { gx, gy } = toGrid(e)
  const factor = e.deltaY < 0 ? 1.3 : 1 / 1.3
  const before = view.zoom
  view.zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, view.zoom * factor))
  if (view.zoom !== before) {
    // keep the grid point under the cursor fixed while zooming
    const r = canvas.value.getBoundingClientRect()
    const s = scale()
    view.cx = gx - (e.clientX - r.left - cssW / 2) / s
    view.cy = gy - (e.clientY - r.top - cssH / 2) / s
  }
  clampView()
  draw()
}

function zoomBy(factor) {
  view.zoom *= factor
  clampView()
  draw()
}

function fit() {
  view.zoom = 1
  view.cx = props.meta.width / 2
  view.cy = props.meta.height / 2
  clampView()
  draw()
}

function goTo(x, y, zoom = 14) {
  view.cx = x + 0.5
  view.cy = y + 0.5
  view.zoom = zoom
  clampView()
  draw()
}

// Jump to a random candidate, zoomed in enough to click comfortably.
function frontline() {
  const arr = [...props.candidates]
  if (!arr.length) return
  const i = arr[Math.floor(Math.random() * arr.length)]
  const x = i % props.meta.width
  const y = Math.floor(i / props.meta.width)
  goTo(x, y)
  emit('select', { x, y, i })
}

function resize() {
  cssW = wrap.value.clientWidth
  cssH = Math.round((cssW * props.meta.height) / props.meta.width)
  draw()
}

onMounted(() => {
  off = document.createElement('canvas')
  off.width = props.meta.width
  off.height = props.meta.height
  paintOffscreen()
  fit()
  resize()
  window.addEventListener('resize', resize)
})
onBeforeUnmount(() => window.removeEventListener('resize', resize))

watch(() => props.version, () => {
  paintOffscreen()
  draw()
})
watch(() => props.selected, draw)
watch(() => props.mine, draw)

defineExpose({ frontline, goTo })
</script>

<template>
  <div class="board">
    <div class="controls">
      <button class="go" @click="frontline">
        → find me a square
      </button>
      <span class="spacer" />
      <button @click="zoomBy(1 / 1.5)" aria-label="zoom out">−</button>
      <span class="zoom">{{ zoomPct }}%</span>
      <button @click="zoomBy(1.5)" aria-label="zoom in">+</button>
      <button @click="fit">fit</button>
    </div>
    <div ref="wrap" class="canvas-wrap">
      <canvas
        ref="canvas"
        :style="{ width: '100%', height: 'auto', aspectRatio: '1' }"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointerleave="tip.show = false"
        @wheel="onWheel"
      />
      <div
        v-if="tip.show"
        class="tooltip"
        :style="{ left: tip.x + 'px', top: tip.y + 'px' }"
      >
        {{ tip.text }}
      </div>
    </div>
    <p class="hint">scroll to zoom · drag to pan · click a square</p>
  </div>
</template>

<style scoped>
.controls {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
.controls .spacer {
  flex: 1;
}
.controls button {
  font: inherit;
  font-size: 13.5px;
  padding: 5px 12px;
  border-radius: 999px;
  cursor: pointer;
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--ink);
  min-width: 34px;
}
.controls button.go {
  background: var(--accent);
  color: var(--bg);
  border-color: var(--accent);
  font-weight: 600;
}
.zoom {
  font-size: 12.5px;
  color: var(--ink-3);
  font-variant-numeric: tabular-nums;
  min-width: 44px;
  text-align: center;
}
.canvas-wrap {
  position: relative;
}
.canvas-wrap canvas {
  display: block;
  border-radius: 6px;
  cursor: crosshair;
  border: 1px solid var(--line);
  touch-action: none;
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
.hint {
  margin-top: 8px;
  font-size: 12.5px;
  color: var(--ink-3);
  text-align: center;
}
</style>
