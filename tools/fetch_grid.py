#!/usr/bin/env python3
"""Fetch a sealed-percent grid (+ sea mask) for any EU bounding box.

Zero dependencies: stdlib only, no GDAL/PIL. Output values:
0-100 = % sealed (Copernicus IMD 2018, 10 m), 254 = sea, 255 = nodata.

Usage:
    python3 fetch_grid.py LONMIN LATMIN LONMAX LATMAX [-o NAME] [--size WxH]

Writes NAME.raw (U8, row 0 = north) and NAME.json (metadata + stats).
"""
import argparse
import json
import math
import struct
import sys
import urllib.error
import urllib.parse
import urllib.request

BASE = "https://image.discomap.eea.europa.eu/arcgis/rest/services/GioLandPublic"
IMD = f"{BASE}/HRL_ImperviousnessDensity_2018/ImageServer/exportImage"
WAW = f"{BASE}/HRL_WaterWetness_2018/ImageServer/exportImage"
SEA_CLASS = 253
SEA, NODATA = 254, 255


def export(url, bbox, size):
    params = {
        "bbox": ",".join(map(str, bbox)),
        "bboxSR": "4326",
        "imageSR": "3035",  # equal-area, square pixels
        "size": f"{size[0]},{size[1]}",
        "format": "tiff",
        "pixelType": "U8",
        "f": "json",
    }
    query = url + "?" + urllib.parse.urlencode(params)
    try:
        with urllib.request.urlopen(query, timeout=60) as r:
            meta = json.load(r)
        if "href" not in meta:
            sys.exit(f"exportImage error: {meta}")
        with urllib.request.urlopen(meta["href"], timeout=120) as r:
            return meta, r.read()
    except (urllib.error.URLError, TimeoutError) as e:
        sys.exit(f"fetch failed ({url.split('/')[-2]}): {e}")


def parse_tiff(data, width, height):
    """Minimal reader for the uncompressed U8 TIFFs this server emits.

    Handles the server's quirks: tiled or stripped layout, tag values stored
    inline when they fit in 4 bytes (count == 1), and the zero-tile "empty"
    TIFF returned for a bbox with no data in the layer (-> all zeros).
    """
    bo = "<" if data[:2] == b"II" else ">"
    off = struct.unpack(bo + "I", data[4:8])[0]
    n = struct.unpack(bo + "H", data[off : off + 2])[0]
    tags = {}
    for i in range(n):
        p = off + 2 + i * 12
        tag, typ, cnt = struct.unpack(bo + "HHI", data[p : p + 8])
        raw = data[p + 8 : p + 12]
        tags[tag] = (typ, cnt, struct.unpack(bo + "I", raw)[0])

    def values(tag):
        """All values of a tag; deref the pointer unless inline."""
        typ, cnt, val = tags[tag]
        size, fmt = (2, "H") if typ == 3 else (4, "I")
        if cnt * size <= 4:
            packed = struct.pack(bo + "I", val)[: cnt * size]
            return [val] if cnt == 1 else list(
                struct.unpack(bo + fmt * cnt, packed)
            )
        return [
            struct.unpack(
                bo + fmt, data[val + i * size : val + (i + 1) * size]
            )[0]
            for i in range(cnt)
        ]

    if tags[259][2] != 1:
        sys.exit(f"unexpected TIFF compression {tags[259][2]} "
                 "(expected 1 = none)")

    img = bytearray(width * height)  # 0-filled: empty exports stay all-zero
    if 322 in tags:  # tiled
        tw, tl = tags[322][2], tags[323][2]
        per_row = math.ceil(width / tw)
        offs, counts = values(324), values(325)
        for t, (toff, tcnt) in enumerate(zip(offs, counts)):
            if toff == 0 or tcnt == 0:
                continue
            ty, tx = divmod(t, per_row)
            tile = data[toff : toff + tw * tl]
            for r in range(tl):
                y = ty * tl + r
                if y >= height:
                    break
                w = min(tw, width - tx * tw)
                dst = y * width + tx * tw
                img[dst : dst + w] = tile[r * tw : r * tw + w]
    else:  # stripped
        rps = tags[278][2] if 278 in tags else height
        offs, counts = values(273), values(279)
        for s, (soff, scnt) in enumerate(zip(offs, counts)):
            if soff == 0 or scnt == 0:
                continue
            start = s * rps * width
            img[start : start + scnt] = data[soff : soff + scnt]
    return bytes(img)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("bbox", nargs=4, type=float,
                    metavar=("LONMIN", "LATMIN", "LONMAX", "LATMAX"))
    ap.add_argument("-o", "--out", default="grid")
    ap.add_argument("--size", default=None,
                    help="WxH pixels (default: native 10 m)")
    args = ap.parse_args()

    if args.size:
        w, h = (int(v) for v in args.size.lower().split("x"))
    else:
        # native 10 m: bbox extent in meters (equal-area approx)
        lonmin, latmin, lonmax, latmax = args.bbox
        mlat = math.radians((latmin + latmax) / 2)
        w = round((lonmax - lonmin) * 111320 * math.cos(mlat) / 10)
        h = round((latmax - latmin) * 110540 / 10)
    if w * h > 4_194_304:  # 2048x2048-equivalent pixel budget
        sys.exit(f"{w}x{h} exceeds the 2048x2048-equivalent budget "
                 "this script asks of the server; pass --size")

    meta, tif = export(IMD, args.bbox, (w, h))
    grid = bytearray(parse_tiff(tif, w, h))
    _, wtif = export(WAW, args.bbox, (w, h))
    water = parse_tiff(wtif, w, h)
    for i in range(w * h):
        if water[i] == SEA_CLASS:
            grid[i] = SEA

    total = w * h
    sealed90 = sum(1 for v in grid if 90 <= v <= 100)
    green10 = sum(1 for v in grid if v <= 10)
    sea = sum(1 for v in grid if v == SEA)
    with open(args.out + ".raw", "wb") as f:
        f.write(bytes(grid))
    with open(args.out + ".json", "w") as f:
        json.dump(
            {
                "bbox_4326": args.bbox,
                "extent_3035": meta.get("extent"),
                "width": w,
                "height": h,
                "row0": "north",
                "values": "0-100 = % sealed (IMD 2018), "
                          "254 = sea (WAW 2018), 255 = nodata",
                "stats": {
                    "pct_hard_sealed_90plus":
                        round(100 * sealed90 / total, 1),
                    "pct_green_10minus":
                        round(100 * green10 / total, 1),
                    "pct_sea": round(100 * sea / total, 1),
                },
            },
            f,
        )
    print(f"{args.out}.raw + {args.out}.json  ({w}x{h}; "
          f"hard-sealed {100 * sealed90 / total:.0f}%, "
          f"green {100 * green10 / total:.0f}%, "
          f"sea {100 * sea / total:.0f}%)")


if __name__ == "__main__":
    main()
