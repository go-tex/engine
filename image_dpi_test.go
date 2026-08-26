// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"encoding/binary"
	"testing"
)

// pngWithPHYs builds a minimal PNG byte stream: the signature, one pHYs chunk
// with the given pixels-per-unit and unit specifier, and an IDAT terminator, so
// the resolution parser has a realistic (if tiny) file to walk.
func pngWithPHYs(ppuX, ppuY uint32, unit byte, includePHYs bool) []byte {
	b := append([]byte(nil), pngMagic...)
	chunk := func(typ string, body []byte) {
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(body)))
		b = append(b, l[:]...)
		b = append(b, typ...)
		b = append(b, body...)
		b = append(b, 0, 0, 0, 0) // CRC placeholder (the parser ignores it)
	}
	// A leading IHDR so the parser must advance past a non-pHYs chunk first.
	chunk("IHDR", make([]byte, 13))
	if includePHYs {
		var body [9]byte
		binary.BigEndian.PutUint32(body[0:], ppuX)
		binary.BigEndian.PutUint32(body[4:], ppuY)
		body[8] = unit
		chunk("pHYs", body[:])
	}
	chunk("IDAT", []byte{0x00})
	return b
}

func jpegWithJFIF(units byte, xd, yd uint16, includeJFIF bool) []byte {
	b := []byte{0xFF, 0xD8} // SOI
	if includeJFIF {
		seg := []byte("JFIF\x00")
		seg = append(seg, 0x01, 0x02) // version
		seg = append(seg, units)
		var d [4]byte
		binary.BigEndian.PutUint16(d[0:], xd)
		binary.BigEndian.PutUint16(d[2:], yd)
		seg = append(seg, d[:]...)
		seg = append(seg, 0, 0) // thumbnail w/h
		var l [2]byte
		binary.BigEndian.PutUint16(l[:], uint16(len(seg)+2))
		b = append(b, 0xFF, 0xE0)
		b = append(b, l[:]...)
		b = append(b, seg...)
	}
	// A start-of-scan marker so a JFIF-less file still reaches a terminating case.
	b = append(b, 0xFF, 0xDA, 0x00, 0x02)
	return b
}

func TestRasterResolution(t *testing.T) {
	const eps = 1e-6
	near := func(a, b float64) bool {
		d := a - b
		return d < eps && d > -eps
	}

	// PNG pHYs, unit 1 (metre): 5906 ppm ≈ 150 dpi.
	if dx, dy := rasterResolution(pngWithPHYs(5906, 5906, 1, true)); !near(dx, 5906*0.0254) || !near(dy, 5906*0.0254) {
		t.Errorf("png metre pHYs = %v/%v", dx, dy)
	}
	// PNG pHYs unit 0 (aspect only) → no physical resolution.
	if dx, dy := rasterResolution(pngWithPHYs(1, 1, 0, true)); dx != 0 || dy != 0 {
		t.Errorf("png aspect pHYs = %v/%v, want 0/0", dx, dy)
	}
	// PNG without a pHYs chunk (pHYs would precede IDAT).
	if dx, dy := rasterResolution(pngWithPHYs(0, 0, 0, false)); dx != 0 || dy != 0 {
		t.Errorf("png no pHYs = %v/%v, want 0/0", dx, dy)
	}
	// A pHYs chunk too short to hold ppuX/ppuY/unit is rejected.
	short := append(append([]byte(nil), pngMagic...), 0, 0, 0, 3, 'p', 'H', 'Y', 's', 1, 2, 3, 0, 0, 0, 0)
	if dx, dy := rasterResolution(short); dx != 0 || dy != 0 {
		t.Errorf("png short pHYs = %v/%v, want 0/0", dx, dy)
	}
	// A chunk whose declared length overruns the buffer stops the walk.
	overrun := append(append([]byte(nil), pngMagic...), 0x7F, 0xFF, 0xFF, 0xFF, 'j', 'U', 'N', 'K')
	if dx, dy := rasterResolution(overrun); dx != 0 || dy != 0 {
		t.Errorf("png overrun = %v/%v, want 0/0", dx, dy)
	}
	// A truncated stream (only the signature) walks to the end and returns 0.
	if dx, dy := rasterResolution(append([]byte(nil), pngMagic...)); dx != 0 || dy != 0 {
		t.Errorf("png sig-only = %v/%v, want 0/0", dx, dy)
	}

	// JPEG JFIF, units 1 (dpi).
	if dx, dy := rasterResolution(jpegWithJFIF(1, 300, 150, true)); dx != 300 || dy != 150 {
		t.Errorf("jpeg dpi = %v/%v, want 300/150", dx, dy)
	}
	// JPEG JFIF, units 2 (dots per cm) → ×2.54.
	if dx, dy := rasterResolution(jpegWithJFIF(2, 100, 100, true)); !near(dx, 254) || !near(dy, 254) {
		t.Errorf("jpeg dpcm = %v/%v, want 254/254", dx, dy)
	}
	// JPEG JFIF, units 0 (aspect ratio only).
	if dx, dy := rasterResolution(jpegWithJFIF(0, 1, 1, true)); dx != 0 || dy != 0 {
		t.Errorf("jpeg aspect = %v/%v, want 0/0", dx, dy)
	}
	// JPEG JFIF with a zero density is ignored.
	if dx, dy := rasterResolution(jpegWithJFIF(1, 0, 72, true)); dx != 0 || dy != 0 {
		t.Errorf("jpeg zero density = %v/%v, want 0/0", dx, dy)
	}
	// JPEG without a JFIF APP0: the walk reaches the SOS marker and stops.
	if dx, dy := rasterResolution(jpegWithJFIF(1, 1, 1, false)); dx != 0 || dy != 0 {
		t.Errorf("jpeg no JFIF = %v/%v, want 0/0", dx, dy)
	}
	// A non-JFIF APP1 (EXIF) segment is skipped before the JFIF APP0 is found —
	// exercising the loop's advance past a segment it does not recognise.
	exifThenJFIF := append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x08, 'E', 'x', 'i', 'f', 0x00, 0x00}, jpegWithJFIF(1, 96, 96, true)[2:]...)
	if dx, dy := rasterResolution(exifThenJFIF); dx != 96 || dy != 96 {
		t.Errorf("jpeg exif-then-jfif = %v/%v, want 96/96", dx, dy)
	}
	// A non-0xFF byte where a marker must begin (with room in the buffer to look).
	if dx, dy := rasterResolution([]byte{0xFF, 0xD8, 0x00, 0x11, 0x00, 0x00}); dx != 0 || dy != 0 {
		t.Errorf("jpeg no-FF = %v/%v, want 0/0", dx, dy)
	}
	// A JPEG whose second marker segment has an impossible length stops the walk.
	badLen := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x01}
	if dx, dy := rasterResolution(badLen); dx != 0 || dy != 0 {
		t.Errorf("jpeg bad seglen = %v/%v, want 0/0", dx, dy)
	}
	// A JPEG with a restart marker (no length) between SOI and any JFIF stops.
	rst := []byte{0xFF, 0xD8, 0xFF, 0xD0, 0x00, 0x00}
	if dx, dy := rasterResolution(rst); dx != 0 || dy != 0 {
		t.Errorf("jpeg RSTn = %v/%v, want 0/0", dx, dy)
	}
	// A byte stream where a marker segment is not led by 0xFF is rejected.
	noFF := []byte{0xFF, 0xD8, 0x00, 0x11}
	if dx, dy := rasterResolution(noFF); dx != 0 || dy != 0 {
		t.Errorf("jpeg no-FF = %v/%v, want 0/0", dx, dy)
	}

	// Neither PNG nor JPEG magic: no resolution.
	if dx, dy := rasterResolution([]byte("GIF89a")); dx != 0 || dy != 0 {
		t.Errorf("non-raster = %v/%v, want 0/0", dx, dy)
	}
}
