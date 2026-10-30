package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ \itemsep and \parsep go BETWEEN bibliography entries, and nothing emitted
// them. This kernel's \thebibliography is an emulation that sets \leftskip rather
// than going through \list, where LaTeX's \item would add \itemsep and the new
// paragraph would add \parskip (which \list sets to \parsep).
//
// Measured against the reference on three one-line entries, baseline to baseline:
// ours 11.956pt — \baselineskip alone — against 19.925pt. 7.97pt short on EVERY
// reference of EVERY paper, so a 40-entry bibliography came out 3 pages against
// the reference's 4.
//
// ⭐ The registers were RIGHT the whole time: inside the environment \itemsep,
// \parsep and \topsep all read identically to the reference (4pt plus 2pt minus
// 1pt, 4pt plus 2pt minus 1pt, 8pt plus 2pt minus 4pt). Only \labelwidth and
// \leftmargin differed, and those are horizontal. Storing a parameter is not
// typesetting it.
func TestBibliographyEntriesAreSeparatedByItemsepAndParsep(t *testing.T) {
	height := func(entries string) int {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Run(`\documentclass{article}\begin{document}` +
			`\setbox0=\vbox{\begin{thebibliography}{99}` + entries +
			`\end{thebibliography}}`); err != nil {
			t.Fatal(err)
		}
		b := e.box[0]
		if b == nil {
			t.Fatal("la boîte 0 est vide")
		}
		return b.height + b.depth
	}
	one := height(`\bibitem{a} AAA`)
	three := height(`\bibitem{a} AAA\bibitem{b} BBB\bibitem{c} CCC`)
	// Two more one-line entries cost two baselines plus two (\itemsep + \parsep)
	// gaps of 4pt each: 2 × (11.956 + 8) ≈ 39.9pt. The tolerance is a point,
	// because the baseline itself depends on the face in use.
	got := three - one
	want := 2 * (ptToSP(11.956) + ptToSP(8))
	if got < want-ptToSP(1) || got > want+ptToSP(1) {
		t.Errorf("deux entrées de plus coûtent %.2fpt, want %.2fpt ± 1 — "+
			"sans \\itemsep ni \\parsep ce serait %.2fpt",
			float64(got)/float64(unity), float64(want)/float64(unity),
			float64(2*ptToSP(11.956))/float64(unity))
	}
}

// ⚠ And NOT before the FIRST entry, where \topsep already applies.
//
// ⛔ This asserts the SHAPE of the definition, and the two behavioural forms I
// tried first both failed to discriminate. Comparing "one entry" against "none"
// survived putting the skip in front of every entry; so did comparing the cost of
// the first entry against a later one — because an empty thebibliography already
// carries its own heading and \par, so neither difference isolates one entry's
// cost. Rather than calibrate a bound to the number I happened to measure (20pt
// where I expected 8), the guard is the \ifnum that makes the skip conditional,
// and mutate confirms removing it fails this test.
func TestTheBibliographySkipIsConditionalOnTheFirstEntry(t *testing.T) {
	const guard = `\ifnum\c@bibitem>0 \vskip\itemsep\vskip\parsep\fi`
	if !strings.Contains(MiniLaTeXKernel, guard) {
		t.Errorf("\\@bibsep doit être conditionnel: sans le \\ifnum, chaque " +
			"bibliographie descend de 8pt sur tous les papiers")
	}
}
