package engine

import (
	"strconv"
	"strings"
	"testing"
)

// mathWidthSum adds the widths of every math node under nodes.
func mathWidthSum(nodes []node) int {
	w := 0
	for _, n := range nodes {
		switch v := n.(type) {
		case mathNode:
			w += v.width
		case *boxNode:
			w += mathWidthSum(v.list)
		}
	}
	return w
}

// leafBoxes returns every box under nodes that directly holds a char or math node.
func leafBoxes(nodes []node, out *[]*boxNode) {
	for _, n := range nodes {
		if b, ok := n.(*boxNode); ok {
			*out = append(*out, b)
			leafBoxes(b.list, out)
		}
	}
}

func runAlign(t *testing.T, src string) *Engine {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return e
}

// amsmath's align sets each right-hand column as $\displaystyle{}#$, so the empty
// group gives a leading relation an Ord on its left: "&\sim y" is spaced exactly as
// "&{}\sim y" is. Without it the relation was the first atom of its cell and got no
// space before it ("x∼y", "0⇔gene" on page 2 of arXiv 2407.19892).
func TestAlignRightColumnRelationIsSpacedBothSides(t *testing.T) {
	bare := runAlign(t, "\\hsize=300pt\n\\begin{align*}\nx &\\sim y\n\\end{align*}")
	braced := runAlign(t, "\\hsize=300pt\n\\begin{align*}\nx &{}\\sim y\n\\end{align*}")
	flat := runAlign(t, "\\hsize=300pt\n\\begin{align*}\nx \\sim y\n\\end{align*}")
	wb, wc, wf := mathWidthSum(bare.mvl), mathWidthSum(braced.mvl), mathWidthSum(flat.mvl)
	if wb != wc {
		t.Errorf("&\\sim is %d wide, &{}\\sim is %d: the relation is not spaced as in amsmath", wb, wc)
	}
	// And that width is the one a relation has between two atoms in a plain formula.
	if wb != wf {
		t.Errorf("align cells total %d, the unbroken x \\sim y is %d", wb, wf)
	}
}

// A \tag is flush right. When block plus tag cannot share a line, amsmath sets the tag
// on its own line below, flush right; packing both in one row made the row overflow
// and printed the tag straight after the block, off the page ("(Cartesian product [ou").
func TestAlignTagThatDoesNotFitGoesOnItsOwnLine(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"
	e := runAlign(t, "\\hsize=100pt\n\\begin{align*}\nx &\\sim y \\tag{"+long+"}\n\\end{align*}")
	var boxes []*boxNode
	leafBoxes(e.mvl, &boxes)
	// The innermost row box holding the tag chars, and the one holding the math.
	var tagBox, mathBox *boxNode
	for _, b := range boxes {
		var s strings.Builder
		collectChars(b.list, &s)
		// a row is the box packed to the full \hsize
		if b.width != e.hsize {
			continue
		}
		if strings.Contains(s.String(), long) {
			tagBox = b
		} else if hasMathNode(b.list) {
			mathBox = b
		}
	}
	if tagBox == nil || mathBox == nil {
		t.Fatalf("tag box %v, math box %v: expected both", tagBox != nil, mathBox != nil)
	}
	if hasMathNode(tagBox.list) {
		t.Error("the tag shares a line with the equation although the two cannot fit")
	}
	// flush right: an hfil precedes the tag
	if _, ok := tagBox.list[0].(glueNode); !ok {
		t.Errorf("tag line starts with %T, want the filling glue that pushes it right", tagBox.list[0])
	}
}

// A tag that fits stays on the equation's line, after filling glue.
func TestAlignTagThatFitsStaysOnTheLine(t *testing.T) {
	e := runAlign(t, "\\hsize=300pt\n\\begin{align*}\nx &\\sim y \\tag{7}\n\\end{align*}")
	var boxes []*boxNode
	leafBoxes(e.mvl, &boxes)
	for _, b := range boxes {
		var s strings.Builder
		collectChars(b.list, &s)
		if strings.Contains(s.String(), "(7)") {
			if !hasMathNode(b.list) {
				t.Error("a tag that fits was moved to its own line")
			}
			return
		}
	}
	t.Error("tag (7) not found")
}

// amsmath.sty:2142-2153 sends a tag to its own line when
// width + tag + \minalignsep*pairs + 2\mintagsep exceeds \displaywidth, not merely when
// width + tag exceeds it. With a 10pt quad \mintagsep is 5pt, so a row that is 5pt
// inside the plain "fits" rule is still moved, and one 15pt inside is not.
func TestAlignTagBoundaryIsAmsmathsNotPlainFit(t *testing.T) {
	probe := runAlign(t, "\\hsize=300pt\n\\begin{align*}\nx &\\sim y\n\\end{align*}")
	block := mathWidthSum(probe.mvl)
	tag := probe.textToHbox("(7)").width
	own := func(hsizePt float64) bool {
		t.Helper()
		src := "\\hsize=" + strconv.FormatFloat(hsizePt, 'f', 5, 64) + "pt\n\\begin{align}\nx &\\sim y \\tag{7}\n\\end{align}"
		e := runAlign(t, src)
		var boxes []*boxNode
		leafBoxes(e.mvl, &boxes)
		for _, b := range boxes {
			var s strings.Builder
			collectChars(b.list, &s)
			if strings.Contains(s.String(), "(7)") {
				return !hasMathNode(b.list)
			}
		}
		t.Fatal("tag (7) not found")
		return false
	}
	fit := float64(block+tag) / unity
	if !own(fit + 5) {
		t.Errorf("hsize = block+tag+5pt fits the plain rule but not amsmath's: the tag must go on its own line")
	}
	if own(fit + 15) {
		t.Errorf("hsize = block+tag+15pt satisfies amsmath's rule: the tag must stay inline")
	}
}

// The unit test of the port itself, on a case worked by hand from the source: one
// pair, widths 40+60, tag 20, on displays of 200, 150 and 125pt.
func TestAmsAlignShiftsWorkedCases(t *testing.T) {
	const pt = unity
	// one pair, no tag: centred, eqnshift = (200-100)/2 = 50pt.
	eq, _, own := amsAlignShifts(200*pt, []int{40 * pt, 60 * pt}, [][]int{{40 * pt, 60 * pt}}, []int{0}, 10*pt, 5*pt)
	if eq != 50*pt || own[0] {
		t.Errorf("no tag: eqnshift %d own %v, want %d false", eq, own[0], 50*pt)
	}
	// tag 20 on a 200 display: inline needs 100+20+10 = 130 <= 200, and 50+100+20+20 = 190
	// <= 200 so nothing shrinks.
	eq, _, own = amsAlignShifts(200*pt, []int{40 * pt, 60 * pt}, [][]int{{40 * pt, 60 * pt}}, []int{20 * pt}, 10*pt, 5*pt)
	if eq != 50*pt || own[0] {
		t.Errorf("tag fits: eqnshift %d own %v", eq, own[0])
	}
	// display 150: 100+20+10 = 130 <= 150 so inline; 25+100+20+20 = 165 > 150 so the shift
	// shrinks to (150-120)/2 ... d = (150-120)/2 = 15 < 25: eqnshift = 15pt.
	eq, _, own = amsAlignShifts(150*pt, []int{40 * pt, 60 * pt}, [][]int{{40 * pt, 60 * pt}}, []int{20 * pt}, 10*pt, 5*pt)
	if eq != 15*pt || own[0] {
		t.Errorf("shrink: eqnshift %d own %v, want %d false", eq, own[0], 15*pt)
	}
	// display 125: 100+20+10 = 130 > 125, own line.
	_, _, own = amsAlignShifts(125*pt, []int{40 * pt, 60 * pt}, [][]int{{40 * pt, 60 * pt}}, []int{20 * pt}, 10*pt, 5*pt)
	if !own[0] {
		t.Error("130 > 125: the tag must go on its own line")
	}
}
