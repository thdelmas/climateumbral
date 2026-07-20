<script setup>
// The reading list behind the instrument: verified studies, films
// and organizations, grouped by what the reader wants — evidence,
// a way in, or a way to act. Curated in git (lib/resources.js);
// recommending one is an issue/PR, reviewed like code.
import { RESOURCES, RESOURCE_KINDS } from '../lib/resources.js'

const groups = Object.entries(RESOURCE_KINDS).map(([kind, meta]) => ({
  kind,
  ...meta,
  items: RESOURCES.filter((r) => r.kind === kind),
}))
</script>

<template>
  <section id="resources" class="resources">
    <h2>Resources</h2>
    <p class="intro">
      The studies, films and organizations this project stands on —
      each link verified before it landed here. Know one that
      belongs on this list?
      <a href="https://github.com/thdelmas/climateumbral/issues"
        target="_blank" rel="noopener">Recommend it ↗</a>
    </p>
    <div v-for="g in groups" :key="g.kind" class="group">
      <h3>{{ g.icon }} {{ g.label }}</h3>
      <ul>
        <li v-for="r in g.items" :key="r.url">
          <a :href="r.url" target="_blank" rel="noopener">
            {{ r.title }}</a>
          <span class="meta"> — {{ r.source }}, {{ r.year }}</span>
          <p class="note">{{ r.note }}</p>
        </li>
      </ul>
    </div>
  </section>
</template>

<style scoped>
.resources {
  margin-top: 40px;
  border-top: 1px solid var(--line);
  padding-top: 18px;
}
.resources h2 {
  font-size: 22px;
  margin-bottom: 6px;
}
.intro {
  color: var(--ink-2);
  font-size: 14.5px;
  max-width: 60ch;
}
.group h3 {
  margin: 22px 0 8px;
  font-size: 16.5px;
}
.resources ul {
  list-style: none;
  display: grid;
  gap: 10px;
}
.resources li {
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 12px 14px;
}
.resources li a {
  color: var(--ink);
  font-weight: 650;
  text-decoration-color: var(--line);
}
.meta {
  color: var(--ink-3);
  font-size: 13.5px;
  white-space: nowrap;
}
.note {
  color: var(--ink-2);
  font-size: 14px;
  margin-top: 4px;
  max-width: 65ch;
}
</style>
