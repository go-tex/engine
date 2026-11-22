package engine

import (
	"strings"
	"testing"
)

func runHalign(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return mvlText(e.mvl)
}

// tex.web §782 copies the template UNEXPANDED, so a token in it is expanded when
// the CELL runs, after the assignments that precede it in the entry. Expanded at
// preamble time it was frozen. The row's own entry is not expanded ahead of its
// assignment here (\count1=5 is not expandable), so this isolates the PREAMBLE half
// of go-tex/engine#602; witness w2 needs the live cells too.
func TestAPreambleTokenIsExpandedWhenTheCellRuns(t *testing.T) {
	got := runHalign(t, `\count1=0 \noindent\halign{#\the\count1\cr \count1=5 x\cr}`)
	if !strings.Contains(got, "x5") {
		t.Errorf("want x5, got %q", got)
	}
}

// §783-784: an & or \cr inside braces belongs to the template, because
// align_state is not -1000000 there; and §782 expands the token after \span once.
func TestABraceHidesAnAmpersandInTheTemplate(t *testing.T) {
	got := runHalign(t, `\noindent\halign{[{a&b}#]\cr x\cr}`)
	if !strings.Contains(got, "[a&b") && !strings.Contains(got, "[ab") {
		t.Errorf("the braced & ended the column: %q", got)
	}
	if !strings.Contains(got, "x]") {
		t.Errorf("entry/after part lost: %q", got)
	}
}

func TestSpanExpandsTheNextTokenOfTheTemplate(t *testing.T) {
	got := runHalign(t, `\def\b{[}\noindent\halign{\span\b#]\cr x\cr}`)
	if !strings.Contains(got, "[x]") {
		t.Errorf("want [x], got %q", got)
	}
}

// A \span with nothing after it ends the input in the middle of the preamble; the
// alignment is abandoned without a panic or a hang.
func TestASpanAtTheEndOfTheInputIsHarmless(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\halign{#\span`); err != nil {
		t.Fatal(err)
	}
}
