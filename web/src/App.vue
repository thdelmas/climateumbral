<script setup>
import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import MapBoard from './components/MapBoard.vue'
import PixelPanel from './components/PixelPanel.vue'
import Leaderboard from './components/Leaderboard.vue'
import { computeCandidates } from './lib/grid.js'
import {
  myName,
  setMyName,
  tokenFor,
  rememberToken,
  forgetToken,
} from './lib/local.js'

const meta = ref(null)
const grid = shallowRef(null)
const pledged = shallowRef(new Set())
const flipped = shallowRef(new Set())
const watched = shallowRef(new Set())
const claimAt = shallowRef(new Map())
const watchesAt = shallowRef(new Map())
const candidates = shallowRef(new Set())
const leaders = ref([])
const version = ref(0)
const selected = ref(null)
const name = ref(myName())
const error = ref('')
const lastOpened = ref(null)
const pledgedM2 = ref(0)
const flippedM2 = ref(0)

watch(name, (n) => setMyName(n.trim()))

const openedLabel = computed(() =>
  lastOpened.value
    ? `+${lastOpened.value} new candidates opened`
    : 'no new candidates opened',
)

const idx = (c) => c.y * meta.value.width + c.x

async function refresh() {
  const res = await fetch('/api/claims')
  const ledger = await res.json()
  const p = new Set()
  const f = new Set()
  const cm = new Map()
  for (const c of ledger.claims) {
    if (c.status === 'expired') continue
    cm.set(idx(c), c)
    if (c.status === 'flipped') f.add(idx(c))
    else p.add(idx(c))
  }
  const wm = new Map()
  for (const w of ledger.watches) {
    const i = idx(w)
    wm.set(i, [...(wm.get(i) ?? []), w])
  }
  pledged.value = p
  flipped.value = f
  watched.value = new Set(wm.keys())
  claimAt.value = cm
  watchesAt.value = wm
  pledgedM2.value = ledger.pledged_m2
  flippedM2.value = ledger.flipped_m2
  candidates.value = computeCandidates(
    grid.value,
    meta.value.width,
    meta.value.height,
    new Set([...p, ...f]),
  )
  version.value++
  leaders.value = await (await fetch('/api/leaderboard')).json()
}

async function post(path, body) {
  error.value = ''
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    error.value = (await res.json()).error ?? 'request failed'
  }
  return res
}

async function del(path, token) {
  error.value = ''
  const res = await fetch(path, {
    method: 'DELETE',
    headers: { 'X-Tilewhip-Token': token ?? '' },
  })
  if (res.status !== 204) {
    error.value = (await res.json()).error ?? 'request failed'
  }
  return res
}

async function pledge() {
  const { x, y } = selected.value
  const before = candidates.value.size
  const res = await post('/api/claims', {
    x,
    y,
    name: name.value.trim(),
  })
  if (res.ok) {
    const { token } = await res.json()
    rememberToken('claim', x, y, token)
  }
  await refresh()
  if (res.ok) {
    lastOpened.value = Math.max(
      0,
      candidates.value.size - (before - 1),
    )
  }
}

async function flip(photo) {
  const { x, y } = selected.value
  await post(`/api/claims/${x}/${y}/flip`, {
    token: tokenFor('claim', x, y),
    photo,
  })
  await refresh()
}

async function abandon() {
  const { x, y } = selected.value
  const res = await del(`/api/claims/${x}/${y}`, tokenFor('claim', x, y))
  if (res.status === 204) forgetToken('claim', x, y)
  await refresh()
}

async function watchPixel() {
  const { x, y } = selected.value
  const res = await post('/api/watches', {
    x,
    y,
    name: name.value.trim(),
  })
  if (res.ok) {
    const { token } = await res.json()
    rememberToken('watch', x, y, token)
  }
  await refresh()
}

async function unwatch() {
  const { x, y } = selected.value
  const res = await del(`/api/watches/${x}/${y}`, tokenFor('watch', x, y))
  if (res.status === 204) forgetToken('watch', x, y)
  await refresh()
}

function select(p) {
  selected.value = p
  history.replaceState(null, '', `#${p.x},${p.y}`)
}

onMounted(async () => {
  const [metaRes, rawRes] = await Promise.all([
    fetch('/api/grid'),
    fetch('/api/grid.raw'),
  ])
  meta.value = await metaRes.json()
  grid.value = new Uint8Array(await rawRes.arrayBuffer())
  await refresh()
  const m = location.hash.match(/^#(\d+),(\d+)$/)
  if (m) {
    const x = +m[1]
    const y = +m[2]
    if (x < meta.value.width && y < meta.value.height) {
      selected.value = { x, y, i: y * meta.value.width + x }
    }
  }
})
</script>

<template>
  <div class="wrap">
    <header>
      <h1>Tilewhip</h1>
      <p class="sub">
        Central Barcelona, one real 10 × 10 m pixel at a time. Orange
        squares are hard-sealed pixels touching green: pledge one, flip
        it within 90 days, and watch the front line open around it.
        Violet pixels have watchers — coalitions forming on land no one
        can flip alone.
      </p>
      <label class="who">
        I am
        <input v-model="name" placeholder="pseudonym (optional)" size="18" />
      </label>
    </header>

    <div v-if="meta" class="counter">
      <span>
        <strong>{{ flippedM2.toLocaleString() }}</strong> m² flipped
      </span>
      <span>
        <strong>{{ pledgedM2.toLocaleString() }}</strong> m² pledged
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

    <template v-if="grid">
      <MapBoard
        :grid="grid"
        :meta="meta"
        :pledged="pledged"
        :flipped="flipped"
        :watched="watched"
        :candidates="candidates"
        :selected="selected"
        :version="version"
        @select="select"
      />
      <PixelPanel
        v-if="selected"
        :pixel="selected"
        :value="grid[selected.i]"
        :meta="meta"
        :claim="claimAt.get(selected.i) ?? null"
        :watches="watchesAt.get(selected.i) ?? []"
        :is-candidate="candidates.has(selected.i)"
        :my-claim-token="tokenFor('claim', selected.x, selected.y)"
        :my-watch-token="tokenFor('watch', selected.x, selected.y)"
        :key="`${selected.i}v${version}`"
        @pledge="pledge"
        @flip="flip"
        @abandon="abandon"
        @watch="watchPixel"
        @unwatch="unwatch"
      />
      <Leaderboard :rows="leaders" />
    </template>
    <p v-else class="sub">loading the grid…</p>

    <footer>
      <p>
        Data: © European Union, Copernicus Land Monitoring Service /
        EEA — Imperviousness Density 2018, 10 m. Claims are pledges;
        satellites keep the real score. The ledger stores only what
        this board shows; erase your acts anytime from their pixel.
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
.who {
  display: inline-flex;
  gap: 8px;
  align-items: baseline;
  margin-top: 12px;
  font-size: 14px;
  color: var(--ink-2);
}
.who input {
  font: inherit;
  padding: 4px 10px;
  border-radius: 6px;
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--ink);
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
