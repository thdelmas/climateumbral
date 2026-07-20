<script setup>
import { computed, nextTick, onMounted, ref, shallowRef, watch }
  from 'vue'
import EuroMap from './components/EuroMap.vue'
import IntroHeader from './components/IntroHeader.vue'
import PixelPanel from './components/PixelPanel.vue'
import Leaderboard from './components/Leaderboard.vue'
import ScoreBar from './components/ScoreBar.vue'
import Legend from './components/Legend.vue'
import MapControls from './components/MapControls.vue'
import Learn from './components/Learn.vue'
import { meanPenalty, greenSealedSpread, flipsPerDegree, DAY_COEF,
  NIGHT_COEF } from './lib/heat.js'
import { inEurope } from './lib/proj.js'
import { nearestAnchor } from './lib/anchors.js'
import { actNightMC, blockOf, blockKey, blockCoolingSince }
  from './lib/blocks.js'
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
const joins = shallowRef([]) // joinViews — standing petitions
const claimAt = shallowRef(new Map()) // "pe,pn" -> claimView
const leaders = ref([])
const version = ref(0)
const selected = ref(null) // {pe, pn} or null
const name = ref(myName())
const error = ref('')
const lastOpened = ref(null)
const pledgedM2 = ref(0)
const flippedM2 = ref(0)
const nightMC = ref(0)
const opened = ref(openedTotal())
const board = ref(null)
const raster = shallowRef(null) // viewport snapshot from EuroMap
const refugeSources = shallowRef(null) // null until /api/refuges answers
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
  selected.value ? key(selected.value.pe, selected.value.pn) : null)

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
// the sensibilization number: the land map and the night map as one
// felt gap — green ground vs sealed ground in the current view
const nightSpread = computed(() =>
  raster.value
    ? greenSealedSpread(raster.value.g, raster.value.Snight,
      NIGHT_COEF)
    : 0,
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
const selAnchor = computed(() =>
  raster.value ? nearestAnchor(raster.value, selLocal.value) : null,
)
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
// ---- my petition blocks ----
const selBlock = computed(() =>
  selected.value ? blockOf(selected.value.pe, selected.value.pn) : null)
const selBlockKey = computed(() =>
  selBlock.value ? blockKey(...selBlock.value) : null)
const selBlockJoins = computed(() =>
  joins.value.filter((j) => blockKey(j.be, j.bn) === selBlockKey.value))
const myJoins = computed(() => {
  version.value
  const keys = new Set(Object.keys(allTokens('join')))
  return joins.value.filter((j) => keys.has(blockKey(j.be, j.bn)))
})
const myBlockMC = computed(() => {
  const mineJ = myJoins.value
  if (!mineJ.length) return 0
  const sum = mineJ.reduce(
    (s, j) => s + blockCoolingSince(claims.value, j.be, j.bn, j.ts),
    0,
  )
  return sum / mineJ.length
})
const myRank = computed(() => {
  const n = name.value.trim()
  if (!n) return 0
  return leaders.value.findIndex((r) => r.name === n) + 1
})
// same model as the server and the block math — one source of truth
const myNightMC = computed(() =>
  myFlipped.value.reduce((sum, c) => sum + actNightMC(c), 0),
)
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
        'Your first move: find a sealed square — pledge a cooling ' +
        'act if it is yours, join the block petition if not.',
      btn: '→ find me a square',
      goto: null,
    }
  }
  const p = nextPledge.value
  if (p) {
    return {
      text:
        `You promised a ${p.kind === 'coolroof' ? 'cool surface'
          : p.kind} at square ${p.pe},${p.pn} — ` +
        `${daysLeft(p)} days left to do it.`,
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

let refreshSeq = 0 // drop out-of-order responses

async function refresh() {
  const seq = ++refreshSeq
  try {
    const res = await fetch('/api/claims')
    const ledger = await res.json()
    if (seq !== refreshSeq) return // a newer refresh already landed
    const active = ledger.claims.filter((c) => c.status !== 'expired')
    const cm = new Map()
    for (const c of active) cm.set(key(c.pe, c.pn), c)
    claims.value = active
    joins.value = ledger.joins ?? []
    claimAt.value = cm
    pledgedM2.value = ledger.pledged_m2
    flippedM2.value = ledger.flipped_m2
    nightMC.value = ledger.night_mdegc ?? 0
    version.value++
    const rows = await (await fetch('/api/leaderboard')).json()
    if (seq === refreshSeq) leaders.value = rows
  } catch {
    // a board that never loaded should say so; a live board that
    // missed one beat will catch the next event
    if (seq === refreshSeq && !claims.value.length) {
      error.value = 'the board is unreachable — is the API running?'
    }
  }
}

async function errText(res) {
  try {
    return (await res.json()).error ?? 'request failed'
  } catch {
    return `request failed (${res.status})`
  }
}

async function post(path, body) {
  error.value = ''
  try {
    const res = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!res.ok) error.value = await errText(res)
    return res
  } catch {
    error.value = 'network error — nothing was recorded'
    return { ok: false, status: 0 }
  }
}

async function del(path, token) {
  error.value = ''
  try {
    const res = await fetch(path, {
      method: 'DELETE',
      headers: { 'X-ClimateUmbral-Token': token ?? '' },
    })
    if (res.status !== 204) error.value = await errText(res)
    return res
  } catch {
    error.value = 'network error — nothing was erased'
    return { ok: false, status: 0 }
  }
}

async function pledge(kind) {
  const { pe, pn } = selected.value
  const before = candidateCount.value
  const res = await post('/api/claims', {
    pe,
    pn,
    kind,
    name: name.value.trim(),
  })
  if (res.ok) {
    const { token } = await res.json()
    rememberToken('claim', pe, pn, token)
  }
  await refresh()
  if (res.ok) {
    // the raster recomputes in EuroMap's version watcher; wait for
    // the flush or we read the pre-act candidate count
    await nextTick()
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

async function joinBlock() {
  const [be, bn] = selBlock.value
  const res = await post('/api/joins', {
    be,
    bn,
    name: name.value.trim(),
  })
  if (res.ok) {
    const { token } = await res.json()
    rememberToken('join', be, bn, token)
  }
  await refresh()
}

async function leaveBlock() {
  const [be, bn] = selBlock.value
  const res = await del(`/api/joins/${be}/${bn}`,
    tokenFor('join', be, bn))
  if (res.status === 204) forgetToken('join', be, bn)
  await refresh()
}

function select(p) {
  selected.value = p
  history.replaceState(null, '', `#${p.pe},${p.pn}`)
}

onMounted(async () => {
  // live sync: any act by anyone refreshes every open map
  // (?nolive opts out — headless renderers hang on open streams).
  // 'hello' also fires on every EventSource auto-reconnect, so a
  // board that dropped offline resyncs the moment it is back.
  if (!new URLSearchParams(location.search).has('nolive')) {
    const es = new EventSource('/api/events')
    // coalesce bursts: N acts arriving together cost one refresh
    // (ledger + leaderboard + raster recompute), not N
    let debounce = 0
    const scheduleRefresh = () => {
      clearTimeout(debounce)
      debounce = setTimeout(refresh, 400)
    }
    es.addEventListener('ledger', scheduleRefresh)
    es.addEventListener('hello', scheduleRefresh)
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
    <IntroHeader v-model:name="name" />

    <main>
    <ScoreBar
      :mission="mission"
      :has-acts="mine.length > 0 || myJoins.length > 0"
      :my-flipped-m2="myFlipped.length * 100"
      :my-pledged-m2="myPledges.length * 100"
      :my-night-m-c="myNightMC"
      :my-joins="myJoins.length"
      :my-block-m-c="myBlockMC"
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
      :night-spread="nightSpread"
      :night-m-c="nightMC"
      @mission="onMission"
    />
    <p v-if="error" class="error" role="alert">{{ error }}</p>

    <MapControls
      :mode="mode"
      @frontline="board?.frontline()"
      @shelter="board?.shelterTonight()"
      @mode="setMode"
    />

    <EuroMap
      ref="board"
      :claims="claims"
      :joins="joins"
      :mine-keys="mineKeys"
      :selected="selected"
      :mode="mode"
      :version="version"
      @select="select"
      @raster="(r) => (raster = r)"
      @refuges="(s) => (refugeSources = s)"
    />
    <Legend :mode="mode" :refuge-sources="refugeSources" />

    <PixelPanel
      v-if="selected"
      :pixel="selected"
      :value="selValue"
      :claim="claimAt.get(selKey) ?? null"
      :block-joins="selBlockJoins"
      :block-delta="selBlockJoins.length
        ? blockCoolingSince(claims, selBlock[0], selBlock[1],
          selBlockJoins[0].ts)
        : 0"
      :joined="!!(selBlock && tokenFor('join', ...selBlock))"
      :is-candidate="selIsCandidate"
      :my-claim-token="tokenFor('claim', selected.pe, selected.pn)"
      :anchor-label="selAnchor"
      :day-delta="selHeat?.day ?? null"
      :night-delta="selHeat?.night ?? null"
      :flips-per-deg="selHeat?.flips ?? 0"
      :key="selKey"
      @pledge="pledge"
      @flip="flip"
      @abandon="abandon"
      @join="joinBlock"
      @leave="leaveBlock"
    />
    <Leaderboard :rows="leaders" />

    <Learn />
    </main>

    <footer>
      <p>
        Data: © European Union, Copernicus Land Monitoring Service /
        EEA — Imperviousness Density 2018, 10 m. Basemap ©
        OpenStreetMap contributors. Climate shelters: Ajuntament de
        Barcelona, Open Data BCN (CC BY 4.0) — more city adapters
        welcome. Claims are pledges; satellites keep the real score.
        The ledger stores only what this board shows; erase your acts
        anytime from their pixel. Your location, if you share it,
        stays in your browser.
      </p>
    </footer>
  </div>
</template>

<style scoped>
.wrap {
  max-width: 720px;
  margin: 0 auto;
  padding: clamp(16px, 4vw, 48px) clamp(12px, 4vw, 40px)
    calc(64px + env(safe-area-inset-bottom, 0px));
}
.error {
  color: var(--err);
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
