// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"strconv"
	"strings"

	"github.com/go-pdfkit/reader"
)

// fontInfo is as much of a page's font as a geometry report needs: its name, how
// its bytes map to characters (its ToUnicode CMap), and how wide each code is.
//
// twoByte distinguishes the two families this has to tell apart. A simple font
// shows one byte per glyph and keys its widths on the byte through /FirstChar
// and /Widths; a composite (Type0) font shows two bytes per glyph for the
// Identity encodings both TeX engines emit, and keys its widths on the CID
// through /W and /DW. Reading a Type0 font one byte at a time halves every
// position in the report, which is the kind of wrong that still looks plausible.
type fontInfo struct {
	Name     string
	twoByte  bool
	widths   map[int]float64 // glyph space (1/1000 em)
	missing  float64         // /MissingWidth or /DW
	toUni    map[int]string
	haveUnis bool
}

// pageFonts reads the page's /Resources /Font dictionary. A font that cannot be
// read is left out rather than faked, so a run drawn with it reports no text and
// no width instead of a wrong one.
func pageFonts(doc *reader.Document, page int) map[string]*fontInfo {
	out := map[string]*fontInfo{}
	pg, err := doc.Page(page)
	if err != nil {
		return out
	}
	res, ok := doc.GetDict(pg, "Resources")
	if !ok {
		return out
	}
	fonts, ok := doc.GetDict(res, "Font")
	if !ok {
		return out
	}
	for name := range fonts {
		fd, ok := doc.GetDict(fonts, name)
		if !ok {
			continue
		}
		out[string(name)] = readFont(doc, fd)
	}
	return out
}

func readFont(doc *reader.Document, fd reader.Dict) *fontInfo {
	f := &fontInfo{Name: "?", missing: 0}
	if n, ok := reader.ToName(resolve(doc, fd["BaseFont"])); ok {
		f.Name = stripSubsetTag(string(n))
	}
	sub, _ := reader.ToName(resolve(doc, fd["Subtype"]))
	f.toUni, f.haveUnis = readToUnicode(doc, fd)
	if sub == "Type0" {
		f.twoByte = true
		f.readCIDWidths(doc, fd)
		return f
	}
	f.readSimpleWidths(doc, fd)
	return f
}

// readSimpleWidths fills the widths of a simple font from /FirstChar + /Widths.
func (f *fontInfo) readSimpleWidths(doc *reader.Document, fd reader.Dict) {
	first, _ := reader.ToInt(resolve(doc, fd["FirstChar"]))
	arr, ok := reader.ToArray(resolve(doc, fd["Widths"]))
	if !ok {
		return
	}
	f.widths = make(map[int]float64, len(arr))
	for i, o := range arr {
		if w, ok := reader.ToFloat(resolve(doc, o)); ok {
			f.widths[int(first)+i] = w
		}
	}
	if desc, ok := doc.GetDict(fd, "FontDescriptor"); ok {
		if mw, ok := reader.ToFloat(resolve(doc, desc["MissingWidth"])); ok {
			f.missing = mw
		}
	}
}

// readCIDWidths fills the widths of a composite font from its descendant's /W
// array, which comes in two shapes: "c [w w w]" and "c_first c_last w".
func (f *fontInfo) readCIDWidths(doc *reader.Document, fd reader.Dict) {
	f.missing = 1000 // /DW's default
	descArr, ok := reader.ToArray(resolve(doc, fd["DescendantFonts"]))
	if !ok || len(descArr) == 0 {
		return
	}
	desc, ok := reader.ToDict(resolve(doc, descArr[0]))
	if !ok {
		return
	}
	if dw, ok := reader.ToFloat(resolve(doc, desc["DW"])); ok {
		f.missing = dw
	}
	w, ok := reader.ToArray(resolve(doc, desc["W"]))
	if !ok {
		return
	}
	f.widths = map[int]float64{}
	for i := 0; i < len(w); {
		c, ok := reader.ToInt(resolve(doc, w[i]))
		if !ok {
			break
		}
		if i+1 >= len(w) {
			break
		}
		if list, ok := reader.ToArray(resolve(doc, w[i+1])); ok {
			for k, o := range list {
				if v, ok := reader.ToFloat(resolve(doc, o)); ok {
					f.widths[int(c)+k] = v
				}
			}
			i += 2
			continue
		}
		if i+2 >= len(w) {
			break
		}
		last, ok1 := reader.ToInt(resolve(doc, w[i+1]))
		v, ok2 := reader.ToFloat(resolve(doc, w[i+2]))
		if ok1 && ok2 && last >= c && last-c < 65536 {
			for g := c; g <= last; g++ {
				f.widths[int(g)] = v
			}
		}
		i += 3
	}
}

// codes splits a shown string into the glyph codes the font reads it as.
func (f *fontInfo) codes(b []byte) []int {
	if f.twoByte {
		out := make([]int, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			out = append(out, int(b[i])<<8|int(b[i+1]))
		}
		return out
	}
	out := make([]int, len(b))
	for i, c := range b {
		out[i] = int(c)
	}
	return out
}

// Decode renders a shown string as text through the font's ToUnicode CMap, or
// "" when it has none — never by guessing an encoding.
func (f *fontInfo) Decode(b []byte) string {
	if !f.haveUnis {
		return ""
	}
	var sb strings.Builder
	for _, c := range f.codes(b) {
		sb.WriteString(f.toUni[c])
	}
	return sb.String()
}

// Width returns the advance of a shown string in text space, at the given font
// size and with the given character and word spacing. A font whose widths could
// not be read returns 0, which the caller reports as "unknown" rather than 0pt.
func (f *fontInfo) Width(b []byte, size, charSp, wordSp float64) float64 {
	if f.widths == nil && f.missing == 0 {
		return 0
	}
	var w float64
	for _, c := range f.codes(b) {
		g, ok := f.widths[c]
		if !ok {
			g = f.missing
		}
		w += g/1000*size + charSp
		// Word spacing applies to the single byte 32, and never to a two-byte code.
		if !f.twoByte && c == 32 {
			w += wordSp
		}
	}
	return w
}

// stripSubsetTag removes the "ABCDEF+" a subset-embedded font carries.
func stripSubsetTag(name string) string {
	if i := strings.IndexByte(name, '+'); i == 6 {
		return name[i+1:]
	}
	return name
}

func resolve(doc *reader.Document, o reader.Object) reader.Object {
	v, err := doc.Resolve(o)
	if err != nil {
		return o
	}
	return v
}

// readToUnicode parses the font's /ToUnicode CMap: the beginbfchar and
// beginbfrange sections that say what each code means. It reads the CMap with
// the PDF content scanner rather than with a regular expression, so a hex string
// that happens to span a line, or a comment between the sections, is read the
// way a renderer reads it.
func readToUnicode(doc *reader.Document, fd reader.Dict) (map[int]string, bool) {
	st, ok := reader.ToStream(resolve(doc, fd["ToUnicode"]))
	if !ok {
		return nil, false
	}
	data, _, err := doc.DecodeStream(st)
	if err != nil {
		return nil, false
	}
	m := map[int]string{}
	sc := reader.NewContentScanner(data)
	var pending []reader.Object
	for {
		op, ok := sc.Next()
		if !ok {
			break
		}
		switch op.Operator {
		case "begincodespacerange", "beginbfchar", "beginbfrange":
			pending = nil
		case "endbfchar":
			// The entries sit between the two keywords with no operator of their
			// own, so the scanner hands them to THIS operation. A CMap long enough
			// to be split across several operations leaves the rest in pending.
			pending = append(pending, op.Operands...)
			for i := 0; i+1 < len(pending); i += 2 {
				src, ok1 := hexCode(pending[i])
				dst, ok2 := reader.ToString(pending[i+1])
				if ok1 && ok2 {
					m[src] = utf16BEString(dst)
				}
			}
			pending = nil
		case "endbfrange":
			pending = append(pending, op.Operands...)
			for i := 0; i+2 < len(pending); i += 3 {
				lo, ok1 := hexCode(pending[i])
				hi, ok2 := hexCode(pending[i+1])
				if !ok1 || !ok2 || hi < lo || hi-lo > 65535 {
					continue
				}
				if list, ok := reader.ToArray(pending[i+2]); ok {
					for k, o := range list {
						if s, ok := reader.ToString(o); ok {
							m[lo+k] = utf16BEString(s)
						}
					}
					continue
				}
				base, ok := reader.ToString(pending[i+2])
				if !ok {
					continue
				}
				for c := lo; c <= hi; c++ {
					m[c] = utf16BEOffset(base, c-lo)
				}
			}
			pending = nil
		default:
			pending = append(pending, op.Operands...)
			continue
		}
	}
	return m, len(m) > 0
}

// hexCode reads a <..> source code as an integer.
func hexCode(o reader.Object) (int, bool) {
	b, ok := reader.ToString(o)
	if !ok || len(b) == 0 || len(b) > 4 {
		return 0, false
	}
	v := 0
	for _, c := range b {
		v = v<<8 | int(c)
	}
	return v, true
}

// utf16BEString decodes a CMap destination, which is UTF-16BE.
func utf16BEString(b []byte) string {
	var sb strings.Builder
	for i := 0; i+1 < len(b); i += 2 {
		u := rune(b[i])<<8 | rune(b[i+1])
		if u >= 0xD800 && u < 0xDC00 && i+3 < len(b) {
			lo := rune(b[i+2])<<8 | rune(b[i+3])
			sb.WriteRune(0x10000 + (u-0xD800)<<10 + (lo - 0xDC00))
			i += 2
			continue
		}
		sb.WriteRune(u)
	}
	return sb.String()
}

// utf16BEOffset is the destination of a bfrange's n-th code: the base string
// with its LAST unit advanced by n, which is what the CMap specification says.
func utf16BEOffset(base []byte, n int) string {
	if len(base) < 2 {
		return ""
	}
	b := append([]byte(nil), base...)
	last := len(b) - 2
	v := int(b[last])<<8 | int(b[last+1]) + n
	b[last], b[last+1] = byte(v>>8), byte(v)
	return utf16BEString(b)
}

// ftoa prints a length the way a layout report reads best: two decimals.
func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
