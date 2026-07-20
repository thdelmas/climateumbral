<script setup>
const name = defineModel('name')
</script>

<template>
  <header>
    <h1>Tilewhip</h1>
    <p class="sub">
      Europe as the satellite sees it — every square a real
      10 × 10 m of ground. Gray is sealed. Green is alive. The game:
      turn gray into green, square by square, until the nights cool.
    </p>
    <ol class="steps">
      <li>
        Hit <b>“find me a square”</b> — it flies to an
        <b>orange</b> square: sealed ground touching life.
      </li>
      <li>
        Yours to change — legally? <b>Pledge a cooling act</b> —
        depave it, plant a tree pit, brighten the surface — 90 days
        to do it. Unsure? Every square's panel shows the legal path.
      </li>
      <li>
        Not yours to touch (a road, a schoolyard — most squares)?
        <b>Join the block</b> — a standing petition, scored by how
        the block's nights cool from the day you sign.
      </li>
      <li>Depaved for real? <b>Mark it flipped</b> (photo link
        welcome).</li>
    </ol>
    <p class="tonight">
      Too hot <em>tonight</em>? Hit <b>“nearest shelter”</b> — it
      flies to the closest <b>blue pin</b>: an official climate
      shelter, a real room your city keeps cool (tap 📍 first and
      it starts from where you stand). In the heat views,
      <b>deep-green pins</b> mark modeled cool islands: the green
      blocks the night model says stay coolest.
    </p>
    <label class="who">
      I am
      <input
        v-model="name"
        placeholder="pseudonym (optional)"
        size="18"
      />
    </label>
  </header>
</template>

<style scoped>
h1 {
  font-size: clamp(28px, 5vw, 44px);
  font-weight: 800;
  letter-spacing: -0.02em;
  margin-bottom: 6px;
}
.sub {
  color: var(--ink-2);
  max-width: 60ch;
}
.steps {
  margin: 14px 0 0 0;
  padding-left: 22px;
  font-size: 14px;
  color: var(--ink-2);
  max-width: 60ch;
}
.steps li + li {
  margin-top: 4px;
}
.steps b {
  color: var(--ink);
}
.tonight {
  margin-top: 12px;
  font-size: 14px;
  color: var(--ink-2);
  max-width: 60ch;
}
.tonight b {
  color: var(--ink);
}
.who {
  display: inline-flex;
  gap: 8px;
  align-items: baseline;
  margin-top: 12px;
  font-size: 14px;
  color: var(--ink-2);
}
.who input {
  font: inherit;
  padding: 4px 10px;
  border-radius: 6px;
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--ink);
}
</style>
