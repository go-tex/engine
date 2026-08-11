package engine

import (
	"strings"
	"testing"
)

// mixRGB blends two colours by percentage per channel.
func TestMixRGB(t *testing.T) {
	// 50% red + 50% blue = (128,0,128).
	if got := mixRGB(0xFF0000, 0x0000FF, 50); got != 0x800080 {
		t.Errorf("red!50!blue = %06X, want 800080", got)
	}
	// 100% keeps a; 0% keeps b.
	if got := mixRGB(0xFF0000, 0x00FF00, 100); got != 0xFF0000 {
		t.Errorf("mix 100 = %06X, want FF0000", got)
	}
	if got := mixRGB(0xFF0000, 0x00FF00, 0); got != 0x00FF00 {
		t.Errorf("mix 0 = %06X, want 00FF00", got)
	}
}

// resolveColor evaluates xcolor mix expressions.
func TestResolveColorExpr(t *testing.T) {
	e := New()
	if got := e.resolveColor("red!50!blue"); got != 0x800080 {
		t.Errorf("red!50!blue = %06X, want 800080", got)
	}
	// red!50 = red!50!white = (255,128,128).
	if got := e.resolveColor("red!50"); got != 0xFF8080 {
		t.Errorf("red!50 = %06X, want FF8080", got)
	}
	// chained: (red!50!blue)!100 keeps the mix.
	if got := e.resolveColor("red!50!blue!100!green"); got != 0x800080 {
		t.Errorf("chain = %06X, want 800080", got)
	}
	// plain name still works.
	if got := e.resolveColor("blue"); got != 0x0000FF {
		t.Errorf("blue = %06X", got)
	}
}

// \colorlet defines a named colour from an expression.
func TestColorlet(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\colorlet{myviolet}{red!50!blue}`); err != nil {
		t.Fatal(err)
	}
	if got := e.resolveColor("myviolet"); got != 0x800080 {
		t.Errorf("myviolet = %06X, want 800080", got)
	}
}

// The cmyk model converts to RGB.
func TestCMYK(t *testing.T) {
	// pure cyan: c=1 → (0,255,255).
	if got := parseColorSpec("cmyk", "1,0,0,0"); got != 0x00FFFF {
		t.Errorf("cmyk cyan = %06X, want 00FFFF", got)
	}
	// k=1 → black regardless.
	if got := parseColorSpec("cmyk", "0,0,0,1"); got != 0x000000 {
		t.Errorf("cmyk black = %06X, want 000000", got)
	}
}

// \pagecolor sets the page background, which the SVG driver paints.
func TestPagecolor(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if e.pageFill() != "white" {
		t.Errorf("default page fill = %q, want white", e.pageFill())
	}
	if _, err := e.Run(`\pagecolor{yellow}Body.`); err != nil {
		t.Fatal(err)
	}
	if !e.hasPageColor || e.pageColor != 0xFFFF00 {
		t.Errorf("pagecolor not set: has=%v c=%06X", e.hasPageColor, e.pageColor)
	}
	svg := e.RenderPage(72)
	if !strings.Contains(svg, `fill="#ffff00"`) {
		t.Errorf("page SVG should fill yellow background, got: %.120s", svg)
	}
	// \nopagecolor clears it.
	if _, err := e.Run(`\nopagecolor`); err != nil {
		t.Fatal(err)
	}
	if e.hasPageColor {
		t.Error("\\nopagecolor should clear the page colour")
	}
}

// \normalcolor resets the current colour to black (group-scoped).
func TestNormalcolor(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\color{red}\normalcolor`); err != nil {
		t.Fatal(err)
	}
	if e.curColor != 0 {
		t.Errorf("after \\normalcolor curColor = %06X, want 0", e.curColor)
	}
}
