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
	values := func(t tag) ([]int, error) {
		size := 4
		if t.typ == 3 {
			size = 2
		}
		if t.cnt*size <= 4 {
			if t.cnt == 1 {
				return []int{t.val}, nil
			}
			out := make([]int, t.cnt)
			var raw [4]byte
			bo.PutUint32(raw[:], uint32(t.val))
			for i := range out {
				out[i] = int(bo.Uint16(raw[i*2:]))
			}
			return out, nil
		}
		// offsets come from upstream bytes: bound them before
		// dereferencing, or a truncated body panics the handler
		if t.val < 0 || t.val+t.cnt*size > len(data) {
			return nil, fmt.Errorf("tiff: tag values out of range")
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
		return out, nil
	}

	if c, ok := tags[259]; ok && c.val != 1 {
		return nil, fmt.Errorf("tiff: compression %d (want none)", c.val)
	}
	img := make([]byte, width*height) // zero-filled: empty stays zero
	if t, tiled := tags[322]; tiled {
		tw, tl := t.val, tags[323].val
		if tw <= 0 || tl <= 0 {
			return nil, fmt.Errorf("tiff: bad tile size %dx%d", tw, tl)
		}
		perRow := int(math.Ceil(float64(width) / float64(tw)))
		offs, err := values(tags[324])
		if err != nil {
			return nil, err
		}
		counts, err := values(tags[325])
		if err != nil {
			return nil, err
		}
		for i, toff := range offs {
			if toff == 0 || i >= len(counts) || counts[i] == 0 {
				continue
			}
			ty, tx := i/perRow, i%perRow
			w := min(tw, width-tx*tw)
			if w <= 0 {
				continue // tile column beyond the image
			}
			for r := 0; r < tl; r++ {
				y := ty*tl + r
				if y >= height {
					break
				}
				src := toff + r*tw
				if src < 0 || src+w > len(data) {
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
	if rps <= 0 {
		return nil, fmt.Errorf("tiff: bad rows-per-strip %d", rps)
	}
	offs, err := values(tags[273])
	if err != nil {
		return nil, err
	}
	counts, err := values(tags[279])
	if err != nil {
		return nil, err
	}
	for i, soff := range offs {
		if soff == 0 || i >= len(counts) || counts[i] == 0 {
			continue
		}
		start := i * rps * width
		if start >= len(img) {
			return nil, fmt.Errorf("tiff: strip beyond image")
		}
		end := min(start+counts[i], len(img))
		if soff < 0 || soff+end-start > len(data) {
			return nil, fmt.Errorf("tiff: strip out of range")
		}
		copy(img[start:end], data[soff:soff+end-start])
	}
	return img, nil
}
