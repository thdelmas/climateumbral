<script setup>
// The suggestion box — the one free-text door, so the rules are
// stated on it: anonymous, not published, read by a human. Kept off
// the panic path (nothing optional belongs there).
import { ref } from 'vue'

const text = ref('')
const state = ref('idle') // idle | busy | sent | error

async function send() {
  const t = text.value.trim()
  if (t.length < 3) return
  state.value = 'busy'
  try {
    const res = await fetch('/api/suggest', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text: t }),
    })
    state.value = res.ok ? 'sent' : 'error'
    if (res.ok) text.value = ''
  } catch {
    state.value = 'error'
  }
}
</script>

<template>
  <section id="suggest" class="suggest">
    <h2>Ideas? Corrections? Tell us</h2>
    <p class="note">
      Anonymous and unpublished — a human reads every message.
      Prefer email? <a href="mailto:contact@climateumbral.eu">
      contact@climateumbral.eu</a> · or
      <a href="https://github.com/thdelmas/climateumbral/issues"
        target="_blank" rel="noopener">open a GitHub issue</a>.
    </p>
    <p v-if="state === 'sent'" class="sent">
      Received — thank you for making the map better.
    </p>
    <template v-else>
      <textarea
        v-model="text"
        rows="3"
        maxlength="2000"
        placeholder="A missing city, a wrong shelter, an idea…"
      ></textarea>
      <button :disabled="state === 'busy' || text.trim().length < 3"
        @click="send">
        {{ state === 'busy' ? 'Sending…' : 'Send' }}
      </button>
      <p v-if="state === 'error'" class="err" role="alert">
        Could not send — try email instead.
      </p>
    </template>
  </section>
</template>

<style scoped>
.suggest {
  margin-top: 40px;
  border-top: 1px solid var(--line);
  padding-top: 18px;
}
.suggest h2 {
  font-size: 22px;
  margin-bottom: 6px;
}
.note {
  color: var(--ink-2);
  font-size: 14.5px;
  max-width: 60ch;
}
.note a {
  color: var(--accent);
}
textarea {
  display: block;
  width: 100%;
  max-width: 560px;
  margin: 12px 0 10px;
  padding: 10px 12px;
  font: inherit;
  font-size: 16px;
  color: var(--ink);
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: 12px;
  resize: vertical;
}
button {
  font: inherit;
  font-size: 15.5px;
  font-weight: 650;
  padding: 9px 22px;
  border-radius: 10px;
  border: 1px solid var(--accent);
  background: var(--accent);
  color: var(--bg);
  cursor: pointer;
}
button:disabled {
  opacity: 0.55;
  cursor: default;
}
.sent {
  color: var(--accent);
  font-weight: 650;
}
.err {
  color: var(--err);
  font-size: 14.5px;
  margin-top: 8px;
}
</style>
