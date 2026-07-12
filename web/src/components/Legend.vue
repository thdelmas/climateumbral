<script setup>
import { DAY_COEF, NIGHT_COEF, HEAT_GRADIENT_CSS } from '../lib/heat.js'

defineProps({ mode: String })
</script>

<template>
  <div v-if="mode === 'land'" class="legend">
    <span><i style="background: rgb(255, 122, 26)" /> candidate —
      claim me</span>
    <span><i style="background: rgb(235, 179, 66)" /> pledged</span>
    <span><i style="background: rgb(125, 200, 110)" /> flipped</span>
    <span><i style="background: rgb(150, 118, 220)" /> watched</span>
    <span><i style="background: rgb(61, 61, 68)" /> sealed</span>
    <span><i style="background: rgb(46, 107, 62)" /> green</span>
    <span><i class="mine-chip" /> yours</span>
  </div>
  <div v-else class="legend heat">
    <span class="cap">
      {{ mode }} heat penalty vs unsealed (modeled)
    </span>
    <span class="grad-row">
      <span>+0 °C</span>
      <i class="grad" :style="{ background: HEAT_GRADIENT_CSS }" />
      <span>+{{ mode === 'day' ? DAY_COEF : NIGHT_COEF }} °C</span>
    </span>
  </div>
</template>

<style scoped>
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
.legend.heat .cap {
  font-weight: 600;
  color: var(--ink);
}
.legend.heat .grad {
  width: 160px;
  height: 10px;
  border-radius: 5px;
}
.legend.heat .grad-row {
  gap: 8px;
  font-variant-numeric: tabular-nums;
}
</style>
