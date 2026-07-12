<script setup>
import { computed, onMounted, ref, shallowRef } from 'vue'
import MapBoard from './components/MapBoard.vue'
import { computeCandidates } from './lib/grid.js'

const meta = ref(null)
const grid = shallowRef(null)
const claimed = shallowRef(new Set())
const candidates = shallowRef(new Set())
const version = ref(0) // redraw trigger: the Sets are swapped, not mutated
const m2 = ref(0)
const lastOpened = ref(null)
const error = ref('')

const hectares = computed(() => (m2.value / 10_000).toFixed(2))
const openedLabel = computed(() =>
  lastOpened.value
    ? `+${lastOpened.value} new candidates opened`
    : 'no new candidates opened',
)

function recompute() {
  candidates.value = computeCandidates(
    grid.value,
    meta.value.width,
    meta.value.height,
    claimed.value,
  )
  version.value++
}

async function claim({ x, y, i }) {
  error.value = ''
  const res = await fetch('/api/claims', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ x, y }),
  })
  if (!res.ok) {
    error.value = (await res.json()).error ?? 'claim failed'
    return
  }
  const before = candidates.value.size
  claimed.value = new Set(claimed.value).add(i)
  m2.value += 100
  recompute()
  // -1: the claimed pixel left the candidate set; anything beyond
  // that is newly opened front line.
  lastOpened.value = Math.max(0, candidates.value.size - (before - 1))
}

onMounted(async () => {
  const [metaRes, rawRes, claimsRes] = await Promise.all([
    fetch('/api/grid'),
    fetch('/api/grid.raw'),
    fetch('/api/claims'),
  ])
  meta.value = await metaRes.json()
  grid.value = new Uint8Array(await rawRes.arrayBuffer())
  const ledger = await claimsRes.json()
  const idx = (c) => c.y * meta.value.width + c.x
  claimed.value = new Set(ledger.claims?.map(idx))
  m2.value = ledger.m2 ?? 0
  recompute()
})
</script>

<template>
  <div class="wrap">
    <header>
      <h1>Tilewhip</h1>
      <p class="sub">
        Central Barcelona, one real 10 × 10 m pixel at a time. Orange
        squares are hard-sealed pixels touching green — claim one, and
        watch the front line open around it.
      </p>
    </header>

    <div v-if="meta" class="counter">
      <span>
        <strong>{{ claimed.size.toLocaleString() }}</strong> tiles claimed
      </span>
      <span>
        = <strong>{{ m2.toLocaleString() }}</strong>
        m² ({{ hectares }} ha) pledged
      </span>
      <span>
        <strong>{{ candidates.size.toLocaleString() }}</strong>
        candidates on the front line
      </span>
      <span v-if="lastOpened !== null" class="opened">
        {{ openedLabel }}
      </span>
    </div>
    <p v-if="error" class="error">{{ error }}</p>

    <MapBoard
      v-if="grid"
      :grid="grid"
      :meta="meta"
      :claimed="claimed"
      :candidates="candidates"
      :version="version"
      @claim="claim"
    />
    <p v-else class="sub">loading the grid…</p>

    <footer>
      <p>
        Data: © European Union, Copernicus Land Monitoring Service /
        EEA — Imperviousness Density 2018, 10 m. Claims are pledges;
        satellites keep the real score.
      </p>
    </footer>
  </div>
</template>

<style scoped>
.wrap {
  max-width: 640px;
  margin: 0 auto;
  padding: clamp(16px, 4vw, 48px) clamp(12px, 4vw, 40px) 80px;
}
header h1 {
  font-size: clamp(28px, 5vw, 44px);
  font-weight: 800;
  letter-spacing: -0.02em;
  margin-bottom: 6px;
}
.sub {
  color: var(--ink-2);
  max-width: 60ch;
}
.counter {
  display: flex;
  gap: 18px;
  flex-wrap: wrap;
  align-items: baseline;
  margin: 20px 0 14px;
  padding: 12px 16px;
  border-radius: 8px;
  background: var(--card);
  border: 1px solid var(--line);
  font-variant-numeric: tabular-nums;
  font-size: 14.5px;
}
.counter strong {
  font-size: 20px;
  color: var(--accent);
}
.opened {
  color: var(--accent);
  font-weight: 600;
}
.error {
  color: #b3423a;
  margin-bottom: 10px;
  font-size: 14px;
}
footer {
  margin-top: 32px;
  font-size: 13px;
  color: var(--ink-3);
}
footer p {
  max-width: 75ch;
}
</style>
