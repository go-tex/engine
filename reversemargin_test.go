package engine

import "testing"

// \if@reversemargin is declared by the LaTeX kernel (latex.ltx:15037, false) and
// set only by \reversemarginpar / \normalmarginpar (latex.ltx:13155-13156). It was
// the most-reported missing conditional in the fidelity corpus — 5 papers, 222
// occurrences, every one of them fancyhdr.sty's
//
//	\def\fancy@Oolh{\if@reversemargin\hss\else\relax\fi}
//
// which without the test leaves a stray \else … \fi in the input.
//
// ⚠ The engine emulates fancyhdr itself and never runs that .sty's \@oddhead, so
// this moves no ink; what it buys is that a conditional the kernel declares is
// declared, and that a census of missing commands stops being led to it. The test
// therefore asserts the MEANING, which is the thing that changed.
func TestReverseMarginIsDeclaredAndSettable(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// Default false, as the kernel declares it: the \else branch is the one taken.
	if _, err := e.Run(`\if@reversemargin\def\took{yes}\else\def\took{no}\fi`); err != nil {
		t.Fatal(err)
	}
	if e.eq["took"] == nil {
		t.Fatal("\\if@reversemargin is not a conditional at all")
	}
	if got := macroBody(e, "took"); got != "no" {
		t.Errorf("default branch %q, want %q — latex.ltx declares it FALSE", got, "no")
	}
	// \reversemarginpar sets it, which is the only way it ever becomes true.
	if _, err := e.Run(`\reversemarginpar\if@reversemargin\def\then{yes}\else\def\then{no}\fi`); err != nil {
		t.Fatal(err)
	}
	if got := macroBody(e, "then"); got != "yes" {
		t.Errorf("after \\reversemarginpar the branch is %q, want %q", got, "yes")
	}
	// And \normalmarginpar puts it back.
	if _, err := e.Run(`\normalmarginpar\if@reversemargin\def\back{yes}\else\def\back{no}\fi`); err != nil {
		t.Fatal(err)
	}
	if got := macroBody(e, "back"); got != "no" {
		t.Errorf("after \\normalmarginpar the branch is %q, want %q", got, "no")
	}
}

// macroBody renders a macro's replacement text as a plain string.
func macroBody(e *Engine, name string) string {
	m := e.eq[name]
	if m == nil {
		return ""
	}
	s := ""
	for _, t := range m.body {
		if !t.cs_ {
			s += string(t.ch)
		}
	}
	return s
}
