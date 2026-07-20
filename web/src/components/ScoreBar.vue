<script setup>
defineProps({
  mission: Object, // {text, btn}
  hasActs: Boolean,
  myFlippedM2: Number,
  myPledgedM2: Number,
  myNightMC: Number, // my modeled night cooling, milli-degC
  myJoins: Number,
  myBlockMC: Number, // avg block delta since my signatures
  opened: Number,
  myRank: Number,
  flippedM2: Number,
  pledgedM2: Number,
  candidateCount: Number,
  openedLabel: String, // null until the first pledge of the session
  nightAvg: Number, // modeled mean night heat penalty, °C
  nightSpread: Number, // sealed-vs-green night gap in view, °C
  nightMC: Number, // everyone's modeled night cooling, milli-degC
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
      <span v-if="myNightMC">
        <strong>−{{ myNightMC.toFixed(1) }}</strong> m°C night
        cooling (modeled)
      </span>
      <span>
        <strong>{{ myFlippedM2.toLocaleString() }}</strong> m² done
      </span>
      <span>
        <strong>{{ myPledgedM2.toLocaleString() }}</strong> m² pledged
      </span>
      <span v-if="myJoins">
        <strong>{{ myJoins }}</strong>
        {{ myJoins === 1 ? 'block' : 'blocks' }} petitioned
        <template v-if="myBlockMC">
          · <strong>−{{ myBlockMC.toFixed(1) }}</strong> m°C avg since
          you signed
        </template>
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
    <span v-if="nightMC">
      <strong>−{{ nightMC.toFixed(1) }}</strong> m°C night cooling
      (modeled)
    </span>
    <span>
      <strong>{{ flippedM2.toLocaleString() }}</strong> m² done
    </span>
    <span>
      <strong>{{ pledgedM2.toLocaleString() }}</strong> m² pledged
    </span>
    <span v-if="candidateCount">
      <strong>{{ candidateCount.toLocaleString() }}</strong>
      candidates in view
    </span>
    <span v-if="nightAvg">
      <strong>+{{ nightAvg.toFixed(2) }}</strong> °C avg night heat
      penalty in view (modeled)
    </span>
    <span v-if="nightSpread >= 0.3" class="spread">
      sealed ground here runs
      <strong>+{{ nightSpread.toFixed(1) }}</strong> °C hotter at
      night than green ground (modeled)
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
