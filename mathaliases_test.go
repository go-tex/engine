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
		// \pmb is amsbsy's poor man's bold — three overprinted copies, the fallback
		// for when no bold math version exists (amsbsy.sty:40-57). One exists here,
		// so it becomes \boldsymbol, the thing it was imitating. 348 of the formulas
		// the arXiv corpus drops are this one command.
		`\documentclass{article}\begin{document}$\pmb{\alpha}+\pmb{x}$\end{document}`,
	} {
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		// The alias must render, not drop: neither the text-mode key (mathds) nor the
		// math-layer key (\mathds) may appear. A benign class-load "input" tally is
		// always present and unrelated.
		sk := e.SkippedCommands()
		for _, k := range []string{"mathds", "\\mathds", "Tilde", "\\Tilde", "pmb", "\\pmb"} {
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

// Symbols and double-struck alphabets an arXiv corpus census found were dropping
// whole equations, now resolved by the go-tex/math layer (v0.18.0): \mathbbm{1}
// (1045 papers), \intercal (225), \dotplus, \Coloneqq, \mathbbmss/\mathbbb. Each
// must typeset through the engine instead of landing in MathDropped.
func TestMathCorpusCensusSymbols(t *testing.T) {
	src := `\documentclass{article}\begin{document}` +
		`$\mathbbm{1}_{\{x>0\}}$ $A^\intercal$ $a\dotplus b$ ` +
		`$a\Coloneqq b$ $\mathbbmss{X}$ $\mathbbb{Z}$` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatal(err)
	}
	if md := e.Diagnostics().MathDropped; len(md) != 0 {
		t.Errorf("census symbols still dropped whole equations: %v", md)
	}
	if svg := strings.Join(e.RenderPages(e.renderMargin(0)), ""); !strings.Contains(svg, "<path") {
		t.Error("rendered no glyph paths")
	}
}

// \DeclarePairedDelimiter (mathtools) defines a one-argument delimiter macro; real
// papers use it for \ceil \floor \abs \norm \set. It must expand textually so the
// math resolver renders it instead of dropping the equation.
func TestDeclarePairedDelimiter(t *testing.T) {
	src := `\documentclass{article}` +
		`\DeclarePairedDelimiter\ceil{\lceil}{\rceil}` +
		`\DeclarePairedDelimiter\floor{\lfloor}{\rfloor}` +
		`\begin{document}$\ceil{x}+\floor{\frac{y}{2}}$\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"\\ceil", "\\floor", "ceil", "floor"} {
		if e.SkippedCommands()[k] != 0 {
			t.Errorf("dropped %q: %v", k, e.SkippedCommands())
		}
	}
	if svg := strings.Join(e.RenderPages(e.renderMargin(0)), ""); !strings.Contains(svg, "<path") {
		t.Error("rendered no glyph paths")
	}
}

// Non-rendering math commands must not drop the equation: capitalised amsmath
// accents alias to the plain ones, spacing becomes a space, metadata is removed.
func TestMathNoiseAndAccents(t *testing.T) {
	for _, src := range []string{
		`\documentclass{article}\begin{document}$\Bar{x}+\Hat{y}+\Breve{z}+\Vec{w}$\end{document}`,
		`\documentclass{article}\begin{document}$a\hspace{1cm}b+\sfrac{1}{2}$\end{document}`,
		`\documentclass{article}\usepackage{amsmath}\begin{document}\begin{align}x&=1\label{e1}\\y&=2\nonumber\end{align}\end{document}`,
	} {
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if len(e.SkippedCommands()) > 1 { // only the benign class-load "input" tally
			t.Errorf("%s dropped: %v", src, e.SkippedCommands())
		}
	}
	// stripMathNoise unit branches: a space command, a metadata command, a bad arg,
	// and a non-noise name.
	if out, ok := stripMathNoise(`a \hspace {1cm}b`, "hspace"); !ok || out != `a \; b` {
		t.Errorf("hspace strip = (%q,%v)", out, ok)
	}
	if out, ok := stripMathNoise(`x \nonumber y`, "nonumber"); !ok || out != `x y` {
		t.Errorf("nonumber strip = (%q,%v)", out, ok)
	}
	if out, ok := stripMathNoise(`\label `, "label"); ok || out != `\label ` {
		t.Errorf("unparseable \\label must be left verbatim, got (%q,%v)", out, ok)
	}
	if _, ok := stripMathNoise(`x`, "frac"); ok {
		t.Error("a non-noise command must not be stripped")
	}
}

// A nested inline $…$ inside a \mbox/\text group within display or inline math is
// that group's own math, not the closing shift. The scanner must not end the outer
// math there — doing so truncated it and desynchronised every following $, swallowing
// paragraphs of text as "math" (the single largest real-world failure). After the
// group, ordinary math must still parse.
func TestNestedDollarDoesNotDesync(t *testing.T) {
	src := "\\documentclass{article}\\begin{document}" +
		"$$ a=b \\quad \\mbox{for $x<y$, $u>v$.} $$\n" +
		"Then $c=d$ real. More $e=f$ ok.\\end{document}"
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.SkippedCommands()) > 1 { // only the benign class-load "input" tally
		t.Errorf("nested $ desynchronised the scan: %v", e.SkippedCommands())
	}
}

// A math environment nested in \begin{equation} must keep its own \end name; using
// the outer name turned \begin{aligned}…\end{aligned} into …\end{equation} and the
// math layer dropped it.
func TestEquationWithNestedEnv(t *testing.T) {
	for _, body := range []string{
		`\begin{equation}\begin{aligned}a&=b\\c&=d\end{aligned}\end{equation}`,
		`\begin{equation}\begin{bmatrix}a\\b\end{bmatrix}\end{equation}`,
		`\begin{equation}f(x)=\begin{cases}1&x>0\\0&x\le0\end{cases}\end{equation}`,
	} {
		src := `\documentclass{article}\usepackage{amsmath}\begin{document}` + body + `\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if len(e.SkippedCommands()) > 1 {
			t.Errorf("%s dropped: %v", body, e.SkippedCommands())
		}
	}
}

// Cross-reference / citation commands resolve to text inside math (\eqref → "(n)",
// \ref → the label, \cite → a placeholder) so an equation carrying one still renders.
func TestRefInMath(t *testing.T) {
	src := `\documentclass{article}\begin{document}` +
		`\begin{equation}x=1\label{eq:a}\end{equation}` +
		`$y=\eqref{eq:a}\cdot\bm{v}+\ref{eq:a}$ and $z\citep{k}$.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"\\eqref", "\\ref", "\\citep", "\\bm"} {
		if e.SkippedCommands()[k] != 0 {
			t.Errorf("dropped %q: %v", k, e.SkippedCommands())
		}
	}
	// direct unit coverage of the resolver's branches
	if got := e.mathRefText("eqref", "eq:a"); got == "" || got[0] != '(' {
		t.Errorf("eqref text = %q", got)
	}
	if got := e.mathRefText("cite", "k"); got != "[?]" {
		t.Errorf("cite text = %q, want [?]", got)
	}
	if _, ok := e.resolveMathRef(`x + y`, "notaref"); ok {
		t.Error("non-ref command must not resolve")
	}
	if out, ok := e.resolveMathRef(`\ref `, "ref"); ok || out != `\ref ` {
		t.Errorf("a \\ref with no argument must be left verbatim, got (%q,%v)", out, ok)
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
