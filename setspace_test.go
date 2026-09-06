package engine

import "testing"

// The named commands and \setstretch/\linespread scale \baselineskip from the
// single-spaced reference.
func TestSetspaceCommands(t *testing.T) {
	base := 12 * unity
	stretch := func(f float64) int { return int(float64(base)*f + 0.5) }
	cases := []struct {
		src  string
		want int
	}{
		// \onehalfspacing / \doublespacing use setspace's own size-dependent factors
		// (1.25 / 1.667 at the 10pt default \@ptsize), not a flat 1.5 / 2.0.
		{`\doublespacing`, stretch(1.667)},
		{`\onehalfspacing`, stretch(1.25)},
		{`\singlespacing`, base},
		{`\setstretch{1.25}`, base * 5 / 4},
		{`\linespread{3}`, 3 * base},
		{`\setstretch{bad}`, base}, // malformed factor leaves it unchanged
	}
	for _, c := range cases {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(c.src); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if e.baselineskip != c.want {
			t.Errorf("%s → baselineskip %d, want %d", c.src, e.baselineskip, c.want)
		}
	}
}

// \onehalfspacing and \doublespacing pick setspace's per-size factors from the
// class base size \@ptsize (0/1/2 for 10/11/12pt), matching setspace.sty.
func TestSetspaceSizeDependent(t *testing.T) {
	cases := []struct {
		opt     string
		ptsize  int
		oneHalf float64
		double  float64
	}{
		{"10pt", 0, 1.25, 1.667},
		{"11pt", 1, 1.213, 1.618},
		{"12pt", 2, 1.241, 1.655},
	}
	for _, c := range cases {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		e.setPtsize([]string{c.opt})
		if got := e.ptsizeCode(); got != c.ptsize {
			t.Errorf("%s: ptsizeCode = %d, want %d", c.opt, got, c.ptsize)
		}
		base := e.baseBaselineskip
		if _, err := e.Run(`\onehalfspacing`); err != nil {
			t.Fatalf("%s onehalf: %v", c.opt, err)
		}
		if want := int(float64(base)*c.oneHalf + 0.5); e.baselineskip != want {
			t.Errorf("%s \\onehalfspacing → %d, want %d", c.opt, e.baselineskip, want)
		}
		if _, err := e.Run(`\doublespacing`); err != nil {
			t.Fatalf("%s double: %v", c.opt, err)
		}
		if want := int(float64(base)*c.double + 0.5); e.baselineskip != want {
			t.Errorf("%s \\doublespacing → %d, want %d", c.opt, e.baselineskip, want)
		}
	}
}

// An absent or malformed \@ptsize falls back to the 10pt factor.
func TestPtsizeCodeDefault(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	if got := e.ptsizeCode(); got != 0 {
		t.Errorf("unset \\@ptsize: ptsizeCode = %d, want 0", got)
	}
	e.define("@ptsize", &meaning{kind: mMacro, body: stringToToks("9")}, true)
	if got := e.ptsizeCode(); got != 0 {
		t.Errorf("out-of-range \\@ptsize: ptsizeCode = %d, want 0", got)
	}
}

// The spacing environment restores the previous \baselineskip at \end{spacing}.
func TestSpacingEnvironment(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{spacing}{2}\end{spacing}`); err != nil {
		t.Fatal(err)
	}
	if e.baselineskip != 12*unity {
		t.Errorf("after spacing env baselineskip = %d, want restored 12pt", e.baselineskip)
	}

	// Nested: the inner value applies then unwinds to the outer.
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\setstretch{1.5}\begin{spacing}{2}\end{spacing}`); err != nil {
		t.Fatal(err)
	}
	if e2.baselineskip != 12*unity*3/2 {
		t.Errorf("after nested spacing baselineskip = %d, want 18pt (1.5x)", e2.baselineskip)
	}
}

// Wider spacing actually increases interline glue in a rendered paragraph.
func TestSpacingAffectsParagraph(t *testing.T) {
	measure := func(src string) int {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		e.hsize = 40 * unity
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		return vpackSP(e.mvl, packNatural, 0).height
	}
	single := measure(`aaaa aaaa aaaa aaaa`)
	double := measure(`\doublespacing aaaa aaaa aaaa aaaa`)
	if double <= single {
		t.Errorf("double spacing height %d should exceed single %d", double, single)
	}
}

// A class that redefines \normalsize states its body leading in \@setfontsize's
// third argument and nowhere else. neurips_2024.sty is the pattern:
// \@setfontsize\normalsize\@xpt\@xipt — 10pt on an 11pt skip, where the article
// default is 12pt. 24 of the 157 corpus papers set their leading this way.
func TestSetfontsizeTakesTheNormalsizeLeading(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want float64
	}{
		{"a conference style's normalsize", `\@setfontsize\normalsize\@xpt\@xipt`, 10.95},
		{"a plain number", `\@setfontsize\normalsize{10}{13}`, 13},
		{"another size command is ignored", `\@setfontsize\small\@ixpt\@xpt`, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.LoadLaTeX()
			e.SetFont(spMock{})
			before := e.baselineskip
			if _, err := e.Run(`\makeatletter ` + c.src); err != nil {
				t.Fatal(err)
			}
			if c.want == 0 {
				if e.baselineskip != before {
					t.Errorf("baselineskip moved to %d for a non-normalsize switch", e.baselineskip)
				}
				return
			}
			if want := ptToSP(c.want); e.baselineskip != want {
				t.Errorf("baselineskip = %d, want %d (%.2fpt)", e.baselineskip, want, c.want)
			}
		})
	}
}

// The command name must never be EXPANDED to decide this: \normalsize expands to
// the very \@setfontsize call being handled, so grabbing it in Go looped until the
// runaway guard fired and took the document with it — corpus paper 2311.09365 fell
// from 16 pages to 4, losing 26778 glyphs. The comparison is \ifx, in TeX.
func TestSetfontsizeDoesNotRecurse(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\makeatletter\renewcommand\normalsize{\@setfontsize\normalsize\@xpt\@xipt}\normalsize\normalsize A`); err != nil {
		t.Fatal(err)
	}
	if d := e.Diagnostics(); d.Runaway {
		t.Error("the runaway guard tripped: \\normalsize re-entered its own definition")
	}
	if got := mvlText(e.mvl); got != "A" {
		t.Errorf("typeset %q, want %q", got, "A")
	}
}

// \setstretch and \linespread are the two setspace commands a document uses
// INSIDE a group to space one piece of the page — a title block, a quotation —
// and real LaTeX restores \baselinestretch at the closing brace. A thesis cover
// macro whose \begin{center} ... \setstretch{1.05} ... \end{center} meant to
// space its own page imposed 1.05 on the 198 pages after it (issue #247).
func TestSetstretchIsRestoredWithItsGroup(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	before := e.baselineskip
	if _, err := e.Run(`\begingroup\setstretch{2}\endgroup`); err != nil {
		t.Fatal(err)
	}
	if e.baselineskip != before {
		t.Errorf("baselineskip = %d after the group closed, want %d", e.baselineskip, before)
	}
	// Inside the group it must actually apply.
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\begingroup\setstretch{2}`); err != nil {
		t.Fatal(err)
	}
	if want := 2 * before; e2.baselineskip != want {
		t.Errorf("inside the group baselineskip = %d, want %d", e2.baselineskip, want)
	}
	// At the top level there is no group to restore it, and it must persist.
	e3 := New()
	e3.LoadLaTeX()
	e3.SetFont(spMock{})
	if _, err := e3.Run(`\setstretch{2}`); err != nil {
		t.Fatal(err)
	}
	if want := 2 * before; e3.baselineskip != want {
		t.Errorf("at top level baselineskip = %d, want %d", e3.baselineskip, want)
	}
}

// The named commands and the spacing environment are deliberately NOT made
// group-local: they space stretches of a document, and restoring them with the
// enclosing group regressed the corpus by 16 pages with one paper losing content
// (issue #247). \end{spacing} restores its own, through spacingSaved.
func TestNamedSpacingCommandsAreNotGroupLocal(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	before := e.baselineskip
	if _, err := e.Run(`\begingroup\doublespacing\endgroup`); err != nil {
		t.Fatal(err)
	}
	if e.baselineskip == before {
		t.Errorf("baselineskip came back to %d: \\doublespacing must outlive its group", before)
	}
}
