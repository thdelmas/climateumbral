#!/usr/bin/env python3
"""Calibrate the heat model's coefficients against REAL measured LST.

The model (web/src/lib/heat.js) says: penalty = COEF * S, where S is the
neighborhood mean sealed fraction. If that linear form is right, then in
measured land-surface temperature the slope of LST against S *is* the
coefficient. This script fits that slope:

  day    LST from fetch_lst.py day (Landsat 30 m, ~10:30 solar) paired
         with S_day = box mean sealed fraction, radius 5 px / 50 m, the
         exact window heat.js uses — over the same 454x454 10 m grid.

  night  LST from fetch_lst.py night (MODIS 1 km, ~01:30 solar) paired
         with mean sealed fraction of each 1 km cell. At 1 km the model's
         150 m night window is subsumed by the cell itself; the slope is
         the city-scale night surface UHI per unit sealed fraction.

Reports slope (= measured COEF), R², decile bin means (to eyeball
linearity), and the measured green-vs-sealed spread. Stdlib only.

Known confound, reported not corrected: elevation. Green pixels around
Barcelona are also higher (Collserola), and altitude cools; slopes are
therefore upper bounds on the pure sealing effect.

Usage:
    python3 calibrate_heat.py --grid data/bcn --day PATH/bcn --night PATH/bcn
"""
import argparse
import json
import math
import struct
import sys

SEA, NODATA = 254, 255
DAY_RADIUS_PX = 5
GREEN_MAX, HARD_SEALED = 10, 90


def load_f32(path):
    raw = open(path, "rb").read()
    return struct.unpack("<%df" % (len(raw) // 4), raw)


def box_mean_sealed(grid, w, h, r):
    """Mirror of heat.js sealedStats: per land pixel, mean sealed fraction
    over land pixels within the (2r+1)^2 box, via integral images."""
    iw = w + 1
    sums = [0.0] * (iw * (h + 1))
    sumn = [0.0] * (iw * (h + 1))
    for y in range(h):
        for x in range(w):
            i = y * w + x
            land = grid[i] <= 100
            j = (y + 1) * iw + (x + 1)
            sums[j] = (grid[i] / 100 if land else 0) + sums[j - 1] + sums[j - iw] - sums[j - iw - 1]
            sumn[j] = (1 if land else 0) + sumn[j - 1] + sumn[j - iw] - sumn[j - iw - 1]

    def rect(a, x0, y0, x1, y1):
        return a[(y1 + 1) * iw + x1 + 1] - a[y0 * iw + x1 + 1] - a[(y1 + 1) * iw + x0] + a[y0 * iw + x0]

    out = [-1.0] * (w * h)
    for y in range(h):
        for x in range(w):
            if grid[y * w + x] > 100:
                continue
            x0, y0 = max(0, x - r), max(0, y - r)
            x1, y1 = min(w - 1, x + r), min(h - 1, y + r)
            n = rect(sumn, x0, y0, x1, y1)
            out[y * w + x] = rect(sums, x0, y0, x1, y1) / n if n else 0.0
    return out


def fit(pairs):
    n = len(pairs)
    mx = sum(p[0] for p in pairs) / n
    my = sum(p[1] for p in pairs) / n
    cov = sum((p[0] - mx) * (p[1] - my) for p in pairs)
    vx = sum((p[0] - mx) ** 2 for p in pairs)
    vy = sum((p[1] - my) ** 2 for p in pairs)
    b = cov / vx
    return {"slope": b, "intercept": my - b * mx, "r2": cov * cov / (vx * vy) if vy else 0, "n": n}


def deciles(pairs):
    rows = []
    for k in range(10):
        lo, hi = k / 10, (k + 1) / 10
        sel = [p[1] for p in pairs if lo <= p[0] < hi or (k == 9 and p[0] == 1.0)]
        if sel:
            rows.append((f"S {lo:.1f}-{hi:.1f}", len(sel), sum(sel) / len(sel)))
    return rows


def report(title, pairs, current_coef):
    r = fit(pairs)
    print(f"\n== {title} ==")
    print(f"  n={r['n']}  slope={r['slope']:+.2f} degC per unit sealed  "
          f"intercept={r['intercept']:.1f} degC  R2={r['r2']:.2f}")
    for label, n, mean in deciles(pairs):
        bar = "#" * int((mean - min(m for _, _, m in deciles(pairs))) * 4)
        print(f"  {label}  n={n:6d}  mean {mean:5.1f} degC  {bar}")
    print(f"  model coefficient today: {current_coef}  ->  measured: {r['slope']:.1f}")
    return r


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--grid", required=True, help="prefix of fetch_grid.py output (data/bcn)")
    ap.add_argument("--day", help="prefix of fetch_lst.py day output")
    ap.add_argument("--night", help="prefix of fetch_lst.py night output")
    args = ap.parse_args()

    results = {}
    if args.day:
        meta = json.load(open(args.grid + ".json"))
        w, h = meta["width"], meta["height"]
        grid = open(args.grid + ".raw", "rb").read()
        lst = load_f32(args.day + "_day.f32")
        if len(lst) != w * h:
            sys.exit("day raster size does not match grid")
        s = box_mean_sealed(grid, w, h, DAY_RADIUS_PX)
        pairs = [(s[i], lst[i]) for i in range(w * h)
                 if s[i] >= 0 and not math.isnan(lst[i])]
        results["day"] = report("DAY  (Landsat 30 m, ~10:30 solar, 50 m window)", pairs, 6)
        green = [lst[i] for i in range(w * h)
                 if grid[i] <= GREEN_MAX and not math.isnan(lst[i])]
        sealed = [lst[i] for i in range(w * h)
                  if HARD_SEALED <= grid[i] <= 100 and not math.isnan(lst[i])]
        if green and sealed:
            print(f"  measured green-vs-sealed spread: "
                  f"{sum(sealed)/len(sealed) - sum(green)/len(green):+.1f} degC "
                  f"(green n={len(green)}, sealed n={len(sealed)})")

    if args.night:
        nmeta = json.load(open(args.night + "_night.json"))
        nw, nh = nmeta["size"]
        lst = load_f32(args.night + "_night.f32")
        sealed = load_f32(args.night + "_night_sealed.f32")
        pairs = [(sealed[i], lst[i]) for i in range(nw * nh)
                 if not math.isnan(sealed[i]) and not math.isnan(lst[i])]
        results["night"] = report("NIGHT (MODIS 1 km, ~01:30 solar, cell mean)", pairs, 4)

    print("\nSuggested heat.js coefficients (rounded to 0.5):")
    for k, cur in [("day", "DAY_COEF"), ("night", "NIGHT_COEF")]:
        if k in results:
            print(f"  {cur} = {round(results[k]['slope'] * 2) / 2}")


if __name__ == "__main__":
    main()
