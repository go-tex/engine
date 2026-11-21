// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A font switch MERGES into the current shape and series through the kernel's
// rule tables (nfss.go). Against tectonic on testdata/parity/nfssrules.tex, each
// line's face and what \f@series/\f@shape read inside it:
//
//	switches                    tectonic                      before
//	\bfseries\scshape           bx/sc  LMRoman10-Bold         LMRomanCaps10-Regular
//	\scshape\itshape            m/scsl LMRomanCaps10-Oblique  LMRoman10-Italic
//	\slshape\scshape            m/scsl LMRomanCaps10-Oblique  LMRomanCaps10-Regular
//	\bfseries\scshape\mdseries  m/sc   LMRomanCaps10-Regular  LMRomanCaps10-Regular
//	\scshape\itshape\upshape    m/sc   LMRomanCaps10-Regular  LMRoman10-Regular
//	\scshape\ulcshape           m/n    LMRoman10-Regular      (\ulcshape undefined)
//	\bfseries\scshape\itshape   bx/it  LMRoman10-BoldItalic   LMRoman10-Italic
//	\fontshape{it}\selectfont   m/it   LMRoman10-Italic       LMRoman10-Regular
//	\itshape\rmfamily           m/it   LMRoman10-Italic       LMRoman10-Regular
//	\itshape\bf                 bx/n   LMRoman10-Bold         LMRoman10-Bold
//
// After, every face is tectonic's and every line ends where its does (to 0.02bp).
// The last row is the control: the old one-letter switches do not merge.
func TestParityFontSwitchesMergeByTheKernelsRules(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "nfssrules.tex"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var faces []*byte
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox && directChar(b) {
				if ch, ok := firstGlyph(b); ok {
					faces = append(faces, faceKey(ch.face))
				}
				continue
			}
			walk(b.list)
		}
	}
	for _, p := range e.Pages() {
		walk([]node{p})
	}
	key := func(b []byte) *byte {
		if fam := opticalOf(b); len(fam) > 0 {
			return &fam[0].data[0]
		}
		return &b[0]
	}
	want := []struct {
		report string
		key    *byte
		face   string
	}{
		{"BXSC:bx/sc", key(lmRomanBold), "bold: bx/sc is undeclared, so bx/n"},
		{"SCIT:m/scsl", key(lmRomanCapsOblique), "caps oblique: sc+it is scit, else scsl"},
		{"SLSC:m/scsl", key(lmRomanCapsOblique), "caps oblique: sl+sc is scsl"},
		{"BSCM:m/sc", key(lmRomanCaps), "small caps: the shape stayed sc under bold"},
		{"SCITUP:m/sc", key(lmRomanCaps), "small caps: scit+up is sc"},
		{"SCULC:m/n", key(lmRomanRegular), "roman: sc+ulc is n"},
		{"BSCIT:bx/it", key(lmRomanBoldItalic), "bold italic: neither scit nor scsl in bx"},
		{"FSIT:m/it", key(lmRomanItalic), "italic: \\fontshape{it}\\selectfont"},
		{"ITRM:m/it", key(lmRomanItalic), "italic: \\rmfamily keeps the shape"},
		{"ITBF:bx/n", key(lmRomanBold), "bold: \\bf is \\normalfont\\bfseries"},
	}
	if len(faces) != len(want)+1 {
		t.Fatalf("%d lines, want %d plus the folio", len(faces), len(want))
	}
	said := strings.Fields(e.Diagnostics().Messages)
	if len(said) != len(want) {
		t.Fatalf("the document reported %d states, want %d: %q", len(said), len(want), said)
	}
	for i, w := range want {
		if said[i] != w.report {
			t.Errorf("line %d: \\f@series/\\f@shape read %s, tectonic %s", i+1, said[i], w.report)
		}
		if faces[i] != w.key {
			t.Errorf("line %d is not set in the face tectonic uses: %s", i+1, w.face)
		}
	}
}

// \f@series and \f@shape are restored when the group ends, as any macro is.
func TestNFSSStateIsScopedToTheGroup(t *testing.T) {
	src := []byte(`\documentclass{article}
\makeatletter
\begin{document}
{\bfseries\itshape x}\typeout{AFTER:\f@series/\f@shape}
\end{document}
`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(e.Diagnostics().Messages); got != "AFTER:m/n" {
		t.Errorf("after the group: %q, want AFTER:m/n", got)
	}
}

// A request with no rule stands: \fontshape{xx} leaves xx, and \fontseries{eb}
// from medium (no \series@m@eb) leaves eb. The rules, where there are some, give
// the requested pair or its fallback.
func TestNFSSMergeWithNoRuleKeepsTheRequest(t *testing.T) {
	if got := mergeShape("m", "n", "xx"); got != "xx" {
		t.Errorf("mergeShape with no rule = %q, want xx", got)
	}
	if got := mergeSeries("m", "eb", "n"); got != "eb" {
		t.Errorf("mergeSeries with no rule = %q, want eb", got)
	}
	// The first answer of a rule is used when declared, the second when it is not.
	if got := mergeSeries("m", "b", "n"); got != "b" {
		t.Errorf("mergeSeries(m, b, n) = %q, want b (b/n is declared)", got)
	}
	if got := mergeSeries("m", "b", "ui"); got != "b" {
		t.Errorf("mergeSeries(m, b, ui) = %q, want the request b: neither b/ui nor bx/ui is declared", got)
	}
	if got := mergeSeries("l", "b", "it"); got != "b" {
		t.Errorf("mergeSeries(l, b, it) = %q, want b", got)
	}
	// m/sl, bx/n: \series@m@bx is {bx}{b}, and bx/sl is declared.
	if got := mergeSeries("m", "bx", "sl"); got != "bx" {
		t.Errorf("mergeSeries(m, bx, sl) = %q, want bx", got)
	}
	// \series@m@b is {b}{bx}; with no b/sc the second answer is not declared either,
	// so the request is kept.
	if got := mergeSeries("m", "b", "sc"); got != "b" {
		t.Errorf("mergeSeries(m, b, sc) = %q, want b", got)
	}
}

// nfssGet reads a macro's text, and answers the default for a name that is
// undefined or not a macro. A guard that is gone dereferences nil: reported here as
// a failure, not a crash.
func TestNFSSGetAnswersTheDefaultForWhatIsNotAMacro(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	got := func(name string) (s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("nfssGet(%q) panicked: %v", name, r)
			}
		}()
		return e.nfssGet(name, "dflt")
	}
	if s := got("gotex@never@defined"); s != "dflt" {
		t.Errorf("undefined name: %q, want dflt", s)
	}
	if s := got("relax"); s != "dflt" {
		t.Errorf("a primitive: %q, want dflt", s)
	}
	e.nfssSet("f@series", "bx")
	if s := got("f@series"); s != "bx" {
		t.Errorf("a macro: %q, want bx", s)
	}
}

// \selectfont with no face to select and no base face leaves the current font
// alone: it does not install nil, which every later glyph would dereference.
func TestSelectfontWithNoFaceKeepsTheCurrentFont(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range nfssFaces {
		delete(e.eq, name)
	}
	e.baseFont = nil
	before := e.curFont
	if before == nil {
		t.Fatal("the engine has no current font to keep")
	}
	e.eq["gotex@selectfont"].prim(e)
	if e.curFont != before {
		t.Errorf("\\selectfont with no face set the font to %v", e.curFont)
	}
}
