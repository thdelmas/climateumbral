<script setup>
import { computed, ref } from 'vue'
import { pixelCenter } from '../lib/proj.js'

const props = defineProps({
  pixel: Object, // {pe, pn} — continent pixel
  value: Number, // sealed-% at the pixel; null if outside the raster
  claim: Object, // claimView or null
  watches: Array, // watchViews on this pixel
  isCandidate: Boolean,
  myClaimToken: String,
  myWatchToken: String,
  dayDelta: Number, // modeled °C above unsealed, null off land
  nightDelta: Number,
  flipsPerDeg: Number, // flips like this one per modeled night degree
})
const emit = defineEmits([
  'pledge',
  'flip',
  'abandon',
  'watch',
  'unwatch',
])

const photo = ref('')
const copied = ref(false)

const coords = computed(() => {
  const [lon, lat] = pixelCenter(props.pixel.pe, props.pixel.pn)
  return `${lat.toFixed(4)}, ${lon.toFixed(4)}`
})

const surface = computed(() => {
  if (props.value === null) return 'zoom in for ground truth'
  if (props.value === 254) return 'water'
  if (props.value > 100) return 'no data'
  return `${props.value}% sealed`
})

const sealed = computed(
  () => props.value !== null && props.value >= 90 && props.value <= 100,
)

const KIND_LABEL = {
  depave: 'depave',
  tree: 'tree pit',
  coolroof: 'cool surface',
}
const kindLabel = computed(() => KIND_LABEL[props.claim?.kind] ?? 'act')

const daysLeft = computed(() => {
  if (!props.claim || props.claim.status !== 'pledged') return 0
  const ms = new Date(props.claim.deadline) - Date.now()
  return Math.max(0, Math.ceil(ms / 86_400_000))
})

async function copyLink() {
  const url = `${location.origin}${location.pathname}` +
    `#${props.pixel.pe},${props.pixel.pn}`
  await navigator.clipboard.writeText(url)
  copied.value = true
  setTimeout(() => (copied.value = false), 1500)
}
</script>

<template>
  <div class="panel">
    <div class="row head">
      <strong>square {{ pixel.pe }},{{ pixel.pn }}</strong>
      <span class="muted">{{ surface }} · 10 × 10 m at ≈ {{ coords }}</span>
      <button class="link" @click="copyLink">
        {{ copied ? 'copied!' : 'copy link' }}
      </button>
    </div>

    <div v-if="dayDelta !== null" class="row heat">
      block heat, modeled: <b>+{{ dayDelta.toFixed(1) }} °C</b> day ·
      <b>+{{ nightDelta.toFixed(1) }} °C</b> night
      <template v-if="isCandidate && !claim && flipsPerDeg">
        — one of ~{{ flipsPerDeg }} flips its block needs to sleep
        1 °C cooler
      </template>
    </div>

    <div v-if="claim" class="row">
      <template v-if="claim.status === 'flipped'">
        <span class="flipped">
          {{ kindLabel }} done by {{ claim.name || 'anonymous' }}
          on {{ new Date(claim.flipped).toLocaleDateString() }}
        </span>
        <a v-if="claim.photo" :href="claim.photo" target="_blank"
          rel="noopener">proof</a>
      </template>
      <template v-else>
        <span class="pledged">
          {{ kindLabel }} pledged by {{ claim.name || 'anonymous' }} —
          {{ daysLeft }} days to do it
        </span>
      </template>
    </div>

    <div v-if="watches.length" class="row muted">
      {{ watches.length }} watching:
      {{ watches.map((w) => w.name || 'anonymous').join(', ') }}
    </div>

    <div v-if="isCandidate && !claim" class="row muted">
      This square is on the front line: sealed, but touching life —
      depaving it extends the living network.
    </div>
    <div v-else-if="sealed && !claim" class="row muted">
      Not on the front line — but a tree pit shades the day and a cool
      surface reflects it, anywhere sealed.
    </div>
    <div v-if="sealed && !claim" class="row muted">
      Pledge only ground you may legally change; a road or schoolyard
      is a watch, not a pledge.
    </div>
    <div class="row actions">
      <template v-if="sealed && !claim">
        <button v-if="isCandidate" @click="emit('pledge', 'depave')">
          pledge: depave
        </button>
        <button @click="emit('pledge', 'tree')">
          pledge: tree pit
        </button>
        <button @click="emit('pledge', 'coolroof')">
          pledge: cool surface
        </button>
      </template>
      <template v-if="claim?.status === 'pledged' && myClaimToken">
        <input
          v-model="photo"
          placeholder="photo URL (optional proof)"
          size="24"
        />
        <button @click="emit('flip', photo)">mark it done</button>
        <button class="quiet" @click="emit('abandon')">
          abandon &amp; erase
        </button>
      </template>
      <button
        v-if="sealed && claim?.status !== 'flipped' && !myWatchToken"
        @click="emit('watch')"
      >
        watch this pixel
      </button>
      <button v-if="myWatchToken" class="quiet" @click="emit('unwatch')">
        stop watching
      </button>
    </div>
  </div>
</template>

<style scoped>
.panel {
  margin: 14px 0;
  padding: 12px 16px;
  border-radius: 8px;
  background: var(--card);
  border: 1px solid var(--line);
  font-size: 14px;
}
.row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  align-items: baseline;
}
.row + .row {
  margin-top: 8px;
}
.muted {
  color: var(--ink-2);
}
.heat {
  color: var(--ink-2);
}
.heat b {
  color: #c25c2a;
}
.pledged {
  color: #b3831a;
  font-weight: 600;
}
.flipped {
  color: var(--accent);
  font-weight: 600;
}
button {
  font: inherit;
  font-size: 13.5px;
  padding: 5px 12px;
  border-radius: 999px;
  cursor: pointer;
  border: 1px solid var(--line);
  background: var(--ink);
  color: var(--bg);
}
button.quiet,
button.link {
  background: var(--card);
  color: var(--ink-2);
}
button.link {
  border: none;
  text-decoration: underline;
  padding: 0;
}
input {
  font: inherit;
  font-size: 13.5px;
  padding: 5px 10px;
  border-radius: 6px;
  border: 1px solid var(--line);
  background: var(--bg);
  color: var(--ink);
}
</style>
