<script setup>
// The reading list behind the instrument: verified studies, films
// and organizations, grouped by what the reader wants — evidence,
// a way in, or a way to act. Curated in git (lib/resources.js);
// recommending one is an issue/PR, reviewed like code.
//
// Films play in place, but privacy-first: NOTHING loads from
// YouTube until the reader presses play — then the nocookie embed
// spins up with that click. A page that promises "your location
// stays in your browser" doesn't hand Google a visit log for free.
import { ref } from 'vue'
import { RESOURCES, RESOURCE_KINDS } from '../lib/resources.js'

const groups = Object.entries(RESOURCE_KINDS).map(([kind, meta]) => ({
  kind,
  ...meta,
  items: RESOURCES.filter((r) => r.kind === kind),
}))

const playing = ref({}) // yt id -> true once the reader pressed play
const play = (id) => {
  playing.value = { ...playing.value, [id]: true }
}
const embedURL = (id) =>
  `https://www.youtube-nocookie.com/embed/${id}?autoplay=1`
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
          <div v-if="r.yt" class="player">
            <button
              v-if="!playing[r.yt]"
              class="playbtn"
              @click="play(r.yt)"
            >
              <span class="playicon">▶</span>
              Play here
              <span class="playnote">nothing loads from YouTube
                until you press play</span>
            </button>
            <iframe
              v-else
              :src="embedURL(r.yt)"
              :title="r.title"
              allow="autoplay; encrypted-media; fullscreen;
                picture-in-picture"
              allowfullscreen
              referrerpolicy="no-referrer"
            />
          </div>
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
.player {
  margin-top: 10px;
  aspect-ratio: 16 / 9;
  border-radius: 10px;
  overflow: hidden;
  background: var(--bg);
  border: 1px solid var(--line);
}
.player iframe {
  width: 100%;
  height: 100%;
  border: 0;
  display: block;
}
.playbtn {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font: inherit;
  font-size: 16px;
  font-weight: 700;
  color: var(--ink);
  background: transparent;
  border: 0;
  cursor: pointer;
}
.playicon {
  display: grid;
  place-items: center;
  width: 58px;
  height: 58px;
  border-radius: 50%;
  background: var(--accent);
  color: var(--bg);
  font-size: 22px;
  padding-left: 4px;
}
.playbtn:hover .playicon {
  transform: scale(1.06);
}
.playnote {
  font-size: 12.5px;
  font-weight: 400;
  color: var(--ink-3);
}
</style>
