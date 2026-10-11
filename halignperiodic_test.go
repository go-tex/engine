package engine

import (
	"strings"
	"testing"
)

// A preamble whose first token is & is PERIODIC: tex.web §786 does not read that &
// as a column separator but as the mark on the template to repeat ("cur_loop"),
// and §793 lengthens the preamble from it whenever a row asks for a column the
// preamble does not have.
//
// Read as a separator it produced an EMPTY column 0, so every cell landed one
// column right of its template and the template applied to nothing.
//
// The expectation is the REFERENCE's: tectonic sets
//
//	\noindent\halign{&[#]\cr a&b&c\cr d&e\cr}
//
// as the two lines "[a][b][c]" and "[d][e]", and so do we.
func TestAPeriodicPreambleRepeatsItsTemplate(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\halign{&[#]\cr a&b&c\cr d&e\cr}`); err != nil {
		t.Fatal(err)
	}
	got := mvlText(e.mvl)
	for _, want := range []string{"[a][b][c]", "[d][e]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	// And the template must not have been applied to a phantom column: a leading
	// empty column would show as a bracket pair with nothing between it.
	if strings.Contains(got, "[]") {
		t.Errorf("an empty column was built from the leading &: %q", got)
	}
}

// A non-periodic preamble must NOT repeat. This is the control on the test above:
// without it, "repeats its template" could be satisfied by repeating ALWAYS.
//
// ⚠ There is no reference rendering to compare against here, and the expectation
// below is OURS, not tectonic's: a row with more entries than the preamble has
// templates is TeX's "Extra alignment tab has been changed to \cr", which is an
// ERROR — tectonic stops and writes no PDF at all. So the assertion is only that
// our lenient recovery does not INVENT a template for the extra cell, which is
// the property the periodic case must not leak into.
func TestANonPeriodicPreambleDoesNotRepeat(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\halign{[#]\cr a&b\cr}`); err != nil {
		t.Fatal(err)
	}
	got := mvlText(e.mvl)
	if strings.Count(got, "[") != 1 {
		t.Errorf("the extra cell was given a template it was never declared: %q", got)
	}
}

// An alignment opened with { may be closed with \egroup: braces and implicit
// braces match, and align_state counts both (tex.web §774). The row scanner read
// \egroup as ordinary cell material, so the row never ended and the alignment ran
// to end of input.
func TestAnImplicitEgroupClosesAnAlignment(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\halign{[#]\cr a\cr\egroup \noindent b`); err != nil {
		t.Fatal(err)
	}
	got := mvlText(e.mvl)
	if !strings.Contains(got, "[a]") {
		t.Errorf("the row was not built: %q", got)
	}
	// The discriminator is what happens to what FOLLOWS. Unrecognised, \egroup is
	// ordinary cell material and "b" becomes another ENTRY — so it comes out
	// wrapped in the template, "[b]". Recognised, the alignment is over and "b" is
	// ordinary document text.
	//
	// Asserting only that a \def after the \egroup took effect proves nothing: the
	// cell's token list is executed either way, so that test passed without the
	// fix and a mutation of it survived the whole suite.
	if strings.Contains(got, "[b]") {
		t.Errorf("\\egroup did not close the alignment: %q still has \"b\" as an entry", got)
	}
	if !strings.Contains(got, "b") {
		t.Errorf("the text after the alignment was lost: %q", got)
	}
}

// \halign without its mandatory brace is dropped WHOLE, and that must be
// reported: the primitive is defined, every command in the preamble is defined,
// and what reaches the page is the alignment's own source — preamble included, so
// a stray "#" lands on the page and a census of undefined commands reads the
// leaked \noalign and \everycr as missing (go-tex/engine#602).
//
// §403 reads past spaces and \relax on the way to the brace, so neither of those
// counts as a miss.
func TestAnAlignmentDroppedForWantOfItsBraceIsReported(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      int
	}{
		{"implicit brace, not yet accepted", `\halign\bgroup#\cr a\cr\egroup`, 1},
		{"no brace at all", `\halign x`, 1},
		{"a space on the way", `\halign {#\cr a\cr}`, 0},
		{"a \\relax on the way", `\halign\relax{#\cr a\cr}`, 0},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\noindent` + c.src); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := totalOf(e.Diagnostics().AlignmentsAbandoned); got != c.want {
			t.Errorf("%s: %d alignment(s) reported abandoned, want %d", c.name, got, c.want)
		}
	}
}

func totalOf(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}
