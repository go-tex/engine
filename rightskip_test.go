package engine

import "testing"

// \@rightskip and \@flushglue are the kernel's own registers behind \raggedright,
// \centering and \raggedleft:
//
//	\newskip\@rightskip \@rightskip \z@skip                      latex.ltx:11029
//	\let\\\@centercr \@rightskip\@flushglue \rightskip\@rightskip       l.11020
//	\rightskip \@rightskip                                             l.11474
//	\leftskip\z@skip \rightskip\z@skip \@rightskip\z@skip              l.11831
//
// A class that goes through the kernel's own alignment code needs them to EXIST.
// acmart does: 38 uses of \@rightskip surface as undefined the moment its real
// option machinery runs (#306).
//
// The expected values are tectonic's.
func TestKernelAlignmentRegisters(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\message{[\the\@rightskip][\the\@flushglue]}`); err != nil {
		t.Fatal(err)
	}
	if got, want := trimNL(e.out.String()), "[0.0pt][0.0pt plus 1.0fil]"; got != want {
		t.Errorf("= %s, want %s", got, want)
	}
}
