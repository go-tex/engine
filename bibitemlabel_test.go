package engine

import (
	"strings"
	"testing"
)

// \bibitem[⟨label⟩]{⟨key⟩}: the optional argument is the entry's CITATION label —
// what natbib stores so \citet prints "Meyer and Roeder (2014)". In a numbered
// bibliography it is not printed; the marker is the running number.
//
// This kernel used to typeset it at the head of the entry, deliberately: the
// comment said it recovered the author/year "even for a .bbl whose entry BODY
// (apsrev's \bibinfo{author}{…}) this kernel renders only in part". That reason
// no longer holds — \bibinfo renders — so the label was pure duplication.
//
// Checked against tectonic on the very case the comment named, corpus paper
// 2203.15077 (revtex4-2, apsrev .bbl), first entry:
//
//	reference  [1] H. M. Meyer and A. H. Roeder, Stochasticity in plant cellular…
//	before     [1] Meyer and Roeder(2014) H. M. Meyer and A. H. Roeder,
//	after      [1] H. M. Meyer and A. H. Roeder, Stochasticity in plant cellu-…
//
// 56 of the 200 corpus papers carry \bibitem[…], 2562 entries between them.
func TestBibitemLabelIsNotTypeset(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\begin{thebibliography}{9}`+
		`\bibitem[{Meyer and Roeder(2014)}]{M2014}H.~M. Meyer, Stochasticity.`+
		`\end{thebibliography}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if strings.Contains(got, "MeyerandRoeder(2014)") {
		t.Errorf("the citation label reached the page: %q", got)
	}
	if !strings.Contains(got, "Stochasticity") {
		t.Errorf("the entry body is missing: %q", got)
	}
	if !strings.Contains(got, "[1]") {
		t.Errorf("the running number is missing: %q", got)
	}
}
