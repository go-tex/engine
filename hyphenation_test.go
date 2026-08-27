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

// A non-English \language, with no document \patterns, is not hyphenated: no
// patterns are embedded for it, so activeHyphenator returns nil and guessing is
// avoided. (The word here would hyphenate under the default English set.)
func TestNoHyphenationForNonEnglishLanguage(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 32 * unity
	e.parindent = 0
	if _, err := e.Run(`\language=1 hyphenation hyphenation hyphenation\par`); err != nil {
		t.Fatal(err)
	}
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok {
			for _, c := range b.list {
				if ch, ok := c.(charNode); ok && ch.ch == '-' {
					t.Fatal("hyphen inserted for a non-English \\language with no \\patterns")
				}
			}
		}
	}
}

// Out of the box (\language 0, no \patterns), English prose is hyphenated using
// the embedded US-English pattern set: a long word amid glue is broken with a
// discretionary so the narrow measure does not overfull.
func TestDefaultEnglishHyphenation(t *testing.T) {
	e := New()
	e.SetFont(spMock{}) // 5pt letters, 3pt space
	e.hsize = 60 * unity
	e.parindent = 0
	if _, err := e.Run(`the internationalization of hyphenation patterns\par`); err != nil {
		t.Fatal(err)
	}
	hyphenSeen := false
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok {
			for _, c := range b.list {
				if ch, ok := c.(charNode); ok && ch.ch == '-' {
					hyphenSeen = true
				}
			}
		}
	}
	if !hyphenSeen {
		t.Error("expected the default English patterns to hyphenate a long word")
	}
	if e.enHyph == nil {
		t.Error("expected the default English hyphenator to be built and cached")
	}
}

// The embedded patterns produce TeX's own break points for real words, honouring
// the default \lefthyphenmin=2 / \righthyphenmin=3.
func TestEmbeddedPatternsBreakPoints(t *testing.T) {
	h := newEnglishHyphenator()
	cases := map[string][]int{
		"incomprehensibility": {2, 5, 8, 11, 13, 16}, // in-com-pre-hen-si-bil-ity (rmin=3 blocks i-ty)
		"considers":           {3, 6},                // con-sid-ers
		"results":             {2},                   // re-sults
	}
	for w, want := range cases {
		pts := h.Points(w)
		if len(pts) != len(want) {
			t.Errorf("Points(%q)=%v want %v", w, pts, want)
			continue
		}
		for i := range want {
			if pts[i] != want[i] {
				t.Errorf("Points(%q)=%v want %v", w, pts, want)
				break
			}
		}
	}
}

// A document's own \patterns take precedence over the default English set, and
// namedInt falls back to plain TeX's default when a parameter is unbound.
func TestDocumentPatternsOverrideDefault(t *testing.T) {
	e := New()
	if got := e.namedInt("righthyphenmin"); got != 3 {
		t.Errorf("namedInt(righthyphenmin)=%d want 3", got)
	}
	e2 := &Engine{eq: map[string]*meaning{}} // no registers bound at all
	if got := e2.namedInt("lefthyphenmin"); got != 2 {
		t.Errorf("namedInt on unbound engine=%d want the plain-TeX default 2", got)
	}
	// After \patterns, activeHyphenator returns the document's hyphenator, not the
	// cached English one.
	e.Run(`\patterns{a1a}`)
	if e.hyph == nil {
		t.Fatal("\\patterns did not populate the document hyphenator")
	}
	if h := e.activeHyphenator(); h != e.hyph {
		t.Error("activeHyphenator should return the document's own \\patterns hyphenator")
	}
	if e.enHyph != nil {
		t.Error("the default English set should not be built when the document has its own \\patterns")
	}
}
