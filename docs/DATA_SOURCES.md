# Data sources — verified endpoints and their traps

All verified live on 2026-07-12 against central Barcelona. Everything below is
free, open, and needs **no API key or login**.

## Sealed surface: Copernicus HRL Imperviousness Density (IMD)

EEA ArcGIS ImageServer, EU-wide, 10 m pixels, value = % sealed (0–100), U8.

Base:
```
https://image.discomap.eea.europa.eu/arcgis/rest/services/GioLandPublic/HRL_ImperviousnessDensity_2018/ImageServer
```

Epochs on the same open endpoint: `2006`, `2009`, `2012`, `2015`, `2018`, plus
`HRL_ImperviousnessChange_*` and `HRL_ImperviousnessClassifiedChange_*` pairs
(the audit layers for tiers T3/T4). 2021+ lives on the newer Copernicus WCS and
may need EU-Login — unverified.

### Point query (`/identify`)

```
identify?geometry={"x":LON,"y":LAT,"spatialReference":{"wkid":4326}}
        &geometryType=esriGeometryPoint
        &pixelSize={"x":10,"y":10,"spatialReference":{"wkid":3035}}
        &returnGeometry=false&f=json
```

→ `.value` = % sealed of that 10 m pixel.

### Area query (`/exportImage`)

```
exportImage?bbox=LONMIN,LATMIN,LONMAX,LATMAX&bboxSR=4326
           &imageSR=3035&size=W,H&format=tiff&pixelType=U8&f=json
```

→ JSON with `href` to a TIFF. The TIFF is **uncompressed and tiled** (128×128
tiles): parseable in ~40 lines of stdlib Python. See `tools/fetch_grid.py`.

## Water mask: Copernicus HRL Water & Wetness (WAW)

Same server: `GioLandPublic/HRL_WaterWetness_2018`. Same exportImage call.
Classes: 1 permanent water, 2 temporary water, 3/4 wetness, **253 = sea**.

## Official climate shelters: Open Data BCN, Xarxa de refugis climàtics

Verified live 2026-07-18. There is **no EU-wide shelter dataset** — refuge
networks are municipal programs, so coverage is per-city adapters
(`server/refuges.go`); Barcelona is adapter #1. `FOR_CITIES.md` is the
guide we hand to municipalities that want in — what to publish, in what
shape, and how to tell us. 543 refuges, all with
coordinates and addresses, ~130 with an official web link. CC BY 4.0,
updated **weekly** upstream (mirrored with a 7-day TTL).

Dataset page:
```
https://opendata-ajuntament.barcelona.cat/data/es/dataset/xarxa-refugis-climatics
```

Use the **CSV resource** (1.3 MB); the JSON twin of the same data is 40 MB.

### Its traps

1. **The CSV is UTF-16LE** — and carries a stray BOM at the start of *every
   line*, not just the file. Decode, then strip `﻿` globally before the
   CSV parser sees it.
2. **Address columns by header name**, never position — sibling Open Data BCN
   datasets reorder columns between refreshes.
3. **The `values_*` columns are an attribute join.** Today the feed is one row
   per refuge; sibling datasets fan the same schema out to row-per-attribute.
   Dedupe on `register_id` so a fan-out becomes a no-op instead of 5× pins.
4. **`timetable` is embedded HTML** (a full `<table>`). Don't strip-and-truncate
   it into garbled hours — link the refuge's own `Web` attribute instead and
   say "check hours before you go".
5. The `barcelona.cat` refuge pages themselves answer **HTTP 418 to bots** —
   verify links by shape (from the city's own data), not by fetching.

## Official climate shelters: Paris Data, Îlots de fraîcheur

Verified live 2026-07-20. Adapter #2 (`parseParisRefuges`). 533 sites in
the feed, of which **279 make the shelter tier** after filtering (see
trap 1) and deduping. ODbL, refreshed **daily** upstream (hours sync from
paris.fr); mirrored with the same 7-day TTL — we don't show hours, so
daily freshness isn't load-bearing.

Dataset page:
```
https://opendata.paris.fr/explore/dataset/ilots-de-fraicheur-equipements-activites/
```

Use the **JSON export** (`/api/explore/v2.1/.../exports/json`, ~400 KB,
clean UTF-8 — none of Barcelona's encoding traps); the records API pages
at 100 rows. A sibling dataset (`ilots-de-fraicheur-espaces-verts-frais`)
lists parks — that's the modeled cool-island tier's territory, not
adapted here.

### Its traps

1. **Indoor and outdoor types share one dataset.** Libraries, museums,
   mairies (with the plan-canicule cooled rooms), churches, bains-douches
   and pools sit next to misters, shade sails and pétanque grounds. Only
   roofed types are shelters — filter by `type`, as a **whitelist**
   (`parisIndoorTypes`), so a type the city invents next summer defaults
   out until a human reads what it is.
2. **`payant` can be `Oui`** (most museums and pools, 90 of the 279).
   A paywall at the door is part of whether a body gets cooled — the pin's
   addr says `entrée payante`.
3. **A couple of `identifiant`s are duplicated** in the live feed — dedupe
   on it.
4. **No per-site links.** paris.fr venue URLs need a slug the feed doesn't
   carry (`https://www.paris.fr/lieux/<id_dicom>` alone 404s), so Paris
   pins ship without `web`.
5. **Addresses carry doubled spaces** (`15  RUE AMPERE`) — collapse
   whitespace.

## Official climate shelters: Stadt Wien, Coole Zonen

Verified live 2026-07-20. Adapter #3 (`parseWienRefuges`). 36 free
indoor cool rooms (20–24 °C: libraries, pensioners' clubs, municipal
offices) — the whole layer is the tier, no type filter needed. Names,
addresses, opening hours and a weblink per site. CC BY 4.0.

Layer `ogdwien:COOLEZONEOGD` on the city WFS:
```
https://data.wien.gv.at/daten/geo?service=WFS&version=1.1.0&request=GetFeature&typeName=ogdwien:COOLEZONEOGD&outputFormat=json&srsName=EPSG:4326
```

### Its traps

1. **The WFS answers in Gauß-Krüger (EPSG:31256) by default** — ask for
   `srsName=EPSG:4326` in the URL or every pin lands near null island.
   And WFS 1.1 + EPSG:4326 is where lon/lat axis flips live: the parser
   bounds-checks against a loose Vienna bbox and fails loudly rather
   than pin a swapped feed.
2. **`WEBLINK1` carries a trailing newline** in the live feed — trim it.
3. Many sites share one generic name (`Pensionist*innen Klub`) — the
   address is the identity; `OBJECTID` for dedup.
4. A sibling layer `COOLESTRASSENOGD` (cool streets) is **outdoor** —
   same tier trap as Paris; don't adapt it into the shelter layer.

## Official climate shelters: Grand Lyon, Équipements publics climatisés

Verified live 2026-07-20. Adapter #4 (`parseLyonRefuges`). 90 features,
of which **81 make the shelter tier**: the dataset is already curated as
cooled public facilities, but parks, an open-air sports complex and a
cemetery ride along. Licence Ouverte.

Layer on the metropole's WFS (the portal itself is a JS SPA whose API
endpoints all bounce — the WFS at `download.data.grandlyon.com` is the
stable door):
```
https://download.data.grandlyon.com/wfs/grandlyon?SERVICE=WFS&VERSION=2.0.0&REQUEST=GetFeature&typename=metropole-de-lyon:com_donnees_communales.equipementspublicsclimatises&outputFormat=application/json&SRSNAME=EPSG:4326
```

### Its traps

1. **A third of the rows have `type: null`** — they are churches,
   libraries and covered market halls, recognizable by `theme`. The
   whitelist checks `type`, and falls back to a `theme` whitelist only
   for untyped rows.
2. **The `climatise` boolean lies** — it's `false` on sites whose own
   comment says they're cooled. Filter on type/theme, never on it.
3. **`uid` is null on 80 of 90 rows** (only Rillieux-la-Pape fills it) —
   the row identity is `gid`, unique and present on every feature.
4. **Addresses end in a literal `\r`** in the live feed — collapse
   whitespace.
5. Same axis-flip guard as Vienna (loose Grand Lyon bbox, loud failure).

### Cities checked and NOT adaptable (2026-07-20, don't re-derive)

- **Nantes** — `244400404_ilot-fraicheur-nantes-metropole` is 268 sites,
  **all outdoor** (parks, natural spaces, tree-lined squares). It's the
  modeled-cool-island tier published as open data, not a shelter
  network. Do not pin it as shelters.
- **Clermont-Ferrand** — `ilots-de-fraicheur-ville-de-clermont-ferrand`
  is 25,562 rows, 99% individual street trees; ~15 indoor sites buried
  inside. Adaptable in principle, not worth it yet.
- **Zaragoza** — real network (60+ municipal shelters) but published as
  web pages and an app; no machine-readable dataset found.
- **Madrid** — no official operational dataset; a gazette list of 282
  candidate sites exists (protocol CalorMad).

## The traps (each cost real debugging time — do not rediscover them)

1. **`identify` without `pixelSize` answers from a ~21 km overview mosaic**
   (`Ov_*.tif` catalog items) — silently, and the wrong value can coincidentally
   equal the right one. Always pass native `pixelSize`.
2. **The service catalog is stale.** The `SoilSealing/` folder lists services
   that 404. The live layers are under `GioLandPublic/`.
3. **Single 10 m pixels lie.** A 3×3 grid around one Eixample corner reads
   100/8/100/76/41/90/61/83/100. Street-tree canopy masks asphalt (the product
   is NDVI-derived); park promenades read ~45 (gravel). Aggregate ≥3×3 before
   showing a number to a human.
4. **IMD codes the sea as 0** ("not sealed"), not nodata — a naive render paints
   the Mediterranean as a park. Join the WAW sea class (253) as a mask. Coastal
   maps are wrong without it.
5. **WAW top-edge rows contain junk values** outside the class list at bbox
   edges. Use only classes 1–4 and 253; ignore everything else.

## Reference numbers (central Barcelona, 4.5 × 4.5 km, IMD 2018)

- 54% of pixels are 90–100% sealed; <10% of the area is ≤10% sealed.
- Candidate heuristic v1 (pixel ≥90% sealed with ≥3 of 8 neighbors ≤10%): 344
  pixels = **3.4 ha of hard seal touching green**.
- Collserola forest reads a clean 0; Eixample block mean ≈ 73%.

## Adjacent sources (unverified, for later)

- EEA soil-sealing dashboard (context/stats): eea.europa.eu
- NK Tegelwippen counter (the T1/T3 mechanic in production): nk-tegelwippen.nl
- ESA WorldCover 10 m (global land cover — the non-EU fallback)
- GHSL built-up layers (global, coarser — the T4 world view)
