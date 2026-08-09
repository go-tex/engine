// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file is the beginning of TeX's stomach: the part that consumes scanned
// quantities (dimensions and glue) to build node lists and pack them into boxes.
// It implements explicit boxes (\hbox/\vbox with `to`/`spread`), the material
// nodes that go inside them (\kern, \hskip/\vskip and the fil glues, \hrule/
// \vrule, \penalty, nested boxes), box registers (\setbox/\box/\copy) and the
// box-dimension parameters (\wd/\ht/\dp). All arithmetic is in scaled points, so
// packed dimensions match TeX exactly. Glue-setting for rendering comes later;
// this milestone is about faithful box *dimensions*.

const defaultRule = 26214 // 0.4pt in sp, TeX's default_rule thickness

// node is one item of horizontal or vertical material inside a box.
type node interface{ isNode() }

type kernNode struct{ width int }      // \kern (explicit space, no stretch)
type glueNode struct{ spec glueSpec }  // \hskip/\vskip and the fil glues
type penaltyNode struct{ penalty int } // \penalty
type ruleNode struct {                 // \hrule/\vrule
	width, height, depth          int
	widthRun, heightRun, depthRun bool // a "running" dimension follows the box
}

func (kernNode) isNode()    {}
func (glueNode) isNode()    {}
func (penaltyNode) isNode() {}
func (ruleNode) isNode()    {}
func (*boxNode) isNode()    {}

type boxKind uint8

const (
	hbox boxKind = iota
	vbox
)

// boxNode is a packed hbox or vbox with its reference-point dimensions (sp) and
// the glue-set that renders its glue to the target size.
type boxNode struct {
	kind                 boxKind
	width, height, depth int
	shift                int     // shift amount (\raise/\lower/\moveleft — future)
	list                 []node  // contents
	glueSet              float64 // set ratio applied to matching-order glue
	glueOrder            int     // order (0..3) the set applies to
	glueSign             int     // 0 normal, 1 stretching, 2 shrinking
}

type packMode uint8

const (
	packNatural packMode = iota
	packTo
	packSpread
)

// ── packing ─────────────────────────────────────────────────────────────────

// hpack packs a horizontal list into an hbox, setting the width per mode and the
// height/depth to the extremes of the contents (TeX §649). Glue-set computation
// for rendering is deferred; dimensions are exact.
func hpackSP(list []node, mode packMode, target int) *boxNode {
	natural, h, d := 0, 0, 0
	for _, n := range list {
		switch c := n.(type) {
		case kernNode:
			natural += c.width
		case glueNode:
			natural += c.spec.width
		case ruleNode:
			if !c.widthRun {
				natural += c.width
			}
			if !c.heightRun && c.height > h {
				h = c.height
			}
			if !c.depthRun && c.depth > d {
				d = c.depth
			}
		case *boxNode:
			natural += c.width
			if hh := c.height - c.shift; hh > h {
				h = hh
			}
			if dd := c.depth + c.shift; dd > d {
				d = dd
			}
		}
	}
	b := &boxNode{kind: hbox, width: packWidth(natural, mode, target), height: h, depth: d, list: list}
	setGlue(b, b.width-natural, list)
	return b
}

// vpack packs a vertical list into a vbox: the width is the widest item, and the
// height accumulates each item's size carrying the running depth (TeX §668). The
// height is set per mode; the depth is the last box/rule's depth.
func vpackSP(list []node, mode packMode, target int) *boxNode {
	w, x, d := 0, 0, 0
	for _, n := range list {
		switch c := n.(type) {
		case kernNode:
			x += d + c.width
			d = 0
		case glueNode:
			x += d + c.spec.width
			d = 0
		case ruleNode:
			if !c.widthRun && c.width > w {
				w = c.width
			}
			x += d + c.height
			d = c.depth
		case *boxNode:
			if c.width+c.shift > w {
				w = c.width + c.shift
			}
			x += d + c.height
			d = c.depth
		}
	}
	b := &boxNode{kind: vbox, width: w, height: packWidth(x, mode, target), depth: d, list: list}
	setGlue(b, b.height-x, list)
	return b
}

// packWidth resolves a natural size against the packing mode (to/spread/natural).
func packWidth(natural int, mode packMode, target int) int {
	switch mode {
	case packTo:
		return target
	case packSpread:
		return natural + target
	default:
		return natural
	}
}

// setGlue computes the glue-set that stretches or shrinks the list's glue to
// absorb `excess` sp (target − natural), storing it on the box (TeX §657/§664).
// The highest glue order present dominates; a positive excess stretches, a
// negative one shrinks (finite shrink is capped at its total, glueSet ≤ 1).
func setGlue(b *boxNode, excess int, list []node) {
	if excess == 0 {
		return
	}
	var stretch, shrink [4]int
	for _, n := range list {
		if g, ok := n.(glueNode); ok {
			stretch[g.spec.stretchOrder] += g.spec.stretch
			shrink[g.spec.shrinkOrder] += g.spec.shrink
		}
	}
	if excess > 0 {
		order := highestOrder(stretch)
		if total := stretch[order]; total > 0 {
			b.glueSign, b.glueOrder, b.glueSet = 1, order, float64(excess)/float64(total)
		}
		return
	}
	order := highestOrder(shrink)
	if total := shrink[order]; total > 0 {
		set := float64(-excess) / float64(total)
		if order == 0 && set > 1 {
			set = 1 // finite shrink cannot exceed its total
		}
		b.glueSign, b.glueOrder, b.glueSet = 2, order, set
	}
}

// highestOrder returns the largest index with a non-zero total (0 if all zero).
func highestOrder(totals [4]int) int {
	for o := 3; o > 0; o-- {
		if totals[o] != 0 {
			return o
		}
	}
	return 0
}

// setWidth returns a glue node's rendered width under a box's glue-set: its
// natural width adjusted by the matching-order stretch or shrink (TeX §625).
func (b *boxNode) setWidth(g glueSpec) int {
	w := g.width
	switch b.glueSign {
	case 1:
		if g.stretchOrder == b.glueOrder {
			w += int(b.glueSet * float64(g.stretch))
		}
	case 2:
		if g.shrinkOrder == b.glueOrder {
			w -= int(b.glueSet * float64(g.shrink))
		}
	}
	return w
}

// cloneBox deep-copies a box (for \copy, which leaves the register intact).
func cloneBox(b *boxNode) *boxNode {
	if b == nil {
		return nil
	}
	cp := *b
	cp.list = make([]node, len(b.list))
	for i, n := range b.list {
		if sub, ok := n.(*boxNode); ok {
			cp.list[i] = cloneBox(sub)
		} else {
			cp.list[i] = n
		}
	}
	return &cp
}

// ── box construction ─────────────────────────────────────────────────────────

// scanBox reads a <box>: \hbox…/\vbox…, or \box<n>/\copy<n>. Returns nil if the
// next token is not a box producer (backed out).
func (e *Engine) scanBox() *boxNode {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return nil
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil && m.kind == mPrim {
			switch m.name {
			case "hbox":
				return e.makeBox(hbox)
			case "vbox":
				return e.makeBox(vbox)
			case "box":
				n := e.scanInt()
				b := e.getBox(n)
				e.setBox(n, nil) // \box empties the register
				return b
			case "copy":
				return cloneBox(e.getBox(e.scanInt()))
			}
		}
	}
	e.back(t)
	return nil
}

// makeBox reads an optional `to <dimen>` / `spread <dimen>` spec, then the braced
// contents, builds the material list in a group, and packs it.
func (e *Engine) makeBox(kind boxKind) *boxNode {
	mode, target := packNatural, 0
	switch {
	case e.scanKeyword("to"):
		mode, target = packTo, e.scanDimen()
	case e.scanKeyword("spread"):
		mode, target = packSpread, e.scanDimen()
	}
	e.skipOptSpace()
	if t, ok := e.getXToken(); !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return &boxNode{kind: kind} // malformed: empty box
	}
	e.beginGroup()
	list := e.buildBoxList()
	e.endGroup()
	if kind == hbox {
		return hpackSP(list, mode, target)
	}
	return vpackSP(list, mode, target)
}

// buildBoxList builds a material list until the matching '}' (consumed), turning
// node-producing primitives into nodes and executing everything else (including
// assignments) via the shared execCS dispatch.
func (e *Engine) buildBoxList() []node {
	var list []node
	for e.err == nil {
		t, ok := e.getXToken()
		if !ok {
			return list
		}
		if !t.cs_ {
			switch t.cat {
			case catEnd:
				return list
			case catBegin:
				e.beginGroup()
			}
			continue // printable chars need a current font (deferred)
		}
		if n, isNode := e.boxNodeFor(t); isNode {
			if n != nil {
				list = append(list, n)
			}
			continue
		}
		if !e.execCS(t) {
			return list
		}
	}
	return list
}

// boxNodeFor turns a node-producing primitive token into a node. The second
// result is false when the token is not such a primitive (caller executes it).
func (e *Engine) boxNodeFor(t tok) (node, bool) {
	m := e.eq[t.cs]
	if m == nil || m.kind != mPrim {
		return nil, false
	}
	switch m.name {
	case "kern":
		return kernNode{width: e.scanDimen()}, true
	case "hskip", "vskip":
		return glueNode{spec: e.scanGlue()}, true
	case "hfil", "vfil":
		return glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}, true
	case "hfill", "vfill":
		return glueNode{spec: glueSpec{stretch: unity, stretchOrder: 2}}, true
	case "hss", "vss":
		return glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1, shrink: unity, shrinkOrder: 1}}, true
	case "hrule":
		return e.scanRule(true), true
	case "vrule":
		return e.scanRule(false), true
	case "penalty":
		return penaltyNode{penalty: e.scanInt()}, true
	case "hbox":
		return e.makeBox(hbox), true
	case "vbox":
		return e.makeBox(vbox), true
	}
	return nil, false
}

// scanRule reads an \hrule/\vrule with any number of width/height/depth keywords,
// each overriding the corresponding default (TeX §463). Unspecified dimensions in
// the rule's running direction stay "running" (they follow the enclosing box).
func (e *Engine) scanRule(horizontal bool) ruleNode {
	r := ruleNode{}
	if horizontal { // \hrule: default height 0.4pt, depth 0, running width
		r.height, r.widthRun = defaultRule, true
	} else { // \vrule: default width 0.4pt, running height & depth
		r.width, r.heightRun, r.depthRun = defaultRule, true, true
	}
	for {
		switch {
		case e.scanKeyword("width"):
			r.width, r.widthRun = e.scanDimen(), false
		case e.scanKeyword("height"):
			r.height, r.heightRun = e.scanDimen(), false
		case e.scanKeyword("depth"):
			r.depth, r.depthRun = e.scanDimen(), false
		default:
			return r
		}
	}
}

// ── box registers & dimensions ───────────────────────────────────────────────

func (e *Engine) getBox(i int) *boxNode {
	if i < 0 || i >= 256 {
		return nil
	}
	return e.box[i]
}

func (e *Engine) setBox(i int, b *boxNode) {
	if i >= 0 && i < 256 {
		e.box[i] = b
	}
}

// doSetbox handles \setbox<n>=<box>.
func (e *Engine) doSetbox() {
	i := e.scanInt()
	e.scanEquals()
	e.setBox(i, e.scanBox())
}

// boxDimAssign handles \wd/\ht/\dp <n>=<dimen>, overriding a box's dimension.
func (e *Engine) boxDimAssign(which byte) {
	i := e.scanInt()
	e.scanEquals()
	v := e.scanDimen()
	b := e.getBox(i)
	if b == nil {
		return
	}
	switch which {
	case 'w':
		b.width = v
	case 'h':
		b.height = v
	case 'd':
		b.depth = v
	}
}

// boxDim reads a box's dimension for \the (0 for a void register).
func (e *Engine) boxDim(which byte) int {
	b := e.getBox(e.scanInt())
	if b == nil {
		return 0
	}
	switch which {
	case 'w':
		return b.width
	case 'h':
		return b.height
	default:
		return b.depth
	}
}
