#!/usr/bin/env python3
"""Fetch REAL land-surface-temperature rasters for a bbox. No key, no login.

Two modes, two physical windows:

  day    Landsat 8/9 Collection-2 Level-2 Surface Temperature (ST_B10,
         ~10:30 solar overpass, 30 m) via Microsoft Planetary Computer.
         Anonymous STAC search + anonymous SAS token + HTTP range reads
         of the COG tiles that cover the bbox. Median composite over the
         N clearest summer scenes, sampled onto the same EPSG:3035
         10 m grid convention as fetch_grid.py, so it aligns pixel-for-
         pixel with NAME.raw from that tool.

  night  MODIS Aqua L3 LST 8-Day Night (~01:30 solar overpass, 1 km)
         via NASA GIBS WMS. GIBS serves colormapped PNGs; we invert the
         published colormap XML back to Kelvin (WMS resamples nearest,
         so every pixel is an exact palette color). Median over all
         fully-in-summer 8-day periods. Grid is EPSG:4326 (the WMS
         grid). Also fetches a matching sealed-fraction raster (IMD
         sampled at 10x resolution, block-averaged; WAW sea-masked) so
         calibrate_heat.py can pair the two without reprojecting.

Zero dependencies: stdlib only. Outputs float32 LE .f32 rasters
(degC, NaN = nodata, row 0 = north) + .json metadata.

Usage:
    python3 fetch_lst.py day  LONMIN LATMIN LONMAX LATMAX -o NAME \
        [--size WxH] [--years 2023,2024] [--max-cloud 10] [--scenes 8]
    python3 fetch_lst.py night LONMIN LATMIN LONMAX LATMAX -o NAME \
        [--years 2023,2024]

Writes NAME_day.f32/.json or NAME_night.f32/.json + NAME_night_sealed.f32.
"""
import argparse
import datetime
import json
import math
import re
import struct
import sys
import urllib.parse
import urllib.request
import zlib

STAC = "https://planetarycomputer.microsoft.com/api/stac/v1"
SAS = "https://planetarycomputer.microsoft.com/api/sas/v1/token/landsat-c2-l2"
GIBS_WMS = "https://gibs.earthdata.nasa.gov/wms/epsg4326/best/wms.cgi"
GIBS_CMAP = "https://gibs.earthdata.nasa.gov/colormaps/v1.3/MODIS_Land_Surface_Temp.xml"
NIGHT_LAYER = "MODIS_Aqua_L3_Land_Surface_Temp_8Day_Night"
EEA = "https://image.discomap.eea.europa.eu/arcgis/rest/services/GioLandPublic"
# Landsat C2 L2 ST scaling: Kelvin = DN * 0.00341802 + 149.0
ST_SCALE, ST_OFF = 0.00341802, 149.0
# qa_pixel bits that invalidate a thermal sample: fill, dilated cloud,
# cirrus, cloud, cloud shadow.
QA_BAD = 0b11111


def http(url, timeout=60, data=None, headers=None):
    req = urllib.request.Request(url, data=data, headers=headers or {})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return r.read()


def http_json(url, body=None):
    hdr = {"Content-Type": "application/json"} if body else {}
    data = json.dumps(body).encode() if body else None
    return json.loads(http(url, data=data, headers=hdr))


def http_range(url, start, end):
    return http(url, headers={"Range": f"bytes={start}-{end}"})


# ---------------------------------------------------------------- EPSG:3035
# ETRS89 / LAEA Europe — same math as web/src/lib/proj.js and
# server/proj.go; the three must agree.
A = 6378137.0
F = 1 / 298.257222101
E2 = F * (2 - F)
E = math.sqrt(E2)
LAT0 = math.radians(52)
LON0 = math.radians(10)
FE, FN = 4321000.0, 3210000.0


def _q(phi):
    s = math.sin(phi)
    return (1 - E2) * (
        s / (1 - E2 * s * s) - (1 / (2 * E)) * math.log((1 - E * s) / (1 + E * s))
    )


QP = _q(math.pi / 2)
B1 = math.asin(_q(LAT0) / QP)
RQ = A * math.sqrt(QP / 2)
D = (A * math.cos(LAT0)) / math.sqrt(1 - E2 * math.sin(LAT0) ** 2) / (RQ * math.cos(B1))


def to_laea(lon, lat):
    lam = math.radians(lon) - LON0
    phi = math.radians(lat)
    beta = math.asin(_q(phi) / QP)
    b = RQ * math.sqrt(
        2 / (1 + math.sin(B1) * math.sin(beta) + math.cos(B1) * math.cos(beta) * math.cos(lam))
    )
    return (
        FE + b * D * math.cos(beta) * math.sin(lam),
        FN + (b / D) * (math.cos(B1) * math.sin(beta) - math.sin(B1) * math.cos(beta) * math.cos(lam)),
    )


def from_laea(e, n):
    xp = (e - FE) / D
    yp = D * (n - FN)
    rho = math.hypot(xp, yp)
    if rho == 0:
        return math.degrees(LON0), math.degrees(LAT0)
    ce = 2 * math.asin(rho / (2 * RQ))
    qv = QP * (math.cos(ce) * math.sin(B1) + (yp * math.sin(ce) * math.cos(B1)) / rho)
    lam = LON0 + math.atan2(
        (e - FE) * math.sin(ce),
        D * rho * math.cos(B1) * math.cos(ce) - D * D * (n - FN) * math.sin(B1) * math.sin(ce),
    )
    phi = math.asin(qv / 2)
    for _ in range(8):
        s = math.sin(phi)
        phi += ((1 - E2 * s * s) ** 2 / (2 * math.cos(phi))) * (
            qv / (1 - E2) - s / (1 - E2 * s * s) + (1 / (2 * E)) * math.log((1 - E * s) / (1 + E * s))
        )
    return math.degrees(lam), math.degrees(phi)


def laea_envelope(bbox):
    """Envelope of the projected bbox, edges densified (they curve)."""
    lonmin, latmin, lonmax, latmax = bbox
    pts = []
    for t in range(101):
        f = t / 100
        lon = lonmin + (lonmax - lonmin) * f
        lat = latmin + (latmax - latmin) * f
        pts += [(lon, latmin), (lon, latmax), (lonmin, lat), (lonmax, lat)]
    es, ns = zip(*(to_laea(lon, lat) for lon, lat in pts))
    return min(es), min(ns), max(es), max(ns)


# ------------------------------------------------------- UTM (WGS84) forward
def utm_fwd(lon, lat, zone):
    a, f, k0 = 6378137.0, 1 / 298.257223563, 0.9996
    e2 = f * (2 - f)
    ep2 = e2 / (1 - e2)
    phi = math.radians(lat)
    lam = math.radians(lon) - math.radians(zone * 6 - 183)
    n = a / math.sqrt(1 - e2 * math.sin(phi) ** 2)
    t = math.tan(phi) ** 2
    c = ep2 * math.cos(phi) ** 2
    aa = lam * math.cos(phi)
    m = a * (
        (1 - e2 / 4 - 3 * e2**2 / 64 - 5 * e2**3 / 256) * phi
        - (3 * e2 / 8 + 3 * e2**2 / 32 + 45 * e2**3 / 1024) * math.sin(2 * phi)
        + (15 * e2**2 / 256 + 45 * e2**3 / 1024) * math.sin(4 * phi)
        - (35 * e2**3 / 3072) * math.sin(6 * phi)
    )
    east = 500000 + k0 * n * (
        aa + (1 - t + c) * aa**3 / 6 + (5 - 18 * t + t**2 + 72 * c - 58 * ep2) * aa**5 / 120
    )
    north = k0 * (
        m
        + n
        * math.tan(phi)
        * (
            aa**2 / 2
            + (5 - t + 9 * c + 4 * c**2) * aa**4 / 24
            + (61 - 58 * t + t**2 + 600 * c - 330 * ep2) * aa**6 / 720
        )
    )
    return east, north


# ----------------------------------------------------------------- COG reader
class Cog:
    """Minimal reader for Landsat C2 COGs: uint16, DEFLATE, 256x256 tiles,
    no predictor. Reads the header once, then range-requests only the
    tiles it is asked for."""

    def __init__(self, url):
        self.url = url
        head = http_range(url, 0, 131071)
        self.head = head
        bo = "<" if head[:2] == b"II" else ">"
        self.bo = bo
        off = struct.unpack(bo + "I", head[4:8])[0]
        n = struct.unpack(bo + "H", head[off : off + 2])[0]
        tags = {}
        for k in range(n):
            e = off + 2 + k * 12
            tag, typ, cnt = struct.unpack(bo + "HHI", head[e : e + 8])
            tags[tag] = (typ, cnt, e + 8)
        self.tags = tags
        self.width = self._scalar(256)
        self.height = self._scalar(257)
        self.tile_w = self._scalar(322)
        self.tile_h = self._scalar(323)
        if self._scalar(259) != 8:
            sys.exit(f"unexpected compression {self._scalar(259)} (want DEFLATE)")
        if 317 in tags and self._scalar(317) not in (None, 1):
            sys.exit("unexpected TIFF predictor")
        self.offsets = self._array(324)
        self.counts = self._array(325)
        sx, sy, _ = struct.unpack(bo + "3d", self._raw(33550, 24))
        tp = struct.unpack(bo + "6d", self._raw(33922, 48))
        self.sx, self.sy = sx, sy
        self.x0, self.y0 = tp[3], tp[4]  # UL corner of pixel (0,0)
        geo = self._array(34735, fmt="H")
        self.epsg = 0
        for k in range(4, len(geo), 4):
            if geo[k] == 3072:
                self.epsg = geo[k + 3]
        self.tiles_across = (self.width + self.tile_w - 1) // self.tile_w
        self._cache = {}

    def _scalar(self, tag):
        if tag not in self.tags:
            return None
        typ, cnt, at = self.tags[tag]
        fmt = {3: "H", 4: "I"}[typ]
        return struct.unpack(self.bo + fmt, self.head[at : at + struct.calcsize(fmt)])[0]

    def _raw(self, tag, nbytes):
        typ, cnt, at = self.tags[tag]
        ptr = struct.unpack(self.bo + "I", self.head[at : at + 4])[0]
        if ptr + nbytes <= len(self.head):
            return self.head[ptr : ptr + nbytes]
        return http_range(self.url, ptr, ptr + nbytes - 1)

    def _array(self, tag, fmt=None):
        typ, cnt, at = self.tags[tag]
        fmt = fmt or {3: "H", 4: "I", 12: "d"}[typ]
        size = struct.calcsize(fmt) * cnt
        if size <= 4:
            return struct.unpack(self.bo + fmt * cnt, self.head[at : at + size])
        ptr = struct.unpack(self.bo + "I", self.head[at : at + 4])[0]
        buf = (
            self.head[ptr : ptr + size]
            if ptr + size <= len(self.head)
            else http_range(self.url, ptr, ptr + size - 1)
        )
        return struct.unpack(self.bo + fmt * cnt, buf)

    def pixel_of(self, e, n):
        return int((e - self.x0) / self.sx), int((self.y0 - n) / self.sy)

    def tile(self, tx, ty):
        key = ty * self.tiles_across + tx
        if key not in self._cache:
            off, cnt = self.offsets[key], self.counts[key]
            if cnt == 0:
                self._cache[key] = None  # sparse tile = all fill
            else:
                raw = zlib.decompress(http_range(self.url, off, off + cnt - 1))
                self._cache[key] = struct.unpack(
                    self.bo + "H" * (self.tile_w * self.tile_h), raw
                )
        return self._cache[key]

    def sample(self, col, row):
        if not (0 <= col < self.width and 0 <= row < self.height):
            return 0
        t = self.tile(col // self.tile_w, row // self.tile_h)
        if t is None:
            return 0
        return t[(row % self.tile_h) * self.tile_w + (col % self.tile_w)]


# --------------------------------------------------------------------- day
def run_day(bbox, w, h, years, max_cloud, max_scenes):
    feats = []
    for y in years:
        body = {
            "collections": ["landsat-c2-l2"],
            "bbox": list(bbox),
            "datetime": f"{y}-06-01T00:00:00Z/{y}-09-15T23:59:59Z",
            "query": {
                "eo:cloud_cover": {"lt": max_cloud},
                "platform": {"in": ["landsat-8", "landsat-9"]},
            },
            "limit": 100,
        }
        feats += http_json(STAC + "/search", body)["features"]
    feats.sort(key=lambda f: f["properties"]["eo:cloud_cover"])
    feats = feats[:max_scenes]
    if not feats:
        sys.exit("no Landsat scenes matched")
    token = http_json(SAS)["token"]

    e0, n0, e1, n1 = laea_envelope(bbox)
    dx, dy = (e1 - e0) / w, (n1 - n0) / h
    # Per grid pixel: EPSG:3035 center -> lon/lat -> UTM of the scene.
    lonlat = [None] * (w * h)
    for gy in range(h):
        for gx in range(w):
            lonlat[gy * w + gx] = from_laea(e0 + (gx + 0.5) * dx, n1 - (gy + 0.5) * dy)

    per_pixel = [[] for _ in range(w * h)]
    used = []
    for f in feats:
        st = Cog(f["assets"]["lwir11"]["href"] + "?" + token)
        qa = Cog(f["assets"]["qa_pixel"]["href"] + "?" + token)
        zone = st.epsg - 32600
        if not 1 <= zone <= 60:
            print(f"  skip {f['id']}: epsg {st.epsg} not UTM north", file=sys.stderr)
            continue
        utm = {}
        valid = 0
        for i, (lon, lat) in enumerate(lonlat):
            key = (round(lon, 6), round(lat, 6))
            if key not in utm:
                utm[key] = utm_fwd(lon, lat, zone)
            col, row = st.pixel_of(*utm[key])
            dn = st.sample(col, row)
            if dn == 0 or qa.sample(col, row) & QA_BAD:
                continue
            c = dn * ST_SCALE + ST_OFF - 273.15
            if -20 <= c <= 70:
                per_pixel[i].append(c)
                valid += 1
        cover = valid / (w * h)
        print(f"  {f['id']} {f['properties']['datetime'][:10]} "
              f"cloud={f['properties']['eo:cloud_cover']:.1f}% cover={cover:.0%}",
              file=sys.stderr)
        rec = {"id": f["id"], "date": f["properties"]["datetime"][:10],
               "cloud": f["properties"]["eo:cloud_cover"], "coverage": round(cover, 2)}
        if cover < 0.3:
            rec["note"] = "partial overlap"
        used.append(rec)

    out = bytearray()
    nan = struct.pack("<f", float("nan"))
    have = 0
    for vals in per_pixel:
        if len(vals) >= 3:
            vals.sort()
            m = len(vals)
            med = vals[m // 2] if m % 2 else (vals[m // 2 - 1] + vals[m // 2]) / 2
            out += struct.pack("<f", med)
            have += 1
        else:
            out += nan
    meta = {
        "source": "Landsat 8/9 C2 L2 ST_B10 via Microsoft Planetary Computer (anonymous SAS)",
        "overpass_solar_time": "~10:30",
        "composite": "per-pixel median, >=3 clear samples",
        "scenes": used,
        "grid": "EPSG:3035 envelope of bbox, row 0 = north (same as fetch_grid.py)",
        "coverage": round(have / (w * h), 3),
    }
    return bytes(out), meta


# -------------------------------------------------------------------- night
def png_decode(d):
    i, idat, meta, plte = 8, b"", None, None
    while i < len(d):
        ln, typ = struct.unpack(">I4s", d[i : i + 8])
        if typ == b"IHDR":
            meta = struct.unpack(">IIBB", d[i + 8 : i + 18])
        elif typ == b"PLTE":
            plte = d[i + 8 : i + 8 + ln]
        elif typ == b"IDAT":
            idat += d[i + 8 : i + 8 + ln]
        i += 12 + ln
    w, h, depth, ctype = meta
    if depth != 8 or ctype not in (2, 3, 6):
        sys.exit(f"unhandled PNG layout depth={depth} ctype={ctype}")
    bpp = {2: 3, 3: 1, 6: 4}[ctype]
    raw = zlib.decompress(idat)
    stride = w * bpp
    out = bytearray(h * stride)
    pos = 0
    for y in range(h):
        flt = raw[pos]
        pos += 1
        line = bytearray(raw[pos : pos + stride])
        pos += stride
        prev = out[(y - 1) * stride : y * stride] if y else bytearray(stride)
        for x in range(stride):
            a = line[x - bpp] if x >= bpp else 0
            b = prev[x]
            if flt == 1:
                line[x] = (line[x] + a) & 255
            elif flt == 2:
                line[x] = (line[x] + b) & 255
            elif flt == 3:
                line[x] = (line[x] + (a + b) // 2) & 255
            elif flt == 4:
                c = prev[x - bpp] if x >= bpp else 0
                p = a + b - c
                pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
                line[x] = (line[x] + (a if pa <= pb and pa <= pc else b if pb <= pc else c)) & 255
        out[y * stride : (y + 1) * stride] = line

    def rgba(i):
        if ctype == 3:
            j = out[i] * 3
            return plte[j], plte[j + 1], plte[j + 2], 255
        j = i * bpp
        return out[j], out[j + 1], out[j + 2], out[j + 3] if bpp == 4 else 255

    return w, h, rgba


def summer_periods(years):
    """MODIS 8-day period start dates (DOY 1+8k) fully inside June-August."""
    dates = []
    for y in years:
        for k in range(46):
            d = datetime.date(y, 1, 1) + datetime.timedelta(days=8 * k)
            if datetime.date(y, 6, 10) <= d <= datetime.date(y, 8, 24):
                dates.append(d.isoformat())
    return dates


def fetch_sealed_4326(bbox, w, h):
    """Mean sealed fraction + sea mask on the night raster's lon/lat grid.
    IMD is sampled at 10x resolution (nearest) and block-averaged; a cell
    is sea-masked when >20% of its samples are WAW sea."""
    hi_w, hi_h = w * 10, h * 10
    grids = {}
    for name, svc in [("imd", "HRL_ImperviousnessDensity_2018"), ("waw", "HRL_WaterWetness_2018")]:
        params = {
            "bbox": ",".join(map(str, bbox)),
            "bboxSR": "4326",
            "imageSR": "4326",
            "size": f"{hi_w},{hi_h}",
            "format": "tiff",
            "pixelType": "U8",
            "f": "json",
        }
        meta = http_json(f"{EEA}/{svc}/ImageServer/exportImage?" + urllib.parse.urlencode(params))
        if "href" not in meta:
            sys.exit(f"exportImage error: {meta}")
        tif = http(meta["href"], timeout=120)
        grids[name] = parse_tiff_u8(tif, hi_w, hi_h)
    imd, waw = grids["imd"], grids["waw"]
    out = bytearray()
    for cy in range(h):
        for cx in range(w):
            tot = sea = n = 0
            for sy in range(cy * 10, cy * 10 + 10):
                for sx in range(cx * 10, cx * 10 + 10):
                    v = imd[sy * hi_w + sx]
                    if waw[sy * hi_w + sx] == 253 or v > 100:
                        sea += 1
                        continue
                    tot += v
                    n += 1
            if n == 0 or sea > 20:
                out += struct.pack("<f", float("nan"))
            else:
                out += struct.pack("<f", tot / n / 100)
    return bytes(out)


def parse_tiff_u8(data, width, height):
    """Same minimal U8 TIFF reader contract as fetch_grid.py."""
    bo = "<" if data[:2] == b"II" else ">"
    off = struct.unpack(bo + "I", data[4:8])[0]
    n = struct.unpack(bo + "H", data[off : off + 2])[0]
    tags = {}
    for k in range(n):
        e = off + 2 + k * 12
        tag, typ, cnt = struct.unpack(bo + "HHI", data[e : e + 8])
        val = struct.unpack(bo + "I", data[e + 8 : e + 12])[0]
        tags[tag] = (typ, cnt, val, e + 8)
    out = bytearray(width * height)

    def arr(tag):
        typ, cnt, val, at = tags[tag]
        fmt = {3: "H", 4: "I"}[typ]
        size = struct.calcsize(fmt) * cnt
        if size <= 4:
            return struct.unpack(bo + fmt * cnt, data[at : at + size])
        return struct.unpack(bo + fmt * cnt, data[val : val + size])

    if 322 in tags:  # tiled
        tw, th = tags[322][2], tags[323][2]
        offs, cnts = arr(324), arr(325)
        across = (width + tw - 1) // tw
        for ti, (o, c) in enumerate(zip(offs, cnts)):
            if c == 0:
                continue
            tile = data[o : o + c]
            ty, tx = divmod(ti, across)
            for r in range(th):
                gy = ty * th + r
                if gy >= height:
                    break
                row = tile[r * tw : r * tw + tw]
                gx0 = tx * tw
                span = min(tw, width - gx0)
                out[gy * width + gx0 : gy * width + gx0 + span] = row[:span]
    else:  # stripped
        rps = tags.get(278, (0, 0, height, 0))[2]
        offs, cnts = arr(273), arr(279)
        for si, (o, c) in enumerate(zip(offs, cnts)):
            strip = data[o : o + c]
            y0 = si * rps
            for r in range(min(rps, height - y0)):
                out[(y0 + r) * width : (y0 + r + 1) * width] = strip[r * width : (r + 1) * width]
    return out


def run_night(bbox, years):
    lonmin, latmin, lonmax, latmax = bbox
    res = 0.0087890625  # GIBS native "1km" cell in degrees
    w = max(2, round((lonmax - lonmin) / res))
    h = max(2, round((latmax - latmin) / res))
    cmap = {}
    for m in re.finditer(
        r'rgb="(\d+),(\d+),(\d+)"[^/]*?value="\[([\d.eE+-]+),([\d.eE+-]+)[)\]]"',
        http(GIBS_CMAP).decode(),
    ):
        r, g, b, lo, hi = m.groups()
        cmap[(int(r), int(g), int(b))] = (float(lo) + float(hi)) / 2

    periods = summer_periods(years)
    per_pixel = [[] for _ in range(w * h)]
    unmatched = 0
    for date in periods:
        q = urllib.parse.urlencode(
            {
                "SERVICE": "WMS", "VERSION": "1.1.1", "REQUEST": "GetMap",
                "LAYERS": NIGHT_LAYER, "SRS": "EPSG:4326",
                "BBOX": f"{lonmin},{latmin},{lonmax},{latmax}",
                "WIDTH": w, "HEIGHT": h, "FORMAT": "image/png",
                "TIME": date, "STYLES": "",
            }
        )
        pw, ph, rgba = png_decode(http(GIBS_WMS + "?" + q, timeout=120))
        assert (pw, ph) == (w, h)
        got = 0
        for i in range(w * h):
            r, g, b, a = rgba(i)
            if a == 0:
                continue
            k = cmap.get((r, g, b))
            if k is None:
                unmatched += 1
                continue
            per_pixel[i].append(k - 273.15)
            got += 1
        print(f"  {date}: {got}/{w*h} px", file=sys.stderr)

    out = bytearray()
    have = 0
    for vals in per_pixel:
        if len(vals) >= max(3, len(periods) // 3):
            vals.sort()
            m = len(vals)
            med = vals[m // 2] if m % 2 else (vals[m // 2 - 1] + vals[m // 2]) / 2
            out += struct.pack("<f", med)
            have += 1
        else:
            out += struct.pack("<f", float("nan"))
    meta = {
        "source": f"{NIGHT_LAYER} via NASA GIBS WMS, colormap-inverted (~0.6 K bins)",
        "overpass_solar_time": "~01:30 (Aqua, drifting later since 2022)",
        "composite": "per-pixel median over 8-day summer periods",
        "periods": periods,
        "unmatched_colors": unmatched,
        "grid": "EPSG:4326, row 0 = north",
        "size": [w, h],
        "coverage": round(have / (w * h), 3),
    }
    return bytes(out), meta, w, h


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("mode", choices=["day", "night"])
    ap.add_argument("bbox", nargs=4, type=float, metavar=("LONMIN", "LATMIN", "LONMAX", "LATMAX"))
    ap.add_argument("-o", "--out", required=True)
    ap.add_argument("--size", default="454x454", help="day grid WxH (default 454x454)")
    ap.add_argument("--years", default="2023,2024")
    ap.add_argument("--max-cloud", type=float, default=10)
    ap.add_argument("--scenes", type=int, default=8)
    args = ap.parse_args()
    bbox = tuple(args.bbox)
    years = [int(y) for y in args.years.split(",")]

    if args.mode == "day":
        w, h = (int(v) for v in args.size.split("x"))
        raster, meta = run_day(bbox, w, h, years, args.max_cloud, args.scenes)
        meta.update({"bbox_4326": bbox, "width": w, "height": h, "units": "degC f32 LE, NaN=nodata"})
        open(f"{args.out}_day.f32", "wb").write(raster)
        json.dump(meta, open(f"{args.out}_day.json", "w"), indent=1)
        print(f"wrote {args.out}_day.f32 ({w}x{h}), coverage {meta['coverage']:.0%}")
    else:
        raster, meta, w, h = run_night(bbox, years)
        meta.update({"bbox_4326": bbox, "units": "degC f32 LE, NaN=nodata"})
        open(f"{args.out}_night.f32", "wb").write(raster)
        json.dump(meta, open(f"{args.out}_night.json", "w"), indent=1)
        print(f"fetching matching sealed raster ({w}x{h} at 4326)...", file=sys.stderr)
        sealed = fetch_sealed_4326(bbox, w, h)
        open(f"{args.out}_night_sealed.f32", "wb").write(sealed)
        print(f"wrote {args.out}_night.f32 + _night_sealed.f32 ({w}x{h}), "
              f"coverage {meta['coverage']:.0%}")


if __name__ == "__main__":
    main()
