// Minimal reader for the uncompressed U8 TIFFs the EEA image service
// emits — port of tools/fetch_grid.py parse_tiff. Handles tiled or
// stripped layout, inline tag values, and the zero-tile "empty" TIFF
// returned for a bbox with no data (-> all zeros).
package main

import (
	"encoding/binary"
	"fmt"
	"math"
)

func parseTIFF(data []byte, width, height int) ([]byte, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("tiff: too short")
	}
	var bo binary.ByteOrder = binary.LittleEndian
	if data[0] == 'M' {
		bo = binary.BigEndian
	}
	off := int(bo.Uint32(data[4:8]))
	if off+2 > len(data) {
		return nil, fmt.Errorf("tiff: bad IFD offset")
	}
	n := int(bo.Uint16(data[off : off+2]))
	type tag struct {
		typ, cnt, val int
	}
	tags := map[int]tag{}
	for i := 0; i < n; i++ {
		p := off + 2 + i*12
		if p+12 > len(data) {
			return nil, fmt.Errorf("tiff: truncated IFD")
		}
		tags[int(bo.Uint16(data[p:]))] = tag{
			typ: int(bo.Uint16(data[p+2:])),
			cnt: int(bo.Uint32(data[p+4:])),
			val: int(bo.Uint32(data[p+8:])),
		}
	}
	values := func(t tag) []int {
		size := 4
		if t.typ == 3 {
			size = 2
		}
		if t.cnt*size <= 4 {
			if t.cnt == 1 {
				return []int{t.val}
			}
			out := make([]int, t.cnt)
			var raw [4]byte
			bo.PutUint32(raw[:], uint32(t.val))
			for i := range out {
				out[i] = int(bo.Uint16(raw[i*2:]))
			}
			return out
		}
		out := make([]int, t.cnt)
		for i := range out {
			p := t.val + i*size
			if size == 2 {
				out[i] = int(bo.Uint16(data[p:]))
			} else {
				out[i] = int(bo.Uint32(data[p:]))
			}
		}
		return out
	}

	if c, ok := tags[259]; ok && c.val != 1 {
		return nil, fmt.Errorf("tiff: compression %d (want none)", c.val)
	}
	img := make([]byte, width*height) // zero-filled: empty stays zero
	if t, tiled := tags[322]; tiled {
		tw, tl := t.val, tags[323].val
		perRow := int(math.Ceil(float64(width) / float64(tw)))
		offs, counts := values(tags[324]), values(tags[325])
		for i, toff := range offs {
			if toff == 0 || i >= len(counts) || counts[i] == 0 {
				continue
			}
			ty, tx := i/perRow, i%perRow
			for r := 0; r < tl; r++ {
				y := ty*tl + r
				if y >= height {
					break
				}
				w := min(tw, width-tx*tw)
				src := toff + r*tw
				if src+w > len(data) {
					return nil, fmt.Errorf("tiff: tile out of range")
				}
				copy(img[y*width+tx*tw:], data[src:src+w])
			}
		}
		return img, nil
	}
	rps := height
	if t, ok := tags[278]; ok {
		rps = t.val
	}
	offs, counts := values(tags[273]), values(tags[279])
	for i, soff := range offs {
		if soff == 0 || i >= len(counts) || counts[i] == 0 {
			continue
		}
		start := i * rps * width
		end := min(start+counts[i], len(img))
		if soff+end-start > len(data) {
			return nil, fmt.Errorf("tiff: strip out of range")
		}
		copy(img[start:end], data[soff:soff+end-start])
	}
	return img, nil
}
