#!/usr/bin/env python3
"""EVU and CAU for one urban ecosystem area, computed the official way.

Reg. (EU) 2024/1991 art. 8 measures urban green space (EVU) and urban tree
canopy cover (CAU) inside each urban ecosystem area (ZEU). The Spanish
method (MITECO municipal guide, 2026) is a zonal sum on 10 m Copernicus
rasters in EPSG:3035:

  EVU = number of CLC+ Backbone pixels in classes 2, 3, 4 (trees), 5
        (shrubs), 6 (permanent herbaceous), 8 (lichens, mosses), 10 (water)
        inside the ZEU, x 100 m2
  CAU = sum of Tree Cover Density values (0-100 % per pixel) inside the
        ZEU, x 1 m2 (each percent point of a 100 m2 pixel is 1 m2)

Zero dependencies: stdlib only. The ZEU polygon comes from a GeoPackage
(SQLite + GeoPackage WKB); the rasters from the EEA image service.

Usage:
    python3 zeu_metrics.py ZEU.gpkg GISCO_ID [--clc YEAR] [--tcd YEAR]
                           [--ref EVU_KM2 CAU_KM2] [--json OUT]

Example (Barcelona):
    python3 zeu_metrics.py Limites_ZEU_2024_100K_ES.gpkg ES_08019 \\
        --clc 2021 --tcd 2018 --ref 21.536 7.035037

Source: Copernicus Land Monitoring Service (CLC+ Backbone, HRL Tree Cover
Density), adapted: clipped to the ZEU and summed.
"""
import argparse
import json
import math
import sqlite3
import struct
import sys
import urllib.error
import urllib.parse
import urllib.request

from fetch_grid import parse_tiff

EEA = "https://image.discomap.eea.europa.eu/arcgis/rest/services"
# year -> exportImage endpoint on the open EEA image service
CLC_SOURCES = {
    "2018": f"{EEA}/CLC_plus/CLMS_CLCplus_RASTER_2018_010m_eu/ImageServer/exportImage",
    "2021": f"{EEA}/CLC_plus/CLMS_CLCplus_RASTER_2021_010m_eu/ImageServer/exportImage",
}
TCD_SOURCES = {
    "2018": f"{EEA}/GioLandPublic/HRL_TreeCoverDensity_2018/ImageServer/exportImage",
}
GREEN_CLASSES = {2, 3, 4, 5, 6, 8, 10}
PIXEL = 10  # metres
TILE = 2000  # pixels per side per request


# ---------- GeoPackage geometry ----------

def gpkg_wkb(blob):
    """Strip the GeoPackage header, return the WKB body."""
    if blob[:2] != b"GP":
        raise ValueError("not a GeoPackage geometry")
    flags = blob[3]
    env = (flags >> 1) & 0b111
    env_len = {0: 0, 1: 32, 2: 48, 3: 48, 4: 64}[env]
    return blob[8 + env_len:]


def parse_wkb(b):
    """Polygon / MultiPolygon WKB -> list of polygons, each a list of rings
    (lists of (x, y)). Z/M coordinates are read and dropped."""
    polys = []

    def read(off):
        bo = "<" if b[off] == 1 else ">"
        gtype = struct.unpack(bo + "I", b[off + 1:off + 5])[0]
        off += 5
        # ISO WKB: +1000 = Z, +2000 = M, +3000 = ZM; EWKB sets high flag bits
        if gtype & 0xE0000000:
            dims = 2 + bool(gtype & 0x80000000) + bool(gtype & 0x40000000)
            base = gtype & 0xFFFF
        else:
            dims = {0: 2, 1: 3, 2: 3, 3: 4}[gtype // 1000]
            base = gtype % 1000
        if base == 3:
            nr = struct.unpack(bo + "I", b[off:off + 4])[0]
            off += 4
            rings = []
            for _ in range(nr):
                n = struct.unpack(bo + "I", b[off:off + 4])[0]
                off += 4
                pts = []
                for _ in range(n):
                    vals = struct.unpack(bo + "d" * dims, b[off:off + 8 * dims])
                    pts.append((vals[0], vals[1]))
                    off += 8 * dims
                rings.append(pts)
            polys.append(rings)
            return off
        if base == 6:
            n = struct.unpack(bo + "I", b[off:off + 4])[0]
            off += 4
            for _ in range(n):
                off = read(off)
            return off
        raise ValueError(f"unsupported WKB geometry type {gtype}")

    read(0)
    return polys


def load_zeu(path, gisco_id):
    con = sqlite3.connect(path)
    (table, srs), = con.execute(
        "select table_name, srs_id from gpkg_contents where data_type='features'").fetchall()
    if srs != 3035:
        sys.exit(f"{table}: expected EPSG:3035, got {srs}")
    cols = [r[1] for r in con.execute(f"pragma table_info('{table}')")]
    geom = next(c for c in cols if c.lower() in ("geom", "geometry"))
    row = con.execute(
        f"select * from '{table}' where GISCO_ID = ?", (gisco_id,)).fetchone()
    if not row:
        sys.exit(f"{gisco_id} not in {table}")
    rec = dict(zip(cols, row))
    polys = parse_wkb(gpkg_wkb(rec.pop(geom)))
    return rec, polys


# ---------- rasterize (pixel centre inside polygon, even-odd) ----------

def mask_for(polys, x0, y1, w, h):
    """Row 0 = north. x0 = west edge, y1 = north edge, 10 m pixels."""
    edges = []
    for rings in polys:
        for ring in rings:
            for (ax, ay), (bx, by) in zip(ring, ring[1:] + ring[:1]):
                if ay != by:
                    edges.append((ax, ay, bx, by))
    mask = bytearray(w * h)
    for r in range(h):
        yc = y1 - (r + 0.5) * PIXEL
        xs = []
        for ax, ay, bx, by in edges:
            if (ay <= yc < by) or (by <= yc < ay):
                xs.append(ax + (yc - ay) * (bx - ax) / (by - ay))
        xs.sort()
        for i in range(0, len(xs) - 1, 2):
            c0 = max(0, math.ceil((xs[i] - x0) / PIXEL - 0.5))
            c1 = min(w - 1, math.floor((xs[i + 1] - x0) / PIXEL - 0.5))
            if c1 >= c0:
                mask[r * w + c0:r * w + c1 + 1] = b"\x01" * (c1 - c0 + 1)
    return mask


# ---------- rasters ----------

def export(url, x0, y0, x1, y1, w, h):
    params = {
        "bbox": f"{x0},{y0},{x1},{y1}", "bboxSR": "3035", "imageSR": "3035",
        "size": f"{w},{h}", "format": "tiff", "pixelType": "U8",
        "interpolation": "RSP_NearestNeighbor", "f": "json",
    }
    q = url + "?" + urllib.parse.urlencode(params)
    try:
        with urllib.request.urlopen(q, timeout=90) as r:
            meta = json.load(r)
        if "href" not in meta:
            sys.exit(f"exportImage error: {meta}")
        with urllib.request.urlopen(meta["href"], timeout=180) as r:
            return parse_tiff(r.read(), w, h)
    except (urllib.error.URLError, TimeoutError) as e:
        sys.exit(f"fetch failed: {e}")


def grid(url, x0, y1, w, h):
    """Whole w x h grid, fetched in TILE x TILE requests."""
    out = bytearray(w * h)
    for ty in range(0, h, TILE):
        for tx in range(0, w, TILE):
            tw, th = min(TILE, w - tx), min(TILE, h - ty)
            gx0 = x0 + tx * PIXEL
            gy1 = y1 - ty * PIXEL
            part = export(url, gx0, gy1 - th * PIXEL, gx0 + tw * PIXEL, gy1, tw, th)
            for r in range(th):
                d = (ty + r) * w + tx
                out[d:d + tw] = part[r * tw:(r + 1) * tw]
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("zeu")
    ap.add_argument("gisco_id")
    ap.add_argument("--clc", default="2021", choices=sorted(CLC_SOURCES))
    ap.add_argument("--tcd", default="2018", choices=sorted(TCD_SOURCES))
    ap.add_argument("--ref", nargs=2, type=float, metavar=("EVU_KM2", "CAU_KM2"),
                    help="official values to compare against")
    ap.add_argument("--json", help="write the result here")
    a = ap.parse_args()

    rec, polys = load_zeu(a.zeu, a.gisco_id)
    xs = [x for p in polys for ring in p for x, _ in ring]
    ys = [y for p in polys for ring in p for _, y in ring]
    x0 = math.floor(min(xs) / PIXEL) * PIXEL
    y0 = math.floor(min(ys) / PIXEL) * PIXEL
    x1 = math.ceil(max(xs) / PIXEL) * PIXEL
    y1 = math.ceil(max(ys) / PIXEL) * PIXEL
    w, h = (x1 - x0) // PIXEL, (y1 - y0) // PIXEL

    mask = mask_for(polys, x0, y1, w, h)
    clc = grid(CLC_SOURCES[a.clc], x0, y1, w, h)
    tcd = grid(TCD_SOURCES[a.tcd], x0, y1, w, h)

    inside = evu_px = tcd_sum = tcd_nodata = 0
    for i in range(w * h):
        if not mask[i]:
            continue
        inside += 1
        if clc[i] in GREEN_CLASSES:
            evu_px += 1
        t = tcd[i]
        if t <= 100:
            tcd_sum += t
        else:
            tcd_nodata += 1

    area_km2 = inside * PIXEL * PIXEL / 1e6
    evu_km2 = evu_px * PIXEL * PIXEL / 1e6
    cau_km2 = tcd_sum / 1e6  # each % point of a 100 m2 pixel = 1 m2
    res = {
        "gisco_id": a.gisco_id, "name": rec.get("LAU_NAME"),
        "zeu_area_km2_official": rec.get("Area_UA_km2"),
        "zeu_area_km2_pixels": round(area_km2, 3),
        "clc_year": a.clc, "tcd_year": a.tcd,
        "evu_km2": round(evu_km2, 4), "evu_pct": round(100 * evu_km2 / area_km2, 3),
        "cau_km2": round(cau_km2, 4), "cau_pct": round(100 * cau_km2 / area_km2, 3),
        "tcd_nodata_px": tcd_nodata, "grid": [w, h],
    }
    if a.ref:
        res["ref_evu_km2"], res["ref_cau_km2"] = a.ref
        res["evu_diff_pct"] = round(100 * (evu_km2 - a.ref[0]) / a.ref[0], 2)
        res["cau_diff_pct"] = round(100 * (cau_km2 - a.ref[1]) / a.ref[1], 2)
    print(json.dumps(res, indent=2, ensure_ascii=False))
    if a.json:
        with open(a.json, "w") as f:
            json.dump(res, f, indent=2, ensure_ascii=False)


if __name__ == "__main__":
    main()
