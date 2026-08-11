package engine

import "testing"

// A phantom box built from the same content as a visible one has matching
// dimensions but an empty (undrawn) list.
func TestPhantomDimensions(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})

	full := e.makePhantomOf("abc", phantomFull)
	ref := hpackSP(mustList(t, e, "abc"), packNatural, 0)
	if full.width != ref.width || full.height != ref.height || full.depth != ref.depth {
		t.Errorf("\\phantom dims = %d/%d/%d, want %d/%d/%d",
			full.width, full.height, full.depth, ref.width, ref.height, ref.depth)
	}
	if len(full.list) != 0 {
		t.Errorf("\\phantom must be invisible (empty list), got %d nodes", len(full.list))
	}

	h := e.makePhantomOf("abc", phantomH)
	if h.width != ref.width || h.height != 0 || h.depth != 0 {
		t.Errorf("\\hphantom = %d/%d/%d, want %d/0/0", h.width, h.height, h.depth, ref.width)
	}

	v := e.makePhantomOf("abc", phantomV)
	if v.width != 0 || v.height != ref.height || v.depth != ref.depth {
		t.Errorf("\\vphantom = %d/%d/%d, want 0/%d/%d", v.width, v.height, v.depth, ref.height, ref.depth)
	}
}

// \smash keeps the content (drawn) but zeroes height and depth.
func TestSmash(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	ref := hpackSP(mustList(t, e, "abc"), packNatural, 0)

	e.base = []rune("{abc}")
	e.bpos = 0
	s := e.makeSmash()
	if s.width != ref.width {
		t.Errorf("\\smash width = %d, want %d", s.width, ref.width)
	}
	if s.height != 0 || s.depth != 0 {
		t.Errorf("\\smash must zero height/depth, got %d/%d", s.height, s.depth)
	}
	if len(s.list) == 0 {
		t.Error("\\smash must keep its content (non-empty list)")
	}
}

// The commands work end to end and place a box in the list.
func TestPhantomEndToEnd(t *testing.T) {
	for _, cmd := range []string{`\phantom{x}`, `\hphantom{x}`, `\vphantom{x}`, `\smash{x}`} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(cmd); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
	}
}

// makePhantomOf primes {content} and builds the phantom (test helper).
func (e *Engine) makePhantomOf(s string, k phantomKind) *boxNode {
	e.base = []rune("{" + s + "}")
	e.bpos = 0
	return e.makePhantom(k)
}

// mustList primes {content} and returns its packed node list.
func mustList(t *testing.T, e *Engine, s string) []node {
	t.Helper()
	e.base = []rune("{" + s + "}")
	e.bpos = 0
	return e.grabHboxListOnly()
}
