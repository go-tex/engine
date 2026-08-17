package engine

import (
	"strings"
	"testing"
)

// \color also tells the drawing driver, when a package is drawing. This engine
// resolves colour itself and stamps it on each glyph, which is all a page of text
// needs; but a drawing package puts its marks on the page through its own driver
// and asks the colour package — through \color — what colour to use. TikZ's
// shorthand for a colour option, \draw[red], goes exactly that way, so without
// the hand-off it drew in black while draw=red worked.
func TestColorTellsTheDriver(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	// Stand in for the drawing package: a \pgfsetcolor that records its argument,
	// and a picture that is open.
	if _, err := e.Run(`\makeatletter\def\pgfsetcolor#1{\message{[pilote:#1]}}` +
		`\newif\ifpgfpicture\pgfpicturetrue\color{red}X`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[pilote:red]" {
		t.Errorf("= %q, want the colour handed to the driver", got)
	}
	// The engine's own colour is set too: both drivers still paint the text.
	if e.curColor != 0xFF0000 {
		t.Errorf("current colour = %06x, want ff0000", e.curColor)
	}
}

// Outside a picture, and with no drawing package at all, nothing extra happens —
// an ordinary document is untouched.
func TestColorWithoutAPicture(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\makeatletter\def\pgfsetcolor#1{\message{[pilote:#1]}}` +
		`\newif\ifpgfpicture\color{red}X`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "" {
		t.Errorf("= %q, want nothing outside a picture", got)
	}
	e2 := New()
	e2.SetFont(spMock{})
	if err := e2.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e2.Run(`\color{blue}AB`); err != nil { // no drawing package loaded
		t.Fatal(err)
	}
	if got := glyphString(e2.mvl); got != "AB" {
		t.Errorf("typeset %q, want AB", got)
	}
	if e2.curColor != 0x0000FF {
		t.Errorf("current colour = %06x, want 0000ff", e2.curColor)
	}
}

// A colour expression is handed on as written, so the drawing package resolves it
// with the same rules the text does.
func TestColorExpressionReachesTheDriver(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\makeatletter\def\pgfsetcolor#1{\message{[#1]}}` +
		`\newif\ifpgfpicture\pgfpicturetrue\color{red!50!blue}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[red!50!blue]" {
		t.Errorf("= %q", got)
	}
}

// An empty or missing colour name hands nothing on.
func TestColorEmptyName(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\makeatletter\def\pgfsetcolor#1{\message{[#1]}}` +
		`\newif\ifpgfpicture\pgfpicturetrue\color{}\color X`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "" {
		t.Errorf("= %q, want nothing", got)
	}
}

// The hook is only installed by the LaTeX layer: a bare engine hands nothing on
// and behaves exactly as it did.
func TestColorHookNeedsTheLaTeXLayer(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\color{red}A`); err != nil {
		t.Fatal(err)
	}
	if got := glyphString(e.mvl); got != "A" {
		t.Errorf("typeset %q", got)
	}
}

// The kernel's hook itself: it asks the drawing package only when one is loaded
// and it is drawing.
func TestPgfColorHookDefinition(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	m := e.eq["gotex@pgfcolor"]
	if m == nil {
		t.Fatal("the hook is not defined")
	}
	body := e.toksToString(m.body)
	for _, want := range []string{`\ifdefined`, `\pgfsetcolor`, `\ifpgfpicture`} {
		if !strings.Contains(body, want) {
			t.Errorf("the hook does not consult %s: %s", want, body)
		}
	}
}
