package engine

import (
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
