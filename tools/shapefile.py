"""Minimal ESRI shapefile reader, stdlib only: polygons + DBF attributes.

Enough for administrative boundaries (shape types 5 Polygon, 15 PolygonZ,
25 PolygonM). Rings come back as lists of (x, y); a record's polygons are
returned as one list of rings (even-odd filling treats holes correctly).
"""
import struct


def read_dbf(path, encoding="latin-1"):
    with open(path, "rb") as f:
        head = f.read(32)
        n, hlen, rlen = struct.unpack("<IHH", head[4:12])
        fields = []
        while True:
            d = f.read(32)
            if d[0] == 0x0D:
                break
            name = d[:11].split(b"\x00")[0].decode("ascii")
            fields.append((name, d[11:12].decode(), d[16]))
        f.seek(hlen)
        rows = []
        for _ in range(n):
            rec = f.read(rlen)
            if not rec or rec[:1] == b"*":
                rows.append(None)
                continue
            pos, row = 1, {}
            for name, typ, size in fields:
                raw = rec[pos:pos + size].decode(encoding, "replace").strip()
                pos += size
                if typ in "NF" and raw:
                    try:
                        raw = float(raw) if "." in raw else int(raw)
                    except ValueError:
                        pass
                row[name] = raw
            rows.append(row)
    return rows


class Shapes:
    def __init__(self, base):
        self.shp = open(base + ".shp", "rb")
        with open(base + ".shx", "rb") as f:
            f.seek(100)
            idx = f.read()
        self.offsets = [struct.unpack(">I", idx[i:i + 4])[0] * 2 for i in range(0, len(idx), 8)]

    def __len__(self):
        return len(self.offsets)

    def rings(self, i):
        self.shp.seek(self.offsets[i] + 8)
        stype = struct.unpack("<i", self.shp.read(4))[0]
        if stype == 0:
            return []
        if stype not in (5, 15, 25):
            raise ValueError(f"shape type {stype} is not a polygon")
        self.shp.seek(32, 1)  # bbox
        nparts, npts = struct.unpack("<ii", self.shp.read(8))
        parts = list(struct.unpack(f"<{nparts}i", self.shp.read(4 * nparts))) + [npts]
        xy = struct.unpack(f"<{2 * npts}d", self.shp.read(16 * npts))
        pts = list(zip(xy[0::2], xy[1::2]))
        return [pts[parts[k]:parts[k + 1]] for k in range(nparts)]
