package engine

import "testing"

// \normalbaselineskip is 12pt to begin with (latex.ltx:547) and \selectfont keeps
// it equal to \baselineskip from then on (set@fontsize, l.8543). Allocated and
// never set it read ZERO, and what reads it then gets nothing:
// \@arrayparboxrestore sets \baselineskip FROM it inside every array cell and
// parbox (l.11835), and IEEEtran builds every \IEEEeqnarray row strut as
// 0.7/0.3\normalbaselineskip.
//
// The expected values are a real TeX's (tectonic), read side by side.
func TestNormalbaselineskipIsSetAndKeptInStep(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\message{[\the\normalbaselineskip]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[12.0pt]" {
		t.Errorf("at start = %s, want [12.0pt]", got)
	}

	// A class that states its own body leading moves it with \baselineskip.
	e2 := New()
	if err := e2.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\makeatletter\@setfontsize\normalsize{10}{13}` +
		`\message{[\the\baselineskip][\the\normalbaselineskip]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e2.out.String()); got != "[13.0pt][13.0pt]" {
		t.Errorf("after a size table = %s, want [13.0pt][13.0pt]", got)
	}

	// setspace moves both too: \normalbaselineskip is what \selectfont last wrote.
	e3 := New()
	if err := e3.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e3.SetFont(spMock{})
	if _, err := e3.Run(`\linespread{2}\selectfont\message{[\the\normalbaselineskip]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e3.out.String()); got != "[24.0pt]" {
		t.Errorf("after \\linespread{2} = %s, want [24.0pt]", got)
	}
}
