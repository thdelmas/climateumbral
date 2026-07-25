// parseTIFF eats bytes from an upstream we don't control, so every
// bounds check gets a fixture that trips it, plus round-trips for
// the three shapes the EEA service actually emits: stripped, tiled,
// and the zero-tile "empty" response.
package main

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

type ifdEntry struct {
	tag, typ uint16
	cnt, val uint32
}

// buildTIFF assembles header + one IFD + tail. The first tail byte
// lands at dataStart(len(entries)); fixtures compute offsets from it.
func buildTIFF(bo binary.ByteOrder, entries []ifdEntry, tail []byte) []byte {
	var b bytes.Buffer
	if bo == binary.BigEndian {
		b.WriteString("MM")
	} else {
		b.WriteString("II")
	}
	w := func(v any) { binary.Write(&b, bo, v) }
	w(uint16(42))
	w(uint32(8)) // IFD directly after header
	w(uint16(len(entries)))
	for _, e := range entries {
		w(e.tag)
		w(e.typ)
		w(e.cnt)
		w(e.val)
	}
	w(uint32(0)) // no next IFD
	b.Write(tail)
	return b.Bytes()
}

func dataStart(entries int) uint32 {
	return 8 + 2 + 12*uint32(entries) + 4
}

const (
	typeShort = 3
	typeLong  = 4
)

func TestParseTIFFStripped(t *testing.T) {
	// 4x3, one strip holding the whole image
	pix := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	d := dataStart(3)
	data := buildTIFF(binary.LittleEndian, []ifdEntry{
		{259, typeShort, 1, 1},
		{273, typeLong, 1, d},
		{279, typeLong, 1, uint32(len(pix))},
	}, pix)
	img, err := parseTIFF(data, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img, pix) {
		t.Errorf("img = %v, want %v", img, pix)
	}
}

func TestParseTIFFStrippedBigEndian(t *testing.T) {
	pix := []byte{1, 2, 3, 4}
	d := dataStart(3)
	data := buildTIFF(binary.BigEndian, []ifdEntry{
		{259, typeShort, 1, 1},
		{273, typeLong, 1, d},
		{279, typeLong, 1, uint32(len(pix))},
	}, pix)
	img, err := parseTIFF(data, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img, pix) {
		t.Errorf("img = %v, want %v", img, pix)
	}
}

func TestParseTIFFMultiStrip(t *testing.T) {
	// 4x3 as three 1-row strips, offset/count arrays out of line
	pix := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	d := dataStart(4)
	offArr, cntArr := d+12, d+12+12
	var tail bytes.Buffer
	tail.Write(pix)
	for i := 0; i < 3; i++ { // strip offsets
		binary.Write(&tail, binary.LittleEndian, d+uint32(i*4))
	}
	for i := 0; i < 3; i++ { // strip byte counts
		binary.Write(&tail, binary.LittleEndian, uint32(4))
	}
	data := buildTIFF(binary.LittleEndian, []ifdEntry{
		{259, typeShort, 1, 1},
		{273, typeLong, 3, offArr},
		{278, typeShort, 1, 1},
		{279, typeLong, 3, cntArr},
	}, tail.Bytes())
	img, err := parseTIFF(data, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img, pix) {
		t.Errorf("img = %v, want %v", img, pix)
	}
}

func TestParseTIFFTiled(t *testing.T) {
	// 3x3 from 2x2 tiles: right column and bottom row get clipped
	tiles := []byte{
		1, 2, 3, 4, // tile (0,0)
		5, 6, 7, 8, // tile (0,1): only col 0 lands
		9, 10, 11, 12, // tile (1,0): only row 0 lands
		13, 14, 15, 16, // tile (1,1): one pixel lands
	}
	d := dataStart(5)
	offArr, cntArr := d+16, d+16+16
	var tail bytes.Buffer
	tail.Write(tiles)
	for i := 0; i < 4; i++ {
		binary.Write(&tail, binary.LittleEndian, d+uint32(i*4))
	}
	for i := 0; i < 4; i++ {
		binary.Write(&tail, binary.LittleEndian, uint32(4))
	}
	data := buildTIFF(binary.LittleEndian, []ifdEntry{
		{259, typeShort, 1, 1},
		{322, typeShort, 1, 2},
		{323, typeShort, 1, 2},
		{324, typeLong, 4, offArr},
		{325, typeLong, 4, cntArr},
	}, tail.Bytes())
	img, err := parseTIFF(data, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 2, 5, 3, 4, 7, 9, 10, 13}
	if !bytes.Equal(img, want) {
		t.Errorf("img = %v, want %v", img, want)
	}
}

func TestParseTIFFEmpty(t *testing.T) {
	// the no-data bbox response: tiled layout, zero tiles -> all zero
	data := buildTIFF(binary.LittleEndian, []ifdEntry{
		{322, typeShort, 1, 2},
		{323, typeShort, 1, 2},
		{324, typeLong, 1, 0},
		{325, typeLong, 1, 0},
	}, nil)
	img, err := parseTIFF(data, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img, make([]byte, 9)) {
		t.Errorf("img = %v, want all zeros", img)
	}
}

func TestParseTIFFRejects(t *testing.T) {
	d3 := dataStart(3)
	cases := []struct {
		name string
		data []byte
		errs string
	}{
		{"too short", []byte{'I', 'I', 42, 0}, "too short"},
		{"bad IFD offset", buildTIFF(binary.LittleEndian,
			nil, nil)[:8], "bad IFD offset"},
		{"truncated IFD", func() []byte {
			// claims 5 entries, carries none
			b := buildTIFF(binary.LittleEndian, nil, nil)
			binary.LittleEndian.PutUint16(b[8:], 5)
			return b[:10]
		}(), "truncated IFD"},
		{"compression", buildTIFF(binary.LittleEndian, []ifdEntry{
			{259, typeShort, 1, 5},
		}, nil), "compression 5"},
		{"tag values out of range", buildTIFF(binary.LittleEndian,
			[]ifdEntry{
				{259, typeShort, 1, 1},
				{273, typeLong, 3, 9999},
				{279, typeLong, 1, 4},
			}, nil), "tag values out of range"},
		{"strip out of range", buildTIFF(binary.LittleEndian,
			[]ifdEntry{
				{259, typeShort, 1, 1},
				{273, typeLong, 1, 9999},
				{279, typeLong, 1, 4},
			}, nil), "strip out of range"},
		{"bad rows-per-strip", buildTIFF(binary.LittleEndian,
			[]ifdEntry{
				{259, typeShort, 1, 1},
				{273, typeLong, 1, d3},
				{278, typeShort, 1, 0},
			}, []byte{1, 2, 3, 4}), "bad rows-per-strip"},
		{"bad tile size", buildTIFF(binary.LittleEndian,
			[]ifdEntry{
				{322, typeShort, 1, 0},
				{323, typeShort, 1, 0},
			}, nil), "bad tile size"},
		{"tile out of range", buildTIFF(binary.LittleEndian,
			[]ifdEntry{
				{322, typeShort, 1, 2},
				{323, typeShort, 1, 2},
				{324, typeLong, 1, 9999},
				{325, typeLong, 1, 4},
			}, nil), "tile out of range"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseTIFF(c.data, 2, 2)
			if err == nil {
				t.Fatal("parsed, want error")
			}
			if !strings.Contains(err.Error(), c.errs) {
				t.Errorf("err = %q, want %q", err, c.errs)
			}
		})
	}
}
