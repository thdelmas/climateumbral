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

`prototype/index.html` — a self-contained page (no build, no backend, open it)
with:

- Earth in 1,000 tiles (why the global framing fails, as the setup)
- A dense-city km² with clickable depaving
- **A real map of central Barcelona**: 454×454 Copernicus 10 m pixels,
  hoverable, with the gray-touching-green candidate detector (344 pixels = 3.4
  ha found)

`tools/fetch_grid.py` — fetch the sealed-% grid + sea mask for any EU bounding
box. Pure Python 3, zero dependencies (no GDAL, no PIL).

```
python3 tools/fetch_grid.py 2.14 41.375 2.19 41.41 -o bcn
# -> bcn.raw (U8 grid, row 0 = north), bcn.json (metadata + stats)
```

## Roadmap

- **V1 — the map**: slippy map (MapLibre) over the EU imperviousness layers with
  live candidate detection; shareable permalinks to any spot.
- **V2 — the counter**: log flipped m², tegelwippen-style public ledger;
  leaderboards by municipality size class.
- **V3 — the audit**: score regions/governments on *measured* de-sealing between
  Copernicus epochs — pledges don't count, pixels do.

See `docs/GAME_DESIGN.md` for the incentive design across levels and
`docs/DATA_SOURCES.md` for verified endpoints and their traps.

## License

Apache-2.0. Data: © European Union, Copernicus Land Monitoring Service / EEA.
