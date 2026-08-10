package engine

import "testing"

// \vsplit takes the top part of a vbox up to the given height and leaves the rest.
func TestVsplit(t *testing.T) {
	e := New()
	// box1 = five 10pt rules stacked (no interline glue inside a vbox) = 50pt
	e.Run(`\setbox1=\vbox{\hrule height10pt\hrule height10pt\hrule height10pt\hrule height10pt\hrule height10pt}`)
	if b := e.box[1]; b == nil || b.height != 50*unity {
		t.Fatalf("box1 setup: %+v", e.box[1])
	}
	// split off the top 25pt: two rules fit (20pt), the third would overflow
	e.Run(`\setbox0=\vsplit1 to 25pt`)
	top := e.box[0]
	if top == nil || top.height != 25*unity {
		t.Errorf("top height %v want 25pt (packed to)", top)
	}
	// two rules taken ⇒ remainder is three rules = 30pt
	rest := e.box[1]
	if rest == nil || rest.height != 30*unity {
		t.Errorf("remainder height %v want 30pt", rest)
	}
	// the top box holds exactly two rules
	nrules := 0
	for _, n := range top.list {
		if _, ok := n.(ruleNode); ok {
			nrules++
		}
	}
	if nrules != 2 {
		t.Errorf("top has %d rules want 2", nrules)
	}
}
