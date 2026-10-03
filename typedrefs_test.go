// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// newTypedRefEngine builds a LaTeX engine with the mock font, ready to Run source.
func newTypedRefEngine() *Engine {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	return e
}

// Running each numbered construct records its reference type (and, for the ones
// with a title, its name) under the following \label — the data \autoref / \cref /
// \nameref read back.
func TestRefMetaRecorded(t *testing.T) {
	e := newTypedRefEngine()
	src := `\hsize=300pt
\part{Foundations}\label{p}
\section{Intro}\label{s}
\subsection{Details}\label{ss}
\begin{equation} x \label{eq}\end{equation}
\begin{figure}\caption{A plot}\label{fig}\end{figure}
\begin{table}\caption{Some data}\label{tab}\end{table}
\newtheorem{theorem}{Theorem}
\begin{theorem}\label{thm} Body.\end{theorem}
\begin{enumerate}\item\label{it}\end{enumerate}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	wantType := map[string]string{
		"p": "part", "s": "section", "ss": "subsection", "eq": "equation",
		"fig": "figure", "tab": "table", "thm": "theorem", "it": "item",
	}
	for k, want := range wantType {
		if got := e.refTypes[k]; got != want {
			t.Errorf("refType[%q] = %q, want %q", k, got, want)
		}
	}
	wantName := map[string]string{
		"p": "Foundations", "s": "Intro", "ss": "Details",
		"fig": "A plot", "tab": "Some data",
	}
	for k, want := range wantName {
		if got := e.refNames[k]; got != want {
			t.Errorf("refName[%q] = %q, want %q", k, got, want)
		}
	}
	// Constructs without a title record an empty name.
	for _, k := range []string{"eq", "thm", "it"} {
		if got := e.refNames[k]; got != "" {
			t.Errorf("refName[%q] = %q, want empty", k, got)
		}
	}
}

// autorefText prints "<Type> <number>", parenthesising equation numbers, and "??"
// for an unknown key.
func TestAutorefText(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"s": "1", "ss": "1.1", "eq": "2", "fig": "3", "tab": "4", "thm": "5", "p": "II", "it": "6", "x": "7"}
	e.refTypes = map[string]string{"s": "section", "ss": "subsection", "eq": "equation", "fig": "figure", "tab": "table", "thm": "theorem", "p": "part", "it": "item"}
	cases := map[string]string{
		"s": "Section 1", "ss": "Subsection 1.1", "eq": "Equation (2)",
		"fig": "Figure 3", "tab": "Table 4", "thm": "Theorem 5",
		"p": "Part II", "it": "item 6",
		"x":       "7",  // known number, untyped: bare number
		"missing": "??", // unknown key
	}
	for key, want := range cases {
		if got := e.autorefText(key); got != want {
			t.Errorf("autorefText(%q) = %q, want %q", key, got, want)
		}
	}
}

// crefText / crefOne print cleveref's lowercase (\cref) and capitalised (\Cref)
// abbreviations, parenthesising equation numbers.
// The names are cleveref's own, read from cleveref.sty:3962-4055 — NOT this
// table's previous guess, which abbreviated both cases alike ("Fig.", "Tab.",
// "Thm.") and had no entry for a section at all. cleveref abbreviates the
// LOWERCASE equation and figure only; everything capitalised is spelt out, and a
// subsection is named "Section" like a section.
func TestCrefSingle(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"s": "1", "ss": "1.1", "eq": "2", "fig": "3", "tab": "4", "thm": "5", "p": "II", "it": "6", "x": "7"}
	e.refTypes = map[string]string{"s": "section", "ss": "subsection", "eq": "equation", "fig": "figure", "tab": "table", "thm": "theorem", "p": "part", "it": "item"}
	lower := map[string]string{
		"s": "section 1", "ss": "section 1.1", "eq": "eq. (2)",
		"fig": "fig. 3", "tab": "table 4", "thm": "theorem 5", "p": "part II", "it": "item 6",
		"x": "7", "missing": "??",
	}
	upper := map[string]string{
		"s": "Section 1", "ss": "Section 1.1", "eq": "Equation (2)",
		"fig": "Figure 3", "tab": "Table 4", "thm": "Theorem 5", "p": "Part II", "it": "Item 6",
		"x": "7", "missing": "??",
	}
	for key, want := range lower {
		if got := e.crefText([]string{key}, false); got != want {
			t.Errorf("\\cref{%s} = %q, want %q", key, got, want)
		}
	}
	for key, want := range upper {
		if got := e.crefText([]string{key}, true); got != want {
			t.Errorf("\\Cref{%s} = %q, want %q", key, got, want)
		}
	}
}

// A multi-key \cref names the type once (plural) and joins the numbers "a and b" /
// "a, b and c"; equations parenthesise each number.
func TestCrefMultiKey(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"a": "1", "b": "2", "c": "3", "e1": "1", "e2": "2", "u": "9"}
	e.refTypes = map[string]string{"a": "section", "b": "section", "c": "section", "e1": "equation", "e2": "equation"}
	cases := []struct {
		keys    []string
		capital bool
		want    string
	}{
		{[]string{"a", "b"}, false, "sections 1 and 2"},
		{[]string{"a", "b"}, true, "Sections 1 and 2"},
		{[]string{"a", "b", "c"}, false, "sections 1, 2 and 3"},
		{[]string{"e1", "e2"}, false, "eqs. (1) and (2)"},
		{[]string{"e1", "e2"}, true, "Equations (1) and (2)"},
		{[]string{"u", "u"}, false, "9 and 9"}, // untyped: numbers only
	}
	for _, c := range cases {
		if got := e.crefText(c.keys, c.capital); got != c.want {
			t.Errorf("crefText(%v, %v) = %q, want %q", c.keys, c.capital, got, c.want)
		}
	}
	// Empty key list resolves to "??".
	if got := e.crefText(nil, false); got != "??" {
		t.Errorf("crefText(nil) = %q, want %q", got, "??")
	}
}

// \nameref prints the recorded title; an unknown key or a nameless target yields "??".
func TestNamerefText(t *testing.T) {
	e := newTypedRefEngine()
	e.refNames = map[string]string{"s": "Introduction", "eq": ""}
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\nameref{s}|\nameref{eq}|\nameref{missing}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "Introduction|??|??" {
		t.Errorf("nameref typeset %q, want %q", got, "Introduction|??|??")
	}
}

// End-to-end: the typed references typeset the expected characters into the main
// vertical list (spaces are glue, so they do not appear among the charNodes).
func TestTypedRefsTypeset(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"s": "1", "eq": "2", "thm": "3"}
	e.refTypes = map[string]string{"s": "section", "eq": "equation", "thm": "theorem"}
	e.refNames = map[string]string{"s": "Intro"}
	if _, err := e.Run(`\noindent\autoref{s}|\cref{eq}|\Cref{thm}|\nameref{s}`); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	// "Section 1" | "eq. (2)" | "Thm. 3" | "Intro" — inter-word spaces are glue.
	if got, want := b.String(), "Section1|eq.(2)|Theorem3|Intro"; got != want {
		t.Errorf("typeset %q, want %q", got, want)
	}
}

// A forward \autoref (before its \label) resolves on the second pass, exactly as
// the two-pass compile carries labels/refTypes/refNames from the aux run.
func TestForwardTypedRefTwoPass(t *testing.T) {
	src := `\hsize=300pt\noindent\autoref{s} then \nameref{s}.\section{Preliminaries}\label{s}`
	aux := newTypedRefEngine()
	if _, err := aux.Run(src); err != nil {
		t.Fatal(err)
	}
	if aux.refTypes["s"] != "section" || aux.refNames["s"] != "Preliminaries" {
		t.Fatalf("aux meta = %q/%q, want section/Preliminaries", aux.refTypes["s"], aux.refNames["s"])
	}
	// Second pass with the carried maps: the forward \autoref/\nameref resolve.
	e := newTypedRefEngine()
	e.labels = aux.labels
	e.refTypes = aux.refTypes
	e.refNames = aux.refNames
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	// \autoref{s}→"Section 1", \nameref{s}→"Preliminaries", then the heading "1Preliminaries".
	if got, want := b.String(), "Section1thenPreliminaries.1Preliminaries"; got != want {
		t.Errorf("second pass typeset %q, want %q", got, want)
	}
}

// The reference primitives must not panic on a missing brace or an empty key; each
// then yields "??" (or the untyped fallback), as \ref does.
func TestTypedRefErrorBranches(t *testing.T) {
	e := newTypedRefEngine()
	// Missing brace group: readBraceName returns "" and the next token is left alone.
	if _, err := e.Run(`\noindent\autoref x\cref y\Cref z\nameref w`); err != nil {
		t.Fatal(err)
	}
	// Empty key group.
	e2 := newTypedRefEngine()
	if _, err := e2.Run(`\noindent\autoref{}\cref{}\Cref{}\nameref{}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e2.mvl); got != "????????" {
		t.Errorf("empty-key typeset %q, want %q", got, "????????")
	}
	// recordRefMeta lazily allocates its maps.
	e3 := newTypedRefEngine()
	e3.recordRefMeta("k")
	if e3.refTypes == nil || e3.refNames == nil {
		t.Error("recordRefMeta did not allocate maps")
	}
}

// joinAnd formats 0, 1, 2 and 3+ elements without an Oxford comma.
func TestJoinAnd(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"1"}, "1"},
		{[]string{"1", "2"}, "1 and 2"},
		{[]string{"1", "2", "3"}, "1, 2 and 3"},
		{[]string{"1", "2", "3", "4"}, "1, 2, 3 and 4"},
	}
	for _, c := range cases {
		if got := joinAnd(c.in); got != c.want {
			t.Errorf("joinAnd(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A document's own \crefname / \Crefname override cleveref's defaults, and each
// names ONE case while cross-filling the other if the document has not. The
// expectations are real cleveref's, from \@crefname (cleveref.sty:1296-1320):
//
//   - \Crefname alone lowercases for \cref — which is why three of the four
//     corpus papers that name their types give only the capitalised form;
//   - \crefname alone UPPERCASES the whole word for \Cref ("EQ."), a cleveref
//     wart kept here because the reference these runs are measured against is
//     cleveref itself;
//   - whichever came first wins: the second call sees the case already set and
//     leaves it alone;
//   - the parentheses around an equation number come from \creflabelformat, not
//     from the name, so they survive a renaming.
func TestCrefnameOverridesDefaults(t *testing.T) {
	e := newTypedRefEngine()
	src := `\hsize=300pt
\Crefname{figure}{Fig.}{Figs.}
\crefname{section}{Sec.}{Secs.}
\Crefname{section}{Section}{Sections}
\crefname{equation}{eq.}{eqs.}
\Crefname{assumption}{Assumption}{Assumptions}
\section{Intro}\label{s}
\begin{figure}\caption{A plot}\label{fig}\end{figure}
\begin{equation} x \label{eq}\end{equation}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		key     string
		capital bool
		want    string
	}{
		{"fig", true, "Fig. 1"},  // named
		{"fig", false, "fig. 1"}, // cross-filled, lowercased
		{"s", true, "Section 1"}, // named by the second call
		{"s", false, "Sec. 1"},   // named by the first, which the second leaves
		{"eq", false, "eq. (1)"}, // named, parentheses kept
		{"eq", true, "EQ. (1)"},  // cross-filled by \MakeUppercase, parens kept
	}
	for _, c := range cases {
		if got := e.crefOne(c.key, c.capital); got != c.want {
			t.Errorf("crefOne(%q, capital=%v) = %q, want %q", c.key, c.capital, got, c.want)
		}
	}
	// A type the document names but never numbers is still named, and the plural
	// comes from the same pair.
	if got, want := e.crefNames["assumption"].lowerP, "assumptions"; got != want {
		t.Errorf("assumption plural (lower) = %q, want %q", got, want)
	}
	// And naming one type leaves every other default alone.
	if got, want := e.crefOne("fig", true), "Fig. 1"; got != want {
		t.Errorf("after naming: %q, want %q", got, want)
	}
}

// A named type whose other case the document never set prints the BARE number,
// which is what cleveref does for a type it has no name for — the alternative,
// falling back to the built-in default, would print a name the document replaced.
func TestCrefnameUnnamedCaseIsNumberOnly(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"a": "7"}
	e.refTypes = map[string]string{"a": "widget"}
	e.crefNames = map[string]crefForm{"widget": {lower: "widget", lowerP: "widgets"}}
	if got, want := e.crefOne("a", false), "widget 7"; got != want {
		t.Errorf("lower = %q, want %q", got, want)
	}
	if got, want := e.crefOne("a", true), "7"; got != want {
		t.Errorf("upper (unnamed) = %q, want %q", got, want)
	}
	e.labels["b"] = "8"
	e.refTypes["b"] = "widget"
	if got, want := e.crefText([]string{"a", "b"}, true), "7 and 8"; got != want {
		t.Errorf("plural (unnamed) = %q, want %q", got, want)
	}
}

// cleveref ships four default naming sets and selects between them with two
// package options (cleveref.sty:3889-3946). Its own defaults are abbrev ON,
// capitalise OFF (cleveref.sty:3814-3831), so only [capitalise] and [noabbrev]
// change anything — and only equation and figure are abbreviated at all, which
// is why noabbrev shows on those two alone while capitalise moves every type.
//
// Nine corpus papers load cleveref with options; four of them use \cref, 186 of
// the corpus's 905 uses. Measured by channel against tectonic, all four papers
// move toward the reference and none away, several landing on its exact count
// ("Fig." 3 → 65 against a reference 65 in 2201.02101).
func TestCrefPackageOptions(t *testing.T) {
	for _, c := range []struct {
		opts string
		want []string // \cref of section, figure, equation, then \Cref of each
	}{
		{"", []string{"section 1", "fig. 1", "eq. (1)", "Section 1", "Figure 1", "Equation (1)"}},
		{"[capitalise]", []string{"Section 1", "Fig. 1", "Eq. (1)", "Section 1", "Figure 1", "Equation (1)"}},
		{"[capitalize]", []string{"Section 1", "Fig. 1", "Eq. (1)", "Section 1", "Figure 1", "Equation (1)"}},
		{"[noabbrev]", []string{"section 1", "figure 1", "equation (1)", "Section 1", "Figure 1", "Equation (1)"}},
		{"[capitalize,noabbrev]", []string{"Section 1", "Figure 1", "Equation (1)", "Section 1", "Figure 1", "Equation (1)"}},
	} {
		e := newTypedRefEngine()
		src := `\hsize=300pt
\usepackage` + c.opts + `{cleveref}
\section{Intro}\label{s}
\begin{figure}\caption{A plot}\label{fig}\end{figure}
\begin{equation} x \label{eq}\end{equation}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%s: %v", c.opts, err)
		}
		got := []string{
			e.crefOne("s", false), e.crefOne("fig", false), e.crefOne("eq", false),
			e.crefOne("s", true), e.crefOne("fig", true), e.crefOne("eq", true),
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("cleveref%s: form %d = %q, want %q", c.opts, i, got[i], c.want[i])
			}
		}
	}
}

// An option changes the DEFAULTS; a document's own \crefname still wins, because
// cleveref keeps the two in different macros — the option's names are @preamble
// ones, which lose to anything the document set (cleveref.sty:1296-1320).
func TestCrefnameBeatsPackageOption(t *testing.T) {
	e := newTypedRefEngine()
	src := `\hsize=300pt
\usepackage[capitalise]{cleveref}
\crefname{figure}{fig.}{figs.}
\section{Intro}\label{s}
\begin{figure}\caption{A plot}\label{fig}\end{figure}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.crefOne("fig", false), "fig. 1"; got != want {
		t.Errorf("named figure = %q, want %q", got, want)
	}
	if got, want := e.crefOne("s", false), "Section 1"; got != want {
		t.Errorf("unnamed section under [capitalise] = %q, want %q", got, want)
	}
}

// A theorem environment's reference type is the ENVIRONMENT's name, and its
// cleveref name comes from the HEADING it was declared with — cleveref's own
// rule, applied by patching all three of LaTeX's \newtheorem internals
// (cleveref.sty:105-173). Both halves matter and they fail differently:
//
//   - the type was hard-coded "theorem" in \@begintheorem, so every theorem-like
//     environment shared one type and a document's \crefname{theo}{thm.} could
//     not reach it. \refstepcounter cannot supply it either, because
//     \newtheorem{theo}[definition]{Theorem} steps the DEFINITION counter.
//   - the name follows the heading, not the environment: \newtheorem{lem}{Lemma}
//     is "lemma"/"Lemma", never "lem".
//
// The expectations are tectonic's, from a witness run against it: "theorem 1 |
// Theorem 1 | lemma 1 | Lemma 1 | proposition 1.1 | Proposition 1.1", and with a
// shared counter "Thm. 2 | thm. 2 | Definition 1".
func TestTheoremTypeAndName(t *testing.T) {
	e := newTypedRefEngine()
	src := `\hsize=300pt
\usepackage{cleveref}
\newtheorem{definition}{Definition}
\newtheorem{lem}{Lemma}
\newtheorem{theo}[definition]{Theorem}
\begin{definition}\label{d}X\end{definition}
\begin{lem}\label{l}Y\end{lem}
\begin{theo}\label{t}Z\end{theo}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	// Each label carries its own environment's name, not the counter's and not
	// a single shared "theorem".
	wantType := map[string]string{"d": "definition", "l": "lem", "t": "theo"}
	for k, want := range wantType {
		if got := e.refTypes[k]; got != want {
			t.Errorf("refType[%q] = %q, want %q", k, got, want)
		}
	}
	want := map[string]string{
		"d": "definition 1", "l": "lemma 1", "t": "theorem 2",
	}
	wantUp := map[string]string{
		"d": "Definition 1", "l": "Lemma 1", "t": "Theorem 2",
	}
	for k, w := range want {
		if got := e.crefOne(k, false); got != w {
			t.Errorf("cref %q = %q, want %q", k, got, w)
		}
		if got := e.crefOne(k, true); got != wantUp[k] {
			t.Errorf("Cref %q = %q, want %q", k, got, wantUp[k])
		}
	}
}

// A document's \crefname beats the heading, whichever order the two are written
// in — the heading's name sits a tier below, exactly as cleveref's @preamble
// macros do. 2405.18549 depends on this: it declares twelve theorem kinds and
// then renames nine of them to abbreviations, and its reference prints "Prop."
// 16 times where the heading alone would give "Proposition".
func TestCrefnameBeatsTheoremHeading(t *testing.T) {
	for _, order := range []string{"nameFirst", "theoremFirst"} {
		e := newTypedRefEngine()
		decl := `\newtheorem{prop}{Proposition}`
		name := `\Crefname{prop}{Prop.}{Prop.}` + "\n" + `\crefname{prop}{prop.}{prop.}`
		pre := decl + "\n" + name
		if order == "nameFirst" {
			pre = name + "\n" + decl
		}
		src := `\hsize=300pt
\usepackage{cleveref}
` + pre + `
\begin{prop}\label{p}X\end{prop}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%s: %v", order, err)
		}
		if got, want := e.crefOne("p", true), "Prop. 1"; got != want {
			t.Errorf("%s: Cref = %q, want %q", order, got, want)
		}
		if got, want := e.crefOne("p", false), "prop. 1"; got != want {
			t.Errorf("%s: cref = %q, want %q", order, got, want)
		}
	}
}

// A heading names the singular only, because cleveref names the singular only:
// tectonic renders \cref{c:a,c:b} on a \newtheorem{cor}{Corollary} as "?? 1?? 2".
// crefText prints the bare numbers instead — the one place here that does not
// follow the reference, since reproducing "??" would be faithful to a rendering
// nobody wants.
func TestTheoremHeadingHasNoPlural(t *testing.T) {
	e := newTypedRefEngine()
	src := `\hsize=300pt
\usepackage{cleveref}
\newtheorem{cor}{Corollary}
\begin{cor}\label{a}X\end{cor}
\begin{cor}\label{b}Y\end{cor}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.crefText([]string{"a", "b"}, false), "1 and 2"; got != want {
		t.Errorf("plural = %q, want %q", got, want)
	}
}

// \crefformat / \Crefformat replace the whole rendering, NAME included: the
// template's #1 is the number and #2/#3 are cleveref's hyperlink wrappers, empty
// here. One corpus paper in 154 uses them, twice, and it is the whole of that
// paper's remaining deficit — 2406.01525 writes \crefformat{section}{#2\S#1#3},
// so its reference prints "§3" where the engine printed "Section 3", 52 times.
//
// Every expectation here was read off tectonic, including the two that are not
// obvious:
//
//   - a one-sided \crefformat cross-fills the other case by shifting the FIRST
//     LETTER only, so {fig.~#2#1#3} makes \Cref print "Fig. 1" — not "FIG. 1"
//     from a whole-word \MakeUppercase, and not the default "Figure 1";
//   - a subsection with no format of its own follows the SECTION's, the same
//     fallback that makes cleveref's default name for a subsection "section".
func TestCrefformat(t *testing.T) {
	e := newTypedRefEngine()
	src := `\hsize=300pt
\usepackage{cleveref}
\crefformat{section}{#2\S#1#3}
\Crefformat{section}{#2\S#1#3}
\crefformat{figure}{fig.~#2#1#3}
\section{Un}\label{s}
\subsection{Deux}\label{ss}
\begin{figure}\caption{f}\label{fig}\end{figure}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		key     string
		capital bool
		want    string
	}{
		{"s", false, `\S 1`},
		{"s", true, `\S 1`},
		{"ss", false, `\S 1.1`}, // falls back to the section's format
		{"fig", false, `fig.~1`},
		{"fig", true, `Fig.~1`}, // cross-filled, first letter only
	} {
		body, ok := e.crefFormatted(c.key, c.capital)
		if !ok {
			t.Errorf("crefFormatted(%q, %v): no format", c.key, c.capital)
			continue
		}
		if got := e.toksToString(body); got != c.want {
			t.Errorf("crefFormatted(%q, %v) = %q, want %q", c.key, c.capital, got, c.want)
		}
	}
	// A type with no format anywhere in its fallback chain is left to the naming
	// path, which is every type in all but one corpus paper.
	if _, ok := e.crefFormatted("nosuch", false); ok {
		t.Error("an unknown key reported a format")
	}
}

// A template's "#1" arrives as TWO tokens from a braced group — a catParam '#'
// and the digit — where a \def body folds them into one. The first version of
// the substitution looked only for the folded form, found nothing, and printed
// "#2§#1#3" on the page; this pins both shapes.
func TestCrefParamAt(t *testing.T) {
	two := []tok{chTok('#', catParam), chTok('2', catOther)}
	if n, w := crefParamAt(two, 0); n != 2 || w != 2 {
		t.Errorf("unfolded #2 = (%d, %d), want (2, 2)", n, w)
	}
	folded := []tok{{cat: catParam, ch: '1'}}
	if n, w := crefParamAt(folded, 0); n != 1 || w != 1 {
		t.Errorf("folded #1 = (%d, %d), want (1, 1)", n, w)
	}
	if n, _ := crefParamAt([]tok{chTok('x', catLetter)}, 0); n != 0 {
		t.Errorf("a letter reported parameter %d", n)
	}
	if n, _ := crefParamAt([]tok{chTok('#', catParam)}, 0); n != 0 {
		t.Errorf("a trailing # reported parameter %d", n)
	}
}
