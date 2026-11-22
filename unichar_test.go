package engine

import (
	"strings"
	"testing"
)

// \DeclareUnicodeCharacter makes its character mean the text, and it is GLOBAL. Witness
// against tectonic 0.17 on the same document with a guarded definition (the bundle's
// inputenc is ignored under XeTeX): both give "aSNOWb" then "c d" — a declared rule
// box between c and d, which pdftotext shows as a space.
// Papers 2405.10478 and 2408.07843 declare 8 and 5 such characters.
func TestDeclareUnicodeCharacter(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`{\DeclareUnicodeCharacter{2603}{SNOW}}` + // declared inside a group: still global
		`\DeclareUnicodeCharacter{2514}{\mbox{\vrule height2ex width.5em}}` +
		"a☃b\\par c└d\\par \\the\\catcode\"2603 \\end{document}"
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	if !strings.Contains(text, "aSNOWb") {
		t.Errorf("the declared character did not become its text; the page reads %q", firstN(text, 120))
	}
	if strings.ContainsRune(text, '☃') {
		t.Errorf("the character itself was typeset: %q", firstN(text, 120))
	}
	if !strings.Contains(text, "13") {
		t.Errorf(`\the\catcode"2603 is not 13 (active) after the declaration: %q`, firstN(text, 160))
	}
}

// A \catcode past 255 is scoped like any other: the group's end restores it.
func TestCatcodePast255IsGrouped(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`{\catcode"2603=13 \def\x{}}X\the\catcode"2603 Y\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	if !strings.Contains(text, "X12Y") {
		t.Errorf("catcode of U+2603 after the group = ? want 12 (other): %q", firstN(text, 80))
	}
}

// The two sides of every guard around the sparse table, asserted as values. The
// boundary is the last code point, 0x10FFFF: it is addressable, the next is not, and a
// number outside the range reads as `other` instead of indexing past the table.
func TestHighCatcodeBounds(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	other := int(catOther)
	// A guard removed would index past the table and panic; report it as the
	// assertion it stands for.
	guarded := func(what string, f func()) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("%s: out of range was not refused: %v", what, r)
			}
		}()
		f()
	}
	for _, n := range []int{-1, 0x110000, 1 << 30} {
		guarded("catValue", func() {
			if got := e.catValue(n); got != other {
				t.Errorf(`\catcode %d = %d, want %d (other)`, n, got, other)
			}
		})
	}
	before := len(e.catHigh)
	guarded("setCat -1", func() { e.setCat(-1, catActive, true) })
	guarded("setCat 0x110000", func() { e.setCat(0x110000, catActive, true) })
	if len(e.catHigh) != before {
		t.Errorf("an out-of-range character was recorded: %d entries, was %d", len(e.catHigh), before)
	}
	guarded("setCat 0x10FFFF", func() { e.setCat(0x10FFFF, catActive, true) })
	if got := e.catValue(0x10FFFF); got != int(catActive) {
		t.Errorf("the last code point reads %d, want %d (active)", got, int(catActive))
	}
	guarded("setCat 0x2603", func() { e.setCat(0x2603, catActive, true) })
	if got := e.catcode['☃'&0xFF]; got == catActive {
		t.Error("a high character leaked into the 256-entry table")
	}
	// `other` is stored as absence: the map stays the size of what was declared.
	e.setCat(0x2603, catOther, true)
	if _, held := e.catHigh[0x2603]; held {
		t.Error("catcode `other` is kept in the map instead of deleting the entry")
	}
}

// What \DeclareUnicodeCharacter refuses to declare: a code below 256 (it would turn an
// ASCII or Latin-1 byte active), a code past the last code point, a non-hex argument and
// a control sequence in it. Each leaves the character an ordinary one.
func TestDeclareUnicodeCharacterRefusals(t *testing.T) {
	for _, tc := range []struct{ name, decl string }{
		{"below 256", `\DeclareUnicodeCharacter{41}{X}`},
		{"past the last code point", `\DeclareUnicodeCharacter{110000}{X}`},
		{"not hex", `\DeclareUnicodeCharacter{2G03}{X}`},
		{"a control sequence", `\DeclareUnicodeCharacter{\relax}{X}`},
		{"empty", `\DeclareUnicodeCharacter{}{X}`},
		// 2^64 + 0x2603 wraps to 0x2603 if the digits are not bounded as they are read.
		{"digits that wrap around", `\DeclareUnicodeCharacter{10000000000002603}{X}`},
	} {
		e, err := compile([]byte(`\documentclass{article}\begin{document}`+tc.decl+`A\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("[%s] compile: %v", tc.name, err)
		}
		if n := len(e.catHigh); n != 0 {
			t.Errorf("[%s] %d high catcode(s) recorded, want none", tc.name, n)
		}
		if e.catcode['A'] != catLetter {
			t.Errorf("[%s] A is no longer a letter", tc.name)
		}
		text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
		if strings.Contains(text, "X") {
			t.Errorf("[%s] the declaration took effect: %q", tc.name, firstN(text, 60))
		}
	}
	// The boundary is accepted: U+10FFFF is a code point, and "110000" is not.
	e, err := compile([]byte(`\documentclass{article}\begin{document}\DeclareUnicodeCharacter{10FFFF}{X}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if e.catValue(0x10FFFF) != int(catActive) {
		t.Error("U+10FFFF was refused")
	}
}
