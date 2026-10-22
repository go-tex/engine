// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A VBOX's contents are VERTICAL material — finished lines — and they cannot join a
// horizontal list. place() routes to the paragraph list while a paragraph is open, so
// \unvbox inside one handed the lines to the line breaker, which glued them end to end
// and re-broke the whole thing as a single paragraph.
//
// acmart writes exactly that, twice, in its title machinery:
//
//	\noindent\unvbox\@tempboxa                       % acmart.cls:2303
//	\vbox{\noindent\unvbox\mktitle@bx\par\medskip…}  % acmart.cls:2652
//
// \noindent opens a paragraph, so both took that path: on 2405.20319 the title and
// five author blocks came out as ONE line ending at x=3356pt on a 612pt sheet
// (go-tex/engine#629).
//
// ⛔ The assertion is the WIDTH, measured against the measure the box was built at.
// The words are on the page either way — the page count cannot see this, the ink count
// cannot, and the text layer carries them. What differs is whether they are inside the
// sheet.
func TestUnvboxInsideAParagraphKeepsTheBoxLinesBroken(t *testing.T) {
	const measure = 120 // pt, the \hsize the box's lines were broken at
	body := `\newbox\gbx\setbox\gbx=\vbox{\hsize=120pt\noindent ` +
		strings.Repeat("severallines of ordinary prose that must be broken somewhere ", 8) +
		`}AVANT\par\noindent\unvbox\gbx`

	e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// ⚠ The probe must prove the witness reached the page at all: a box that was
	// silently dropped would pass a width assertion trivially.
	if txt := mvlText(e.mvl); !strings.Contains(txt, "severallines") {
		t.Fatalf("le document ne porte pas le texte du témoin: %q", txt)
	}
	// ⚠ Only the boxes that CARRY the witness. Two near misses before this line was
	// right: a walk over every box reports the vbox that holds the lines, packed to
	// the enclosing measure, and a walk over every hbox reports the document's own
	// "AVANT" line, packed to \hsize=345pt. Both numbers are identical before and
	// after the fix — a test that can neither fail for the right reason nor pass for
	// it. The claim is about the box's lines, so the measurement has to be too.
	var widest int
	var lines int
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox && strings.Contains(mvlText([]node{b}), "severallines") {
				lines++
				if b.width > widest {
					widest = b.width
				}
			}
			walk(b.list)
		}
	}
	walk(e.mvl)
	if lines == 0 {
		t.Fatalf("aucune ligne ne porte le témoin:\n%s", dumpBoxes(e.mvl, 0))
	}
	if want := measure * unity; widest > want {
		t.Errorf("la ligne la plus large portant le témoin fait %.2fpt, la mesure de la "+
			"boîte %dpt — les lignes ont rejoint le paragraphe\n%s",
			float64(widest)/unity, measure, dumpBoxes(e.mvl, 0))
	}
}

// And \unhbox is untouched: an HBOX's contents ARE horizontal material, so unboxing
// one inside a paragraph is the normal, correct thing and must keep working.
func TestUnhboxInsideAParagraphStillJoinsTheLine(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\newbox\ghx\setbox\ghx=\hbox{MILIEU}AVANT \unhbox\ghx APRES`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := mvlText(e.mvl)
	if !strings.Contains(got, "AVANTMILIEUAPRES") {
		t.Errorf("= %q, want les trois morceaux sur la même ligne", got)
	}
}
