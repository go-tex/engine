// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	_ "embed"
	"strconv"
	"sync"

	"github.com/go-opentype/opentype"
)

// Latin Modern is an OPTICALLY SIZED family, and this engine was treating it as
// one face scaled up and down.
//
// ⛔ lmroman9 IS NOT lmroman10 MADE SMALLER. It is a face drawn for 9pt, with
// relatively wider letters, looser spacing and a wider word space, because that
// is what reads well at 9pt. Scaling the 10pt design instead gets every
// footnote, every \scriptsize index, every \Large heading wrong — measured on
// the distribution's own files, advance ratios against lmroman10 at the same
// nominal size:
//
//	lmroman9  +2.7 % to +3.0 %      (\footnotesize, \small in an 11pt class)
//	lmroman12 about -2.2 %          (\large, \Large)
//	lmroman17 -7 % to -9 %          (\LARGE, \huge, \Huge)
//	lmroman5  +30 % to +42 %        (\tiny, sub-subscripts)
//
// The body at 10 to 10.95pt was the ONE size where a single lmroman10 was the
// right file, which is why this stayed invisible in a corpus measured mostly on
// body text.
//
// # The tables are the reference's own
//
// XeTeX — which tectonic is — resolves a face through NFSS: book.cls + [11pt]
// gives \@setfontsize\normalsize\@xipt, \@xipt is 10.95 (latex.ltx:7858), and
// TU/lmr/m/n is looked up in tulmr.fd. The tables below are that file's
// \DeclareFontShape ranges, transcribed:
//
//	m/n  (regular)  <-5.5> 5  <5.5-6.5> 6  <6.5-7.5> 7  <7.5-8.5> 8
//	                <8.5-9.5> 9  <9.5-11> 10  <11-15> 12  <15-> 17
//	bx/n (bold)     the same through 9.5-11, then <11-> 12 — there is NO lmroman17-bold
//	m/it (italic)   <-7.5> 7  <7.5-8.5> 8  <8.5-9.5> 9  <9.5-11> 10  <11-> 12
//	lmtt m/n (mono) <-8.5> 8  <8.5-9.5> 9  <9.5-11> 10  <11-> 12   (tulmtt.fd)
//
// ⛔ THE RANGES ARE HALF-OPEN: latex.ltx:8642-8659 (\check@range) tests
// lower <= f@size < upper. So 11pt body text takes lmroman10 and not lmroman12,
// which is exactly the case that matters and exactly the one an inclusive upper
// bound would get wrong.
//
// ⚠ Bold stops at 12 and italic at 12 because the distribution has no larger
// file for them — that is upstream's table, not a gap here. A \Huge\bfseries
// heading therefore scales lmroman12-bold, in both engines.

//go:embed fonts/lm/lmroman5-regular.otf
var lmRoman5Regular []byte

//go:embed fonts/lm/lmroman6-regular.otf
var lmRoman6Regular []byte

//go:embed fonts/lm/lmroman7-regular.otf
var lmRoman7Regular []byte

//go:embed fonts/lm/lmroman8-regular.otf
var lmRoman8Regular []byte

//go:embed fonts/lm/lmroman9-regular.otf
var lmRoman9Regular []byte

//go:embed fonts/lm/lmroman12-regular.otf
var lmRoman12Regular []byte

//go:embed fonts/lm/lmroman17-regular.otf
var lmRoman17Regular []byte

//go:embed fonts/lm/lmroman5-bold.otf
var lmRoman5Bold []byte

//go:embed fonts/lm/lmroman6-bold.otf
var lmRoman6Bold []byte

//go:embed fonts/lm/lmroman7-bold.otf
var lmRoman7Bold []byte

//go:embed fonts/lm/lmroman8-bold.otf
var lmRoman8Bold []byte

//go:embed fonts/lm/lmroman9-bold.otf
var lmRoman9Bold []byte

//go:embed fonts/lm/lmroman12-bold.otf
var lmRoman12Bold []byte

//go:embed fonts/lm/lmroman7-italic.otf
var lmRoman7Italic []byte

//go:embed fonts/lm/lmroman8-italic.otf
var lmRoman8Italic []byte

//go:embed fonts/lm/lmroman9-italic.otf
var lmRoman9Italic []byte

//go:embed fonts/lm/lmroman12-italic.otf
var lmRoman12Italic []byte

//go:embed fonts/lm/lmmono8-regular.otf
var lmMono8Regular []byte

//go:embed fonts/lm/lmmono9-regular.otf
var lmMono9Regular []byte

//go:embed fonts/lm/lmmono12-regular.otf
var lmMono12Regular []byte

// opticalRange is one \DeclareFontShape entry: the file to use while the
// requested size is below upTo. upTo == 0 means "and everything above", which
// is the table's final <N-> entry.
type opticalRange struct {
	upTo float64
	data []byte
	name string // for diagnostics and for the test that pins the table
}

// opticalFamily is a size→file table, in ascending order of upTo.
type opticalFamily []opticalRange

// pick returns the file for a requested size, following \check@range's
// half-open rule: the first range whose upTo is strictly greater than pt.
func (fam opticalFamily) pick(pt float64) (data []byte, name string) {
	for _, r := range fam {
		if r.upTo == 0 || pt < r.upTo {
			return r.data, r.name
		}
	}
	if len(fam) == 0 {
		return nil, ""
	}
	last := fam[len(fam)-1]
	return last.data, last.name
}

var (
	lmRomanRegularSizes = opticalFamily{
		{5.5, lmRoman5Regular, "lmroman5-regular"},
		{6.5, lmRoman6Regular, "lmroman6-regular"},
		{7.5, lmRoman7Regular, "lmroman7-regular"},
		{8.5, lmRoman8Regular, "lmroman8-regular"},
		{9.5, lmRoman9Regular, "lmroman9-regular"},
		{11, lmRomanRegular, "lmroman10-regular"},
		{15, lmRoman12Regular, "lmroman12-regular"},
		{0, lmRoman17Regular, "lmroman17-regular"},
	}
	lmRomanBoldSizes = opticalFamily{
		{5.5, lmRoman5Bold, "lmroman5-bold"},
		{6.5, lmRoman6Bold, "lmroman6-bold"},
		{7.5, lmRoman7Bold, "lmroman7-bold"},
		{8.5, lmRoman8Bold, "lmroman8-bold"},
		{9.5, lmRoman9Bold, "lmroman9-bold"},
		{11, lmRomanBold, "lmroman10-bold"},
		{0, lmRoman12Bold, "lmroman12-bold"},
	}
	lmRomanItalicSizes = opticalFamily{
		{7.5, lmRoman7Italic, "lmroman7-italic"},
		{8.5, lmRoman8Italic, "lmroman8-italic"},
		{9.5, lmRoman9Italic, "lmroman9-italic"},
		{11, lmRomanItalic, "lmroman10-italic"},
		{0, lmRoman12Italic, "lmroman12-italic"},
	}
	lmMonoSizes = opticalFamily{
		{8.5, lmMono8Regular, "lmmono8-regular"},
		{9.5, lmMono9Regular, "lmmono9-regular"},
		{11, lmMonoRegular, "lmmono10-regular"},
		{0, lmMono12Regular, "lmmono12-regular"},
	}
)

// opticalDesignPt is the size a Latin Modern file was DRAWN for, read out of
// its own name: lmroman9-regular is a 9pt design.
//
// ⚠ IT IS NOT THE SIZE THE FACE IS SET AT. lmroman10 at 10.95pt is a 10pt
// design used at 10.95, which is exactly the case tex.web §472 prints
// " at 10.95pt" for and a bare name otherwise. Reading it from the filename
// rather than from the font is deliberate: these are the distribution's own
// optical sizes and the number in the name is what tulmr.fd's ranges are
// written against, so the two cannot drift apart without the table being wrong
// too.
func opticalDesignPt(name string) float64 {
	digits := ""
	for _, r := range name {
		if r >= '0' && r <= '9' {
			digits += string(r)
			continue
		}
		if digits != "" {
			break // the first run of digits is the design size
		}
	}
	if digits == "" {
		return 0
	}
	v, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return float64(v)
}

// opticalOf returns the table a set of font bytes belongs to, or nil for bytes
// that are not one of ours.
//
// ⛔ KEYED ON THE SLICE'S IDENTITY, not on a name the caller passes. A host that
// supplies its own font through Options.Font must NOT be silently re-faced into
// Latin Modern at another size: only the files this package embeds carry a
// table, and &b[0] is what says so.
func opticalOf(b []byte) opticalFamily {
	switch {
	case sameBytes(b, lmRomanRegular):
		return lmRomanRegularSizes
	case sameBytes(b, lmRomanBold):
		return lmRomanBoldSizes
	case sameBytes(b, lmRomanItalic):
		return lmRomanItalicSizes
	case sameBytes(b, lmMonoRegular):
		return lmMonoSizes
	}
	return nil
}

// sameBytes reports whether two slices are the same underlying array, which is
// what identifies an embedded file. It is deliberately not a content compare:
// a host passing a byte-identical copy of Latin Modern has still passed its own
// font, and re-facing it would be a surprise.
func sameBytes(a, b []byte) bool {
	return len(a) > 0 && len(a) == len(b) && &a[0] == &b[0]
}

// parsedOptical caches the parsed program per embedded file, so crossing a size
// boundary does not re-parse a 110KB font on every \footnotesize.
var (
	parsedOpticalMu sync.Mutex
	parsedOptical   = map[*byte]*opentype.Font{}
)

// opticalProgram parses (once) the file a table names for pt.
func opticalProgram(fam opticalFamily, pt float64) (*opentype.Font, []byte, string) {
	data, name := fam.pick(pt)
	if len(data) == 0 {
		return nil, nil, ""
	}
	key := &data[0]
	parsedOpticalMu.Lock()
	defer parsedOpticalMu.Unlock()
	if f, ok := parsedOptical[key]; ok {
		return f, data, name
	}
	f, err := opentype.Parse(data)
	if err != nil {
		// An embedded file that will not parse is a build-time mistake, not a
		// runtime condition; the caller falls back to scaling what it has rather
		// than failing a document over it.
		return nil, nil, ""
	}
	parsedOptical[key] = f
	return f, data, name
}
