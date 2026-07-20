<script setup>
// Compact by design: the map is the product, so the header spends
// three lines, and everything teachable folds into "how to play"
// or the understand & act section below the board.
</script>

<template>
  <header>
    <h1>ClimateUmbral</h1>
    <p class="sub">
      Europe as the satellite sees it — every square a real
      10 × 10 m of ground. Gray is sealed, green is alive: turn gray
      green, square by square, until the nights cool.
    </p>
    <details class="how">
      <summary>How it works — 60 seconds</summary>
      <ol class="steps">
        <li>
          Hit <b>“find me a square”</b> — it flies to an
          <b>orange</b> square: sealed ground touching life, near
          you if you've shared your location.
        </li>
        <li>
          Yours to change — legally? <b>Pledge a cooling act</b> —
          depave it, plant a tree pit, brighten the surface — 90
          days to do it. Unsure? Every square's panel shows the
          legal path.
        </li>
        <li>
          Not yours to touch (a road, a schoolyard — most squares)?
          <b>Join the block</b> — a standing petition, counted by
          how the block's nights cool from the day you sign.
        </li>
        <li>Depaved for real? <b>Mark it flipped</b> (photo link
          welcome).</li>
      </ol>
      <p class="more">
        Too hot <em>tonight</em>? <b>“Nearest shelter”</b> flies to
        an official climate shelter — a real room your city keeps
        cool. And <em>umbral</em> means threshold: the night a body
        can sleep through — the full story, the levers and the law
        live in <a href="#learn">the guide</a>.
      </p>
    </details>
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
.how {
  margin: 10px 0 14px;
  color: var(--ink-2);
  font-size: 14px;
  max-width: 60ch;
}
.how summary {
  cursor: pointer;
  font-weight: 600;
  color: var(--ink);
}
@media (pointer: coarse) {
  .how summary {
    padding: 8px 0;
  }
}
.steps {
  margin: 10px 0 0 0;
  padding-left: 22px;
}
.steps li + li {
  margin-top: 4px;
}
.steps b,
.more b {
  color: var(--ink);
}
.more {
  margin-top: 10px;
}
.more a {
  color: var(--accent);
  font-weight: 600;
}
</style>
