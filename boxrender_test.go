package engine

import (
	"strings"
	"testing"
)

func renderBox0(t *testing.T, src string) string {
	e := New()
	if _, err := e.Run(src); err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return e.RenderBox(0, 0)
}

func mustContain(t *testing.T, svg, want string) {
	if !strings.Contains(svg, want) {
		t.Errorf("SVG missing %q\n---\n%s", want, svg)
	}
}

// The page's root group must set color="black", not only fill="black": math
// fragments (from go-tex/math) paint with fill="currentColor" so a formula can
// follow the surrounding text colour. currentColor reads the CSS `color`
// property, which on a host page (the playground) is a faint default — so without
// color="black" on the root every formula renders nearly invisible.
func TestRenderRootSetsColor(t *testing.T) {
	svg := renderBox0(t, `\setbox0=\hbox{\vrule width2pt height10pt}`)
	mustContain(t, svg, `<g fill="black" color="black">`)
}

// Two vertical rules 5pt apart in an hbox: exact rect positions and page size.
func TestRenderHBoxRules(t *testing.T) {
	svg := renderBox0(t, `\setbox0=\hbox{\vrule width2pt height10pt depth0pt\kern5pt\vrule width2pt height10pt}`)
	mustContain(t, svg, `width="9pt" height="10pt"`)                 // box is 9pt wide, 10pt tall
	mustContain(t, svg, `<rect x="0" y="0" width="2" height="10"/>`) // first bar
	mustContain(t, svg, `<rect x="7" y="0" width="2" height="10"/>`) // second bar, after 2pt+5pt kern
}

// A vbox stacks two horizontal rules with a 3pt kern between them.
func TestRenderVBoxRules(t *testing.T) {
	svg := renderBox0(t, `\setbox0=\vbox{\hrule width8pt height2pt\kern3pt\hrule width8pt height1pt}`)
	mustContain(t, svg, `width="8pt" height="6pt"`)                 // 8pt wide, 2+3+1 tall
	mustContain(t, svg, `<rect x="0" y="0" width="8" height="2"/>`) // top rule
	mustContain(t, svg, `<rect x="0" y="5" width="8" height="1"/>`) // bottom rule after 2+3
}

// Infinite glue in an \hbox to pushes the second bar flush right.
func TestRenderGlueSetPositions(t *testing.T) {
	svg := renderBox0(t, `\setbox0=\hbox to 20pt{\vrule width2pt height5pt\hskip0pt plus1fil\vrule width2pt height5pt}`)
	mustContain(t, svg, `width="20pt" height="5pt"`)
	mustContain(t, svg, `<rect x="0" y="0" width="2" height="5"/>`)  // left bar
	mustContain(t, svg, `<rect x="18" y="0" width="2" height="5"/>`) // 16pt fil gap ⇒ x=18
}

// A running-width \hrule inside a \vbox takes the box width (12pt from the inner
// hbox's kern). The empty hbox has zero height, so the rule sits at the top.
func TestRenderRunningRule(t *testing.T) {
	svg := renderBox0(t, `\setbox0=\vbox{\hbox{\kern12pt}\hrule height1pt}`)
	mustContain(t, svg, `width="12pt" height="1pt"`)                 // box width = inner hbox width
	mustContain(t, svg, `<rect x="0" y="0" width="12" height="1"/>`) // running-width rule spans 12pt
}
