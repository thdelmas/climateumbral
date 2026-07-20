# Intent — interview-confirmed 2026-07-12

The confirmed statement of what ClimateUmbral (ex-Tilewhip, renamed
2026-07-20) is for. Downstream design decisions answer to this file.

- **Outcome:** a European instrument that shows every place's distance
  from *human-livable temperature* — day and night separately, night
  is the killer — and makes closing that gap playable: cooling acts
  are the levers (depaving, tree shade, cool surfaces — anything with
  a defensible day/night signature), modeled °C-per-act is the score,
  satellite LST is the honesty check. (Amended 2026-07-12: depaving
  is one way to fight temperature, not the definition of play.)
- **User:** anyone with a body in a European summer. Cities, NGOs, and
  Tegelwippen-style orgs run competitions *on top of* the instrument —
  the maintainer does not run the movement.
- **Why now:** climate hits Europe hardest through heat; the data
  (Copernicus imperviousness + LST) is free and pan-EU; the Dutch
  proved the mechanic works.
- **Success:** the heatmap in degrees-above-livable exists and cities
  compete in *degrees cooled*, not hectares; one real city's summer
  gets measurably discussed in Tilewhip numbers.
- **Constraint:** money ≈ 0. Everything must stay runnable by one
  person on open data and near-free infra. Monetization (municipal
  reports/API) is welcome only to sustain that — never ads, never
  user data.
- **Out of scope:** native app (until web retention earns it), running
  seasons/press ourselves, accounts, and *claiming measured
  attribution* of cooling — the model scores, the satellite audits
  annually.

## Big picture — amended 2026-07-20

**Name:** ClimateUmbral — *umbral* = threshold (ES), from *umbra* =
shade. The instrument measures distance from the livability threshold;
the old name (Tilewhip) described one lever and undersold the intent.

**Position:** this is the citizen-climate-agency instrument that
survives where climate apps die. An adversarial review (3 refuter
lenses, KB incubator 2026-07-20) killed a sibling idea — a
climate-action contagion feed — on three fatals. ClimateUmbral answers
all three structurally:

1. *Ambient contagion:* norm spread (smoking/seatbelts) needs
   involuntary visibility; opt-in feeds only reach the converted.
   Physical cooling acts are visible from the street to
   non-installers — the world is the feed, the instrument counts.
2. *Honest numbers that still carry hope:* citizen-scale global-CO2
   arithmetic is numerology (10⁻¹³ °C/t); block-scale night heat is
   local physics with felt, non-trivial degrees. Claim degrees only
   where physics is local.
3. *Zero custody:* no accounts, no behavior ledger, no view-graph —
   acts on public pixels, satellite audit instead of surveillance.

**Attribution doctrine (three labels, never blended):** fact (counted,
photo + satellite audit) / model (labeled as model, uncroppable, until
V4 calibration) / promise (a named institution's commitment). Labels
don't survive screenshots — the UI must make the label part of the
number.

**Three layers, one threshold:** survive tonight (shelters) · cool
tomorrow (acts) · govern the gap (petitions + V5 audit). Features that
don't serve one of the three layers don't enter.

**Scope — amended 2026-07-20:** Europe-first is a data subsidy, not a
ceiling. The EEA streams 10 m sealed-% pixels for free — that service
is what makes zero-infra solo hosting possible, and it ends at
Europe's edge. Going global needs a funded tile layer (GHSL/GAIA) and
per-region grids; already world-capable: the heat physics, the LST
audit, the game design, the ledger. Home: **climateumbral.eu**.
