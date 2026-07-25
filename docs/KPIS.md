# KPIs — how the instrument reads its own dials

One question sits above every other: **how many people did we help?**
Everything below exists to answer it honestly — which means the
claimed-vs-measured discipline of GAME_DESIGN applies to our own
numbers exactly as it applies to tiles. A KPI we can't measure
honestly within the zero-custody doctrine (no accounts, no tracking,
no view-graph — INTENT) is a KPI we don't report.

## The north star splits in two

The instrument helps on two different clocks. Blending them hides
both, so they stay separate columns, always:

**Helped tonight** — someone found a cool place when they needed
one. The honest proxy is `cool_route`: the tap on directions to an
actual shelter. Opens are curiosity, locates are intent, routes are
someone going. We cannot verify arrival without tracking, so routes
is where measurement ends — stated plainly, like every model caveat.
This number only means something weighted by heat: routes during a
heatwave are the mission; routes in April are a smoke test.

**Cooled tomorrow** — physical change delivered. The unit is the
**flip**, never the pledge: a pledge is a promise, a flip is a
changed tile. Alongside flips, the modeled night cooling they add
(`night_mdegc`, small numbers are honest numbers) and flipped m².
Expired pledges count for nothing — except in the flip rate below.

## Supporting KPIs

Each answers one distinct question:

- **Flip rate** — is the funnel honest? `flipped / (flipped +
  expired)`, resolved pledges only. Open pledges are neither success
  nor failure yet. A falling flip rate means we're collecting
  promises, not change.
- **Coverage** — how many people *could* we help? Cities with an
  official shelter adapter, shelters on the map, and the
  demand-vs-coverage gap: a city page (`page_<slug>` counter) with
  traffic but no adapter behind it is the next adapter to build,
  ranked by evidence.
- **Contagion** — does it spread without us? Joins per block,
  distinct blocks with live acts, `cool_share`. The
  world-is-the-feed doctrine caps what's knowable here by design;
  the ledger's geographic spread is the honest proxy.
- **Institutional adoption** — do cities move? Outreach replies →
  published shelter lists → cities linking back. Manual,
  spreadsheet-grade, slow. One new published list outweighs a
  thousand map opens. Tracked in OUTREACH, not by the server.
- **Trust (guardrails, not growth)** — reports open, time to
  resolve, moderation erasures. These keep "helped tonight" honest:
  a growing finder over rotting shelter data is negative help.

## Anti-vanity list

Map opens, page views, raw pledge counts: diagnostics, never impact.
They may explain a number; they are not the number.

## Instrumentation

- **`GET /api/kpis`** (public, like the leaderboard — it aggregates
  only what the board already shows): act counts, flip rate, night
  m°C, spread, open reports, and a 30-day daily series of the usage
  counters.
- **Daily counter snapshots** (`data/stats/YYYY-MM-DD.json`): the
  counters are lifetime totals; the server snapshots them hourly
  into a per-day file, and a day's activity is the difference
  between two snapshots. Approximate by design — a lost hour is
  noise. This is the time axis that lets "routes during the
  heatwave" be a real number.
- **The back office** (`/admin`) renders the panel: flips + m°C,
  pledged, flip rate, blocks with acts, joins, reports open, and
  7-day sums of routes / opens / shares / city-page visits.

## Cadence

Weekly, one line per question, from the back office panel:

> routes on hot days · flips + night-m°C · flip rate · adapter
> cities + top demand-gap pages · joins per block · reports open

The panel is for steering, not celebrating: each number should
change a decision (build the next adapter, chase flips not pledges,
fix the data a city reported broken) or it drops off the panel.
