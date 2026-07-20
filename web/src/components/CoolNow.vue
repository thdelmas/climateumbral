<script setup>
// The panic path. Everything else in this app is an instrument; this
// screen is for a person who is too hot RIGHT NOW — possibly old,
// dizzy, on a cheap phone, not a map reader, not an English reader.
// Rules of this screen: one decision per step, words a tired brain
// can parse, tap targets a shaking hand can hit, no map, no jargon,
// and honest absence (no network here ≠ no shelters exist). The
// advice block and 112 render even when everything else fails —
// the screen must help even with no data and no location.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { fetchRefuges } from '../lib/refuges.js'
import { STRINGS, LANGS, pickLang } from '../lib/cooltext.js'

const emit = defineEmits(['close'])

// ---- language: auto-pick, one-tap override ----------------------
const lang = ref(pickLang())
const t = computed(() => STRINGS[lang.value])

// ---- the finder -------------------------------------------------
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
const results = ref([]) // nearest shelters, with .km
const farthest = ref(null) // nearest-known when it is too far to walk

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

async function findNear(lon, lat) {
  step.value = 'locating'
  const list = await loadRefuges()
  if (!list.length) {
    step.value = 'nodata'
    return
  }
  const here = [lon, lat]
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

function useMyLocation() {
  note.value = ''
  if (!navigator.geolocation) {
    note.value = t.value.denied
    return
  }
  step.value = 'locating'
  navigator.geolocation.getCurrentPosition(
    (pos) => findNear(pos.coords.longitude, pos.coords.latitude),
    () => {
      step.value = 'start'
      note.value = t.value.denied
    },
    { enableHighAccuracy: false, timeout: 10000, maximumAge: 300000 },
  )
}

// Walking directions in whatever maps app the phone owns. Only the
// destination leaves the page — the user's own position never
// touches our server, and goes to the maps app only when they tap.
const routeURL = (r) =>
  'https://www.google.com/maps/dir/?api=1&destination=' +
  `${r.lat},${r.lon}&travelmode=walking`

// The page behind must not move: a shaky swipe that scroll-chains
// out of this screen strands the user back on the instrument.
onMounted(() => {
  loadRefuges() // warm the list while the user reads the first screen
  document.documentElement.style.overflow = 'hidden'
})
onUnmounted(() => {
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
        <h2>{{ t.near }}</h2>
        <div v-for="r in results" :key="r.lon + ',' + r.lat"
          class="card">
          <div class="name">{{ r.name }}</div>
          <div class="dist">
            🚶 {{ walkMin(r.km) }} {{ t.walkMin }}
            · {{ distLabel(r.km) }}
          </div>
          <div v-if="r.addr" class="addr">{{ r.addr }}</div>
          <a class="big route" :href="routeURL(r)" target="_blank"
            rel="noopener">➜ {{ t.route }}</a>
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

      <section class="advice">
        <h2>{{ t.advice }}</h2>
        <ul>
          <li v-for="(tip, i) in t.tips.slice(0, 3)" :key="i">
            {{ tip }}
          </li>
        </ul>
        <p class="danger">{{ t.tips[3] }}</p>
        <a class="big call" href="tel:112">📞 {{ t.call }}</a>
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
  margin: 22px 0 10px;
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
.name {
  font-size: 23px;
  font-weight: 800;
}
.dist {
  font-size: 21px;
  margin-top: 4px;
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
@media (prefers-color-scheme: dark) {
  .find,
  .route,
  .call {
    color: #10130f;
  }
}
</style>
