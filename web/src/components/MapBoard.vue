<script setup>
import { onMounted, ref, watch } from 'vue'
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
  selected: Object, // {x, y, i} or null
  version: Number, // bumped by the parent to trigger a redraw
})
const emit = defineEmits(['select'])

const canvas = ref(null)
const readout = ref('hover the map — every pixel is a real 10 × 10 m')

function draw() {
  const { grid, meta, pledged, flipped, watched, candidates } = props
  if (!canvas.value || !grid) return
  const ctx = canvas.value.getContext('2d')
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
  if (props.selected) {
    const { x, y } = props.selected
    ctx.strokeStyle = '#ffffff'
    ctx.lineWidth = 1
    ctx.strokeRect(x - 2.5, y - 2.5, 5, 5)
    ctx.strokeStyle = '#111111'
    ctx.strokeRect(x - 3.5, y - 3.5, 7, 7)
  }
}

function pixelAt(e) {
  const { width, height } = props.meta
  const r = canvas.value.getBoundingClientRect()
  const x = Math.floor(((e.clientX - r.left) / r.width) * width)
  const y = Math.floor(((e.clientY - r.top) / r.height) * height)
  if (x < 0 || y < 0 || x >= width || y >= height) return null
  return { x, y, i: y * width + x }
}

function onMove(e) {
  const p = pixelAt(e)
  if (!p) return
  const v = props.grid[p.i]
  if (props.flipped.has(p.i)) readout.value = 'flipped — soil again'
  else if (props.pledged.has(p.i)) readout.value = 'pledged — click for details'
  else if (props.candidates.has(p.i)) {
    readout.value = 'candidate — click to pledge or watch these 100 m²'
  } else if (props.watched.has(p.i)) {
    readout.value = 'watched — a coalition is forming, click to join'
  } else if (v === SEA) readout.value = 'the sea'
  else if (v === NODATA) readout.value = 'no data'
  else readout.value = `${v}% sealed — click for details`
}

function onClick(e) {
  const p = pixelAt(e)
  if (p) emit('select', p)
}

onMounted(draw)
watch(() => props.version, draw)
watch(() => props.selected, draw)
</script>

<template>
  <div class="board">
    <canvas
      ref="canvas"
      :width="meta.width"
      :height="meta.height"
      @mousemove="onMove"
      @click="onClick"
    />
    <p class="readout">{{ readout }}</p>
  </div>
</template>

<style scoped>
.board canvas {
  width: 100%;
  height: auto;
  display: block;
  border-radius: 6px;
  image-rendering: pixelated;
  cursor: crosshair;
  border: 1px solid var(--line);
}
.readout {
  margin-top: 10px;
  font-size: 13.5px;
  color: var(--ink-2);
  font-variant-numeric: tabular-nums;
  min-height: 1.5em;
}
</style>
