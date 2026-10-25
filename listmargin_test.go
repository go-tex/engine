// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \list{label}{settings} used to throw its second argument away, and that argument is
// where every \list-based environment states its margins. article.cls:405:
//
//	\newenvironment{quote}{\list{}{\rightmargin\leftmargin}\item\relax}{\endlist}
//
// so a quotation got NO indentation at all. Measured against tectonic, the quotation
// ran 133.77–477.50 where the reference sets it 158.67–452.58: 24.9pt on each side,
// which is \leftmargini = 2.5em at 10pt (article.cls:325, size10.clo:216).
// go-tex/engine#587.
//
// ⛔ The margin arrives as the LEADING GLUE of each line, because \list indents
// through \parshape (latex.ltx:11453) and \@trivlist has just set \leftskip to zero
// (11473). Asserting the line box's width measures nothing: every line is packed to
// \hsize either way.
func TestQuoteIsIndentedByItsListSettings(t *testing.T) {
	boxes := shapedLineBoxes(t, `\begin{quote}`+witnessProse(4)+`\end{quote}`)
	if len(boxes) < 2 {
		t.Fatalf("%d ligne(s), want au moins 2", len(boxes))
	}
	// ⛔ The line is still packed to the FULL \hsize. \linewidth was \let to \hsize,
	// and \list decrements it by both margins (latex.ltx:11450-11451), so with the
	// \let in place that arithmetic shrank \hsize itself and every line of the
	// quotation came out packed to 295pt instead of 345pt. latex.ltx:11414 makes
	// \linewidth its own \newdimen, and this assertion is the only thing that holds
	// the two apart — mutating the register away leaves every other test here green.
	for i, b := range boxes {
		if want := 345 * unity; b.width != want {
			t.Errorf("ligne %d empaquetée à %.2fpt, want la pleine mesure %.2fpt — "+
				"\\list a rétréci \\hsize", i, float64(b.width)/unity, float64(want)/unity)
		}
	}
	for i, ln := range linesOf(boxes) {
		g, ok := ln[0].(glueNode)
		if !ok {
			t.Errorf("ligne %d ne commence pas par une marge: %T", i, ln[0])
			continue
		}
		// 2.5em at 10pt. Exact rather than "greater than zero": a quotation indented
		// by the wrong amount is still wrong, and the reference figure is known.
		if want := 25 * unity; g.spec.width != want {
			t.Errorf("ligne %d: marge %.2fpt, want %.2fpt",
				i, float64(g.spec.width)/unity, float64(want)/unity)
		}
	}
}

// ⚠ And the text AROUND the list keeps its own measure. \linewidth was \let to
// \hsize, so \list's `\advance\linewidth -\leftmargin` shrank \hsize itself and the
// enclosing text came out narrowed by every list it contained (latex.ltx:11414 makes
// \linewidth its own \newdimen).
func TestTextAfterAListKeepsTheFullMeasure(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\begin{quote}`+witnessProse(2)+`\end{quote}`+
		`\noindent GOTEXAPRES `+witnessProse(3)+`\par`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	lines := linesContaining(e, "GOTEXAPRES")
	if len(lines) == 0 {
		t.Fatal("le paragraphe d'après n'est pas sur la page")
	}
	if g, ok := lines[0][0].(glueNode); ok && g.spec.width > 0 {
		t.Errorf("le texte d'après porte une marge de %.2fpt — la liste a rétréci la mesure",
			float64(g.spec.width)/unity)
	}
	if got, want := e.hsize, 345*unity; got != want {
		t.Errorf("\\hsize = %.2fpt après la liste, want %.2fpt", float64(got)/unity, float64(want)/unity)
	}
}

// The per-depth defaults run BEFORE the settings argument (latex.ltx:11442 then
// 11446), so the argument can override them — which is the whole reason quote says
// `\rightmargin\leftmargin` and gets a symmetric indent.
func TestListSettingsOverrideThePerDepthDefaults(t *testing.T) {
	lines := shapedLines(t, `\makeatletter`+
		`\list{}{\leftmargin=50pt \rightmargin=0pt}\item\relax `+witnessProse(3)+`\endlist`+
		`\makeatother`)
	if len(lines) == 0 {
		t.Fatal("le témoin n'est pas sur la page")
	}
	g, ok := lines[0][0].(glueNode)
	if !ok {
		t.Fatalf("la première ligne ne commence pas par une marge: %T", lines[0][0])
	}
	if want := 50 * unity; g.spec.width != want {
		t.Errorf("marge %.2fpt, want 50pt — l'argument de réglages n'a pas surchargé \\@listi",
			float64(g.spec.width)/unity)
	}
}
