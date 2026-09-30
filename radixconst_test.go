// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ TeX's hexadecimal and octal constants (tex.web §445) were not scanned, and the failure
// was not silent: the scanner returned 0 AND left the constant in the stream, so its digits
// were typeset as prose.
//
// 2603.18955 writes \mathchardef\emptyset="001F in its preamble, and the string "001F came
// out on page 1 ahead of the title. Measured on 999 papers: 296 hexadecimal constants over
// 51 papers and 18 octal ones over 2, sitting on the primitives a symbol setup is made of —
// \mathchardef 95, \mathcode 24, \chardef 9. Over those 52 papers the fix removes 196 glyph
// paths of spurious literals and moves no page count; 2606.21462 also regains real text.

func radixValue(t *testing.T, src string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\newcount\zzn `+src+
		`\begin{document}[\the\zzn]\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(e.RenderPages(e.renderMargin(0)), "")
}

func radixGlyphs(t *testing.T, src string) int {
	t.Helper()
	return strings.Count(radixValue(t, src), "<path")
}

// Each notation of tex.web §445 must give the same number, which is what a symbol setup
// assumes when it writes "0180 for a slot it also knows as 384.
func TestEveryRadixNotationScansTheSameNumber(t *testing.T) {
	dec := radixGlyphs(t, `\zzn=65 `)
	for _, c := range []struct{ name, src string }{
		{"hexadecimal", `\zzn="41 `},
		{"octal", `\zzn='101 `},
		{"alphabetic", "\\zzn=`A "},
	} {
		if got := radixGlyphs(t, c.src); got != dec {
			t.Errorf("%s: %d glyph path(s), decimal 65 draws %d", c.name, got, dec)
		}
	}
}

// ⛔ The constant must be CONSUMED, not merely valued: an unscanned one left its digits to
// the paragraph, which is how "001F reached a title page. The witness is a document with no
// other content, so anything drawn beyond the number and the page number is the leak.
func TestAnUnscannedConstantDoesNotReachThePage(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\makeatletter`+
		`\mathchardef\zzsym="001F \makeatother\begin{document}Z\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	// Z and the page number. "001F would add five more.
	if n := strings.Count(svg, "<path"); n != 2 {
		t.Errorf("%d glyph path(s), want 2 (Z and the page number): the constant's digits "+
			"were typeset", n)
	}
}

// ⛔ Only an UPPERCASE letter is a hexadecimal digit. TeX stops at anything else and leaves
// it in the stream, so "1f is the number 1 followed by a letter f — checked against
// tectonic, where that stray f is what produces "Missing \begin{document}". Accepting
// lowercase would read 31 where the reference reads 1, which is a wrong number rather than
// a missing one.
func TestALowercaseHexDigitEndsTheConstant(t *testing.T) {
	one := radixGlyphs(t, `\zzn="1 `)
	got := radixGlyphs(t, `\zzn="1F `)
	if got == one {
		t.Error(`"1F scanned as 1: an uppercase F must be a hexadecimal digit`)
	}
	// "1f is 1, and the f is left to the paragraph — so it draws one glyph MORE than "1.
	if lc := radixGlyphs(t, `\zzn="1f `); lc != one+1 {
		t.Errorf(`"1f drew %d glyph path(s), want %d (the number 1, then a stray f)`, lc, one+1)
	}
}

// An octal constant stops at 8 and 9, which are not octal digits.
func TestAnOctalConstantStopsAtEight(t *testing.T) {
	seven := radixGlyphs(t, `\zzn='7 `)
	if got := radixGlyphs(t, `\zzn='78 `); got != seven+1 {
		t.Errorf(`'78 drew %d glyph path(s), want %d (the number 7, then a stray 8)`, got, seven+1)
	}
}

func TestRadixDigitKnowsItsRadix(t *testing.T) {
	for _, c := range []struct {
		ch    rune
		radix int
		want  int
		ok    bool
	}{
		{'7', 8, 7, true}, {'8', 8, 0, false}, {'9', 16, 9, true},
		{'A', 16, 10, true}, {'F', 16, 15, true}, {'a', 16, 0, false},
		{'G', 16, 0, false},
	} {
		got, ok := radixDigit(tok{ch: c.ch, cat: catOther}, c.radix)
		if got != c.want || ok != c.ok {
			t.Errorf("radixDigit(%q, %d) = %d,%v want %d,%v", c.ch, c.radix, got, ok, c.want, c.ok)
		}
	}
	if _, ok := radixDigit(tok{cs: "relax", cs_: true}, 16); ok {
		t.Error("a control sequence was taken for a digit")
	}
}
