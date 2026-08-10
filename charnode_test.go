package engine

import "testing"

// spMock is a deterministic fontFace: every letter is 5pt wide, 7pt tall, 2pt
// deep; digits are 4pt wide; the interword space is 3pt (no stretch/shrink here).
type spMock struct{}

func (spMock) charDimsSP(r rune) (int, int, int) {
	w := 5 * unity
	if r >= '0' && r <= '9' {
		w = 4 * unity
	}
	return w, 7 * unity, 2 * unity
}
func (spMock) spaceSP() glueSpec {
	return glueSpec{width: 3 * unity, stretch: 3 * unity / 2, shrink: unity} // 3pt plus 1.5pt minus 1pt
}
func (spMock) glyphPathAt(rune) string { return "" }
func (spMock) kernSP(_, _ rune) int    { return 0 }

func TestCharNodeMetrics(t *testing.T) {
	cases := []struct {
		src, dim, want string
	}{
		// "ab" = 5+5 = 10pt wide, 7pt tall, 2pt deep
		{`\setbox0=\hbox{ab}`, "wd", "10.0pt"},
		{`\setbox0=\hbox{ab}`, "ht", "7.0pt"},
		{`\setbox0=\hbox{ab}`, "dp", "2.0pt"},
		// "a b" = 5 + 3(space) + 5 = 13pt
		{`\setbox0=\hbox{a b}`, "wd", "13.0pt"},
		// digits are narrower: "12" = 4+4 = 8pt
		{`\setbox0=\hbox{12}`, "wd", "8.0pt"},
		// macros expand and their letters are measured: \def\w{abc} → 15pt
		{`\def\w{abc}\setbox0=\hbox{\w}`, "wd", "15.0pt"},
		// \hbox to overrides but content still measured for glue-set (natural 10)
		{`\setbox0=\hbox spread 4pt{ab}`, "wd", "14.0pt"},
	}
	for _, c := range cases {
		e := New()
		e.SetFont(spMock{})
		if _, err := e.Run(c.src + `\message{\the\` + c.dim + `0}`); err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		got := trimNL(e.out.String())
		if got != c.want {
			t.Errorf("%q [%s]: got %q want %q", c.src, c.dim, got, c.want)
		}
	}
}
