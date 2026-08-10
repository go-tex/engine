package engine

import "testing"

func interwordSpace() glueSpec { return glueSpec{width: 3 * unity, stretch: unity, shrink: unity} }

func lineSeg() []node {
	return []node{
		charNode{ch: 'a', width: 5 * unity},
		glueNode{spec: interwordSpace()},
		charNode{ch: 'a', width: 5 * unity},
	}
}

// With \rightskip = 0pt plus 1fil, the fil (order 1) dominates the finite
// interword stretch, so an underfull line leaves the space at natural width.
func TestRaggedRightLeavesSpaceNatural(t *testing.T) {
	e := New()
	e.rightskip = glueSpec{stretch: unity, stretchOrder: 1}
	box := hpackSP(e.applyLineSkips(lineSeg()), packTo, 20*unity) // natural 13pt → target 20pt
	space := box.setWidth(interwordSpace())
	if space != 3*unity {
		t.Errorf("ragged interword space = %d sp, want natural %d", space, 3*unity)
	}
}

// Without \rightskip (justified), the interword glue stretches to fill the line.
func TestJustifiedStretchesSpace(t *testing.T) {
	e := New()
	e.rightskip = glueSpec{}
	box := hpackSP(e.applyLineSkips(lineSeg()), packTo, 20*unity)
	space := box.setWidth(interwordSpace())
	if space <= 3*unity {
		t.Errorf("justified interword space = %d sp, want > natural (stretched)", space)
	}
}

// \raggedright / \justified toggle \rightskip through the Plain prelude.
func TestRaggedrightMacroSetsRightskip(t *testing.T) {
	e := New()
	e.LoadPlain()
	e.Run(`\raggedright`)
	if e.rightskip.stretchOrder != 1 {
		t.Errorf("\\raggedright should set an infinite \\rightskip, got %+v", e.rightskip)
	}
	e.Run(`\justified`)
	if e.rightskip != (glueSpec{}) {
		t.Errorf("\\justified should zero \\rightskip, got %+v", e.rightskip)
	}
}
