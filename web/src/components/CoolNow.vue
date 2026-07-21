<script setup>
// The panic path. Everything else in this app is an instrument; this
// screen is for a person who is too hot RIGHT NOW — possibly old,
// dizzy, on a cheap phone, not a map reader, not an English reader.
// Rules of this screen: one decision per step, words a tired brain
// can parse, tap targets a shaking hand can hit, no map, no jargon,
// and honest absence (no network here ≠ no shelters exist). The
// advice block and 112 render even when everything else fails —
// the screen must help even with no data and no location.
//
// Two tiers, never blended: official climate shelters (a city's
// promise, /api/refuges) and other cool public places (crowd
// knowledge from OSM, /api/coolplaces) — separate sections,
// separate words, so a mall never borrows a city's authority.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { fetchRefuges } from '../lib/refuges.js'
import { KIND_ICON } from '../lib/coolplaces.js'
import { STRINGS, LANGS, pickLang } from '../lib/cooltext.js'

const emit = defineEmits(['close'])

const lang = ref(pickLang())
const t = computed(() => STRINGS[lang.value])

// Cities with a published network, for when location is denied,
// unavailable, or wrong. Order = current adapter order.
const CITIES = [
  { name: 'Barcelona', lon: 2.17, lat: 41.387 },
  { name: 'Paris', lon: 2.352, lat: 48.857 },
  { name: 'Wien', lon: 16.372, lat: 48.208 },
  { name: 'Lyon', lon: 4.835, lat: 45.758 },
]

const step = ref('start') // start | locating | list | far | nodata
const note = ref('') // denied/error line above the city buttons
const results = ref([]) // nearest official shelters, with .km
const farthest = ref(null) // nearest-known when it is too far to walk
const others = ref([]) // a/c public places (OSM), with .km

let refuges = null // fetched once, on first need
async function loadRefuges() {
  if (refuges) return refuges
  const { refuges: list } = await fetchRefuges()
  refuges = list
  return list
}

const km = (a, b) => {
  const dx = (a[0] - b[0]) *
    Math.cos(((a[1] + b[1]) / 2) * Math.PI / 180)
  return Math.hypot(dx, a[1] - b[1]) * 111.32
}

// A tired walker does ~4 km/h. Rounded up: better to promise 10
// minutes and arrive in 8 than the reverse.
const walkMin = (d) => Math.ceil(d * 15)
const distLabel = (d) =>
  d < 1 ? `${Math.round(d * 100) * 10} m` : `${d.toFixed(1)} km`

// Open-now is only ever claimed from structured per-day hours
// (r.week, Monday-first). Freeform hour strings display as-is — a
// guessed "open" sends a body to a locked door.
function status(r) {
  if (!r.week) return null
  const today = r.week[(new Date().getDay() + 6) % 7]
  if (!today) return null
  if (/ferm/i.test(today)) return { open: false, text: t.value.closedToday }
  const m = today.match(
    /^(\d{1,2})[h:](\d{2})?\s*-\s*(\d{1,2})[h:](\d{2})?$/)
  if (!m) return { open: null, text: `${t.value.todayW}: ${today}` }
  const now = new Date()
  const cur = now.getHours() * 60 + now.getMinutes()
  const from = +m[1] * 60 + +(m[2] ?? 0)
  const to = +m[3] * 60 + +(m[4] ?? 0)
  if (cur >= from && cur < to) {
    return {
      open: true,
      text: `${t.value.openNow} · ${t.value.until} ${m[3]}h${m[4] ?? ''}`,
    }
  }
  return {
    open: false,
    text: `${t.value.closedNow} · ${t.value.todayW} ${today}`,
  }
}

async function findNear(lon, lat) {
  step.value = 'locating'
  const here = [lon, lat]
  // the not-official ring loads in parallel and fills in when ready
  others.value = []
  fetch(`/api/coolplaces?lon=${lon}&lat=${lat}`)
    .then((r) => (r.ok ? r.json() : { places: [] }))
    .then(({ places }) => {
      others.value = (places ?? [])
        .map((p) => ({ ...p, km: km(here, [p.lon, p.lat]) }))
        .sort((a, b) => a.km - b.km)
        .slice(0, 4)
    })
    .catch(() => {}) // this ring is a bonus, never a blocker
  const list = await loadRefuges()
  if (!list.length) {
    step.value = 'nodata'
    return
  }
  const scored = list
    .map((r) => ({ ...r, km: km(here, [r.lon, r.lat]) }))
    .sort((a, b) => a.km - b.km)
  // 30 km ≈ the edge of "this city has a network at all"
  if (scored[0].km > 30) {
    farthest.value = scored[0]
    step.value = 'far'
    return
  }
  results.value = scored.slice(0, 3)
  step.value = 'list'
}

// One tap must be enough — even on an Android where that tap has to
// walk through the site-permission prompt, then the OS "turn on
// device location" dialog, then a cold GPS fix. Discrete
// getCurrentPosition calls die at each of those steps (each error
// fired before the user finished the dialog — seen on a real
// phone). watchPosition instead keeps the attempt ALIVE across the
// dialogs and the warm-up: transient errors are ignored, the first
// fix wins, and only a real denial or the overall deadline ends it.
// The city buttons stay on screen the whole time as the bail-out.
let watchId = null
let watchTimer = 0
function stopWatch() {
  if (watchId !== null) navigator.geolocation.clearWatch(watchId)
  watchId = null
  clearTimeout(watchTimer)
}

async function useMyLocation() {
  ping('cool_locate')
  note.value = ''
  if (!navigator.geolocation) {
    note.value = t.value.locFail
    return
  }
  let perm = 'prompt'
  try {
    perm = (await navigator.permissions.query(
      { name: 'geolocation' })).state
  } catch { /* Safari < 16: no query — just try */ }
  if (perm === 'denied') {
    note.value = t.value.locDenied
    return
  }
  step.value = 'locating'
  stopWatch()
  // granted: only GPS warm-up to wait for. prompt: the user may
  // spend most of a minute inside two dialogs first.
  const deadline = perm === 'granted' ? 20000 : 60000
  watchId = navigator.geolocation.watchPosition(
    (pos) => {
      stopWatch()
      findNear(pos.coords.longitude, pos.coords.latitude)
    },
    (err) => {
      if (err.code === 1) { // PERMISSION_DENIED is final
        stopWatch()
        step.value = 'start'
        note.value = t.value.locDenied
      }
      // code 2/3 are transient while dialogs resolve and GPS
      // warms — the watch keeps trying until the deadline
    },
    { enableHighAccuracy: true, timeout: deadline, maximumAge: 60000 },
  )
  watchTimer = setTimeout(() => {
    if (watchId === null) return
    stopWatch()
    step.value = 'start'
    note.value = t.value.locFail
  }, deadline)
}

// Walking directions in whatever maps app the phone owns. Only the
// destination leaves the page — the user's own position never
// touches our server, and goes to the maps app only when they tap.
const routeURL = (r) =>
  'https://www.google.com/maps/dir/?api=1&destination=' +
  `${r.lat},${r.lon}&travelmode=walking`

// ping: a bare event name to our own server, nothing else — the
// whole-number tallies are public at /api/stats. No IPs, no IDs,
// no coordinates ever ride along.
const ping = (e) => {
  try {
    fetch('/api/ping', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ e }),
      keepalive: true,
    }).catch(() => {})
  } catch { /* counting must never break the panic path */ }
}

// The caregiver loop: the person who opens this screen is often not
// the person who needs it next. Native share sheet where phones
// have one; clipboard everywhere else.
const SHARE_URL = 'https://climateumbral.eu/#cool'
const justCopied = ref(false)
async function share() {
  ping('cool_share')
  if (navigator.share) {
    try {
      await navigator.share({ title: t.value.title, url: SHARE_URL })
      return
    } catch { /* user closed the sheet — fall through to copy */ }
  }
  try {
    await navigator.clipboard.writeText(SHARE_URL)
    justCopied.value = true
    setTimeout(() => (justCopied.value = false), 2500)
  } catch { /* no clipboard either — the URL is in the bar */ }
}

// The page behind must not move: a shaky swipe that scroll-chains
// out of this screen strands the user back on the instrument.
onMounted(() => {
  loadRefuges() // warm the list while the user reads the first screen
  document.documentElement.style.overflow = 'hidden'
  ping('cool_open')
})
onUnmounted(() => {
  stopWatch() // no orphan GPS tracking after the screen closes
  document.documentElement.style.overflow = ''
})
</script>

<template>
  <div class="coolnow" role="dialog" aria-modal="true"
    :aria-label="t.title">
    <div class="bar">
      <span class="langs" role="group" aria-label="language">
        <button v-for="l in LANGS" :key="l"
          :aria-pressed="lang === l" @click="lang = l">
          {{ l.toUpperCase() }}
        </button>
      </span>
      <button class="close" @click="emit('close')">
        ✕ <span class="closeword">{{ t.back }}</span>
      </button>
    </div>

    <div class="body">
      <h1>{{ t.title }}</h1>

      <template v-if="step === 'start' || step === 'locating'">
        <button class="big find" :disabled="step === 'locating'"
          @click="useMyLocation">
          {{ step === 'locating' ? t.locating : `📍 ${t.find}` }}
        </button>
        <p v-if="note" class="note" role="alert">{{ note }}</p>
        <p v-else class="orcity">{{ t.orCity }}</p>
        <div class="cities">
          <button v-for="c in CITIES" :key="c.name" class="big city"
            @click="findNear(c.lon, c.lat)">
            {{ c.name }}
          </button>
        </div>
      </template>

      <template v-else-if="step === 'list'">
        <h2>✔ {{ t.nearOfficial }}</h2>
        <p class="tiernote">{{ t.officialNote }}</p>
        <div v-for="r in results" :key="r.lon + ',' + r.lat"
          class="card">
          <div class="name">{{ r.name }}</div>
          <div class="dist">
            🚶 {{ walkMin(r.km) }} {{ t.walkMin }}
            · {{ distLabel(r.km) }}
          </div>
          <div v-if="status(r)" class="status"
            :class="{ open: status(r).open === true,
                      shut: status(r).open === false }">
            {{ status(r).text }}
          </div>
          <div v-else-if="r.hours" class="status">
            🕐 {{ r.hours }}
          </div>
          <div v-if="r.addr" class="addr">{{ r.addr }}</div>
          <a class="big route" :href="routeURL(r)" target="_blank"
            rel="noopener" @click="ping('cool_route')">➜
            {{ t.route }}</a>
        </div>
        <p class="hours">{{ t.hours }}</p>
      </template>

      <template v-else-if="step === 'far'">
        <p class="note big-note">{{ t.far }}</p>
        <p v-if="farthest" class="addr">
          {{ t.farNearest }} {{ farthest.name }} —
          {{ Math.round(farthest.km) }} km
        </p>
      </template>

      <template v-else-if="step === 'nodata'">
        <p class="note big-note" role="alert">{{ t.noData }}</p>
      </template>

      <template v-if="(step === 'list' || step === 'far')
        && others.length">
        <h2>{{ t.nearOther }}</h2>
        <p class="tiernote">{{ t.otherNote }}</p>
        <div v-for="p in others" :key="p.lon + ',' + p.lat"
          class="card other">
          <div class="name">
            {{ KIND_ICON[p.kind] ?? '🏢' }} {{ p.name }}
          </div>
          <div class="dist">
            🚶 {{ walkMin(p.km) }} {{ t.walkMin }}
            · {{ distLabel(p.km) }}
          </div>
          <div v-if="p.hours" class="status">🕐 {{ p.hours }}</div>
          <a class="big route" :href="routeURL(p)" target="_blank"
            rel="noopener" @click="ping('cool_route')">➜
            {{ t.route }}</a>
        </div>
        <p class="hours">© OpenStreetMap contributors</p>
      </template>

      <section class="advice">
        <h2>{{ t.advice }}</h2>
        <ul>
          <li v-for="(tip, i) in t.tips.slice(0, 3)" :key="i">
            {{ tip }}
          </li>
        </ul>
        <p class="danger">{{ t.tips[3] }}</p>
        <a class="big call" href="tel:112">📞 {{ t.call }}</a>
        <button class="big sharebtn" @click="share">
          📤 {{ justCopied ? t.copied : t.share }}
        </button>
      </section>
    </div>
  </div>
</template>

<style scoped>
/* One column, huge type, generous targets. Uses the app palette so
   both themes keep their AA contrast. */
.coolnow {
  position: fixed;
  inset: 0;
  z-index: 50;
  overflow-y: auto;
  overscroll-behavior: contain;
  background: var(--bg);
  color: var(--ink);
  font-size: 20px;
  line-height: 1.45;
}
.bar {
  position: sticky;
  top: 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  padding: 10px clamp(12px, 4vw, 32px);
  background: var(--bg);
  border-bottom: 1px solid var(--line);
}
.langs {
  display: flex;
  gap: 4px;
}
.langs button {
  font: inherit;
  font-size: 14px;
  min-width: 44px;
  min-height: 44px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--card);
  color: var(--ink-2);
  cursor: pointer;
}
.langs button[aria-pressed='true'] {
  background: var(--ink);
  color: var(--bg);
  border-color: var(--ink);
  font-weight: 700;
}
.close {
  font: inherit;
  font-size: 16px;
  min-height: 44px;
  padding: 6px 14px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--card);
  color: var(--ink);
  cursor: pointer;
}
.body {
  max-width: 560px;
  margin: 0 auto;
  padding: 18px clamp(12px, 4vw, 32px)
    calc(48px + env(safe-area-inset-bottom, 0px));
}
h1 {
  font-size: 34px;
  margin: 6px 0 16px;
  letter-spacing: -0.01em;
}
h2 {
  font-size: 22px;
  margin: 22px 0 4px;
}
.tiernote {
  color: var(--ink-2);
  font-size: 16px;
  margin-bottom: 8px;
}
.big {
  display: block;
  width: 100%;
  min-height: 64px;
  padding: 14px 18px;
  margin: 10px 0;
  font: inherit;
  font-size: 21px;
  font-weight: 700;
  text-align: center;
  text-decoration: none;
  border-radius: 16px;
  border: 2px solid transparent;
  cursor: pointer;
}
.find {
  background: var(--cool);
  color: #ffffff;
}
.find:disabled {
  opacity: 0.75;
  cursor: wait;
}
.orcity {
  margin-top: 18px;
  color: var(--ink-2);
}
.cities {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.city {
  margin: 0;
  background: var(--card);
  color: var(--ink);
  border-color: var(--line);
}
.note {
  margin: 14px 0 6px;
  color: var(--err);
  font-weight: 600;
}
.big-note {
  font-size: 22px;
  color: var(--ink);
}
.card {
  border: 2px solid var(--line);
  border-radius: 16px;
  background: var(--card);
  padding: 16px;
  margin: 12px 0;
}
.card.other {
  border-style: dashed;
}
.name {
  font-size: 23px;
  font-weight: 800;
}
.dist {
  font-size: 21px;
  margin-top: 4px;
}
.status {
  margin-top: 4px;
  font-size: 18px;
  font-weight: 700;
  color: var(--ink-2);
}
.status.open {
  color: var(--accent);
}
.status.shut {
  color: var(--err);
}
.addr {
  color: var(--ink-2);
  margin-top: 4px;
  font-size: 18px;
}
.route {
  background: var(--accent);
  color: #ffffff;
  margin-bottom: 0;
}
.hours {
  color: var(--ink-2);
  font-size: 17px;
}
.advice {
  margin-top: 28px;
  border-top: 1px solid var(--line);
  padding-top: 8px;
}
.advice ul {
  list-style: none;
}
.advice li {
  padding: 8px 0;
  border-bottom: 1px solid var(--line);
}
.danger {
  margin-top: 14px;
  font-weight: 800;
  font-size: 22px;
}
.call {
  background: var(--err);
  color: #ffffff;
}
.sharebtn {
  background: var(--card);
  color: var(--ink);
  border: 2px solid var(--line);
}
@media (prefers-color-scheme: dark) {
  .find,
  .route,
  .call {
    color: #10130f;
  }
}
</style>
