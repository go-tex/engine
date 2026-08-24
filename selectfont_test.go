package engine

import "testing"

// \selectfont installs the font the current family/series/shape calls for. This
// engine keeps one text face per family rather than NFSS's tables, so it
// re-selects the base (roman, \normalsize) face — and that is the part that
// matters in practice: a package switches to \nullfont around material it wants
// measured but not set, then brings the real font back with \selectfont. With
// \selectfont doing nothing, everything typeset afterwards was set in a font
// with no characters at all. pgf does exactly this for every picture, so every
// node's text came out empty.
func TestSelectfontRestoresTheFont(t *testing.T) {
	e := New()
	if err := e.LoadPlain(); err != nil {
		t.Fatal(err)
	}
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(testFont(t))
	out, err := e.Run(`\setbox0=\hbox{A}\message{[avant:\the\wd0]}` +
		`{\nullfont\global\setbox1=\hbox{A}\message{[nullfont:\the\wd1]}` +
		`\selectfont\global\setbox2=\hbox{A}\message{[apres selectfont:\the\wd2]}}`)
	if err != nil {
		t.Fatal(err)
	}
	got := trimNL(out)
	if got == "" {
		t.Fatal("no output")
	}
	// The exact width depends on the face; what matters is that \nullfont makes it
	// zero and \selectfont brings it back to what it was.
	w0, w1, w2 := e.getBox(0), e.getBox(1), e.getBox(2)
	if w0 == nil || w1 == nil || w2 == nil {
		t.Fatalf("a box is void (%v)", got)
	}
	if w0.width == 0 {
		t.Fatalf("the test font has no width: %v", got)
	}
	if w1.width != 0 {
		t.Errorf("\\nullfont still set characters: %d (%s)", w1.width, got)
	}
	if w2.width != w0.width {
		t.Errorf("\\selectfont did not restore the font: %d, want %d (%s)", w2.width, w0.width, got)
	}
}

// The font switch is scoped like any other, so a group restores what was in
// force before it.
func TestSelectfontIsScoped(t *testing.T) {
	e := New()
	e.LoadPlain()
	e.LoadLaTeX()
	e.SetFont(testFont(t))
	if _, err := e.Run(`{\nullfont{\selectfont\global\setbox0=\hbox{A}}\global\setbox1=\hbox{A}}`); err != nil {
		t.Fatal(err)
	}
	if b := e.getBox(0); b == nil || b.width == 0 {
		t.Error("\\selectfont did not take effect inside its group")
	}
	if b := e.getBox(1); b == nil || b.width != 0 {
		t.Error("\\selectfont escaped its group: \\nullfont should be back in force")
	}
}

// With no base font bound (a bare engine), \selectfont is simply inert rather
// than clearing the current font.
func TestSelectfontWithoutABaseFont(t *testing.T) {
	e := New()
	e.LoadPlain()
	e.LoadLaTeX()
	if _, err := e.Run(`\selectfont\message{[ok]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[ok]" {
		t.Errorf("= %q", got)
	}
}

// \voidb@x is the kernel's permanently empty box: \setbox<n>=\box\voidb@x is how
// a package empties a register. Without it that idiom read register 0 instead
// and stole whatever was in it.
func TestVoidBox(t *testing.T) {
	e := New()
	e.LoadPlain()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\makeatletter\setbox0=\hbox{\kern5pt}\setbox3=\hbox{\kern7pt}` +
		`\setbox3=\box\voidb@x\message{[b0=\the\wd0][b3=\the\wd3]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[b0=5.0pt][b3=0.0pt]" {
		t.Errorf("= %q, want box 3 emptied and box 0 untouched", got)
	}
}

// The boxes above are set \global on purpose: a plain \setbox inside a group is
// restored when the group closes (measured against a real TeX), so a local one
// would be gone before the assertion could read it. What is under test is the
// scope of the FONT, and the box is only the ruler.
