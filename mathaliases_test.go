package engine

import (
	"strings"
	"testing"
)

// \mathds (dsfont) and \Tilde (amsmath) are aliased in the class kernel to the
// blackboard-bold and tilde forms the math layer already renders, so real papers
// using them typeset instead of dropping the whole equation. These were the two
// largest math-command gaps found in an arXiv real-world sweep.
func TestMathAliases(t *testing.T) {
	for _, src := range []string{
		`\documentclass{article}\begin{document}$\mathds{R}\subset\mathds{C}$\end{document}`,
		`\documentclass{article}\begin{document}$\Tilde{x}+\Tilde{y}$\end{document}`,
	} {
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		// The alias must render, not drop: neither the text-mode key (mathds) nor the
		// math-layer key (\mathds) may appear. A benign class-load "input" tally is
		// always present and unrelated.
		sk := e.SkippedCommands()
		for _, k := range []string{"mathds", "\\mathds", "Tilde", "\\Tilde"} {
			if sk[k] != 0 {
				t.Errorf("%q still dropped %q (%d): %v", src, k, sk[k], sk)
			}
		}
		svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
		if !strings.Contains(svg, "<path") {
			t.Errorf("%q rendered no glyph paths", src)
		}
	}
}

// Colour commands are primitives go-tex/math cannot render; in math they are
// stripped (content kept) so the equation typesets in the surrounding colour
// instead of being dropped — the single largest remaining arXiv math gap.
func TestColorInMath(t *testing.T) {
	for _, src := range []string{
		`\documentclass{article}\begin{document}$\color{red} x + y$\end{document}`,
		`\documentclass{article}\begin{document}$a\textcolor{blue}{b}+c$\end{document}`,
		`\documentclass{article}\begin{document}$\color[rgb]{1,0,0} z^2$\end{document}`,
		`\documentclass{article}\usepackage{amsmath}\begin{document}\[\textcolor{green}{\frac{a}{b}}+\colorbox{yellow}{c}\]\end{document}`,
	} {
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		sk := e.SkippedCommands()
		for _, k := range []string{"\\color", "\\textcolor", "\\colorbox"} {
			if sk[k] != 0 {
				t.Errorf("%q dropped %q: %v", src, k, sk)
			}
		}
		if svg := strings.Join(e.RenderPages(e.renderMargin(0)), ""); !strings.Contains(svg, "<path") {
			t.Errorf("%q rendered no glyph paths", src)
		}
	}
}

func TestStripMathColor(t *testing.T) {
	// scanMathSource encodes a control sequence as "\name " (trailing space).
	cases := []struct {
		name, src, want string
		changed         bool
	}{
		{"color", `\color {red}x+1`, `x+1`, true},
		{"color", `\color [rgb]{1,0,0}y`, `y`, true},
		{"textcolor", `a\textcolor {blue}{b}c`, `abc`, true},
		{"colorbox", `\colorbox {y}{z}`, `z`, true},
		{"fcolorbox", `\fcolorbox {a}{b}{w}`, `w`, true},
		{"normalcolor", `\normalcolor q`, `q`, true},
		{"color", `no colour here`, `no colour here`, false},         // no occurrence
		{"color", `\color {unbalanced`, `\color {unbalanced`, false}, // unparseable arg, left verbatim
		{"unknowncmd", `\unknowncmd {x}`, `\unknowncmd {x}`, false},  // not a colour command
	}
	for _, c := range cases {
		got, changed := stripMathColor(c.src, c.name)
		if got != c.want || changed != c.changed {
			t.Errorf("stripMathColor(%q, %q) = (%q, %v), want (%q, %v)", c.src, c.name, got, changed, c.want, c.changed)
		}
	}
}

func TestSkipMathOptArg(t *testing.T) {
	cases := []struct{ in, want string }{
		{`[rgb]{1,0,0}`, `{1,0,0}`},
		{`  [x]rest`, `rest`},
		{`{no opt}`, `{no opt}`},
		{`[unterminated`, ``},
	}
	for _, c := range cases {
		if got := skipMathOptArg(c.in); got != c.want {
			t.Errorf("skipMathOptArg(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
