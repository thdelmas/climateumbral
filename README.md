# ClimateUmbral

**The map of every place's distance from human-livable temperature — and
the levers that close the gap.**

*Umbral*: Spanish for **threshold**, from Latin *umbra* — **shade**. The
instrument measures how far each block sits from the livability
threshold, day and night separately — night is the killer — and makes
closing that gap playable: cooling acts are the levers, modeled °C is
the score, satellite LST is the honesty check.

> Renamed from **Tilewhip** 2026-07-20: the old name described one
> mechanic (tile-flipping, after NK Tegelwippen); the intent outgrew it —
> depaving is one lever, not the game. Repo/module/URL rename is a
> pending mechanical pass; docs lead.

## Why

- Climate change hits Europe hardest through heat, and heat kills at
  night, indoors, in the neighborhoods with the fewest trees and the
  most concrete.
- The planet-scale framing fails: all built-up area is ~3 tiles in a
  1,000-tile waffle of Earth. Nobody moves for a rounding error.
- The local framing works: dense European city cores are 60–90% sealed
  (Copernicus imperviousness, 10 m, free and open), and sealed surface
  is a lever citizens can actually pull.
- The Netherlands proved the mechanic: [NK
  Tegelwippen](https://www.nk-tegelwippen.nl/) flipped ~17 million
  garden tiles to green since 2020 by making the unit of action
  absurdly small and the score public.

## Why this shape, when climate apps keep failing

Citizen-climate products die on three axes (adversarial review,
2026-07-20). This design answers each structurally, not by doctrine:

1. **Norm contagion needs ambient visibility, not an opt-in feed.** A
   depaved tile, a tree pit, a white roof are visible from the street
   to people who never installed anything. **The world is the feed;
   the instrument just counts.**
2. **Honest numbers must stay meaningful at citizen scale.** Your share
   of global CO2 is 10⁻¹³ °C — numerology. Your block's night
   temperature is local physics: tenths of degrees, yours, felt in
   your bedroom. Degrees are only ever claimed where the physics is
   local (urban heat), never for emissions.
3. **No data custody.** No accounts: a pseudonym plus a per-act bearer
   token kept by your browser. The ledger stores acts on public pixels,
   not people. Satellites audit instead of surveillance.

## Three layers, one threshold

- **Survive tonight** — blue pins are official climate shelters
  (per-city open data; Barcelona's Xarxa de refugis climàtics first),
  deep-green pins are modeled cool islands. The two tiers never mix:
  **a shelter is a promise by a city; a cool island is a model's
  reading.** "Nearest shelter" flies from where you look; geolocation
  never leaves the browser.
- **Cool tomorrow** — pledge a cooling act on a sealed square:
  **depave** (front-line squares: hard-sealed touching green; every
  live claim opens its neighbours — the cascade), plant a **tree
  pit**, brighten a **cool surface**. 90 days to do it or the square
  returns to the pool. Mark it done with photo proof; each act cools
  the model with its own day/night signature.
- **Govern the gap** — **join the block** of any square you can't flip
  yourself: a standing, revocable petition local governance can see,
  scored by how the block's nights cool from the day you sign. V5
  audits regions and governments on *measured* de-sealing and LST
  between satellite epochs — pledges don't count, pixels do.

Every square's panel teaches the **legal path**: your own ground you
may flip (utilities check first); rented needs the owner's yes; public
land — most of the board — routes to city programs and the block
petition. The game never asks for an illegal act — see
`docs/LEGALITY.md`.

## Honesty rules

Every number wears exactly one of three labels, never blended:

| Layer | Claim | Backing |
|---|---|---|
| **Fact** | m² flipped, acts done | photo proof; satellite change-layer audit at scale |
| **Model** | °C per act, cool islands | modeled day/night signatures — labeled as model, uncroppably, until V4 calibrates against measured LST |
| **Promise** | shelters, pledges | a named institution's or person's commitment, shown as such |

Individuals are trusted; cities and governments are audited. No
measured-attribution claims before V4.

## Status

**V3 — Europe is the board.** `make dev` starts the stack (Docker:
Vite/Vue + MapLibre frontend on :5173, Go API on :8080). A slippy map
of the continent: OpenStreetMap under EU-wide Copernicus
imperviousness. Zoom into any city and the front line loads — live
10 m values streamed (and cached) from the EEA image service, no local
data. Claims key to the continent-wide EPSG:3035 10 m pixel grid; the
server validates every pledge against the same upstream pixels. Water
(WAW) merged in. Day/night heat views are structurally different maps
(50 m surface window vs 150 m banked-heat window). The ledger streams
live (SSE); mutation endpoints are rate-limited per IP. Permalink any
square on the continent (`#pe,pn`); the ledger shows flipped and
pledged m² in separate columns, always.

`prototype/index.html` — the original self-contained page (no build,
no backend): Earth in 1,000 tiles, a clickable dense-city km², and a
real 454×454-pixel map of central Barcelona with the
gray-touching-green candidate detector (227 pixels = 2.3 ha found).

`tools/fetch_grid.py` — sealed-% grid + sea mask for any EU bounding
box. Pure Python 3, zero dependencies.

```
python3 tools/fetch_grid.py 2.14 41.375 2.19 41.41 -o bcn
# -> bcn.raw (U8 grid, row 0 = north), bcn.json (metadata + stats)
```

## Roadmap

- **V4 — degrees get real**: calibrate the heat model against measured
  Sentinel-3/MODIS day+night LST; city leaderboards in degrees cooled,
  by municipality size class. See `docs/INTENT.md`.
- **V5 — the audit**: score regions/governments on *measured*
  de-sealing and LST between satellite epochs.

See `docs/GAME_DESIGN.md` for incentive design and
`docs/DATA_SOURCES.md` for verified endpoints and their traps.

## License

Apache-2.0. Data: © European Union, Copernicus Land Monitoring Service
/ EEA. Climate shelters: Ajuntament de Barcelona, Open Data BCN (CC BY
4.0).
