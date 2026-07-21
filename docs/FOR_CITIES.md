# Put your city's climate shelters on the map

*A guide for municipal staff and official representatives. We read
English, French, Spanish, Catalan and German — write in whichever is
easiest.*

ClimateUmbral shows official climate shelters — rooms a city promises
its residents on dangerous heat days — as pins on a public map of
Europe, next to each block's modeled night heat. Barcelona, Paris,
Vienna and Grand Lyon are live today. Adding your city costs us a day
and costs you nothing but publishing data you likely already have.

There is no EU-wide shelter dataset, so we integrate one municipal
dataset at a time, straight from your own open-data portal. Your data
stays yours: we mirror it read-only, refresh it on your cadence, and
show your attribution and license on the map.

## What qualifies as a shelter

A pin on our shelter layer is a promise: **a real room — roof, cool
air, opening hours — that a person can walk into on a hot day or
night.** Libraries, museums, civic and senior centres, town halls,
cooled rooms, churches, pools, covered markets.

Parks, fountains, misters, shade structures and other outdoor cool
spots are valuable, but they are a **different layer** on our map. If
your dataset mixes both (many do), that's fine — give each site a
`type` or `category` field so the two can be told apart. What we will
never do is pin an outdoor site as a shelter: a wrong "shelter" sends
a body somewhere that won't cool it.

## What to publish

One dataset, one row per site, at a stable public URL on your
open-data portal.

**Required per site:**

| Field | Notes |
|---|---|
| Stable unique ID | Survives refreshes; lets updates replace, not duplicate |
| Name | As signposted on the street |
| Latitude, longitude | WGS84 (EPSG:4326) decimal degrees |
| Address | Street + number, human-readable |
| Type/category | Enough to tell indoor rooms from outdoor cool spots |

**Strongly recommended:**

| Field | Notes |
|---|---|
| Opening hours | Plain text is fine; a shelter closed at night should say so |
| Free / paid entry | A paywall at the door matters to who gets cooled |
| Per-site web page | Where people check current hours |

**Format:** CSV, JSON, or GeoJSON — whatever your portal produces
(Opendatasoft/CKAN exports and WFS endpoints all work). UTF-8. No
authentication, no API key.

**License:** any open license — CC BY 4.0, ODbL, Licence Ouverte, or
your national equivalent. We display your attribution verbatim.

**Update cadence:** state it on the dataset page. Weekly is plenty;
we mirror with a 7-day cache and serve your last good data if your
portal has an outage.

## Five pitfalls we keep meeting (please avoid them)

Each of these is live in a real city feed we've integrated:

1. **Non-UTF-8 encodings** — one city serves UTF-16 CSVs with a byte-
   order mark on every line.
2. **Local coordinate systems** — one city's API answers in national
   grid coordinates unless WGS84 is explicitly requested. Publish
   WGS84, and in longitude-latitude order for GeoJSON.
3. **Unreliable IDs** — one city's ID column is empty on 80 of 90
   rows; another duplicates IDs. Every row, one stable ID.
4. **HTML inside data fields** — one city embeds a full HTML table in
   its opening-hours column. Plain text, please.
5. **Status fields that don't mean what they say** — one city's
   `air-conditioned` boolean is false on sites its own comment field
   calls cooled. If a field exists, keep it true.

## How to tell us

Once the dataset is public (or if it already is), open an issue at
[github.com/thdelmas/climateumbral](https://github.com/thdelmas/climateumbral/issues)
— or email **contact@climateumbral.eu** — with:

1. **Dataset URL** — the direct download/API link, not just the portal
   page.
2. **License** and the exact attribution line to display.
3. **Update cadence** — how often it refreshes.
4. **A contact** — for questions while we build the adapter, and so we
   can tell you when your city is live.
5. If your dataset mixes indoor and outdoor sites: **which `type`
   values are real rooms**. You know your sites; we'd rather ask than
   guess.

We'll build and test the adapter, and your shelters appear on the map
with your attribution. If the format later changes, the map keeps
serving the last good copy while we adapt — but a heads-up saves days.
