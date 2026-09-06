package engine

import "testing"

// A display places \abovedisplayskip glue before its box and \belowdisplayskip
// after it — the leading TeX puts around a display rather than the ordinary
// interline glue. The two skips carry the values in the like-named registers.
func TestDisplaySkipsAroundDisplay(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 200 * unity
	// Allocate and set the two display-skip registers, then a display between two
	// text paragraphs; the display's box must be flanked by glue of those values.
	src := `\newskip\abovedisplayskip \abovedisplayskip=10pt` +
		`\newskip\belowdisplayskip \belowdisplayskip=7pt` +
		`before $$x$$ after\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	dispIdx := -1
	for i, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			for _, c := range b.list {
				if _, ok := c.(mathNode); ok {
					dispIdx = i
				}
			}
		}
	}
	if dispIdx <= 0 || dispIdx+1 >= len(e.mvl) {
		t.Fatalf("display box not found with glue on both sides (idx %d of %d)", dispIdx, len(e.mvl))
	}
	above, ok := e.mvl[dispIdx-1].(glueNode)
	if !ok || above.spec.width != 10*unity {
		t.Errorf("above-display glue = %+v, want width 10pt", e.mvl[dispIdx-1])
	}
	below, ok := e.mvl[dispIdx+1].(glueNode)
	if !ok || below.spec.width != 7*unity {
		t.Errorf("below-display glue = %+v, want width 7pt", e.mvl[dispIdx+1])
	}
}

// namedSkip reads a \newskip register's glue, and returns the zero glue for a name
// that is not a skip register (undefined, or bound to something else).
func TestNamedSkip(t *testing.T) {
	e := New()
	if _, err := e.Run(`\newskip\myskip \myskip=3pt plus 1pt \def\notaskip{x}`); err != nil {
		t.Fatal(err)
	}
	if g := e.namedSkip("myskip"); g.width != 3*unity || g.stretch != 1*unity {
		t.Errorf("namedSkip(myskip) = %+v, want 3pt plus 1pt", g)
	}
	if g := e.namedSkip("notaskip"); g != (glueSpec{}) {
		t.Errorf("namedSkip(non-skip macro) = %+v, want zero glue", g)
	}
	if g := e.namedSkip("nosuchthing"); g != (glueSpec{}) {
		t.Errorf("namedSkip(undefined) = %+v, want zero glue", g)
	}
}

// placeDisplay is a no-op on an empty box list and skips nil boxes while still
// emitting the surrounding skips.
func TestPlaceDisplayEmptyAndNil(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	before := len(e.mvl)
	e.placeDisplay(nil)
	if len(e.mvl) != before {
		t.Errorf("placeDisplay(nil) changed the vertical list: %d → %d", before, len(e.mvl))
	}
	e.placeDisplay([]*boxNode{nil})
	// Two skips go on the list even though the (nil) box itself is not appended.
	boxes := 0
	for _, n := range e.mvl {
		if _, ok := n.(*boxNode); ok {
			boxes++
		}
	}
	if boxes != 0 {
		t.Errorf("a nil display box was appended: %d boxes on the list", boxes)
	}
}

// \parskip glue is inserted between paragraphs, but suppressed for the paragraph
// that resumes right after a display (which is one paragraph in TeX), and restored
// by an explicit \par between the display and the following text.
func TestParskipBetweenParagraphs(t *testing.T) {
	countGlue := func(src string) (glues int, list []node) {
		e := New()
		e.SetFont(spMock{})
		e.hsize = 200 * unity
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		for _, n := range e.mvl {
			if g, ok := n.(glueNode); ok && g.spec.width == 5*unity {
				glues++
			}
		}
		return glues, e.mvl
	}
	pre := `\newskip\parskip \parskip=5pt `
	// Two paragraphs ⇒ one \parskip glue (none before the very first paragraph).
	if n, _ := countGlue(pre + `one\par two\par`); n != 1 {
		t.Errorf("two paragraphs: got %d parskip glues, want 1", n)
	}
	// A display then continuing text: the resumed paragraph gets no \parskip.
	nd, _ := countGlue(pre + `\newskip\abovedisplayskip\newskip\belowdisplayskip one $$x$$ two\par`)
	if nd != 0 {
		t.Errorf("text resuming after a display: got %d parskip glues, want 0", nd)
	}
	// An explicit \par after the display restores \parskip for the next paragraph.
	np, _ := countGlue(pre + `\newskip\abovedisplayskip\newskip\belowdisplayskip one $$x$$\par two\par`)
	if np != 1 {
		t.Errorf("explicit \\par after a display: got %d parskip glues, want 1", np)
	}
}

// The rows of a multi-line display are set \jot further apart than ordinary
// lines. Measured against real LaTeX on an align of 1 to 4 rows, our cost rose
// 13.6pt per row — a plain \baselineskip — where the reference rises 16.5. With
// \jot the slope is 16.6.
func TestDisplayRowsAreJotApart(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if v := e.jotSkip(); v != 3*unity {
		t.Errorf("jot = %d, want %d (3pt): \\newdimen\\jot is allocated but never set, and a zero reading is not a request for no leading", v, 3*unity)
	}
	// A document that sets \jot is followed.
	if _, err := e.Run(`\jot=5pt`); err != nil {
		t.Fatal(err)
	}
	if v := e.jotSkip(); v != 5*unity {
		t.Errorf("after \\jot=5pt, jot = %d, want %d", v, 5*unity)
	}
}
