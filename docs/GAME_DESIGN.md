# Game design — incentives per level

Design premise: the same act (unsealing surface) must be scoreable at every
scale, but the *verification* and the *reward* differ per level. One ledger,
four tiers.

## The unit

The **tile**: 1 m² claimed by a human, 100 m² (one 10 m pixel) measured by
satellite. Claims are entered in m²; audits happen in pixels. The two units
reconcile at the next satellite epoch.

## Tiers

### T1 — Individuals & households
- **Action**: flip tiles in yards, façades, tree pits. Even 1 m² counts — the
  entry ticket must be absurdly small (tegelwippen's core lesson).
- **Verification**: self-report + before/after photo. Trust by default; the
  tier's totals are labeled *claimed*, not *measured*.
- **Reward**: personal ledger, street/neighborhood aggregation, "your m² is part
  of X" visibility. No money, no badges-for-nothing: the number itself is the
  trophy.

### T2 — Communities (schools, associations, blocks)
- **Action**: group depaves — schoolyards, parking corners, church lots.
- **Verification**: photo + location; large projects (>100 m²) become visible in
  satellite data and get flagged for epoch-audit.
- **Reward**: project pages, twinning (adopt a candidate site from the map and
  claim it as a goal before it's done — public intention = commitment device).

### T3 — Municipalities
- **Action**: policy + projects (depave programs, permeable re-paving, tree
  pits).
- **Verification**: **measured**. Municipal score = change in sealed area
  between Copernicus imperviousness epochs, per the change layers. Self-reported
  municipal numbers are displayed alongside, and divergence is itself a public
  signal.
- **Reward**: leaderboard in three size classes (NK Tegelwippen's proven format:
  large / medium / small), so a village can beat a capital.

### T4 — Regions & states
- **Action**: legislation (sealing caps, desealing mandates, façade-garden
  rights).
- **Verification**: measured only. Net sealed-area trajectory per epoch,
  published as a sparkline per country/region. The EU's own soil strategy
  targets (no net land take by 2050) become a scoreboard, not a communiqué.
- **Reward/pressure**: ranking + trajectory. Pledges don't move the number;
  pixels do.

## Heat — the currency

Intent-confirmed (see INTENT.md): the score people feel is not
hectares, it is degrees. Every place gets a **delta from human-livable
temperature**, day and night as separate layers with separate optima —
an active body in shade versus a sleeping body at ~19 °C. Night is the
layer that kills: heatwave mortality tracks tropical nights, and night
is where sealed thermal mass does its damage, radiating the day's sun
back until dawn. A park cools after sunset; a parking lot does not.

- **Display:** heatmap of degrees-above-livable (UTCI/PET-style
  comfort framing), day layer and night layer.
- **Score:** modeled °C per flip; city leaderboards read in modeled
  degrees cooled, not m². The m² stays the ledger unit — degrees are
  what the m² *means*.
- **Audit:** measured LST (Sentinel-3 / MODIS, day and night passes)
  is the annual honesty check. The game never claims measured
  attribution of cooling; the model scores, the satellite audits.

Data caveat: LST is ~1 km native, sealed-% is 10 m. The delta map is
therefore a *downscaled model* (LST calibrated against imperviousness
and land cover) and must be labeled modeled. Rule 1 applies to degrees
exactly as it applies to m²: modeled and measured never mix.

Model v0.1 (shipped, `web/src/lib/heat.js`): penalty = coefficient ×
mean sealed fraction, with day 6 °C and night 4 °C as placeholder
magnitudes in the range European SUHI literature reports. Day and
night differ in *structure*, not just scale: the day window is 50 m
(surface heat tracks what the sun hits where you stand), the night
window 150 m (the block's banked thermal mass releasing until dawn) —
so the two views are genuinely different maps.
Flipped pixels count as unsealed, so every flip cools its block in the
model; a candidate's panel states how many flips like it the block
needs to sleep 1 °C cooler. v1 replaces the coefficients with a
calibration against real day/night LST — that step needs a Copernicus
Data Space or NASA Earthdata registration (free, but no longer
keyless; the zero-cost doctrine survives, the no-account purity does
not).

Sustainability note (the binding constraint is money ≈ 0): open data
only, infra one person can run. If monetization exists it sustains the
project — municipal heat-adaptation targeting reports/API from the
same engine — never ads, never user data (rule 5).

## Many levers, one delta

The goal is closing the gap to livable temperature; desealing is one
lever, not the definition of play. An act is any intervention with a
defensible cooling signature. v0 act kinds, each with its own day /
night physics in the model:

| act              | day effect        | night effect      | why |
|------------------|-------------------|-------------------|-----|
| **depave**       | full (s → 0)      | full (s → 0)      | soil neither absorbs like asphalt nor banks heat |
| **tree**         | full (shade)      | none              | canopy blocks sun; the mass under it still releases at night |
| **cool surface** | strong (s × 0.4)  | slight (s × 0.9)  | albedo reflects solar gain; some banked heat avoided |

Validation differs per act. Depave keeps the front-line rule (hard-
sealed touching ≥3 green-or-claimed — extend life, don't pot it).
Trees and cool surfaces are claimable on any hard-sealed square: a
tree pit breaks the middle of a parking lot precisely where no front
line reaches. Depaves and trees count as green for the cascade (both
extend the living network); cool surfaces do not (still sealed).

Only *flipped* acts cool the model — a pledge is a promise, and
promises don't lower anyone's night temperature (rule 1, again).
The ledger unit stays m²; the meaning of an act is its degrees.

Future acts follow the same contract (a day/night signature the model
can defend): façade greening, shade sails, water. The Copernicus Tree
Cover Density layer is the natural audit for tree acts, epochs apart.

## Presence & exposure — the stake layer

Where the tiers define *who scores how*, this layer defines *which pixels are
yours to care about*. Inspiration: Zenly's footsteps (the map you've painted by
living) and Happn's crossed paths (people whose territories overlap). The
generalization: your **stake** in a place is proportional to the time you spend
there.

**Exposure** is the personal metric: your time-weighted distance from
livable temperature (with sealed-% as its proxy until the heat layer
ships) — the same shape as an environmental-health exposure score
(time × condition, summed over places). "Your summer runs +7° above
livable" is the most personal argument for depaving that can exist —
and it is improvable in your own self-interest, because your places
are weighted by the hours you live in them.

Three levels of the same concept, in build order:

1. **Exposure-ranked candidates** (now, server-side, no tracking). Weight the
   candidate detector by estimated human-hours from open data: population
   density, schools, playgrounds, plazas, transit stops. A sealed schoolyard
   holding 400 kids × 6 h beats an empty logistics lot at any sealed-%. Second
   axis next to gray-touching-green: ecological leverage × human leverage.
2. **Personal front line** (web first, zero permissions). Users declare their
   places — draw a commute, drop home/work pins, ten seconds — and the map
   centers their exposure and their candidates. This tests the familiarity
   hypothesis with no permission dialog. Only if it works does the native
   refinement (an on-device dwell-time histogram, nothing leaving the phone)
   become worth building — and even then the permission ask itself carries an
   optics cost no architecture can remove: users experience the OS dialog, not
   the data flow.
3. **Watch coalitions** (explicit acts only). People who watch the same pixel
   are the coalition for it: "3 others watch this square" is matchmaking for a
   depave, and contested demand is the petition forming itself. Watches are
   claim-shaped server-side acts — chosen, visible, revocable. Automatic
   Happn-style path-overlap detection is rejected: computing that two paths
   cross requires comparing them somewhere, which either breaks rule 7 or
   demands private-set-intersection cryptography (Google and Apple needed a
   joint OS framework for exposure notification; a small team does not ship
   that as a side feature).

Valence rule, non-negotiable: **time = stake, never blame.** Dwell-time
responsibility would punish exactly the people with the least power over their
environment (renters, schoolkids, warehouse workers — the tenant dwells in the
courtyard the landlord owns). Presence grants standing and first sight of a
place's candidates; it never assigns guilt and never scores. Score stays what
the ledger says: claims and flips.

Exposure and score stay separate for the same reason claimed and measured do
(rule 1): if exposure were the score, the optimal move would be sitting in
parks, and nothing would get depaved. Exposure is the mirror that motivates; the
ledger is what counts.

Platform note (council-reviewed 2026-07-12): the game stays a webapp until
flips-per-week proves the core loop. The growth engine — permalinks,
city-vs-city rivalry — lives on the web; an install wall would trade a working
viral loop for a speculative one. PWAs get no usable background geolocation on
either platform, so the only native trigger is proven retention plus users
asking for a daily companion. If retention never comes, native was never the
answer.

## Design rules

1. **Claimed vs measured are separate columns, always.** Mixing them is the
   greenwashing vector. The game's credibility = the audit tier.
2. **Candidates, not guilt.** The map never says "you are 92% sealed, shame"; it
   says "these 344 squares touch green — start here."
3. **The smallest action must be logabble.** If the entry ticket exceeds one
   tile, T1 dies and the upper tiers lose their base.
4. **Public intention is a mechanic.** Claiming a candidate site as a goal
   (before flipping) creates the commitment device tegelwippen lacks.
5. **No paid ads, no data collection beyond the ledger.** The scarce resource is
   trust.
6. **Verification degrades gracefully.** Where the satellite can't see (canopy
   over asphalt, sub-pixel flips), the ledger stays honest about its error bars.
7. **Presence data never leaves the device.** A project whose moral authority is
   auditing others with satellites cannot itself run location surveillance.
   Dwell histograms are computed and kept on the phone; only explicit acts
   (claims, flips, opt-in watches) reach the server. Rule 5, applied to the most
   sensitive data class there is.

## Known data caveats the design must absorb

- Tree canopy over pavement reads as *unsealed* in the NDVI-based product: some
  "green" is shaded asphalt. Feature, not bug, for candidate detection (shaded
  streets depave well) — but audits must not credit canopy growth as desealing.
- Single pixels lie (see DATA_SOURCES traps); all displayed scores aggregate
  ≥3×3.
- Epochs are ~3 years apart: T3/T4 scoring is a slow game by design. T1/T2 keeps
  the fast loop.
- The satellite cannot police the players: 10 m pixels never verify a 1 m²
  flip, so T1/T2 scores rest on photo proof — which means fraud handling and
  moderation cost the moment city rivalry makes cheating worth it. Budget for
  it before the leaderboard ships.
- Most sealed pixels are land the player cannot legally touch (roads,
  municipal lots, other people's property). Tegelwippen works on *own
  gardens*. Candidates need a tenure hint: plausibly-yours (flip it) vs
  public/institutional (watch it, petition it) — never "go depave the road."
- The ledger is already location data: a pseudonym's claims cluster around
  their home. GDPR discipline (minimal fields, right to erasure, no
  IP-to-claim linkage kept) starts at V1.5, not at the native app.
