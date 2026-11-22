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
