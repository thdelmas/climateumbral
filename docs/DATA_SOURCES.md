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
4. **`timetable` is embedded HTML** (a full `<table>`) — since 2026-08-20
   reduced to structured per-day hours by `parseBCNTimetable` (the
   Ajuntament confirmed the column's shape by mail, 2026-08-20; ~369 of
   545 rows reduce cleanly, the rest claim nothing). Its own sub-traps,
   all live in the feed:
   - `&nbsp;` arrives as a **literal entity**, undecoded — unescape before
     any regex sees the text.
   - Seasonal tables mark the applicable period with a
     `timetable-period current` class. Periods present but **no `current`
     mark = every period is out of season today** — claim nothing.
   - A day cell whose `rowspan` spans several hour rows is the feed's
     row-by-row spelling of a **split day** — ranges accumulate. A fresh
     day cell naming an already-set day is a refinement — it replaces.
   - Day phrases containing digits are **dated exceptions** ("Dimarts 23
     de juny") — skip them rather than smear one date over every week.
   - "Tancat"/"Tancada" can live in the day cell itself ("Tots els dies
     tancat"), with the hour cell empty.
   The JSON twin additionally carries per-refuge **iCal** timetables
   (Ajuntament's pointer) — richer, but 40 MB against an 8 MB fetch cap;
   the CSV tables carry the same weekly truth.
5. The `barcelona.cat` refuge pages themselves answer **HTTP 418 to bots** —
   verify links by shape (from the city's own data), not by fetching.

## Official climate shelters: Paris Data, Îlots de fraîcheur

Verified live 2026-07-20. Adapter #2 (`parseParisRefuges`). 533 sites in
the feed, of which **279 make the shelter tier** after filtering (see
trap 1) and deduping. ODbL, refreshed **daily** upstream (hours sync from
paris.fr); mirrored with the same 7-day TTL — hours display and open-now
come from the weekly mirror, so a few days of upstream drift is the
accepted cost.

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

> **DEPUBLISHED upstream 2026-07 — endpoint returns 401.** Grand Lyon
> support (Valérie, 2026-07-29, tracking code MKFFGWVJ, replying to our
> 2026-07-25 request) says the dataset was pulled because the data was
> old; they have asked the owning service for a refresh and will
> republish. Live site correctly reports `lyon: ok:false, count:0` —
> designed behavior, no action needed in code. When it comes back,
> re-verify the typename (it may change) and the traps below before
> trusting the weekly fetch again. Follow-up lives on
> data.grandlyon.com support under that tracking code (their mail is
> no-reply).
>
> 2026-08-03: ticket hit "Précisions demandées à l'utilisateur"
> (29-07); précisions submitted in-portal same day (our usage, exact
> typename, two asks: notify on republication + does the typename
> change, and any interim equivalent dataset; offered that the old
> vintage republished with its reference date still serves us). Ball
> is theirs (producing service). Probe the ticket if silent by
> ~24-08.

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

## Official climate shelters: Dades Obertes Catalunya, Refugis climàtics municipals

Verified live 2026-08-20. Adapter #5 (`parseCatRefuges`) — the first
**region** adapter: one dataset carries every Catalan municipality's
declared refuges (pointed to us by the Ajuntament de Barcelona's own
refuges team). 2,590 rows live, of which **2,040 pin** after yielding
Barcelona to adapter #1, deduping, and dropping 3 unrecoverable rows.
Generalitat de Catalunya, Departament d'Interior; CC BY 4.0; updated
continuously as municipalities file (mirrored with the 7-day TTL).

Dataset page:
```
https://analisi.transparenciacatalunya.cat/Seguretat/Refugis-clim-tics-dels-municipis-de-Catalunya/9gu7-iwci
```

Use the Socrata JSON API (`/resource/9gu7-iwci.json`) with
`$limit=10000` — the default page is 1,000 rows — and
`$order=indicador_refugi` for a stable read.

### Its traps

1. **~46% of coordinates lost their decimal point.** Municipality-typed
   values arrive as digit runs at arbitrary precision: `"41521503"` is
   41.521503, `"4199888810552947"` is 41.998888…. Latitude re-points
   unambiguously (all of Catalunya is 40–43°N, the point goes after two
   digits). Longitude is genuinely ambiguous — `"1901389"` could read
   1.901389 or 0.1901389, both inside Catalunya.
2. **Longitudes below 1°E also lost their leading zero** (`"621424"` =
   0.621424), which is why the digit count alone can't give the scale.
   Resolution: per-province longitude windows keyed off the INE
   municipality code (Barcelona 1.30–2.90, Girona 1.60–3.35, Lleida
   0.30–1.90, Tarragona 0.20–1.70) admit at most one of the two
   candidate readings. Verified over the full feed: zero ambiguous.
3. **Municipality codes are mangled too** — `"080193"` arrives as
   `"80193"` or even `"8180"` (leading zero AND check digit gone). Only
   `08` has a zero to lose, so any code leading with 8 is Barcelona
   province; 17/25/43 pass through.
4. **Barcelona files its 546 rows here as well.** Adapter #1 has the
   same network with timetables (→ open-now), so the region adapter
   skips `municipi == "Barcelona"` or every city shelter double-pins.
5. **3 rows are mis-geocoded upstream** (Casal Cívic rows for Berga,
   Sabadell, Vilafranca sitting 50–100 km west of their towns). Their
   clean-looking longitudes fall outside their province's window, so
   the same guard that fixes trap 2 drops them — logged, counted.
6. **`informacio_web` is a mixed column**: URLs, bare `www.` hosts, and
   free text (opening hours, phone numbers). Only link-shaped values
   become links; the free text is NOT surfaced as hours because nothing
   marks which rows are hours.
7. **`indicador_refugi` is the identity**; one metropolitan park
   spanning two municipalities (Parc de Torreblanca) appears twice.
8. No opening hours anywhere in the schema — under PROCICAT heat
   activation the refuges must open in defined windows, but that's
   policy, not per-row data. Pins carry no open-now claim.

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

## Real temperature: Landsat 8/9 Surface Temperature (day, 30 m)

Verified live 2026-07-25. Feeds `tools/fetch_lst.py day` and the
2026-07-25 calibration of `heat.js` (`tools/calibrate_heat.py`).
Collection-2 Level-2 `ST_B10` (Kelvin = DN × 0.00341802 + 149, DN 0 =
fill), ~10:30 solar overpass, via **Microsoft Planetary Computer** —
STAC search and data access both anonymous:

```
POST https://planetarycomputer.microsoft.com/api/stac/v1/search
     {"collections":["landsat-c2-l2"], ...}
GET  https://planetarycomputer.microsoft.com/api/sas/v1/token/landsat-c2-l2
```

Append the SAS token as the query string of the asset href, then plain
HTTP **range reads** against the COG: uint16, DEFLATE, 256×256 tiles,
no predictor — a header read plus a handful of tile reads per scene,
stdlib-only (same spirit as the IMD TIFF reader).

### Its traps

1. **The ST asset key is `lwir11`**, not `ST_B10`; the blob path carries
   a second (processing) date the scene id lacks — always take the href
   from the STAC item, never build it.
2. **Mask with `qa_pixel`** (bits 0–4: fill, dilated cloud, cirrus,
   cloud, shadow) — same tile layout, one more COG.
3. **~10:30 is not the day's peak**: a day slope fit from Landsat
   understates mid-afternoon. Sub-100 m afternoon+night LST exists
   (ECOSTRESS) but needs an Earthdata login.
4. Scene footprints clip bboxes near WRS path edges — composite several
   scenes and require ≥3 clear samples per pixel.

## Real temperature: MODIS Aqua night LST via GIBS (night, 1 km)

Verified live 2026-07-25. Layer
`MODIS_Aqua_L3_Land_Surface_Temp_8Day_Night` (~01:30 solar) on NASA
GIBS, keyless. GIBS serves **colormapped PNGs**, but the colormap is
published and WMS resamples nearest, so every pixel inverts exactly
back to Kelvin (~0.6 K bins — fine for slope fitting, not for absolute
precision):

```
https://gibs.earthdata.nasa.gov/wms/epsg4326/best/wms.cgi?...&VERSION=1.1.1
https://gibs.earthdata.nasa.gov/colormaps/v1.3/MODIS_Land_Surface_Temp.xml
```

### Its traps

1. **The colormap is shared**, named `MODIS_Land_Surface_Temp.xml` — the
   per-layer name 404s. Find it via the layer's `ows:Metadata` in the
   WMTS capabilities.
2. **Use WMS 1.1.1**, which keeps lon/lat bbox order; 1.3.0 flips axes
   on EPSG:4326.
3. **TIME must be an 8-day period start** (DOY 1+8k per year).
4. Sea/cloud pixels come back alpha-0; count non-palette colors and
   fail loudly if any appear (0 observed with nearest resampling).
5. Aqua's orbit has been drifting later since 2022 — "~01:30" is
   nominal.

## Station records: NOAA GHCN-hourly (air temperature, keyless)

Verified live 2026-07-26; adapted as the measured anchor pins
(`server/stations.go`, `/api/stations`). Real thermometers, hourly,
public HTTP, no key. Design rule 1 at its strictest: these measured
degrees never mix into the modeled heat layers, and every surface
they appear on says "measured".

```
.../global-historical-climatology-network/hourly/access/by-year/{Y}/psv/GHCNh_{ID}_{Y}.psv
.../global-historical-climatology-network/hourly/doc/ghcnh-station-list.txt
```

(base `https://www.ncei.noaa.gov/oa/`). Pipe-separated, 1-indexed:
field 3 = ISO date (UTC), field 12 = air temp °C. The by-year file is
append-only — a `Range: bytes=-262144` tail read is weeks of data.

Live Barcelona anchors: `SPM00008180` (city, WMO 08180, 176 m) and
`SPUP00000Y9` (Port — ZAL Prat, 4 m). Feed lags ~2–3 days; the pin
shows the observation time, never "now".

### Its traps

1. **ISD-lite is dead for Spain.** The predecessor product
   (`pub/data/noaa/isd-lite/`) verified fine on 2026-07-25 and turned
   out to have no Spanish data since mid-2025 and no 2026 directory
   at all. Currency of the *specific station's current-year file* is
   the test, not the product's existence.
2. **Most area stations have no current file.** Of 12 GHCNh ids
   around Barcelona (airport, Fabra, Raval…), only the two above are
   live in 2026. Probe the by-year URL before adding any station.
3. **Misses can answer HTTP 200** with an XML error document from the
   backing bucket — check the first byte, not just the status.
4. A ranged read lands mid-line: drop the fragment's first line.
5. Missing temps are empty fields; a bad parse reads them as 0 °C —
   skip, never zero. Sanity-clamp to −60…60.

## Adjacent sources (unverified, for later)

- EEA soil-sealing dashboard (context/stats): eea.europa.eu
- NK Tegelwippen counter (the T1/T3 mechanic in production): nk-tegelwippen.nl
- ESA WorldCover 10 m (global land cover — the non-EU fallback)
- GHSL built-up layers (global, coarser — the T4 world view)

### Shelter-adapter leads (2026-07-20)

EXTREMA's partner cities (extrema-global.com) — **Athens, Milan,
Mallorca, Rotterdam, Greater London** — each maintain a cooling-places
list to feed that app, so the data exists even where no open dataset
surfaced yet. When hunting adapter #5+, start by asking these cities
(via `FOR_CITIES.md`) to publish what they already give EXTREMA.
Neighbor maps worth studying while at it: Berliner Erfrischungskarte
(hour-by-hour LIDAR shade — erfrischungskarte.odis-berlin.de) and
Cool Walks (shade-optimal routing — cool.bcnregional.com).
