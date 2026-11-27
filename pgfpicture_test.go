// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdfkit/pdfkit"
)

// A closed stream of specials is painted before the glyph or rule that follows
// it, so the mark set after a drawing lies over it (beamer's ball number on its
// sphere, go-tex/engine#884); a stream whose <g> is still open waits.
func TestClosedSpecialsArePaintedBeforeWhatFollows(t *testing.T) {
	doc := pdfkit.New(pdfkit.Options{})
	d := &pdfDraw{p: doc.AddPage(pdfkit.NewPageSize(100, 100)), pageH: 100}
	d.specials = []string{`<g fill="red"><rect width="5" height="5"/></g>`}
	d.paintClosedSpecials()
	if len(d.specials) != 0 {
		t.Errorf("a closed stream was kept for the end of the page: %q", d.specials)
	}
	d.specials, d.specialGroups = []string{`<g fill="red">`}, 1
	d.paintClosedSpecials()
	if len(d.specials) != 1 {
		t.Errorf("an open <g> was painted before it closed")
	}
}

const pgfStubsOn = "\\catcode`\\@=11 \\gotex@pgfstubs "

// \pgfuseshading of a radial shading places a box of the shading's size: a square
// twice the last stop's radius (pgfcoreshade.code.tex:479-488), times the scale.
func TestARadialShadingHasItsSize(t *testing.T) {
	got := ckRun(t, pgfStubsOn+
		`\pgfdeclareradialshading[bg]{zz}{\pgfpoint{0pt}{0pt}}{color(0pt)=(red);color(2pt)=(blue);color(3pt)=(white)}`+
		`\setbox0\hbox{\pgfuseshading{zz}}\message{[\the\wd0/\the\ht0/\the\dp0]}`+
		`\setbox0\hbox{\def\gotex@pgfscale{2}\pgfuseshading{zz}}\message{[\the\wd0]}`+
		`\setbox0\hbox{\pgfuseshading{undeclared}}\message{[\the\wd0]}`)
	for _, want := range []string{"[6.0pt/6.0pt/0.0pt]", "[12.0pt]", "[0.0pt]"} {
		if !strings.Contains(got, want) {
			t.Errorf("messages %q lack %q", got, want)
		}
	}
}

// A body the scanner cannot read whole is refused, never typeset in part.
func TestCleanPgfTextBodyRefusesWhatItCannotRead(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"no closing end", `\pgftext{X}`},
		{"group left open", `{\pgftext{X}`},
		{"unknown command in a group", `{\foo}\pgftext{X}\end`},
		{"unknown command", `\foo\pgftext{X}\end`},
		{"option never closed", `\color[rgb`},
		{"argument missing", `\pgftext`},
		{"argument missing before end", `\pgftext\end`},
		{"option never closed in a group", `{\color[rgb}\pgftext{X}\end`},
	} {
		if _, _, ok := cleanPgfTextBody(tokenizeTeX(c.src), true); ok {
			t.Errorf("%s: %q was accepted", c.name, c.src)
		}
	}
	out, hasText, ok := cleanPgfTextBody(tokenizeTeX(`{\color[rgb]{1,0,0} \pgftext{X}} \end`), true)
	if !ok || !hasText || len(out) == 0 {
		t.Errorf("a valid body was refused: ok=%v text=%v", ok, hasText)
	}
}

// cleanPgfTextBody leaves an unclosed [..] to the argument read that follows it,
// so every command taking an option must take an argument.
func TestEveryPgfTextCommandWithAnOptionTakesAnArgument(t *testing.T) {
	for name, cmd := range pgfTextCommands {
		if cmd.opt && cmd.args == 0 {
			t.Errorf("\\%s takes [..] and no argument: an unclosed [ would be accepted", name)
		}
	}
}

// A shading with no stop it can read places nothing, and one whose radii are
// not positive places no box of negative width.
func TestAShadingWithNoSizePlacesNothing(t *testing.T) {
	got := ckRun(t, pgfStubsOn+
		`\pgfdeclareradialshading{none}{\pgfpoint{0pt}{0pt}}{junk}`+
		`\pgfdeclareradialshading{neg}{\pgfpoint{0pt}{0pt}}{color(-3pt)=(red);color(-1pt)=(blue)}`+
		`\setbox0\hbox{\pgfuseshading{none}}\message{[\the\wd0]}`+
		`\setbox0\hbox{\pgfuseshading{neg}}\message{[\the\wd0]}`)
	if strings.Count(got, "[0.0pt]") != 2 {
		t.Errorf("messages %q, want two empty boxes", got)
	}
}

// specialTexts collects the driver literals in a box, depth first.
func specialTexts(b *boxNode) []string {
	var out []string
	for _, n := range b.list {
		switch c := n.(type) {
		case specialNode:
			out = append(out, c.text)
		case *boxNode:
			out = append(out, specialTexts(c)...)
		}
	}
	return out
}

// The disk ends at the next-to-last stop, or at the only one, and takes the
// colour of the last stop inside its edge, or its own when none is.
func TestTheShadingDiskEndsAtTheNextToLastStop(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(pgfStubsOn +
		`\pgfdeclareradialshading{three}{\pgfpoint{0pt}{0pt}}{color(0pt)=(red);color(1pt)=(green);color(2pt)=(blue);color(3pt)=(white)}` +
		`\pgfdeclareradialshading{one}{\pgfpoint{0pt}{0pt}}{color(3pt)=(blue)}` +
		`\setbox1\hbox{\pgfuseshading{three}}\setbox2\hbox{\pgfuseshading{one}}`); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		box        int
		want, fill string
	}{
		{1, `r="2"`, "#00FF00"}, // edge at 2pt, coloured by the 1pt stop
		{2, `r="3"`, "#0000FF"}, // one stop: it is the edge and the colour
	} {
		got := strings.Join(specialTexts(e.box[c.box]), " ")
		if !strings.Contains(got, c.want) || !strings.Contains(strings.ToUpper(got), c.fill) {
			t.Errorf("box %d: %q, want %s filled %s", c.box, got, c.want, c.fill)
		}
	}
}

// The stops are parsed with their models, sorted by radius, and the disk takes
// the colour of the last stop inside its edge.
func TestShadeStops(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	stops := e.parseShadeStops(tokenizeTeX(`color(2pt)=(blue); rgb(0pt)=(1,0,0);gray(3pt)=(1); junk; color(x)=(red)`))
	if len(stops) != 3 || stops[0].r != 0 || stops[1].color != "blue" || stops[2].r != 3*65536 {
		t.Fatalf("stops = %+v", stops)
	}
	if e.resolveColor(stops[0].color) != 0xFF0000 || e.resolveColor(stops[2].color) != 0xFFFFFF {
		t.Errorf("rgb/gray stops resolve to %06X/%06X", e.resolveColor(stops[0].color), e.resolveColor(stops[2].color))
	}
}

// A declared-size picture whose body is only \pgftext is typeset in the box pgf
// declares, its text kept; one with anything else is gobbled as before.
func TestAPgfTextPictureKeepsItsText(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(pgfStubsOn +
		`\setbox1\hbox{\begin{pgfpicture}{-5pt}{-2pt}{5pt}{6pt}` +
		` {\pgftransformshift{\pgfpoint{0pt}{1pt}} \pgftext{QQ}} \end{pgfpicture}}` +
		`\setbox2\hbox{\begin{pgfpicture}{0pt}{0pt}{4pt}{4pt}\pgfpathcircle{\pgfpoint{0pt}{0pt}}{1pt}\pgftext{RR}\end{pgfpicture}}` +
		`\setbox3\hbox{\begin{pgfpicture}{0pt}{0pt}{4pt}{4pt}\pgftext[left]{SS}\end{pgfpicture}}` +
		`\setbox4\hbox{\begin{pgfpicture}{0pt}{0pt}{4pt}{4pt}TT\pgftext{UU}\end{pgfpicture}}` +
		`\setbox5\hbox{\begin{pgfpicture}{0pt}{0pt}{4pt}{4pt}\usebeamercolor\end{pgfpicture}}`); err != nil {
		t.Fatal(err)
	}
	pic, ok := e.box[1].list[0].(*boxNode)
	if !ok || pic.width != 10*65536 || pic.height != 8*65536 || pic.depth != 0 {
		t.Errorf("declared box: %+v, want 10x8pt, depth 0", e.box[1].list[0])
	}
	if got := boxText(e.box[1]); !strings.Contains(got, "QQ") {
		t.Errorf("the \\pgftext text was lost: %q", got)
	}
	for i, gone := range []string{"RR", "SS", "TT", "UU"} {
		if got := boxText(e.box[i+2]); strings.Contains(got, gone) {
			t.Errorf("%s: a body with more than text must stay gobbled: %q", gone, got)
		}
	}
}

// The text sits centred on the origin: the origin is -x0 from the left and -y0
// above the baseline, and \pgftransformshift moves it.
func TestPgfTextIsCentredOnTheOrigin(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(pgfStubsOn + `\setbox1\hbox{\begin{pgfpicture}{-5pt}{-2pt}{5pt}{6pt}` +
		`\pgftransformshift{\pgfpoint{1pt}{3pt}}\pgftext{\vrule width 4pt height 2pt depth 0pt}\end{pgfpicture}}`); err != nil {
		t.Fatal(err)
	}
	pic := e.box[1]
	if pic == nil || len(pic.list) == 0 {
		t.Fatal("no picture box")
	}
	inner, ok := pic.list[0].(*boxNode)
	if !ok {
		t.Fatalf("first item = %T, want the picture's own box", pic.list[0])
	}
	// The text's box: hbox to 0pt{\kern ox \kern -wd/2 \raise ...}. Origin x is 5+1=6pt,
	// so the 4pt rule starts at 4pt; origin y is 2+3=5pt and the rule is raised by
	// 5pt - (2pt-0pt)/2 = 4pt.
	var x, lift int
	var found bool
	var walk func(b *boxNode, x0 int)
	walk = func(b *boxNode, x0 int) {
		cx := x0
		for _, n := range b.list {
			switch c := n.(type) {
			case kernNode:
				cx += c.width
			case *boxNode:
				for _, m := range c.list {
					if r, isRule := m.(ruleNode); isRule && r.width == 4*65536 {
						x, lift, found = cx, -c.shift, true
					}
				}
				walk(c, cx)
				cx += c.width
			}
		}
	}
	walk(inner, 0)
	if !found {
		t.Fatalf("the text's rule was not found in %#v", inner)
	}
	if x != 4*65536 || lift != 4*65536 {
		t.Errorf("text at x=%.2fpt raised %.2fpt, want 4pt and 4pt", float64(x)/65536, float64(lift)/65536)
	}
}

// beamer's numbered ball items keep their numbers and draw their balls
// (go-tex/engine#884), with the real class.
func TestBeamerBallItemsKeepTheirNumbers(t *testing.T) {
	tree := os.Getenv("GOTEX_TEXMF")
	if tree == "" {
		tree = "/Users/Shared/gotex/measure/texmf"
	}
	if _, err := os.Stat(filepath.Join(tree, "beamer.cls")); err != nil {
		t.Skip("no real beamer.cls under GOTEX_TEXMF: the emulation has no ball template")
	}
	t.Setenv("GOTEX_TEXMF", tree)
	src := `\documentclass{beamer}\setbeamertemplate{items}[ball]\begin{document}` +
		`\begin{frame}{T}\begin{enumerate}\item AAAA\item BBBB\end{enumerate}\end{frame}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	txt := mvlText(pages[0].list)
	i1, ia, i2, ib := strings.Index(txt, "1"), strings.Index(txt, "AAAA"), strings.Index(txt, "2"), strings.Index(txt, "BBBB")
	if i1 < 0 || i2 < 0 || i1 > ia || i2 > ib || ia > i2 {
		t.Errorf("want 1 AAAA 2 BBBB in order: %q", txt)
	}
}
