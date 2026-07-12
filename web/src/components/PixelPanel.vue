<script setup>
import { computed, ref } from 'vue'
import { SEA, NODATA } from '../lib/grid.js'

const props = defineProps({
  pixel: Object, // {x, y, i}
  value: Number, // grid value at the pixel
  meta: Object,
  claim: Object, // claimView or null
  watches: Array, // watchViews on this pixel
  isCandidate: Boolean,
  myClaimToken: String,
  myWatchToken: String,
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
  const { width, height, bbox_4326: bb } = props.meta
  const lon = (bb[0] + (props.pixel.x / width) * (bb[2] - bb[0])).toFixed(4)
  const lat = (bb[3] - (props.pixel.y / height) * (bb[3] - bb[1])).toFixed(4)
  return `${lat}, ${lon}`
})

const surface = computed(() => {
  if (props.value === SEA) return 'the sea'
  if (props.value === NODATA) return 'no data'
  return `${props.value}% sealed`
})

const sealed = computed(
  () => props.value >= 90 && props.value < SEA,
)

const daysLeft = computed(() => {
  if (!props.claim || props.claim.status !== 'pledged') return 0
  const ms = new Date(props.claim.deadline) - Date.now()
  return Math.max(0, Math.ceil(ms / 86_400_000))
})

async function copyLink() {
  const url = `${location.origin}${location.pathname}` +
    `#${props.pixel.x},${props.pixel.y}`
  await navigator.clipboard.writeText(url)
  copied.value = true
  setTimeout(() => (copied.value = false), 1500)
}
</script>

<template>
  <div class="panel">
    <div class="row head">
      <strong>pixel {{ pixel.x }},{{ pixel.y }}</strong>
      <span class="muted">{{ surface }} · 10 × 10 m at ≈ {{ coords }}</span>
      <button class="link" @click="copyLink">
        {{ copied ? 'copied!' : 'copy link' }}
      </button>
    </div>

    <div v-if="claim" class="row">
      <template v-if="claim.status === 'flipped'">
        <span class="flipped">
          flipped by {{ claim.name || 'anonymous' }}
          on {{ new Date(claim.flipped).toLocaleDateString() }}
        </span>
        <a v-if="claim.photo" :href="claim.photo" target="_blank"
          rel="noopener">proof</a>
      </template>
      <template v-else>
        <span class="pledged">
          pledged by {{ claim.name || 'anonymous' }} —
          {{ daysLeft }} days to flip it
        </span>
      </template>
    </div>

    <div v-if="watches.length" class="row muted">
      {{ watches.length }} watching:
      {{ watches.map((w) => w.name || 'anonymous').join(', ') }}
    </div>

    <div v-if="isCandidate && !claim" class="row muted">
      This square is on the front line: sealed, but touching life.
    </div>
    <div class="row actions">
      <button v-if="isCandidate && !claim" @click="emit('pledge')">
        pledge to flip it — 90 days
      </button>
      <template v-if="claim?.status === 'pledged' && myClaimToken">
        <input
          v-model="photo"
          placeholder="photo URL (optional proof)"
          size="24"
        />
        <button @click="emit('flip', photo)">mark flipped</button>
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
