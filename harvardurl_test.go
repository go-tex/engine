package engine

import (
	"strings"
	"testing"
)

// harvard.sty's \harvardurl is the URL line at the end of a reference entry
// (harvard.sty:28, the branch taken without hyperref):
//
//	\newcommand{\harvardurl}[1]{\textbf{URL:} \textit{#1}}
//
// Its hyperref branch only adds a link around the same words. The kernel already
// carried \harvarditem, \harvardand and \harvardyearleft/right; this was the last
// of the family still missing, and undefined it took the whole line with it.
func TestHarvardURLSetsTheURLLine(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\harvardurl{https://doi.org/10.1063/1.3618672}\par`); err != nil {
		t.Fatal(err)
	}
	got := glyphString(e.mvl) + glyphString(e.parList)
	for _, want := range []string{"URL:", "doi.org/10.1063"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q does not contain %q", got, want)
		}
	}
}

// In place: a harvard .bbl entry keeps its URL line under the entry it belongs to.
func TestHarvardURLInABibliographyEntry(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\begin{thebibliography}{9}` +
		`\harvarditem{Trampert}{1990}{T90}Trampert, J. \harvardyearleft 1990\harvardyearright{}.` +
		`\harvardurl{https://agupubs.example/abs/10.1029}` +
		`\end{thebibliography}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	got := glyphString(e.mvl) + glyphString(e.parList)
	if !strings.Contains(got, "Trampert") {
		t.Fatalf("the entry itself is missing: %q", got)
	}
	if !strings.Contains(got, "URL:") || !strings.Contains(got, "agupubs.example") {
		t.Errorf("the entry lost its URL line: %q", got)
	}
}
