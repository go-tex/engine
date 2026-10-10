package engine

import "testing"

// \noalign inside a tabular contributes its vertical material between the rows,
// as tex.web §785 says and as halign.go already did for \halign. The tabular
// parser dropped it — 64 uses over 8 papers of the fidelity corpus, almost all of
// them the one idiom
//
//	\hline\noalign{\vskip .5mm}
//
// so those tables sat tighter than the reference's by a skip per use.
//
// The expectation is the REFERENCE's, measured: tectonic puts the first row's
// baseline at y=133.57bp without the \noalign and at 143.53bp with a \vskip 10pt
// after the \hline — a shift of 9.96bp, which is 10pt read in big points. Ours
// moved 133.55 → 143.51, the same 9.96. This test asks the box tree for that
// growth, which is where the skip either lands or does not.
func TestANoalignInATabularContributesItsSkip(t *testing.T) {
	height := func(body string) int {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\begin{tabular}{ll}` + body + `\end{tabular}`); err != nil {
			t.Fatal(err)
		}
		tb := lastVbox(e)
		if tb == nil {
			t.Fatal("no tabular vbox contributed")
		}
		return tb.height + tb.depth
	}
	with := height(`\hline\noalign{\vskip 10pt}a & b \\ c & d \\ \hline`)
	without := height(`\hline a & b \\ c & d \\ \hline`)
	if got, want := with-without, 10*unity; got != want {
		t.Errorf("the table grew by %dsp, not by the \\vskip 10pt the \\noalign asks for (%dsp)", got, want)
	}
}

// An empty \noalign, and one with no group at all, must be harmless: TeX allows
// \noalign{}, and a parser that mistook either for a data row would build a row of
// no cells — which is how the \noalign of a TikZ node once took the rest of the
// document with it (see scanNoalign).
func TestAnEmptyNoalignLeavesTheTableStanding(t *testing.T) {
	for _, body := range []string{
		`\hline\noalign{}a & b \\ c & d`,
		`\hline\noalign a & b \\ c & d`,
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\begin{tabular}{ll}` + body + `\end{tabular}\def\survived{yes}`); err != nil {
			t.Fatalf("%q: %v", body, err)
		}
		if tb := lastVbox(e); tb == nil || tb.height+tb.depth <= 0 {
			t.Errorf("%q: the table did not survive", body)
		}
		if e.eq["survived"] == nil {
			t.Errorf("%q: the document after the table was swallowed", body)
		}
	}
}
