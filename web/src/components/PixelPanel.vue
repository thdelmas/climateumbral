<script setup>
import { computed, ref } from 'vue'
import { pixelCenter } from '../lib/proj.js'

const props = defineProps({
  pixel: Object, // {pe, pn} — continent pixel
  value: Number, // sealed-% at the pixel; null if outside the raster
  claim: Object, // claimView or null
  blockJoins: Array, // signatures on this square's 150 m block
  blockDelta: Number, // modeled block cooling since first signature
  joined: Boolean, // this browser holds a signature here
  isCandidate: Boolean,
  myClaimToken: String,
  dayDelta: Number, // modeled °C above unsealed, null off land
  nightDelta: Number,
  flipsPerDeg: Number, // flips like this one per modeled night degree
  anchorLabel: String, // nearest human-hours anchor, null if none
})
const emit = defineEmits([
  'pledge',
  'flip',
  'abandon',
  'join',
  'leave',
])

const name = defineModel('name')

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
  try {
    // clipboard API needs a secure context; plain-http LAN dev
    // falls back to the selection trick
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(url)
    } else {
      const ta = document.createElement('textarea')
      ta.value = url
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      ta.remove()
    }
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    prompt('copy this link:', url)
  }
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

    <div v-if="anchorLabel" class="row anchor">
      people are here: {{ anchorLabel }} — this square's heat is felt
      in human-hours
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
          rel="noopener noreferrer">proof</a>
      </template>
      <template v-else>
        <span class="pledged">
          {{ kindLabel }} pledged by {{ claim.name || 'anonymous' }} —
          {{ daysLeft }} days to do it
        </span>
      </template>
    </div>

    <div v-if="blockJoins.length" class="row muted">
      this block's petition: <b>{{ blockJoins.length }}</b>
      {{ blockJoins.length === 1 ? 'signature' : 'signatures' }}
      ({{ blockJoins.map((j) => j.name || 'anonymous').join(', ') }})
      — <b>−{{ blockDelta.toFixed(1) }} m°C</b> night since it began
    </div>

    <div v-if="isCandidate && !claim" class="row muted">
      This square is on the front line: sealed, but touching life —
      depaving it extends the living network.
    </div>
    <div v-else-if="sealed && !claim" class="row muted">
      Not on the front line — but a tree pit shades the day and a cool
      surface reflects it, anywhere sealed.
    </div>
    <details v-if="sealed && !claim" class="legal">
      <summary>
        Pledge only ground you may legally change — the legal path
      </summary>
      <ul>
        <li>
          <b>Your own ground</b> (garden, courtyard, roof): usually
          yours to depave, plant or brighten. Check for pipes and
          cables before digging — every country has a free
          call-before-you-dig service — and heritage rules before
          painting.
        </li>
        <li>
          <b>Rented or shared</b>: the owner's or association's
          written yes first. A planter needs nobody's permission.
        </li>
        <li>
          <b>Public street, school, lot</b> — most squares: never
          pry it yourself. Ask your city instead: façade-garden
          schemes, tree-pit adoption, official depave programs. And
          join this block's petition — always legal, everywhere,
          scored by how the block's nights cool from the day you
          sign.
        </li>
      </ul>
      <p>
        Rules are local — your city hall decides. This is education,
        not legal advice. More in
        <a href="#learn">understand &amp; act</a> below.
      </p>
    </details>
    <label
      v-if="(sealed && !claim) || !joined"
      class="row who"
    >
      sign as
      <input
        v-model="name"
        placeholder="pseudonym (optional)"
        size="14"
      />
    </label>
    <div class="row actions">
      <template v-if="sealed && !claim">
        <button
          v-if="isCandidate"
          class="primary"
          @click="emit('pledge', 'depave')"
        >
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
          type="url"
          inputmode="url"
          aria-label="photo URL, optional proof"
          placeholder="photo URL (optional proof)"
        />
        <button @click="emit('flip', photo)">mark it done</button>
        <button class="quiet" @click="emit('abandon')">
          abandon &amp; erase
        </button>
      </template>
      <button v-if="!joined" @click="emit('join')">
        join this block — sign the petition
      </button>
      <button v-if="joined" class="quiet" @click="emit('leave')">
        withdraw my signature
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
.legal {
  margin-top: 8px;
  color: var(--ink-2);
}
.legal summary {
  cursor: pointer;
}
.legal ul {
  margin: 6px 0;
  padding-left: 18px;
}
.legal li + li {
  margin-top: 4px;
}
.legal p {
  margin-top: 6px;
  font-size: 13px;
}
.heat {
  color: var(--ink-2);
}
.anchor {
  color: var(--accent);
  font-weight: 600;
}
.heat b {
  color: var(--ember);
}
.pledged {
  color: var(--warm);
  font-weight: 600;
}
.who {
  color: var(--ink-2);
  font-size: 13.5px;
  align-items: center;
}
.who input {
  flex: 0 1 200px;
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
button.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--bg);
  font-weight: 600;
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
  flex: 1 1 180px;
  min-width: 0;
  max-width: 100%;
}
</style>
