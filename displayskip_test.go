package engine

import "testing"

// A display places \abovedisplayskip glue before its box and \belowdisplayskip
// after it, carrying the values in the like-named registers — and the ordinary
// interline glue stays. TeX ADDS the two skips to it rather than replacing it:
// after appending \abovedisplayskip it contributes the display box through
// append_to_vlist like any other (tex.web:22602), which puts
// baselineskip-prev_depth-height(b) in front of it, and it never sets prev_depth
// to ignore_depth around a display. So the list above the box reads
// [\abovedisplayskip][interline][box].
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
	if dispIdx < 2 {
		t.Fatalf("no room for both glues above the display (idx %d)", dispIdx)
	}
	above, ok := e.mvl[dispIdx-2].(glueNode)
	if !ok || above.spec.width != 10*unity {
		t.Errorf("above-display glue = %+v, want width 10pt", e.mvl[dispIdx-2])
	}
	// The interline glue TeX would have put there anyway, still there.
	if _, ok := e.mvl[dispIdx-1].(glueNode); !ok {
		t.Errorf("no interline glue between \\abovedisplayskip and the display box: %+v", e.mvl[dispIdx-1])
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

// An ALIGNMENT display — align, gather, multline — carries a full \baselineskip
// above its first row that an ordinary display does not. TeX does not contribute an
// alignment through append_to_vlist at all: it appends \abovedisplayskip and splices
// the alignment's own rows in directly (tex.web:22626-22631, "Finish an alignment in
// a display": link(tail):=p), so the space above the first row is the one the
// alignment's own vertical list carries, not the append_to_vlist glue \lineskip
// clamps to almost nothing under a tall box.
//
// Measured against tectonic, ink to ink above the first row: an align holding a
// \rule of 20pt sat 11.28pt below the preceding line where the reference puts 23.28,
// and an align of `x = y` 14.88 against 26.64. One \baselineskip lands on the
// reference in both, and the per-construct cost of an align goes from −11.78pt to
// −0.00.
func TestAlignmentDisplayCarriesABaselineskipAboveItsFirstRow(t *testing.T) {
	// The glue widths between the last text box and the display's first box.
	glues := func(t *testing.T, src string) (int, []int) {
		t.Helper()
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		e.hsize = 300 * unity
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		var run []int
		seenBox := false
		for _, n := range e.mvl {
			switch c := n.(type) {
			case *boxNode:
				if seenBox && len(run) > 0 {
					return e.baselineskip, run
				}
				seenBox, run = true, nil
			case glueNode:
				if seenBox {
					run = append(run, c.spec.width)
				}
			}
		}
		return e.baselineskip, run
	}
	has := func(v []int, want int) bool {
		for _, x := range v {
			if x == want {
				return true
			}
		}
		return false
	}
	bs, plain := glues(t, `\hsize=300pt before\par $$x$$ after\par`)
	_, aligned := glues(t, `\hsize=300pt before\par \begin{align} x &= y \end{align} after\par`)
	if bs <= 0 {
		t.Fatal("no \\baselineskip to look for")
	}
	if !has(aligned, bs) {
		t.Errorf("no \\baselineskip glue (%d sp) above an alignment's first row: %v", bs, aligned)
	}
	if has(plain, bs) {
		t.Errorf("an ordinary display got the alignment's extra \\baselineskip: %v", plain)
	}
}
