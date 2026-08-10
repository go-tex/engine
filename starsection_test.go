package engine

import "testing"

// \section numbers and advances \c@section; \section* typesets the title with no
// number and does not touch the counter. \@ifstar branches on the trailing '*'.
func TestStarredSection(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	_, err := e.Run(
		`\section{A}\edef\x{\thesection}` +
			`\section*{B}\edef\y{\thesection}` + // starred: counter unchanged
			`\section{C}\edef\z{\thesection}` + // next number continues from 1
			`\message{\x|\y|\z}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "1|1|2" {
		t.Errorf("section counters = %q want 1|1|2", got)
	}
}

// \subsection* leaves \c@subsection alone; the unstarred form advances it.
func TestStarredSubsection(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	_, err := e.Run(
		`\section{S}` +
			`\subsection{a}\edef\x{\thesubsection}` +
			`\subsection*{b}\edef\y{\thesubsection}` +
			`\subsection{c}\edef\z{\thesubsection}` +
			`\message{\x|\y|\z}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "1.1|1.1|1.2" {
		t.Errorf("subsection counters = %q want 1.1|1.1|1.2", got)
	}
}

// Both \subsubsection and its starred form run without error (subsubsection is
// unnumbered either way).
func TestStarredSubsubsection(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\subsubsection{x}\subsubsection*{y}`); err != nil {
		t.Fatal(err)
	}
}
