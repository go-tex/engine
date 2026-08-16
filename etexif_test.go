package engine

import (
	"strings"
	"testing"
)

// \ifdefined asks whether a control sequence has a meaning, without expanding it.
func TestIfdefined(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\ifdefined\relax Y\else N\fi`, "Y"},
		{`\ifdefined\notdefinedatall Y\else N\fi`, "N"},
		{`\ifdefined\relax Y\else N\fi`, "Y"}, // a primitive is defined
		{`\ifdefined xY\else N\fi`, "Y"},      // a character always is
	}
	for _, c := range cases {
		if got := runExpr(t, `\message{`+c.src+`}`); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// A control sequence defined by the document, and one \let to a primitive, are
// both defined; the tests above run on a bare engine, so the definition has to be
// executed rather than sit inside the \message being expanded.
func TestIfdefinedUserMacros(t *testing.T) {
	if got := runExpr(t, `\def\a{}\message{\ifdefined\a Y\else N\fi}`); got != "Y" {
		t.Errorf("a \\def'd macro = %q, want Y", got)
	}
	if got := runExpr(t, `\let\b\relax\message{\ifdefined\b Y\else N\fi}`); got != "Y" {
		t.Errorf("a \\let'd macro = %q, want Y", got)
	}
	if got := runExpr(t, `\def\a{}\message{\unless\ifdefined\a Y\else N\fi}`); got != "N" {
		t.Errorf("\\unless of a defined macro = %q, want N", got)
	}
	if got := runExpr(t, `\def\a{}\message{\ifx\a\undefined Y\else N\fi}`); got != "N" {
		t.Errorf("a defined macro must differ from undefined: %q", got)
	}
	if got := runExpr(t, `\let\a\relax\let\b\relax\message{\ifx\a\b Y\else N\fi}`); got != "Y" {
		t.Errorf("two \\let copies of the same primitive = %q, want Y", got)
	}
}

// \ifdefined must not expand the token it tests: a macro that would loop or
// consume its surroundings is merely looked up.
func TestIfdefinedDoesNotExpand(t *testing.T) {
	got := runExpr(t, `\def\eats#1{GONE}\ifdefined\eats\message{safe}\else\message{no}\fi\message{X}`)
	if got != "safe X" {
		t.Errorf("the tested macro was expanded: %q", got)
	}
}

// \ifcsname tests a name built from text, and — unlike \csname — leaves no trace:
// the control sequence it asked about is still undefined afterwards.
func TestIfcsname(t *testing.T) {
	if got := runExpr(t, `\def\abc{}\message{\ifcsname abc\endcsname Y\else N\fi}`); got != "Y" {
		t.Errorf("existing name = %q, want Y", got)
	}
	if got := runExpr(t, `\message{\ifcsname zz\endcsname Y\else N\fi}`); got != "N" {
		t.Errorf("missing name = %q, want N", got)
	}
	// Asking twice must give the same answer: the first ask must not define it.
	got := runExpr(t, `\message{\ifcsname zz\endcsname Y\else N\fi\ifcsname zz\endcsname Y\else N\fi}`)
	if got != "NN" {
		t.Errorf("\\ifcsname defined the name it asked about: %q", got)
	}
	// \csname, by contrast, does bring the name into existence (as \relax).
	if got := runExpr(t, `\expandafter\relax\csname zz\endcsname\message{\ifcsname zz\endcsname Y\else N\fi}`); got != "Y" {
		t.Errorf("\\csname did not create the name: %q", got)
	}
}

// \unless reverses the conditional it prefixes, and leaves anything else alone.
func TestUnless(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\unless\ifnum1=1 Y\else N\fi`, "N"},
		{`\unless\ifnum1=2 Y\else N\fi`, "Y"},
		{`\unless\ifdefined\nope Y\else N\fi`, "Y"},
		{`\unless\ifdefined\relax Y\else N\fi`, "N"},
		{`\unless\iftrue Y\else N\fi`, "N"},
		{`\unless\iffalse Y\else N\fi`, "Y"},
	}
	for _, c := range cases {
		if got := runExpr(t, `\message{`+c.src+`}`); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// \expanded expands its text completely and leaves the result in the input — the
// primitive pgfkeys refuses to load without.
func TestExpanded(t *testing.T) {
	if got := runExpr(t, `\def\a{A}\def\b{\a B}\message{\expanded{\b}}`); got != "AB" {
		t.Errorf("\\expanded = %q, want AB", got)
	}
	// Its meaning is \expanded, which is how a package checks for it.
	if got := runExpr(t, `\let\x\expanded\message{\meaning\x}`); got != `\expanded` {
		t.Errorf("\\meaning of a \\let copy = %q", got)
	}
	if got := runExpr(t, `\message{\string\expanded}`); got != `\expanded` {
		t.Errorf("\\string = %q", got)
	}
}

// \unexpanded keeps its text out of the expansion it sits in.
func TestUnexpanded(t *testing.T) {
	got := runExpr(t, `\def\a{A}\edef\x{\unexpanded{\a}}\message{\meaning\x}`)
	if !strings.Contains(got, `\a`) {
		t.Errorf("\\unexpanded lost its protection: %q", got)
	}
}

// \detokenize turns its text into the characters that spell it, so a control
// sequence in it is no longer one.
func TestDetokenize(t *testing.T) {
	if got := runExpr(t, `\message{\detokenize{\undefinedthing x}}`); got != `\undefinedthing x` {
		t.Errorf("\\detokenize = %q", got)
	}
}

// \scantokens re-reads its text as input, under the catcodes in force now.
func TestScantokens(t *testing.T) {
	if got := runExpr(t, `\def\a{A}\message{\scantokens{\a B}}`); got != "AB" {
		t.Errorf("\\scantokens = %q, want AB", got)
	}
}

// \protected is accepted as a definition prefix, and the definition is made.
func TestProtectedPrefix(t *testing.T) {
	if got := runExpr(t, `\protected\def\a{A}\message{\a}`); got != "A" {
		t.Errorf("\\protected\\def = %q, want A", got)
	}
}

// The engine identifies itself as extended, which is what a package tests before
// using the primitives above.
func TestETeXVersion(t *testing.T) {
	if got := runExpr(t, `\message{\the\eTeXversion\eTeXrevision}`); got != "2.6" {
		t.Errorf("e-TeX version = %q, want 2.6", got)
	}
}

// \ifx compares meanings, and every undefined control sequence has the same one.
// \ifx\foo\undefined is how nearly every package asks whether \foo exists, so
// two differently-named undefined sequences must compare equal.
func TestIfxUndefinedAreEqual(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\ifx\notthere\undefined Y\else N\fi`, "Y"},
		{`\ifx\alsonotthere\stillnotthere Y\else N\fi`, "Y"},
		{`\ifx\relax\undefined Y\else N\fi`, "N"}, // a defined one differs
		{`\ifx\undefined A Y\else N\fi`, "N"},     // a character is not undefined
		{`\ifx AAY\else N\fi`, "Y"},               // characters still compare as before
		{`\ifx ABY\else N\fi`, "N"},
	}
	for _, c := range cases {
		if got := runExpr(t, `\message{`+c.src+`}`); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// \the of a token register inside an \edef inserts the tokens WITHOUT expanding
// them again — TeX's rule, and the reason \the\toks is the way to carry a token
// list through an expansion intact. A package that stores a macro call in a
// register and rebuilds it this way loops forever if the tokens are re-expanded.
func TestTheToksNotReexpandedInEdef(t *testing.T) {
	got := runExpr(t, `\def\b{B}\toks0{\b}\edef\x{\the\toks0}\message{\meaning\x}`)
	if got != `macro:->\b ` {
		t.Errorf("\\the\\toks in \\edef = %q, want the unexpanded \\b", got)
	}
	got = runExpr(t, `\newtoks\t\def\b{B}\t{\b}\edef\x{\the\t}\message{\meaning\x}`)
	if got != `macro:->\b ` {
		t.Errorf("\\the of a named register in \\edef = %q", got)
	}
	// A self-referential macro — the sentinel idiom — survives the round trip.
	got = runExpr(t, `\def\stop{\stop}\toks0{\stop}\edef\x{\the\toks0}\message{ok}`)
	if got != "ok" {
		t.Errorf("a self-referential sentinel did not survive \\edef: %q", got)
	}
	// Outside an expansion the tokens are read normally, so the macro does run.
	got = runExpr(t, `\def\b{\message{ran}}\toks0{\b}\the\toks0`)
	if got != "ran" {
		t.Errorf("\\the\\toks in ordinary execution = %q, want the macro to run", got)
	}
}

// The conditionals and expansion primitives cope with a truncated source rather
// than failing: a document cut off mid-construct still compiles what it has.
func TestETeXPrimitivesAtEndOfInput(t *testing.T) {
	for _, src := range []string{
		`\ifdefined`,
		`\ifcsname`,
		`\ifcsname abc`,
		`\unless`,
		`\expanded`,
		`\expanded x`, // not a group
		`\unexpanded`,
		`\detokenize`,
		`\scantokens`,
	} {
		e := New()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
}

// A control sequence inside the name is read the way \csname already reads one —
// by the letters that spell it — so the two primitives agree about what name a
// given text builds. (TeX itself reports an error here; the engine keeps
// \csname's existing reading rather than inventing a second one.)
func TestIfcsnameAgreesWithCsname(t *testing.T) {
	if got := runExpr(t, `\message{\ifcsname \relax\endcsname Y\else N\fi}`); got != "Y" {
		t.Errorf(`= %q, want Y (the name "relax", as \csname would build it)`, got)
	}
}

// A \unless in front of something that is not a conditional leaves it to run.
func TestUnlessOfNonConditional(t *testing.T) {
	if got := runExpr(t, `\def\a{A}\unless\a\message{done}`); got != "done" {
		t.Errorf("= %q, want done", got)
	}
}

// \nullfont's inter-word space is empty, like the rest of its metrics.
func TestNullfontSpace(t *testing.T) {
	if s := (nullFont{}).spaceSP(); s != (glueSpec{}) {
		t.Errorf("\\nullfont space = %+v, want no glue", s)
	}
}
