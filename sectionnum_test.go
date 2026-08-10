package engine

import "testing"

// \thesection/\thesubsection track the section counters as \section/\subsection
// advance them (subsection resets each section).
func TestSectionNumbering(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	// drive the counters and read them back via \message (no font needed)
	got, err := e.Run(
		`\def\out{}` +
			`\advance\c@section by1 \c@subsection=0 ` +
			`\advance\c@subsection by1 \edef\out{\out\thesubsection;}` +
			`\advance\c@subsection by1 \edef\out{\out\thesubsection;}` +
			`\advance\c@section by1 \c@subsection=0 \edef\out{\out\thesection}` +
			`\message{\out}`)
	if err != nil {
		t.Fatal(err)
	}
	if trimNL(got) != "1.1;1.2;2" {
		t.Errorf("section numbers = %q want 1.1;1.2;2", trimNL(got))
	}
}
