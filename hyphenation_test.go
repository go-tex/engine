package engine

import "testing"

// The Liang algorithm on a controlled pattern set: "a1b" allows a break between
// any a and b; "2a1b" would suppress it (even wins). Test the odd/even merge.
func TestHyphenationPoints(t *testing.T) {
	h := newHyphenator()
	h.SetMins(1, 1)
	h.AddPattern("a1b") // break allowed between a,b
	h.AddPattern("b2a") // break suppressed between b,a
	// word "abab": positions after 1(a|b odd),2(b|a even→no),3(a|b odd)
	pts := h.Points("abab")
	got := map[int]bool{}
	for _, p := range pts {
		got[p] = true
	}
	if !got[1] || got[2] || !got[3] {
		t.Errorf("points(abab)=%v want {1,3}", pts)
	}
}

// lefthyphenmin/righthyphenmin suppress breaks too near the word edges.
func TestHyphenationAffixLimits(t *testing.T) {
	h := newHyphenator()
	h.SetMins(2, 2)
	h.AddPattern("a1a")
	// "aaaa": raw odd points at 1,2,3; limits keep only t in [2, 4-2]=[2,2]
	pts := h.Points("aaaa")
	if len(pts) != 1 || pts[0] != 2 {
		t.Errorf("affix-limited points=%v want [2]", pts)
	}
}

// Integration: a long word amid interword glue is hyphenated when leaving it
// unbroken would overfull the line — the hyphenated solution has feasible lines
// and thus lower demerits, so Knuth-Plass chooses it and a hyphen is emitted.
func TestHyphenationInsertsHyphen(t *testing.T) {
	e := New()
	e.SetFont(spMock{}) // letters 5pt, interword space 3pt (+1.5/−1)
	e.hyph = newHyphenator()
	e.hyph.SetMins(1, 1)
	e.hyph.AddPattern("a1a") // breaks allowed between a's
	e.hsize = 32 * unity
	e.parindent = 0
	if _, err := e.Run(`aa aaaaaa\par`); err != nil {
		t.Fatal(err)
	}
	var lines []*boxNode
	hyphenSeen := false
	for _, n := range e.mvl {
		b, ok := n.(*boxNode)
		if !ok || b.kind != hbox {
			continue
		}
		lines = append(lines, b)
		for _, c := range b.list {
			if ch, ok := c.(charNode); ok && ch.ch == '-' {
				hyphenSeen = true
			}
		}
	}
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 lines from a hyphenated long word, got %d", len(lines))
	}
	if !hyphenSeen {
		t.Errorf("expected a hyphen glyph on some line (word was hyphenated)")
	}
}

// Without patterns, the same input is not hyphenated (no discretionary nodes).
func TestNoHyphenationWithoutPatterns(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 32 * unity
	e.parindent = 0
	e.Run(`aa aaaaaa\par`)
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok {
			for _, c := range b.list {
				if ch, ok := c.(charNode); ok && ch.ch == '-' {
					t.Fatal("hyphen inserted without any \\patterns loaded")
				}
			}
		}
	}
}
