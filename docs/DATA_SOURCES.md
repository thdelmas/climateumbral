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
