<script setup>
import { onMounted, ref, watch } from 'vue'
import {
  colorFor,
  CLAIMED_COLOR,
  CANDIDATE_COLOR,
  SEA,
  NODATA,
} from '../lib/grid.js'

const props = defineProps({
  grid: Uint8Array,
  meta: Object,
  claimed: Set,
  candidates: Set,
  version: Number, // bumped by the parent after each claim to trigger a redraw
})
const emit = defineEmits(['claim'])

const canvas = ref(null)
const readout = ref('hover the map — every pixel is a real 10 × 10 m')

function draw() {
  const { grid, meta, claimed, candidates } = props
  if (!canvas.value || !grid) return
  const ctx = canvas.value.getContext('2d')
  const im = ctx.createImageData(meta.width, meta.height)
  for (let i = 0; i < grid.length; i++) {
    let c
    if (claimed.has(i)) c = CLAIMED_COLOR
    else if (candidates.has(i)) c = CANDIDATE_COLOR
    else c = colorFor(grid[i])
    im.data[i * 4] = c[0]
    im.data[i * 4 + 1] = c[1]
    im.data[i * 4 + 2] = c[2]
    im.data[i * 4 + 3] = 255
  }
  ctx.putImageData(im, 0, 0)
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
  const { width, height, bbox_4326: bb } = props.meta
  const lon = (bb[0] + (p.x / width) * (bb[2] - bb[0])).toFixed(4)
  const lat = (bb[3] - (p.y / height) * (bb[3] - bb[1])).toFixed(4)
  if (props.claimed.has(p.i)) {
    readout.value = `claimed — 100 m² pledged at ≈ ${lat}, ${lon}`
  } else if (props.candidates.has(p.i)) {
    readout.value = `candidate — click to claim 100 m² (≈ ${lat}, ${lon})`
  } else if (v === SEA) {
    readout.value = 'the sea'
  } else if (v === NODATA) {
    readout.value = 'no data'
  } else {
    readout.value = `${v}% sealed — 10 × 10 m at ≈ ${lat}, ${lon}`
  }
}

function onClick(e) {
  const p = pixelAt(e)
  if (p && props.candidates.has(p.i)) emit('claim', p)
}

onMounted(draw)
watch(() => props.version, draw)
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
