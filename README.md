# Tilewhip

**Gamified de-sealing of the Earth's surface — from one square meter to national
policy.**

Every paved square that becomes soil again is a win you can count. Tilewhip
turns sealed-surface data into a game board: see exactly which concrete near you
touches green, flip it, log it, and let satellites keep the score — for
individuals, neighborhoods, cities, and governments alike.

## Why

- The planet-scale framing fails: all built-up area is ~3 tiles in a 1,000-tile
  waffle of Earth. Nobody moves for a rounding error.
- The local framing works: dense European city cores are 60–90% sealed
  (Copernicus imperviousness, 10 m resolution, free and open).
- The Netherlands proved the mechanic: [NK
  Tegelwippen](https://www.nk-tegelwippen.nl/) has flipped ~17 million garden
  tiles to green since 2020 by making the unit of action absurdly small (one
  30×30 cm tile) and the score public (city vs city).

Tilewhip generalizes that mechanic and grounds it in measured data instead of
self-report alone.

## The core loop

1. **See** — a map of your area colored by sealed-%, with *candidates*
   highlighted: hard-sealed pixels touching existing green, where one flip
   extends life instead of creating an isolated pot.
2. **Flip** — depave something. A tile in your yard. A parking lane. A
   schoolyard.
3. **Log** — claim your m² on the public counter (photo before/after).
4. **Verify** — satellite epochs close the loop: claims at scale should appear
   in the next Copernicus imperviousness *change* layer. Individuals are
   trusted; cities and governments are audited.

## Status

**V3 — Europe is the board.** `make dev` starts the stack (Docker:
Vite/Vue + MapLibre frontend on :5173, Go API on :8080). A slippy map
of the whole continent: OpenStreetMap under the EU-wide Copernicus
imperviousness layer. Zoom into any city and the front line loads —
live 10 m values streamed (and cached) from the EEA image service, no
local data at all. Claims are keyed to the continent-wide EPSG:3035
10 m pixel grid, and the server validates every pledge against the
same upstream pixels. Day/night modeled heat views work everywhere.

On any city you can:

- **pledge** a candidate pixel (hard-sealed touching green) — 90 days to
  flip it or it returns to the pool, and every live claim counts as
  green, opening its neighbours (the cascade)
- **flip** it with an optional photo URL as proof
- **watch** any sealed pixel you can't flip yourself — coalitions for
  public land
- share a **permalink** to any square on the continent (`#pe,pn`); see
  the **ledger** with flipped and pledged m² in separate columns,
  always

No accounts: a pseudonym plus a per-act bearer token (kept by your
browser) that lets you flip or erase your own acts — the ledger stores
nothing else.

`prototype/index.html` — the original self-contained page (no build, no
backend, open it) with:

- Earth in 1,000 tiles (why the global framing fails, as the setup)
- A dense-city km² with clickable depaving
- **A real map of central Barcelona**: 454×454 Copernicus 10 m pixels,
  hoverable, with the gray-touching-green candidate detector (227 pixels = 2.3
  ha found)

`tools/fetch_grid.py` — fetch the sealed-% grid + sea mask for any EU bounding
box. Pure Python 3, zero dependencies (no GDAL, no PIL).

```
python3 tools/fetch_grid.py 2.14 41.375 2.19 41.41 -o bcn
# -> bcn.raw (U8 grid, row 0 = north), bcn.json (metadata + stats)
```

## Roadmap

- **V4 — degrees get real**: calibrate the heat model against measured
  Sentinel-3/MODIS day+night LST; city leaderboards in degrees cooled,
  by municipality size class. See `docs/INTENT.md`.
- **V5 — the audit**: score regions/governments on *measured* de-sealing
  and LST between satellite epochs — pledges don't count, pixels do.

See `docs/GAME_DESIGN.md` for the incentive design across levels and
`docs/DATA_SOURCES.md` for verified endpoints and their traps.

## License

Apache-2.0. Data: © European Union, Copernicus Land Monitoring Service / EEA.
