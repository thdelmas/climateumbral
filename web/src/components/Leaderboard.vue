<script setup>
defineProps({ rows: Array })
</script>

<template>
  <div class="lb">
    <h2>Ledger</h2>
    <p v-if="!rows.length" class="note">
      No acts on the ledger yet — the first flipped m² makes history.
    </p>
    <table v-if="rows.length">
      <thead>
        <tr>
          <th>who</th>
          <th>night cooling</th>
          <th>done</th>
          <th>pledged</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.name">
          <td>{{ r.name }}</td>
          <td class="num cool">
            −{{ r.night_mdegc.toFixed(1) }} m°C
          </td>
          <td class="num">{{ r.flipped_m2.toLocaleString() }} m²</td>
          <td class="num">{{ r.pledged_m2.toLocaleString() }} m²</td>
        </tr>
      </tbody>
    </table>
    <p v-if="rows.length" class="note">
      Night cooling is modeled block-average °C, in thousandths (m°C)
      — small numbers are honest numbers; ~250 acts cool a block's
      nights by 1 °C. Done and pledged never mix; the satellite audit
      comes per epoch.
    </p>
  </div>
</template>

<style scoped>
.lb {
  margin-top: 28px;
}
h2 {
  font-size: 18px;
  font-weight: 700;
  margin-bottom: 10px;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}
th,
td {
  text-align: left;
  padding: 6px 10px;
  border-bottom: 1px solid var(--line);
}
th {
  color: var(--ink-2);
  font-weight: 600;
  font-size: 13px;
}
td.num,
th:not(:first-child) {
  text-align: right;
}
.cool {
  color: #4e8fbf;
  font-weight: 600;
}
.note {
  margin-top: 8px;
  font-size: 12.5px;
  color: var(--ink-3);
}
</style>
