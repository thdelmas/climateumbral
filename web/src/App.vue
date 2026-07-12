<script setup>
import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import EuroMap from './components/EuroMap.vue'
import PixelPanel from './components/PixelPanel.vue'
import Leaderboard from './components/Leaderboard.vue'
import ScoreBar from './components/ScoreBar.vue'
import Legend from './components/Legend.vue'
import MapControls from './components/MapControls.vue'
import { meanPenalty, flipsPerDegree, DAY_COEF, NIGHT_COEF }
  from './lib/heat.js'
import { inEurope } from './lib/proj.js'
import {
  myName,
  setMyName,
  tokenFor,
  rememberToken,
  forgetToken,
  allTokens,
  openedTotal,
  addOpened,
} from './lib/local.js'

const claims = shallowRef([]) // active claimViews
const watches = shallowRef([])
const claimAt = shallowRef(new Map()) // "pe,pn" -> claimView
const watchesAt = shallowRef(new Map()) // "pe,pn" -> watchViews
const leaders = ref([])
const version = ref(0)
const selected = ref(null) // {pe, pn} or null
const name = ref(myName())
const error = ref('')
const lastOpened = ref(null)
const pledgedM2 = ref(0)
const flippedM2 = ref(0)
const opened = ref(openedTotal())
const board = ref(null)
const raster = shallowRef(null) // viewport snapshot from EuroMap
const mode = ref(
  ['day', 'night'].includes(
    new URLSearchParams(location.search).get('view'),
  )
    ? new URLSearchParams(location.search).get('view')
    : 'land',
)

watch(name, (n) => setMyName(n.trim()))

const key = (pe, pn) => `${pe},${pn}`
const selKey = computed(() =>
  selected.value ? key(selected.value.pe, selected.value.pn) : null,
)

function setMode(m) {
  mode.value = m
  const q = m === 'land' ? '' : `?view=${m}`
  history.replaceState(null, '', location.pathname + q + location.hash)
}

// ---- viewport-derived numbers (from the EuroMap raster snapshot) ----

const candidateCount = computed(() => raster.value?.cands?.size ?? 0)
const nightAvg = computed(() =>
  raster.value ? meanPenalty(raster.value.Snight, NIGHT_COEF) : 0,
)
const selLocal = computed(() => {
  const r = raster.value
  const s = selected.value
  if (!r || !s) return -1
  const col = s.pe - r.pe0
  const row = r.H - 1 - (s.pn - r.pn0)
  if (col < 0 || row < 0 || col >= r.W || row >= r.H) return -1
  return row * r.W + col
})
const selValue = computed(() =>
  selLocal.value >= 0 ? raster.value.g[selLocal.value] : null,
)
const selHeat = computed(() => {
  const i = selLocal.value
  if (i < 0 || raster.value.Snight[i] < 0) return null
  return {
    day: DAY_COEF * raster.value.Sday[i],
    night: NIGHT_COEF * raster.value.Snight[i],
    flips: flipsPerDegree(raster.value.g, i, raster.value.C),
  }
})
const selIsCandidate = computed(
  () => selLocal.value >= 0 && raster.value.cands?.has(selLocal.value),
)

// ---- personal score ----

const mine = computed(() => {
  version.value // recompute when the ledger refreshes
  const list = []
  for (const k of Object.keys(allTokens('claim'))) {
    const c = claimAt.value.get(k)
    if (c) list.push(c)
  }
  return list
})
const mineKeys = computed(
  () => new Set(mine.value.map((c) => key(c.pe, c.pn))),
)
const myPledges = computed(() =>
  mine.value.filter((c) => c.status === 'pledged'),
)
const myFlipped = computed(() =>
  mine.value.filter((c) => c.status === 'flipped'),
)
const myWatchCount = computed(() => {
  version.value
  return Object.keys(allTokens('watch')).filter((k) =>
    watchesAt.value.has(k),
  ).length
})
const myRank = computed(() => {
  const n = name.value.trim()
  if (!n) return 0
  return leaders.value.findIndex((r) => r.name === n) + 1
})
const daysLeft = (c) =>
  Math.max(0, Math.ceil((new Date(c.deadline) - Date.now()) / 86_400_000))
const nextPledge = computed(() =>
  [...myPledges.value].sort(
    (a, b) => new Date(a.deadline) - new Date(b.deadline),
  )[0] ?? null,
)

const mission = computed(() => {
  if (!mine.value.length) {
    return {
      text:
        'Your first move: find a square on the front line — pledge it ' +
        'if it is yours to flip, watch it if not.',
      btn: '→ find me a square',
      goto: null,
    }
  }
  const p = nextPledge.value
  if (p) {
    return {
      text:
        `You promised square ${p.pe},${p.pn} — ` +
        `${daysLeft(p)} days left to flip it.`,
      btn: 'go to my pledge',
      goto: p,
    }
  }
  return {
    text:
      `All your pledges are flipped — ` +
      `${myFlipped.value.length * 100} m² breathing again. ` +
      'Open a new front?',
    btn: '→ find me a square',
    goto: null,
  }
})

function onMission() {
  const m = mission.value
  if (!m.goto) {
    board.value?.frontline()
    return
  }
  selected.value = { pe: m.goto.pe, pn: m.goto.pn }
  board.value?.goTo(m.goto.pe, m.goto.pn)
  history.replaceState(null, '', `#${m.goto.pe},${m.goto.pn}`)
}

// ---- ledger sync + acts ----

async function refresh() {
  const res = await fetch('/api/claims')
  const ledger = await res.json()
  const active = ledger.claims.filter((c) => c.status !== 'expired')
  const cm = new Map()
  for (const c of active) cm.set(key(c.pe, c.pn), c)
  const wm = new Map()
  for (const w of ledger.watches) {
    const k = key(w.pe, w.pn)
    wm.set(k, [...(wm.get(k) ?? []), w])
  }
  claims.value = active
  watches.value = ledger.watches
  claimAt.value = cm
  watchesAt.value = wm
  pledgedM2.value = ledger.pledged_m2
  flippedM2.value = ledger.flipped_m2
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
  const { pe, pn } = selected.value
  const before = candidateCount.value
  const res = await post('/api/claims', {
    pe,
    pn,
    name: name.value.trim(),
  })
  if (res.ok) {
    const { token } = await res.json()
    rememberToken('claim', pe, pn, token)
  }
  await refresh()
  if (res.ok) {
    lastOpened.value = Math.max(0, candidateCount.value - (before - 1))
    addOpened(lastOpened.value)
    opened.value = openedTotal()
  }
}

async function flip(photo) {
  const { pe, pn } = selected.value
  await post(`/api/claims/${pe}/${pn}/flip`, {
    token: tokenFor('claim', pe, pn),
    photo,
  })
  await refresh()
}

async function abandon() {
  const { pe, pn } = selected.value
  const res = await del(`/api/claims/${pe}/${pn}`,
    tokenFor('claim', pe, pn))
  if (res.status === 204) forgetToken('claim', pe, pn)
  await refresh()
}

async function watchPixel() {
  const { pe, pn } = selected.value
  const res = await post('/api/watches', {
    pe,
    pn,
    name: name.value.trim(),
  })
  if (res.ok) {
    const { token } = await res.json()
    rememberToken('watch', pe, pn, token)
  }
  await refresh()
}

async function unwatch() {
  const { pe, pn } = selected.value
  const res = await del(`/api/watches/${pe}/${pn}`,
    tokenFor('watch', pe, pn))
  if (res.status === 204) forgetToken('watch', pe, pn)
  await refresh()
}

function select(p) {
  selected.value = p
  history.replaceState(null, '', `#${p.pe},${p.pn}`)
}

onMounted(async () => {
  // live sync: any act by anyone refreshes every open map
  // (?nolive opts out — headless renderers hang on open streams)
  if (!new URLSearchParams(location.search).has('nolive')) {
    new EventSource('/api/events').addEventListener('ledger', refresh)
  }
  await refresh()
  const m = location.hash.match(/^#(\d+),(\d+)$/)
  if (m) {
    const pe = +m[1]
    const pn = +m[2]
    if (inEurope(pe, pn)) {
      selected.value = { pe, pn }
      board.value?.goTo(pe, pn)
    } else {
      // pre-V3 permalink (local grid coords) — not addressable anymore
      history.replaceState(null, '', location.pathname + location.search)
    }
  }
})
</script>

<template>
  <div class="wrap">
    <header>
      <h1>Tilewhip</h1>
      <p class="sub">
        Europe as the satellite sees it — every square a real
        10 × 10 m of ground. Gray is sealed. Green is alive. The game:
        turn gray into green, square by square, until the nights cool.
      </p>
      <ol class="steps">
        <li>
          Hit <b>“find me a square”</b> — it flies to an
          <b>orange</b> square: sealed ground touching life.
        </li>
        <li>
          Yours to flip (your yard, your façade)? <b>Pledge</b> it — a
          public promise to depave those 100 m² within 90 days.
        </li>
        <li>
          Not yours to touch (a road, a schoolyard — most squares)?
          <b>Watch</b> it — watchers form the coalition that gets it
          flipped.
        </li>
        <li>Depaved for real? <b>Mark it flipped</b> (photo link
          welcome).</li>
      </ol>
      <label class="who">
        I am
        <input v-model="name" placeholder="pseudonym (optional)" size="18" />
      </label>
    </header>

    <ScoreBar
      :mission="mission"
      :has-acts="mine.length > 0 || myWatchCount > 0"
      :my-flipped-m2="myFlipped.length * 100"
      :my-pledged-m2="myPledges.length * 100"
      :my-watch-count="myWatchCount"
      :opened="opened"
      :my-rank="myRank"
      :flipped-m2="flippedM2"
      :pledged-m2="pledgedM2"
      :candidate-count="candidateCount"
      :opened-label="lastOpened !== null
        ? (lastOpened
          ? `+${lastOpened} new candidates opened`
          : 'no new candidates opened')
        : null"
      :night-avg="nightAvg"
      @mission="onMission"
    />
    <p v-if="error" class="error">{{ error }}</p>

    <MapControls
      :mode="mode"
      @frontline="board?.frontline()"
      @mode="setMode"
    />

    <EuroMap
      ref="board"
      :claims="claims"
      :watches="watches"
      :mine-keys="mineKeys"
      :selected="selected"
      :mode="mode"
      :version="version"
      @select="select"
      @raster="(r) => (raster = r)"
    />
    <Legend :mode="mode" />

    <PixelPanel
      v-if="selected"
      :pixel="selected"
      :value="selValue"
      :claim="claimAt.get(selKey) ?? null"
      :watches="watchesAt.get(selKey) ?? []"
      :is-candidate="selIsCandidate"
      :my-claim-token="tokenFor('claim', selected.pe, selected.pn)"
      :my-watch-token="tokenFor('watch', selected.pe, selected.pn)"
      :day-delta="selHeat?.day ?? null"
      :night-delta="selHeat?.night ?? null"
      :flips-per-deg="selHeat?.flips ?? 0"
      :key="`${selKey}v${version}`"
      @pledge="pledge"
      @flip="flip"
      @abandon="abandon"
      @watch="watchPixel"
      @unwatch="unwatch"
    />
    <Leaderboard :rows="leaders" />

    <footer>
      <p>
        Data: © European Union, Copernicus Land Monitoring Service /
        EEA — Imperviousness Density 2018, 10 m. Basemap ©
        OpenStreetMap contributors. Claims are pledges; satellites keep
        the real score. The ledger stores only what this board shows;
        erase your acts anytime from their pixel.
      </p>
    </footer>
  </div>
</template>

<style scoped>
.wrap {
  max-width: 720px;
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
.steps {
  margin: 14px 0 0 0;
  padding-left: 22px;
  font-size: 14px;
  color: var(--ink-2);
  max-width: 60ch;
}
.steps li + li {
  margin-top: 4px;
}
.steps b {
  color: var(--ink);
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
