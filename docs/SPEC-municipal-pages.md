# One page per municipality: green space and shade, the law's way

v1, 2026-11-14. Round 1: four seats answered by the owner (below).

## Intent

A council technician, or a resident, can see where their own town lacks green
space and shade, counted the way the EU counts it, and where action would
help most, without having to trust a number nobody can check.

## Goal

1. **The official number, reproduced and checkable.** For each of Spain's 1,342
   urban ecosystem areas: EVU (green space) and CAU (tree canopy) for the
   reference years (CLC+ Backbone 2023, Tree Cover Density 2024), computed by
   `tools/zeu_metrics.py` and shown next to the value the ministry published.
   Acceptance: every page's number equals the published one, or the page says
   by how much and why.
2. **Inside the town.** The same two numbers per neighbourhood (unit: seat S1),
   so a council sees which neighbourhoods are greenest and which have least
   shade.
3. **Over time.** Canopy every year 2018-2024 (the yearly Tree Cover Density
   product is one consistent series), and sealed soil change 2021-2024.
4. **Where to act.** Desealing candidates (the existing "sealed touching green"
   detector) and planting candidates (seat S4), shown as candidates, never as
   faults.
5. **One public page per municipality**, generated from those numbers, linked
   from the map, findable by the municipality's name, in Spanish and English.
6. **Europe is the frame, Spain the first country.** The regulation and the
   Copernicus layers are the same in every Member State; only the urban
   ecosystem area boundaries and the published reference values are
   national. Nothing in the pages or tools assumes Spain beyond those two
   inputs.

## Limits

- **Copernicus and the ministry stay the reference.** A page never presents its
  numbers as the official value; it shows the official value and ours side by
  side. Fire: a page whose number differs from the published one without
  saying so.
- **Candidates, never accusations.** No page names an owner, a building or a
  person. Fire: any page element that identifies who is responsible for a
  sealed or treeless spot.
- **No legal determination.** A page never says a municipality complies or
  fails the regulation; the test is national and set in 2030. Fire: a
  "compliant / non-compliant" label anywhere.
- **Zero custody, as everywhere in ClimateUmbral.** The pages need no account
  and keep no data about who reads them. Fire: a sign-in or a tracker on a page.
- **Licence conditions on every page:** source named (Copernicus Land
  Monitoring Service, ministry values), adaptation stated, no implied EU
  endorsement. Fire: a page missing that footer.

## Seats (owner, round 1, 2026-11-14)

- **S1 Neighbourhood unit: accepted.** INE census sections (secciones
  censales), because they cover every municipality the same way and are
  open; municipal barrios exist only in large cities. Pages could group
  sections into barrios where a city publishes them.
- **S2 Live map imperviousness: (c) accepted.** The map still streams 2018 values. 2024 is
  only downloadable as tiles (not served openly), so moving the map to 2024
  means hosting the tiles on our server. Options: (a) host Spain only (about
  80 tiles; size to measure) and keep 2018 elsewhere, labelled per area;
  (b) host all of Europe (several GB); (c) keep 2018 on the map and use 2024
  on the pages only. Proposal: (c) now, (a) once the pages are live.
- **S3 Languages: Spanish and English first**, "because it's not only about
  Spain but all Europe" (goal 6). Catalan, Basque, Galician later.
- **S4 Planting candidate: accepted.** a pixel that is sealed (imperviousness
  ≥ 80 %) with no canopy (Tree Cover Density 0), within the urban ecosystem
  area, in a block where canopy is below the town's median. Shown
  aggregated, never pixel by pixel on a page.

## Tested

- 2026-11-14: Barcelona, Girona, Madrid, València reproduce the published
  EVU and CAU exactly on the reference years (see DATA_SOURCES.md).
- 2026-11-14, per census section (Barcelona, 1,068 sections, INE 2025):
  sections inside the ZEU add up to 78.025 of the official 78.208 km²; the
  0.183 km² left (port, coastline) lies in the ZEU but in no section, and
  pages show it as its own line. EVU 21.448 / CAU 7.007 km² summed, -0.4 %.
- 2026-11-14, **the official canopy layer does not see street trees.** 554 of
  1,067 Barcelona sections read exactly 0 % canopy (18.1 km², dense
  neighbourhoods); no missing data. A 210 m window at C/ Mallorca x Girona
  (Eixample, rows of plane trees) reads 0 %. Barcelona's own figure (CREAF
  study in the 2024 tree master plan) is 25.2 %, the official one 9.0 %.
  Consequence for goal 2: a page must say that "0 % canopy" means "0 % as the
  regulation counts it at 10 m", never "no trees". Open question for the
  owner: add a second, finer canopy measure (PNOA LiDAR, free, 5 pts/m²), which
  art. 3 allows as "supplementary data" when more precise.
