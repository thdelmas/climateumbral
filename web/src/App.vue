<script setup>
import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import MapBoard from './components/MapBoard.vue'
import PixelPanel from './components/PixelPanel.vue'
import Leaderboard from './components/Leaderboard.vue'
import ScoreBar from './components/ScoreBar.vue'
import { computeCandidates } from './lib/grid.js'
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
const opened = ref(openedTotal())
const board = ref(null)

watch(name, (n) => setMyName(n.trim()))

const openedLabel = computed(() =>
  lastOpened.value
    ? `+${lastOpened.value} new candidates opened`
    : 'no new candidates opened',
)

const idx = (c) => c.y * meta.value.width + c.x

// ---- personal score: my acts are the ones whose tokens I hold ----

const mine = computed(() => {
  version.value // recompute when the ledger refreshes
  const list = []
  for (const key of Object.keys(allTokens('claim'))) {
    const [x, y] = key.split(',').map(Number)
    const c = claimAt.value.get(y * meta.value.width + x)
    if (c) list.push(c)
  }
  return list
})
const mineSet = computed(
  () => new Set(mine.value.map((c) => idx(c))),
)
const myPledges = computed(() =>
  mine.value.filter((c) => c.status === 'pledged'),
)
const myFlipped = computed(() =>
  mine.value.filter((c) => c.status === 'flipped'),
)
const myWatchCount = computed(() => {
  version.value
  return Object.keys(allTokens('watch')).filter((key) => {
    const [x, y] = key.split(',').map(Number)
    return watchesAt.value.has(y * meta.value.width + x)
  }).length
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

// The mission: always one obvious next move.
const mission = computed(() => {
  if (!mine.value.length) {
    return {
      text: 'Your first move: claim 100 m² of concrete on the front line.',
      btn: '→ find me a square',
      goto: null,
    }
  }
  const p = nextPledge.value
  if (p) {
    return {
      text:
        `You promised square ${p.x},${p.y} — ` +
        `${daysLeft(p)} days left to flip it.`,
      btn: 'go to my pledge',
      goto: p,
    }
  }
  return {
    text:
      `All your pledges are flipped — ` +
      `${myFlipped.value.length * 100} m² breathing again. Open a new front?`,
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
  const p = { x: m.goto.x, y: m.goto.y, i: idx(m.goto) }
  selected.value = p
  board.value?.goTo(p.x, p.y)
  history.replaceState(null, '', `#${p.x},${p.y}`)
}

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
    addOpened(lastOpened.value)
    opened.value = openedTotal()
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
        This is central Barcelona as the satellite sees it — every
        square a real 10 × 10 m of ground. Gray is sealed. Green is
        alive. The game: turn gray into green, square by square.
      </p>
      <ol class="steps">
        <li>
          Hit <b>“find me a square”</b> — it zooms to an
          <b>orange</b> square: sealed ground touching life.
        </li>
        <li>
          <b>Pledge</b> it — a public promise to depave those 100 m²
          within 90 days.
        </li>
        <li>Depave for real, then <b>mark it flipped</b> (photo link
          welcome).</li>
        <li>Can't flip it yourself (a road, a schoolyard)?
          <b>Watch</b> it — watchers form the coalition.</li>
      </ol>
      <label class="who">
        I am
        <input v-model="name" placeholder="pseudonym (optional)" size="18" />
      </label>
    </header>

    <ScoreBar
      v-if="grid"
      :mission="mission"
      :has-acts="mine.length > 0 || myWatchCount > 0"
      :my-flipped-m2="myFlipped.length * 100"
      :my-pledged-m2="myPledges.length * 100"
      :my-watch-count="myWatchCount"
      :opened="opened"
      :my-rank="myRank"
      :flipped-m2="flippedM2"
      :pledged-m2="pledgedM2"
      :candidate-count="candidates.size"
      :opened-label="lastOpened !== null ? openedLabel : null"
      @mission="onMission"
    />
    <p v-if="error" class="error">{{ error }}</p>

    <template v-if="grid">
      <MapBoard
        ref="board"
        :grid="grid"
        :meta="meta"
        :pledged="pledged"
        :flipped="flipped"
        :watched="watched"
        :candidates="candidates"
        :mine="mineSet"
        :selected="selected"
        :version="version"
        @select="select"
      />
      <div class="legend">
        <span><i style="background: rgb(255, 122, 26)" /> candidate —
          claim me</span>
        <span><i style="background: rgb(235, 179, 66)" /> pledged</span>
        <span><i style="background: rgb(125, 200, 110)" /> flipped</span>
        <span><i style="background: rgb(150, 118, 220)" /> watched</span>
        <span><i style="background: rgb(61, 61, 68)" /> sealed</span>
        <span><i style="background: rgb(46, 107, 62)" /> green</span>
        <span><i class="mine-chip" /> yours</span>
      </div>
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
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  margin-top: 10px;
  font-size: 12.5px;
  color: var(--ink-2);
}
.legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.legend i {
  width: 11px;
  height: 11px;
  border-radius: 3px;
  display: inline-block;
}
.legend i.mine-chip {
  background: transparent;
  border: 2px solid var(--ink);
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
