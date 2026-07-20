<script setup>
defineProps({ mode: String })
const emit = defineEmits(['frontline', 'shelter', 'mode'])
const labels = { land: 'map', day: 'day °C', night: 'night °C' }
</script>

<template>
  <div class="controls">
    <button class="go" @click="emit('frontline')">
      → find me a square
    </button>
    <button class="shelter" @click="emit('shelter')">
      → nearest shelter
    </button>
    <span class="seg" role="group" aria-label="map view">
      <button
        v-for="(label, m) in labels"
        :key="m"
        :aria-pressed="mode === m"
        @click="emit('mode', m)"
      >
        {{ label }}
      </button>
    </span>
  </div>
</template>

<style scoped>
.controls {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
  flex-wrap: wrap;
}
.controls button {
  font: inherit;
  font-size: 13.5px;
  padding: 5px 12px;
  border-radius: 999px;
  cursor: pointer;
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--ink);
}
.controls button.go {
  background: var(--accent);
  color: var(--bg);
  border-color: var(--accent);
  font-weight: 600;
}
.controls button.shelter {
  background: rgb(43, 108, 196); /* the blue of the shelter pins */
  color: #ffffff;
  border-color: rgb(43, 108, 196);
  font-weight: 600;
}
.controls .seg {
  display: inline-flex;
  margin-left: auto;
}
.controls .seg button {
  border-radius: 0;
  margin-left: -1px;
}
.controls .seg button:first-child {
  border-radius: 999px 0 0 999px;
  margin-left: 0;
}
.controls .seg button:last-child {
  border-radius: 0 999px 999px 0;
}
.controls .seg button[aria-pressed='true'] {
  background: var(--ink);
  color: var(--bg);
  border-color: var(--ink);
}
</style>
