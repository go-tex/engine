package engine

import (
	"strings"
	"testing"
)

// \textwidth and \columnwidth are two different registers. \textwidth is the text
// BLOCK — both columns and the gutter in two-column mode — and \columnwidth is one
// column; \columnwidth is \let to \hsize here, which is the paragraph measure.
//
// A two-column class states both, and states them in that order:
//
//	\textwidth 170.5mm      % oupau.cls:68
//	\columnwidth 83.25mm    % oupau.cls:70
//
// With \textwidth writing only \hsize the second line overwrote the first, and
// \the\textwidth read back the COLUMN. The expected values are tectonic's.
func TestTextwidthIsNotClobberedByColumnwidth(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\textwidth=300pt \columnwidth=150pt \message{[\the\textwidth][\the\columnwidth]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[300.0pt][150.0pt]" {
		t.Errorf("= %s, want [300.0pt][150.0pt]", got)
	}
}

// \begin{document} recomputes the measure from \textwidth — the class's own
// \columnwidth is discarded there (latex.ltx:6682-6688):
//
//	\columnwidth\textwidth
//	\if@twocolumn \advance\columnwidth -\columnsep \divide\columnwidth\tw@ \fi
//	\hsize\columnwidth \linewidth\hsize
//
// Without it a stray \columnwidth stayed the paragraph measure for the whole
// document: oupau.cls set its body 236.9pt wide against the reference's 483.3pt,
// which is twice the lines and seven pages too many on a thirteen-page paper.
func TestDocumentRecomputesTheMeasureFromTextwidth(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\textwidth=400pt \columnwidth=180pt \begin{document}` +
		`\message{[\the\hsize][\the\textwidth]}\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	got := trimNL(e.out.String())
	if !strings.Contains(got, "[400.0pt][400.0pt]") {
		t.Errorf("= %s, want the measure back at \\textwidth (400.0pt)", got)
	}
}
