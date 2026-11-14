"""Read a window of a local GeoTIFF, stdlib only.

Covers what the Copernicus Land rasters ship as: single-band U8, tiled or
stripped, compression none (1), LZW (5) or Deflate (8, 32946), horizontal
predictor (2), north-up with ModelPixelScale + ModelTiepoint. Only the
blocks that overlap the window are decoded.
"""
import collections
import struct
import zlib

_OPEN = {}


def open_tif(path):
    """One reader per file, kept open so its block cache is reused."""
    if path not in _OPEN:
        _OPEN[path] = GeoTIFF(path)
    return _OPEN[path]


def lzw_decode(data):
    """TIFF LZW: MSB-first codes, 9-12 bits, 256 = clear, 257 = end, early change."""
    out = bytearray()
    table = [bytes([i]) for i in range(256)] + [b"", b""]
    bits, nbits, pos, prev = 0, 0, 0, None
    width = 9
    n = len(data)
    while True:
        while nbits < width:
            if pos >= n:
                return bytes(out)
            bits = (bits << 8) | data[pos]
            pos += 1
            nbits += 8
        nbits -= width
        code = (bits >> nbits) & ((1 << width) - 1)
        if code == 256:
            table = table[:258]
            width = 9
            prev = None
            continue
        if code == 257:
            return bytes(out)
        if prev is None:
            entry = table[code]
        elif code < len(table):
            entry = table[code]
            table.append(prev + entry[:1])
        else:
            entry = prev + prev[:1]
            table.append(entry)
        out += entry
        prev = entry
        if len(table) + 1 >= (1 << width) and width < 12:
            width += 1


class GeoTIFF:
    def __init__(self, path):
        self.f = open(path, "rb")
        head = self.f.read(8)
        self.bo = "<" if head[:2] == b"II" else ">"
        if struct.unpack(self.bo + "H", head[2:4])[0] != 42:
            raise ValueError(f"{path}: not a classic TIFF (BigTIFF unsupported)")
        off = struct.unpack(self.bo + "I", head[4:8])[0]
        self.tags = self._ifd(off)
        t = self.tags
        self.width, self.height = t[256][0], t[257][0]
        if t.get(258, [8])[0] != 8 or t.get(277, [1])[0] != 1:
            raise ValueError(f"{path}: only single-band 8-bit rasters")
        self.compression = t.get(259, [1])[0]
        self.predictor = t.get(317, [1])[0]
        sx, sy = t[33550][0], t[33550][1]
        _, _, _, tx, ty, _ = t[33922][:6]
        self.x0, self.y1, self.sx, self.sy = tx, ty, sx, sy
        if 322 in t:
            self.bw, self.bh = t[322][0], t[323][0]
            self.offs, self.counts = t[324], t[325]
        else:
            self.bw, self.bh = self.width, t.get(278, [self.height])[0]
            self.offs, self.counts = t[273], t[279]
        self.per_row = -(-self.width // self.bw)
        self.cache = collections.OrderedDict()  # LRU of decoded blocks

    def _ifd(self, off):
        bo, f = self.bo, self.f
        f.seek(off)
        n = struct.unpack(bo + "H", f.read(2))[0]
        raw = f.read(12 * n)
        sizes = {1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 7: 1, 11: 4, 12: 8, 16: 8}
        fmts = {1: "B", 2: "B", 3: "H", 4: "I", 5: "II", 7: "B", 11: "f", 12: "d", 16: "Q"}
        tags = {}
        for i in range(n):
            tag, typ, cnt = struct.unpack(bo + "HHI", raw[i * 12:i * 12 + 8])
            size = sizes.get(typ, 1) * cnt
            val = raw[i * 12 + 8:i * 12 + 12]
            if size > 4:
                f.seek(struct.unpack(bo + "I", val)[0])
                val = f.read(size)
            fmt = fmts.get(typ, "B")
            k = cnt * (2 if typ == 5 else 1)
            tags[tag] = list(struct.unpack(bo + fmt[0] * k, val[:size]))
        return tags

    def _block(self, i):
        if i in self.cache:
            self.cache.move_to_end(i)
            return self.cache[i]
        off, cnt = self.offs[i], self.counts[i]
        if off == 0 or cnt == 0:
            data = bytes(self.bw * self.bh)
        else:
            self.f.seek(off)
            raw = self.f.read(cnt)
            if self.compression == 1:
                data = raw
            elif self.compression == 5:
                data = lzw_decode(raw)
            elif self.compression in (8, 32946):
                data = zlib.decompress(raw)
            else:
                raise ValueError(f"compression {self.compression} unsupported")
            if self.predictor == 2:
                b = bytearray(data)
                w = self.bw
                for r in range(len(b) // w):
                    s = r * w
                    for c in range(s + 1, s + w):
                        b[c] = (b[c] + b[c - 1]) & 0xFF
                data = bytes(b)
        self.cache[i] = data
        if len(self.cache) > 96:  # ~96 MB per open file at 1024x1024 blocks
            self.cache.popitem(last=False)
        return data

    def window(self, x0, y1, w, h, fill=255):
        """w x h pixels whose north-west corner is (x0, y1) in the raster's
        CRS, on its own pixel grid; outside the raster = fill. Row 0 = north."""
        c0 = round((x0 - self.x0) / self.sx)
        r0 = round((self.y1 - y1) / self.sy)
        out = bytearray([fill]) * (w * h)
        for r in range(max(0, r0), min(self.height, r0 + h)):
            br = r // self.bh
            for bc in range(max(0, c0) // self.bw, (min(self.width, c0 + w) - 1) // self.bw + 1):
                blk = self._block(br * self.per_row + bc)
                cs, ce = max(c0, bc * self.bw), min(c0 + w, (bc + 1) * self.bw, self.width)
                if ce <= cs:
                    continue
                src = (r - br * self.bh) * self.bw + (cs - bc * self.bw)
                dst = (r - r0) * w + (cs - c0)
                out[dst:dst + ce - cs] = blk[src:src + ce - cs]
        return out


def mosaic(paths, x0, y1, w, h, fill=255):
    """The same window read across several tiles (first non-fill wins)."""
    out = bytearray([fill]) * (w * h)
    for p in paths:
        part = open_tif(p).window(x0, y1, w, h, fill)
        for i, v in enumerate(part):
            if v != fill and out[i] == fill:
                out[i] = v
    return out
