<script setup>
defineProps({
  mission: Object, // {text, btn}
  hasActs: Boolean,
  myFlippedM2: Number,
  myPledgedM2: Number,
  myWatchCount: Number,
  opened: Number,
  myRank: Number,
  flippedM2: Number,
  pledgedM2: Number,
  candidateCount: Number,
  openedLabel: String, // null until the first pledge of the session
  nightAvg: Number, // modeled mean night heat penalty, °C
})
const emit = defineEmits(['mission'])
</script>

<template>
  <div class="mission">
    <span>{{ mission.text }}</span>
    <button @click="emit('mission')">{{ mission.btn }}</button>
  </div>

  <div class="counter you">
    <template v-if="hasActs">
      <span class="label">you</span>
      <span>
        <strong>{{ myFlippedM2.toLocaleString() }}</strong> m² flipped
      </span>
      <span>
        <strong>{{ myPledgedM2.toLocaleString() }}</strong> m² pledged
      </span>
      <span v-if="myWatchCount">
        <strong>{{ myWatchCount }}</strong> watching
      </span>
      <span v-if="opened">
        <strong>{{ opened }}</strong> candidates opened by your claims
      </span>
      <span v-if="myRank" class="rank">#{{ myRank }} on the ledger</span>
    </template>
    <template v-else>
      <span class="label">you</span>
      <span class="muted">
        no squares yet — your score starts with one pledge
      </span>
    </template>
  </div>

  <div class="counter">
    <span class="label">everyone</span>
    <span>
      <strong>{{ flippedM2.toLocaleString() }}</strong> m² flipped
    </span>
    <span>
      <strong>{{ pledgedM2.toLocaleString() }}</strong> m² pledged
    </span>
    <span>
      <strong>{{ candidateCount.toLocaleString() }}</strong>
      candidates on the front line
    </span>
    <span v-if="nightAvg">
      <strong>+{{ nightAvg.toFixed(2) }}</strong> °C avg night heat
      penalty (modeled)
    </span>
    <span v-if="openedLabel" class="opened">{{ openedLabel }}</span>
  </div>
</template>

<style scoped>
.mission {
  display: flex;
  gap: 14px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  margin: 20px 0 14px;
  padding: 12px 16px;
  border-radius: 8px;
  border: 1px solid var(--accent);
  background: var(--card);
  font-size: 15px;
  font-weight: 600;
}
.mission button {
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  padding: 7px 16px;
  border-radius: 999px;
  cursor: pointer;
  border: none;
  background: var(--accent);
  color: var(--bg);
  white-space: nowrap;
}
.counter {
  display: flex;
  gap: 18px;
  flex-wrap: wrap;
  align-items: baseline;
  margin-bottom: 14px;
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
.counter .label {
  font-size: 11.5px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--ink-3);
}
.counter .rank {
  font-weight: 700;
  color: var(--accent);
}
.counter .muted {
  color: var(--ink-2);
}
.counter.you {
  margin-bottom: 0;
  border-bottom-left-radius: 0;
  border-bottom-right-radius: 0;
}
.counter.you + .counter {
  margin-top: -1px;
  border-top-left-radius: 0;
  border-top-right-radius: 0;
}
.opened {
  color: var(--accent);
  font-weight: 600;
}
</style>
