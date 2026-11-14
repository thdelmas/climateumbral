#!/usr/bin/env python3
"""EVU and CAU per census section, inside one urban ecosystem area.

Same counting as zeu_metrics.py (CLC+ Backbone green classes, Tree Cover
Density sum, 10 m EPSG:3035 grid), split by the sections of the INE census
sectioning (secciones censales, ETRS89 / UTM 30N shapefile). Only pixels
inside the ZEU count, so the sections add up to the municipality's official
number; a section's share of pixels inside the ZEU is reported too.

Usage:
    python3 section_metrics.py ZEU.gpkg GISCO_ID SECTIONS_BASE \\
        --tiles DIR [--csv OUT.csv]

SECTIONS_BASE = path of the shapefile without extension
(e.g. .../SECC_CE_20250101). Stdlib only.
"""
import argparse
import csv
import math
import sys

from geotiff import mosaic
from proj import utm30_to_laea
from shapefile import Shapes, read_dbf
from zeu_metrics import GREEN_CLASSES, PIXEL, bbox_of, load_zeu, mask_for, tiles_in
import os


def section_polys(base, ine_code):
    rows = read_dbf(base + ".dbf")
    sh = Shapes(base)
    out = []
    for i, r in enumerate(rows):
        if r and r.get("CUMUN") == ine_code:
            rings = [[utm30_to_laea(x, y) for x, y in ring] for ring in sh.rings(i)]
            out.append((r["CUSEC"], r.get("CDIS"), r.get("CSEC"), [rings]))
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("zeu")
    ap.add_argument("gisco_id")
    ap.add_argument("sections")
    ap.add_argument("--tiles", required=True)
    ap.add_argument("--clc-year", default="2023")
    ap.add_argument("--tcd-year", default="2024")
    ap.add_argument("--csv")
    a = ap.parse_args()

    rec, zpolys = load_zeu(a.zeu, a.gisco_id)
    x0, y0, x1, y1 = bbox_of(zpolys)
    w, h = (x1 - x0) // PIXEL, (y1 - y0) // PIXEL
    zmask = mask_for(zpolys, x0, y1, w, h)
    clc = mosaic(tiles_in(os.path.join(a.tiles, "clc", a.clc_year), x0, y0, x1, y1), x0, y1, w, h)
    tcd = mosaic(tiles_in(os.path.join(a.tiles, "tcd", a.tcd_year), x0, y0, x1, y1), x0, y1, w, h)

    code = a.gisco_id.split("_")[-1]
    secs = section_polys(a.sections, code)
    if not secs:
        sys.exit(f"no census sections for {code}")
    claimed = bytearray(w * h)  # a pixel belongs to one section only
    rows = []
    for cusec, dist, sec, polys in secs:
        sx0, sy0, sx1, sy1 = bbox_of(polys)
        # clip the section's window to the ZEU grid
        cx0, cy1 = max(sx0, x0), min(sy1, y1)
        cx1, cy0 = min(sx1, x1), max(sy0, y0)
        sec_px = zeu_px = evu = tsum = 0
        if cx1 > cx0 and cy1 > cy0:
            sw, shh = int((cx1 - cx0) // PIXEL), int((cy1 - cy0) // PIXEL)
            m = mask_for(polys, cx0, cy1, sw, shh)
            c_off, r_off = int((cx0 - x0) // PIXEL), int((y1 - cy1) // PIXEL)
            for r in range(shh):
                base = (r_off + r) * w + c_off
                row = m[r * sw:(r + 1) * sw]
                for c in range(sw):
                    if not row[c]:
                        continue
                    i = base + c
                    if claimed[i]:
                        continue
                    claimed[i] = 1
                    sec_px += 1
                    if not zmask[i]:
                        continue
                    zeu_px += 1
                    if clc[i] in GREEN_CLASSES:
                        evu += 1
                    t = tcd[i]
                    if t <= 100:
                        tsum += t
        # section pixels beyond the ZEU grid are outside the ZEU by construction
        full = sec_px + max(0, round(area_m2(polys) / 100) - sec_px)
        zkm = zeu_px / 1e4
        rows.append({
            "cusec": cusec, "district": dist, "section": sec,
            "zeu_km2": round(zkm, 4),
            "share_in_zeu": round(zeu_px / full, 3) if full else 0,
            "evu_km2": round(evu / 1e4, 4), "evu_pct": round(100 * evu / zeu_px, 2) if zeu_px else None,
            "cau_km2": round(tsum / 1e6, 4), "cau_pct": round(100 * tsum / 1e6 / zkm, 2) if zeu_px else None,
        })
    tot = {k: round(sum(r[k] for r in rows), 3) for k in ("zeu_km2", "evu_km2", "cau_km2")}
    print(f"{rec.get('LAU_NAME')}: {len(rows)} sections; inside ZEU {tot['zeu_km2']} km2 "
          f"(official {rec.get('Area_UA_km2')}), EVU {tot['evu_km2']}, CAU {tot['cau_km2']}")
    if a.csv:
        with open(a.csv, "w", newline="") as f:
            wr = csv.DictWriter(f, fieldnames=list(rows[0]))
            wr.writeheader()
            wr.writerows(rows)


def area_m2(polys):
    """Shoelace area of rings in EPSG:3035 (equal-area, so this is m2)."""
    tot = 0.0
    for rings in polys:
        for ring in rings:
            s = 0.0
            for (ax, ay), (bx, by) in zip(ring, ring[1:] + ring[:1]):
                s += ax * by - bx * ay
            tot += s / 2
    return abs(tot)


if __name__ == "__main__":
    main()
