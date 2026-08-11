package engine

import "testing"

// The named commands and \setstretch/\linespread scale \baselineskip from the
// single-spaced reference.
func TestSetspaceCommands(t *testing.T) {
	base := 12 * unity
	cases := []struct {
		src  string
		want int
	}{
		{`\doublespacing`, 2 * base},
		{`\onehalfspacing`, base * 3 / 2},
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
