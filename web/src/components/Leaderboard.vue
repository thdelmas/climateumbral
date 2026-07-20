<script setup>
defineProps({ rows: Array })
</script>

<template>
  <div class="lb">
    <h2>Ledger</h2>
    <p v-if="!rows.length" class="note">
      No acts on the ledger yet — the first flipped m² makes history.
    </p>
    <div v-if="rows.length" class="scroll">
    <table>
      <thead>
        <tr>
          <th scope="col">who</th>
          <th scope="col">blocks Δ</th>
          <th scope="col">own acts</th>
          <th scope="col">done</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.name">
          <td>{{ r.name }}</td>
          <td class="num cool">
            <template v-if="r.blocks">
              −{{ r.block_mdegc.toFixed(1) }} m°C
              ({{ r.blocks }})
            </template>
            <template v-else>—</template>
          </td>
          <td class="num cool">
            −{{ r.night_mdegc.toFixed(1) }} m°C
          </td>
          <td class="num">{{ r.flipped_m2.toLocaleString() }} m²</td>
        </tr>
      </tbody>
    </table>
    </div>
    <p v-if="rows.length" class="note">
      Blocks Δ = average modeled night cooling of the blocks you
      petitioned, counted from the day you signed — everyone's acts
      move it. Own acts = cooling from deeds you did yourself. m°C =
      thousandths of a °C; small numbers are honest numbers (~250
      acts cool a block's nights by 1 °C). The satellite audit comes
      per epoch.
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
/* narrow screens scroll the table inside its own box — the page
   never scrolls sideways */
.scroll {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}
td,
th {
  white-space: nowrap;
}
td:first-child {
  white-space: normal;
  overflow-wrap: anywhere;
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
  color: var(--cool);
  font-weight: 600;
}
.note {
  margin-top: 8px;
  font-size: 12.5px;
  color: var(--ink-3);
}
</style>
