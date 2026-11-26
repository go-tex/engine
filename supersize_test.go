package engine

import (
	"math"
	"strings"
	"testing"
)

// \textsuperscript sets its argument at \sf@size (latex.ltx:13171-13172,
// \fontsize\sf@size\sf@size), and \sf@size is the script size of the CURRENT text
// size: fontmath.ltx:75-86 \DeclareMathSizes, else .7 of it (latex.ltx:7782-7786).
// The engine used \scriptsize (7pt) at every size.
func TestTextsuperscriptFollowsTheCurrentSize(t *testing.T) {
	cmds := []string{"tiny", "scriptsize", "footnotesize", "small", "normalsize",
		"large", "Large", "LARGE", "huge", "Huge"}
	digits := "0123456789"
	for _, tc := range []struct {
		opt  string
		want [10]float64
	}{
		// 10pt: \tiny 5, 7, 8, 9, 10, 12, 14.4, 17.28, 20.74, 24.88
		{"10pt", [10]float64{5, 5, 6, 6, 7, 8, 10, 12, 14.4, 17.28}},
		// 11pt: 6, 8, 9, 10, 10.95, 12, 14.4, 17.28, 20.74, 24.88
		{"11pt", [10]float64{5, 6, 6, 7, 8, 8, 10, 12, 14.4, 17.28}},
		// 12pt: 6, 8, 10, 10.95, 12, 14.4, 17.28, 20.74, 24.88, 24.88
		{"12pt", [10]float64{5, 6, 7, 8, 8, 10, 12, 14.4, 17.28, 17.28}},
	} {
		src := `\documentclass[` + tc.opt + `]{article}\begin{document}\noindent`
		for i, c := range cmds {
			src += `{\` + c + ` x\textsuperscript{` + string(digits[i]) + `}}`
		}
		src += `\end{document}`
		e, err := compile([]byte(src), Options{})
		if err != nil {
			t.Fatalf("%s: %v", tc.opt, err)
		}
		sizes := glyphSizes(e)
		for i, c := range cmds {
			if got := sizes[rune(digits[i])]; math.Abs(got-tc.want[i]) > 1e-3 {
				t.Errorf("[%s] \\textsuperscript under \\%s = %gpt, want %gpt", tc.opt, c, got, tc.want[i])
			}
		}
	}
}

// A font that cannot rescale, or has no size, keeps its size under \gotexsupsize.
func TestSupSizeUnscalable(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{}) // no atSize
	defer func() {      // a panic here is the guard missing: say so as an assertion
		if r := recover(); r != nil {
			t.Errorf("\\gotexsupsize on a non-scalable face panicked: %v", r)
		}
	}()
	if _, err := e.Run(`\gotexsupsize X`); err != nil {
		t.Fatal(err)
	}
	if got := e.curFont.sizePt(); got != 10 {
		t.Errorf("non-scalable \\gotexsupsize changed size to %g, want 10", got)
	}
	if got := scriptSizeOf(13); math.Abs(got-9.1) > 1e-9 {
		t.Errorf("an undeclared size 13 gives %g, want .7 of it (9.1)", got)
	}
}

// elsarticle.cls:171-173, :180-181 and :186-187 write the correspondence mark as
// $\ast$ (math, cmsy), not the text character U+2217 the engine wrote.
func TestElsarticleCorrespondenceMarkIsMathAst(t *testing.T) {
	src := `\documentclass{elsarticle}\begin{document}\begin{frontmatter}` +
		`\title{T}\author[a1]{AU\cormark[c1]}\cortext[c1]{CORNOTE}` +
		`\address[a1]{ADDR}\begin{abstract}ABS\end{abstract}\end{frontmatter}BODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var b strings.Builder
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case charNode:
				b.WriteRune(c.ch)
			case mathNode:
				b.WriteString("[" + c.src + "]")
			case *boxNode:
				walk(c.list)
			}
		}
	}
	for _, p := range e.Pages() {
		walk(p.list)
	}
	text := b.String()
	if strings.Contains(text, "∗") {
		t.Errorf("a U+2217 character is left in a correspondence mark: %q", text)
	}
	if !strings.Contains(text, `\ast`) {
		t.Errorf("no $\\ast$ math mark for \\cormark/\\cortext: %q", text)
	}
}
