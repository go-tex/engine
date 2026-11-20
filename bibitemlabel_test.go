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
//
// ⛔ WHICH \bibitem THAT IS DEPENDS ON WHAT IS LOADED, and this test used to pin the
// natbib answer on a document without natbib. revtex4-2 loads natbib, whose \bibitem
// numbers every entry; plain article does not, and LaTeX's own \@lbibitem
// (ltbibl.dtx:162) makes the optional label THE MARKER and the \cite text. Measured
// against tectonic, the same two entries:
//
//	                   marker 1                   marker 2   \cite
//	article + natbib   [1]                        [2]        [1], [2]
//	article            [Meyer and Roeder(2014)]   [1]        [Meyer and Roeder(2014)], [1]
//
// An entry with its own label does not step the counter (\item[…] raises no
// \@noitemarg), which is why the plain one is [1]. 2203.15077 itself is unchanged:
// "[1] H. M. Meyer and A. H. Roeder…" before and after, as in the reference.
func TestBibitemLabelIsNotTypeset(t *testing.T) {
	const entries = `\begin{thebibliography}{9}` +
		`\bibitem[{Meyer and Roeder(2014)}]{M2014}H.~M. Meyer, Stochasticity.` +
		`\bibitem{P2023} Plain Key, Title.` +
		`\end{thebibliography}\end{document}`
	for _, c := range []struct {
		name, preamble string
		want           []string
		absent         string
	}{
		{"with natbib", `\usepackage[numbers]{natbib}`, []string{"[1]H.M.Meyer", "[2]PlainKey"}, "MeyerandRoeder(2014)"},
		{"without natbib", ``, []string{"[MeyerandRoeder(2014)]H.M.Meyer", "[1]PlainKey"}, "[2]"},
	} {
		e, err := compile([]byte(`\documentclass{article}`+c.preamble+`\begin{document}`+entries), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		got := pageChars(e)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: %q not on the page (tectonic sets it): %q", c.name, w, got)
			}
		}
		if strings.Contains(got, c.absent) {
			t.Errorf("%s: %q on the page, which tectonic does not set: %q", c.name, c.absent, got)
		}
	}
}
