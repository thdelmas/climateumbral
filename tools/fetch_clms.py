#!/usr/bin/env python3
"""Download the official-year Copernicus Land tiles for one urban area.

The open EEA image service stops at older years. The years Reg. (EU)
2024/1991 art. 8 is measured on come from two free accounts:

  tcd  HRL Tree Cover Density, yearly 10 m (2024 = CAU reference year)
       Copernicus Data Space Ecosystem: env CDSE_USER, CDSE_PASSWORD
  clc  CLC+ Backbone 10 m (2023 = EVU reference year)
       WEkEO HDA API: env WEKEO_USER, WEKEO_PASSWORD; the account must have
       accepted the "Copernicus_Land_Monitoring_Service_Data_Policy" term

Tiles are the 100 km EPSG:3035 grid (E36N20 = x 3600-3700 km, y
2000-2100 km). Stdlib only.

Usage:
    python3 fetch_clms.py tiles ZEU.gpkg GISCO_ID
    python3 fetch_clms.py tcd 2024 E36N20 [E37N21 ...] [-o DIR]
    python3 fetch_clms.py clc 2023 E36N20 [...] [-o DIR]

Each tile lands in DIR/<product>/<tile>/*.tif (zip extracted).
Source: Copernicus Land Monitoring Service. Free, full and open access;
name the source, state adaptations, never imply EU endorsement.
"""
import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile

TCD_CSV = ("https://s3.waw3-1.cloudferro.com/swift/v1/CatalogueCSV/landcover_landuse/"
           "tree_cover_density/clms_vlcc_tree-cover-density_europe_10m_yearly_v1/"
           "clms_vlcc_tree-cover-density_europe_10m_yearly_v1_cog.csv")
CDSE_TOKEN = "https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token"
CDSE_DL = "https://download.dataspace.copernicus.eu/odata/v1/Products({})/$value"
WEKEO = "https://gateway.prod.wekeo2.eu/hda-broker"


def env(name):
    v = os.environ.get(name)
    if not v:
        sys.exit(f"set {name} (free account, see the docstring)")
    return v


def save(resp, path):
    with open(path, "wb") as f:
        while True:
            b = resp.read(1 << 20)
            if not b:
                break
            f.write(b)


def unpack(zpath, dest):
    zipfile.ZipFile(zpath).extractall(dest)
    os.remove(zpath)
    tifs = [os.path.join(r, n) for r, _, fs in os.walk(dest) for n in fs if n.endswith(".tif")]
    print(dest, *tifs, sep="\n  ")


# ---------- tiles for an urban area ----------

def tiles_for(gpkg, gisco_id):
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    from zeu_metrics import load_zeu
    _, polys = load_zeu(gpkg, gisco_id)
    xs = [x for p in polys for r in p for x, _ in r]
    ys = [y for p in polys for r in p for _, y in r]
    return sorted({f"E{int(x // 100000)}N{int(y // 100000)}"
                   for x in range(int(min(xs)) // 100000 * 100000, int(max(xs)) + 1, 100000)
                   for y in range(int(min(ys)) // 100000 * 100000, int(max(ys)) + 1, 100000)})


# ---------- Copernicus Data Space: Tree Cover Density ----------

def cdse_token():
    body = urllib.parse.urlencode({"grant_type": "password", "client_id": "cdse-public",
                                   "username": env("CDSE_USER"),
                                   "password": env("CDSE_PASSWORD")}).encode()
    with urllib.request.urlopen(urllib.request.Request(CDSE_TOKEN, data=body), timeout=60) as r:
        return json.load(r)["access_token"]


def fetch_tcd(year, tiles, out):
    with urllib.request.urlopen(TCD_CSV, timeout=120) as r:
        rows = [line.split(";") for line in r.read().decode().splitlines()[1:]]
    tok = cdse_token()

    class KeepAuth(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, hdrs, newurl):
            new = super().redirect_request(req, fp, code, msg, hdrs, newurl)
            if new:
                new.add_unredirected_header("Authorization", "Bearer " + tok)
            return new

    opener = urllib.request.build_opener(KeepAuth)
    for tile in tiles:
        hit = [r for r in rows if f"_S{year}_R10m_{tile}_" in r[1]]
        if not hit:
            print(f"tcd {year} {tile}: not in the catalogue")
            continue
        pid = hit[0][0]
        dest = os.path.join(out, "tcd", str(year), tile)
        os.makedirs(dest, exist_ok=True)
        req = urllib.request.Request(CDSE_DL.format(pid), headers={"Authorization": "Bearer " + tok})
        with opener.open(req, timeout=900) as r:
            save(r, dest + ".zip")
        unpack(dest + ".zip", dest)


# ---------- WEkEO: CLC+ Backbone ----------

def wekeo(path, body=None, tok=None):
    h = {"Content-Type": "application/json", "Accept": "application/json"}
    if tok:
        h["Authorization"] = "Bearer " + tok
    req = urllib.request.Request(WEKEO + path, data=json.dumps(body).encode() if body is not None else None,
                                 headers=h)
    try:
        return urllib.request.urlopen(req, timeout=900)
    except urllib.error.HTTPError as e:
        sys.exit(f"WEkEO {path}: {e.code} {e.read()[:300].decode('utf8', 'replace')}")


def fetch_clc(year, tiles, out):
    tok = json.load(wekeo("/gettoken", {"username": env("WEKEO_USER"), "password": env("WEKEO_PASSWORD")}))["access_token"]
    for tile in tiles:
        # the tile's own extent, as lon/lat, is enough to find it
        e, n = int(tile[1:3]) * 100000 + 50000, int(tile[4:6]) * 100000 + 50000
        lon, lat = laea_inverse(e, n)
        q = {"dataset_id": "EO:EEA:DAT:CLC-PLUS", "productType": "Raster Layer", "resolution": "10m",
             "year": str(year), "bbox": [lon - .01, lat - .01, lon + .01, lat + .01],
             "itemsPerPage": 20, "startIndex": 0}
        feats = [f for f in json.load(wekeo("/api/v1/dataaccess/search", q, tok))["features"] if f"_{tile}_" in f["id"]]
        if not feats:
            print(f"clc {year} {tile}: not found")
            continue
        f = feats[0]
        job = json.load(wekeo("/api/v1/dataaccess/download", {"dataset_id": q["dataset_id"], "product_id": f["id"],
                                                               "location": f["properties"]["location"]}, tok))
        dest = os.path.join(out, "clc", str(year), tile)
        os.makedirs(dest, exist_ok=True)
        for _ in range(90):  # the broker prepares the file, then serves it
            r = wekeo(f"/api/v1/dataaccess/download/{job['download_id']}", None, tok)
            if "json" in r.headers.get("Content-Type", ""):
                r.close()
                time.sleep(10)
                continue
            save(r, dest + ".zip")
            break
        else:
            sys.exit(f"clc {tile}: download never became ready")
        unpack(dest + ".zip", dest)


def laea_inverse(x, y):
    """EPSG:3035 -> lon/lat (spherical approximation, plenty to find a tile)."""
    import math
    r = 6371007.2
    lon0, lat0 = math.radians(10), math.radians(52)
    x, y = x - 4321000, y - 3210000
    rho = math.hypot(x, y)
    c = 2 * math.asin(rho / (2 * r))
    lat = math.asin(math.cos(c) * math.sin(lat0) + y * math.sin(c) * math.cos(lat0) / rho)
    lon = lon0 + math.atan2(x * math.sin(c), rho * math.cos(lat0) * math.cos(c) - y * math.sin(lat0) * math.sin(c))
    return math.degrees(lon), math.degrees(lat)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = ap.add_subparsers(dest="cmd", required=True)
    t = sub.add_parser("tiles")
    t.add_argument("gpkg")
    t.add_argument("gisco_id")
    for name in ("tcd", "clc"):
        p = sub.add_parser(name)
        p.add_argument("year", type=int)
        p.add_argument("tiles", nargs="+")
        p.add_argument("-o", "--out", default="clms")
    a = ap.parse_args()
    if a.cmd == "tiles":
        print(*tiles_for(a.gpkg, a.gisco_id))
    elif a.cmd == "tcd":
        fetch_tcd(a.year, a.tiles, a.out)
    else:
        fetch_clc(a.year, a.tiles, a.out)


if __name__ == "__main__":
    main()
