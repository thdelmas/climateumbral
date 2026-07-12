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

## Known data caveats the design must absorb

- Tree canopy over pavement reads as *unsealed* in the NDVI-based product: some
  "green" is shaded asphalt. Feature, not bug, for candidate detection (shaded
  streets depave well) — but audits must not credit canopy growth as desealing.
- Single pixels lie (see DATA_SOURCES traps); all displayed scores aggregate ≥3×3.
- Epochs are ~3 years apart: T3/T4 scoring is a slow game by design. T1/T2
  keeps the fast loop.
