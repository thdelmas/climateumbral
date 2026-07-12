<script setup>
defineProps({ rows: Array })
</script>

<template>
  <div v-if="rows.length" class="lb">
    <h2>Ledger</h2>
    <table>
      <thead>
        <tr>
          <th>who</th>
          <th>flipped</th>
          <th>pledged</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.name">
          <td>{{ r.name }}</td>
          <td class="num">{{ r.flipped_m2.toLocaleString() }} m²</td>
          <td class="num">{{ r.pledged_m2.toLocaleString() }} m²</td>
        </tr>
      </tbody>
    </table>
    <p class="note">
      Flipped and pledged never mix — flips are photo-backed claims;
      the satellite audit comes per epoch.
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
.note {
  margin-top: 8px;
  font-size: 12.5px;
  color: var(--ink-3);
}
</style>
