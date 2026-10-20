// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ \end{X} IS \endX. LaTeX2e pairs an environment by running \X then \endX, so a
// class that composes one environment out of another writes the bare control
// sequences. sagej.cls:238-242:
//
//	\newbox\absbox
//	\def\abstract{\lrbox\absbox\minipage{\textwidth}\section*{Abstract}}
//	\def\endabstract{\endminipage\endlrbox}
//
// The minipage that gives that abstract its WIDTH is closed by \endminipage, never
// by \end{minipage}. Hunting only the braced spelling, the body scan ran past it and
// the minipage constrained nothing: the whole abstract came out as one horizontal
// list at its natural width, drawn off the side of the sheet (go-tex/engine#629, the
// two widest lines in the corpus at 6619pt and 6019pt on a 612pt page).
//
// The assertion is SYMMETRY: the same minipage written both ways must produce the
// same geometry. A threshold would pass on a witness whose text happens to fit.
func TestABareEndClosesTheEnvironmentItsBeginOpened(t *testing.T) {
	const text = `This text is long enough that a minipage of two hundred points must ` +
		`break it into several lines rather than one long run.`
	braced := `\sbox\gotexboxa{\begin{minipage}{200pt}` + text + `\end{minipage}}\usebox\gotexboxa`
	bare := `\sbox\gotexboxb{\minipage{200pt}` + text + `\endminipage}\usebox\gotexboxb`

	bw, hsize := widestBoxIn(t, `\newbox\gotexboxa`+braced)
	rw, _ := widestBoxIn(t, `\newbox\gotexboxb`+bare)
	if bw > hsize {
		t.Fatalf("la forme \\begin déborde déjà (%d > %d): le contrôle est cassé", bw, hsize)
	}
	if rw != bw {
		t.Errorf("forme nue: boîte la plus large %d, forme \\begin %d (justification %d) — "+
			"le \\endminipage nu n'a pas fermé l'environnement", rw, bw, hsize)
	}
}

// And the closing half of a composed environment belongs INSIDE the box: what
// \endabstract does before \endlrbox closes what \abstract opened after \lrbox.
func TestLrboxTakesTheClosersThatBelongInsideIt(t *testing.T) {
	src := `\makeatletter\newbox\gotexabs
\def\gotexabstract{\lrbox\gotexabs\minipage{200pt}}
\def\endgotexabstract{\endminipage\endlrbox}\makeatother
\begin{gotexabstract}This text is long enough that a minipage of two hundred points must break it into several lines rather than one long run.\end{gotexabstract}
\usebox\gotexabs`
	w, hsize := widestBoxIn(t, src)
	if w > hsize {
		t.Errorf("boîte la plus large %d pour une justification de %d — le \\minipage ouvert "+
			"par \\gotexabstract n'a jamais été fermé dans la boîte", w, hsize)
	}
}

// lrboxClosers takes only what precedes \endlrbox, and only when \endlrbox is there:
// that control sequence is the boundary between "inside the box" and "after it", and
// a closing macro that does not name it is not this shape.
func TestLrboxClosersStopAtTheBoundary(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.define("endwithbox", &meaning{kind: mMacro, body: []tok{
		csTok("endminipage"), csTok("endlrbox"), csTok("afterwards"),
	}}, false)
	e.define("endnobox", &meaning{kind: mMacro, body: []tok{csTok("endminipage")}}, false)

	got := e.lrboxClosers("withbox")
	if len(got) != 1 || !got[0].cs_ || got[0].cs != "endminipage" {
		t.Errorf("closers = %v, attendu le seul \\endminipage avant \\endlrbox", got)
	}
	if got := e.lrboxClosers("nobox"); got != nil {
		t.Errorf("une macro sans \\endlrbox a rendu %v: ses jetons tourneraient deux fois", got)
	}
	if got := e.lrboxClosers("nosuchenvironment"); got != nil {
		t.Errorf("une fermeture inexistante a rendu %v", got)
	}
}

// widestBoxIn returns the widest box the document built and the measure it had to
// fit in. The width IS the defect: the words are on the page either way, and what
// differs is whether they are inside the sheet.
func widestBoxIn(t *testing.T, body string) (widest, hsize int) {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// mvlText collects characters, not spaces (those are glue), so the probe is one
	// word: its absence means the witness never reached the page at all.
	if txt := mvlText(e.mvl); !strings.Contains(txt, "severallines") {
		t.Fatalf("le document ne porte pas le texte du témoin: %q", txt)
	}
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			if b, ok := n.(*boxNode); ok {
				if b.width > widest {
					widest = b.width
				}
				walk(b.list)
			}
		}
	}
	walk(e.mvl)
	return widest, e.hsize
}
