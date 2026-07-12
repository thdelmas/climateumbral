# Game design — incentives per level

Design premise: the same act (unsealing surface) must be scoreable at every scale,
but the *verification* and the *reward* differ per level. One ledger, four tiers.

## The unit

The **tile**: 1 m² claimed by a human, 100 m² (one 10 m pixel) measured by satellite.
Claims are entered in m²; audits happen in pixels. The two units reconcile at the
next satellite epoch.

## Tiers

### T1 — Individuals & households
- **Action**: flip tiles in yards, façades, tree pits. Even 1 m² counts — the entry
  ticket must be absurdly small (tegelwippen's core lesson).
- **Verification**: self-report + before/after photo. Trust by default; the tier's
  totals are labeled *claimed*, not *measured*.
- **Reward**: personal ledger, street/neighborhood aggregation, "your m² is part of
  X" visibility. No money, no badges-for-nothing: the number itself is the trophy.

### T2 — Communities (schools, associations, blocks)
- **Action**: group depaves — schoolyards, parking corners, church lots.
- **Verification**: photo + location; large projects (>100 m²) become visible in
  satellite data and get flagged for epoch-audit.
- **Reward**: project pages, twinning (adopt a candidate site from the map and
  claim it as a goal before it's done — public intention = commitment device).

### T3 — Municipalities
- **Action**: policy + projects (depave programs, permeable re-paving, tree pits).
- **Verification**: **measured**. Municipal score = change in sealed area between
  Copernicus imperviousness epochs, per the change layers. Self-reported municipal
  numbers are displayed alongside, and divergence is itself a public signal.
- **Reward**: leaderboard in three size classes (NK Tegelwippen's proven format:
  large / medium / small), so a village can beat a capital.

### T4 — Regions & states
- **Action**: legislation (sealing caps, desealing mandates, façade-garden rights).
- **Verification**: measured only. Net sealed-area trajectory per epoch, published
  as a sparkline per country/region. The EU's own soil strategy targets (no net
  land take by 2050) become a scoreboard, not a communiqué.
- **Reward/pressure**: ranking + trajectory. Pledges don't move the number;
  pixels do.

## Presence & exposure — the stake layer

Where the tiers define *who scores how*, this layer defines *which pixels
are yours to care about*. Inspiration: Zenly's footsteps (the map you've
painted by living) and Happn's crossed paths (people whose territories
overlap). The generalization: your **stake** in a place is proportional
to the time you spend there.

**Exposure** is the personal metric: your time-weighted sealed-%, the
same shape as an environmental-health exposure score (time × condition,
summed over places). "82% of your waking hours happen on 74% sealed
ground" is the most personal argument for depaving that can exist — and
it is improvable in your own self-interest, because your places are
weighted by the hours you live in them.

Three levels of the same concept, in build order:

1. **Exposure-ranked candidates** (now, server-side, no tracking).
   Weight the candidate detector by estimated human-hours from open
   data: population density, schools, playgrounds, plazas, transit
   stops. A sealed schoolyard holding 400 kids × 6 h beats an empty
   logistics lot at any sealed-%. Second axis next to gray-touching-
   green: ecological leverage × human leverage.
2. **Personal front line** (native app, on-device only). The phone
   keeps its own dwell-time histogram, intersects it with locally
   downloaded grid tiles, and shows *your* streets' candidates and
   *your* exposure. Nothing leaves the device.
3. **Crossing coalitions** (opt-in only). People whose stakes overlap
   on a pixel are the coalition for it: "3 others watch this square"
   is matchmaking for a depave. Contested demand is not a conflict —
   it is the petition forming itself.

Valence rule, non-negotiable: **time = stake, never blame.** Dwell-time
responsibility would punish exactly the people with the least power over
their environment (renters, schoolkids, warehouse workers — the tenant
dwells in the courtyard the landlord owns). Presence grants standing and
first sight of a place's candidates; it never assigns guilt and never
scores. Score stays what the ledger says: claims and flips.

Exposure and score stay separate for the same reason claimed and
measured do (rule 1): if exposure were the score, the optimal move would
be sitting in parks, and nothing would get depaved. Exposure is the
mirror that motivates; the ledger is what counts.

## Design rules

1. **Claimed vs measured are separate columns, always.** Mixing them is the
   greenwashing vector. The game's credibility = the audit tier.
2. **Candidates, not guilt.** The map never says "you are 92% sealed, shame";
   it says "these 344 squares touch green — start here."
3. **The smallest action must be logabble.** If the entry ticket exceeds one
   tile, T1 dies and the upper tiers lose their base.
4. **Public intention is a mechanic.** Claiming a candidate site as a goal
   (before flipping) creates the commitment device tegelwippen lacks.
5. **No paid ads, no data collection beyond the ledger.** The scarce resource is
   trust.
6. **Verification degrades gracefully.** Where the satellite can't see (canopy
   over asphalt, sub-pixel flips), the ledger stays honest about its error bars.
7. **Presence data never leaves the device.** A project whose moral authority
   is auditing others with satellites cannot itself run location surveillance.
   Dwell histograms are computed and kept on the phone; only explicit acts
   (claims, flips, opt-in watches) reach the server. Rule 5, applied to the
   most sensitive data class there is.

## Known data caveats the design must absorb

- Tree canopy over pavement reads as *unsealed* in the NDVI-based product: some
  "green" is shaded asphalt. Feature, not bug, for candidate detection (shaded
  streets depave well) — but audits must not credit canopy growth as desealing.
- Single pixels lie (see DATA_SOURCES traps); all displayed scores aggregate ≥3×3.
- Epochs are ~3 years apart: T3/T4 scoring is a slow game by design. T1/T2
  keeps the fast loop.
