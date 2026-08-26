package engine

import (
	"strings"
	"testing"
)

// findGlue returns the first glue node in a box's list (nil if none).
func findGlue(b *boxNode) *glueNode {
	if b == nil {
		return nil
	}
	for _, n := range b.list {
		if g, ok := n.(glueNode); ok {
			gg := g
			return &gg
		}
	}
	return nil
}

// \hspace{d} inside an \hbox contributes fixed horizontal glue of width d, so
// the box width is the sum of the letters and the space. \hspace*{d} is the same.
func TestHspaceInBox(t *testing.T) {
	for _, src := range []string{
		`\setbox0=\hbox{a\hspace{4pt}b}`,
		`\setbox0=\hbox{a\hspace*{4pt}b}`,
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		g := findGlue(e.box[0])
		if g == nil {
			t.Fatalf("%q: no glue node produced", src)
		}
		if g.spec.width != 4*unity || g.spec.stretch != 0 || g.leader != leaderNone {
			t.Errorf("%q: glue = %+v, want fixed 4pt", src, g.spec)
		}
		// a(5) + hspace(4) + b(5) = 14pt
		if e.box[0].width != 14*unity {
			t.Errorf("%q: box width %d want 14pt", src, e.box[0].width)
		}
	}
}

// \vspace{d} inside a \vbox contributes fixed vertical glue of width d.
func TestVspaceInBox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\vbox{\hbox{a}\vspace{5pt}\hbox{b}}`); err != nil {
		t.Fatal(err)
	}
	g := findGlue(e.box[0])
	if g == nil {
		t.Fatal("no glue node produced by \\vspace")
	}
	if g.spec.width != 5*unity {
		t.Errorf("\\vspace glue width %d want 5pt", g.spec.width)
	}
	// vbox height carries the running depth AND the interline glue an explicit
	// \vskip does not suppress: a.height(7) + a.depth(2) + vspace(5) +
	// interline(\baselineskip 12 − prevdepth 2 − height 7 = 3) + b.height(7) = 24pt;
	// depth = last b.depth(2). Checked against real TeX with \baselineskip=12pt,
	// which gives \ht = 24.0pt.
	if e.box[0].height != 24*unity {
		t.Errorf("vbox height %d want 24pt", e.box[0].height)
	}
}

// \vspace at top level (the primitive path) contributes vertical glue to the
// main vertical list.
func TestVspaceTopLevel(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	if _, err := e.Run(`\vspace{7pt}`); err != nil {
		t.Fatal(err)
	}
	if len(e.mvl) != 1 {
		t.Fatalf("main vertical list = %d nodes, want 1", len(e.mvl))
	}
	g, ok := e.mvl[0].(glueNode)
	if !ok || g.spec.width != 7*unity {
		t.Errorf("mvl[0] = %+v, want glue of 7pt", e.mvl[0])
	}
}

// \hrulefill inside \hbox to N{...} produces fill glue (order 2) tagged as a rule
// leader; it stretches to absorb the whole target, filling the line.
func TestHrulefillInBox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox to 20pt{a\hrulefill b}`); err != nil {
		t.Fatal(err)
	}
	b := e.box[0]
	if b.width != 20*unity {
		t.Fatalf("box width %d want 20pt", b.width)
	}
	g := findGlue(b)
	if g == nil || g.leader != leaderRule {
		t.Fatalf("no rule-leader glue, got %+v", g)
	}
	if g.spec.stretchOrder != 2 {
		t.Errorf("hrulefill stretch order %d want 2", g.spec.stretchOrder)
	}
	// set width fills the gap: 20 - a(5) - b(5) = 10pt
	if w := b.setWidth(g.spec); w != 10*unity {
		t.Errorf("hrulefill set width %d want 10pt", w)
	}
}

// \dotfill likewise makes fill glue tagged as a dot leader.
func TestDotfillInBox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox to 30pt{\dotfill}`); err != nil {
		t.Fatal(err)
	}
	g := findGlue(e.box[0])
	if g == nil || g.leader != leaderDots {
		t.Fatalf("no dot-leader glue, got %+v", g)
	}
	if w := e.box[0].setWidth(g.spec); w != 30*unity {
		t.Errorf("dotfill set width %d want 30pt", w)
	}
}

// The top-level \hrulefill primitive path renders a filled rule spanning the
// stretched glue (a <rect> beyond the page background).
func TestHrulefillRendersRule(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox to 40pt{\hrulefill}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderBox(0, 1)
	// background rect + one leader rect = at least two <rect
	if n := strings.Count(svg, "<rect"); n < 2 {
		t.Errorf("expected a leader rule rect, got %d rects in %q", n, svg)
	}
}

// dotFont is spMock with a visible glyph for '.', so the dot leader paints.
type dotFont struct{ spMock }

func (dotFont) glyphPathAt(r rune) string {
	if r == '.' {
		return "DOT"
	}
	return ""
}

// \dotfill tiles the dot glyph across its set width (one dot per .44em cell).
func TestDotfillRendersDots(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(dotFont{})
	// 45pt / (.44 * 10pt = 4.4pt) = 10.2 → 10 dots
	if _, err := e.Run(`\setbox0=\hbox to 45pt{\dotfill}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderBox(0, 1)
	if n := strings.Count(svg, "DOT"); n != 10 {
		t.Errorf("dot leader painted %d dots, want 10; svg=%q", n, svg)
	}
}

// The \hspace/\hrulefill/\dotfill primitives also work at the top level (the
// primitive path), both when a paragraph is already open and when they start one.
func TestSpacingPrimsTopLevel(t *testing.T) {
	for _, src := range []string{
		`\hspace{5pt}`,   // starts a paragraph (not yet in horizontal mode)
		`a\hspace{5pt}b`, // joins an open paragraph
		`x\hrulefill y`,  // rule leader on an open line
		`p\dotfill q`,    // dot leader on an open line
		`\hrulefill\par`, // starts a paragraph, then ends it
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

// A leader glue with no set width (natural pack, nothing to fill) paints nothing:
// only the page-background rect remains.
func TestLeaderZeroWidthNoPaint(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{\hrulefill}`); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(e.RenderBox(0, 1), "<rect"); n != 1 {
		t.Errorf("zero-width rule leader painted %d rects, want 1 (background only)", n)
	}
}

// A dot leader whose font lacks a '.' glyph (or when there is no font) paints no
// dots: the tiling loop is guarded on a usable glyph path.
func TestDotLeaderWithoutGlyph(t *testing.T) {
	// spMock has a '.' width but returns an empty glyph path.
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox to 30pt{\dotfill}`); err != nil {
		t.Fatal(err)
	}
	if svg := e.RenderBox(0, 1); strings.Count(svg, "<path") != 0 {
		t.Errorf("dot leader painted paths with an empty glyph: %q", svg)
	}
	// With no current font the leader must also paint nothing (no panic).
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\setbox0=\hbox to 30pt{\dotfill}`); err != nil {
		t.Fatal(err)
	}
	e2.SetFont(nil) // render with no font
	_ = e2.RenderBox(0, 1)
}

// dotLeaderGeom fits floor(w / .44em) cells and yields none for a non-positive
// width or font size (the zero-size-font guard).
func TestDotLeaderGeom(t *testing.T) {
	if n, cell := dotLeaderGeom(45, 10); n != 10 || cell != 4.4 {
		t.Errorf("dotLeaderGeom(45,10) = %d,%v want 10,4.4", n, cell)
	}
	if n, _ := dotLeaderGeom(0, 10); n != 0 { // non-positive width
		t.Errorf("dotLeaderGeom(0,10) n = %d want 0", n)
	}
	if n, _ := dotLeaderGeom(-5, 10); n != 0 { // negative width
		t.Errorf("dotLeaderGeom(-5,10) n = %d want 0", n)
	}
	if n, _ := dotLeaderGeom(45, 0); n != 0 { // zero-size font
		t.Errorf("dotLeaderGeom(45,0) n = %d want 0", n)
	}
}

// An ordinary (non-leader) glue with a real set width paints nothing but advances
// the cursor — the leaderNone path of the renderer.
func TestOrdinaryGlueNoLeader(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox to 30pt{a\hfil b}`); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(e.RenderBox(0, 1), "<rect"); n != 1 {
		t.Errorf("ordinary glue painted %d rects, want 1 (background only)", n)
	}
}
