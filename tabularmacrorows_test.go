// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A tabular's body scanner reads RAW tokens, so a macro standing for several rows
// arrived as one cell and every row it was meant to make collapsed into it.
//
// That is not a corner case. article.cls sets the author block as
//
//	\begin{tabular}[t]{c}%
//	  \@author
//	\end{tabular}
//
// (article.cls:246-248) and \@author is exactly such a macro: authblk appends
// "\\[\affilsep]" between the authors and each affiliation (authblk.sty:154-155,
// 167) and this engine's own \affil does the same. 49 of the 154 corpus papers drew
// a line past the right edge of their own page because of it — up to x=6619pt on a
// 612pt sheet, with author names, affiliations and e-mails DRAWN OUTSIDE THE PAPER
// (go-tex/engine#629).
//
// The control is the literal spelling beside it: both must produce the same rows,
// because they are the same table.
func TestAMacroStandingForRowsMakesRows(t *testing.T) {
	const body = `\begin{tabular}[t]{c}
%s
\end{tabular}`
	literal := strings.Replace(body, "%s", `ALPHA\\BRAVO\\CHARLIE`, 1)
	viaMacro := `\def\gotexrows{ALPHA\\BRAVO\\CHARLIE}` + strings.Replace(body, "%s", `\gotexrows`, 1)

	// ⛔ The assertion is SYMMETRY, not a threshold. A first version asserted that
	// the macro's widest box fits the measure — and passed with the fix disabled,
	// because three short words collapsed into one cell still fit. Two spellings of
	// the SAME table must produce the same geometry, whatever the words are; that
	// holds for a witness of any size and cannot be satisfied by a narrow one.
	litW, hsize := widestBox(t, literal)
	macW, _ := widestBox(t, viaMacro)
	if litW > hsize {
		t.Fatalf("le témoin littéral déborde déjà (%d > %d): le contrôle est cassé", litW, hsize)
	}
	if macW != litW {
		t.Errorf("à travers une macro la boîte la plus large fait %d, en littéral %d "+
			"(justification %d) — les rangées sont écrasées en une seule cellule",
			macW, litW, hsize)
	}
}

// ⛔ Narrow, like hidesTabularEnd beside it. A macro whose \\ sits inside a braced
// group is NOT a row macro: expanding it would change what a cell contains.
func TestHidesRowSeparatorIsNarrow(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	rowSep := csTok(`\`)
	def := func(name string, body ...tok) { e.define(name, &meaning{kind: mMacro, body: body}, false) }
	def("gotexrows", chTok('a', catLetter), rowSep, chTok('b', catLetter))
	def("gotexbraced", chTok('{', catBegin), chTok('a', catLetter), rowSep, chTok('b', catLetter), chTok('}', catEnd))
	def("gotexplain", chTok('a', catLetter), chTok('b', catLetter))
	e.define("gotexparam", &meaning{kind: mMacro, params: []tok{chTok('#', catParam)}, body: []tok{rowSep}}, false)

	for _, c := range []struct {
		name string
		cs   string
		want bool
	}{
		{"une macro de rangées", "gotexrows", true},
		{"un \\\\ dans un groupe", "gotexbraced", false},
		{"une macro de contenu", "gotexplain", false},
		{"une macro AVEC paramètre", "gotexparam", false},
		{"une séquence indéfinie", "gotexnosuchthing", false},
	} {
		if got := e.hidesRowSeparator(csTok(c.cs)); got != c.want {
			t.Errorf("%s: hidesRowSeparator(\\%s) = %v, attendu %v", c.name, c.cs, got, c.want)
		}
	}
}

// widestBox returns the widest box the document built, and the measure it had to
// fit in.
//
// ⛔ The assertion is on WIDTH, not on a row count, because the width IS the defect:
// all three words are on the page either way — the text layer carries them, the ink
// count counts them — and what separates the two cases is whether they are on three
// lines or on one line that runs off the sheet. Counting cells would ask the builder
// what it thinks it made; measuring the box asks what reaches the page.
func widestBox(t *testing.T, body string) (widest, hsize int) {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	for _, w := range []string{"ALPHA", "BRAVO", "CHARLIE"} {
		if !strings.Contains(txt, w) {
			t.Fatalf("le document ne porte pas %q: %q", w, txt)
		}
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
