package engine

import "testing"

// firstFrame returns the first frameNode reachable in a node tree, descending
// through boxes and into a frame's own inner box.
func firstFrame(nodes []node) (frameNode, bool) {
	for _, n := range nodes {
		switch c := n.(type) {
		case frameNode:
			return c, true
		case *boxNode:
			if fr, ok := firstFrame(c.list); ok {
				return fr, true
			}
		}
	}
	return frameNode{}, false
}

// \fbox{ab} frames a natural-width hbox: with spMock's 5pt letters the content is
// 10pt wide, and the frame adds fboxsep+fboxrule on every side.
func TestFbox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\fbox{ab}`); err != nil {
		t.Fatal(err)
	}
	fr, ok := firstFrame(e.mvl)
	if !ok {
		t.Fatal("no frameNode placed")
	}
	if fr.sep != fboxSep || fr.rule != fboxRule {
		t.Errorf("sep/rule = %d/%d, want %d/%d", fr.sep, fr.rule, fboxSep, fboxRule)
	}
	if want := 10 * unity; fr.inner.width != want {
		t.Errorf("inner width = %d sp, want %d (10pt)", fr.inner.width, want)
	}
	pad := fboxSep + fboxRule
	if want := 10*unity + 2*pad; fr.width() != want {
		t.Errorf("frame width = %d sp, want %d", fr.width(), want)
	}
	// spMock letters are 7pt tall, 2pt deep; the frame grows each by pad.
	if want := 7*unity + pad; fr.height() != want {
		t.Errorf("frame height = %d sp, want %d", fr.height(), want)
	}
	if want := 2*unity + pad; fr.depth() != want {
		t.Errorf("frame depth = %d sp, want %d", fr.depth(), want)
	}
}

// \framebox[50pt]{a} forces the content box to the requested width; the frame is
// then that width plus padding on both sides.
func TestFrameboxWidth(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\framebox[50pt]{a}`); err != nil {
		t.Fatal(err)
	}
	fr, ok := firstFrame(e.mvl)
	if !ok {
		t.Fatal("no frameNode placed")
	}
	if want := 50 * unity; fr.inner.width != want {
		t.Errorf("inner width = %d sp, want %d (50pt)", fr.inner.width, want)
	}
	pad := fboxSep + fboxRule
	if want := 50*unity + 2*pad; fr.width() != want {
		t.Errorf("frame width = %d sp, want %d", fr.width(), want)
	}
}

// \framebox with left/right alignment forces the same width; the fil glue that
// positions the content lives inside the packed inner box.
func TestFrameboxAlign(t *testing.T) {
	for _, pos := range []string{"l", "c", "r"} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(`\framebox[40pt][` + pos + `]{a}`); err != nil {
			t.Fatalf("pos %s: %v", pos, err)
		}
		fr, ok := firstFrame(e.mvl)
		if !ok {
			t.Fatalf("pos %s: no frameNode placed", pos)
		}
		if want := 40 * unity; fr.inner.width != want {
			t.Errorf("pos %s: inner width = %d sp, want %d", pos, fr.inner.width, want)
		}
	}
}

// alignList places fil glue on the correct side(s) for each position.
func TestAlignList(t *testing.T) {
	glyph := charNode{ch: 'x', width: unity}
	isFil := func(n node) bool {
		g, ok := n.(glueNode)
		return ok && g.spec.stretchOrder == 1
	}
	l := alignList([]node{glyph}, 'l')
	if len(l) != 2 || !isFil(l[1]) {
		t.Errorf("left: want glyph then fil, got %v", l)
	}
	r := alignList([]node{glyph}, 'r')
	if len(r) != 2 || !isFil(r[0]) {
		t.Errorf("right: want fil then glyph, got %v", r)
	}
	c := alignList([]node{glyph}, 'c')
	if len(c) != 3 || !isFil(c[0]) || !isFil(c[2]) {
		t.Errorf("center: want fil-glyph-fil, got %v", c)
	}
}
