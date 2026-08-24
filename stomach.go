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

type kernNode struct{ width int } // \kern (explicit space, no stretch)
type glueNode struct {            // \hskip/\vskip and the fil glues
	spec   glueSpec
	leader glueLeader // if set, the glue's set width is painted as a leader
}

// glueLeader marks a glue node that should be rendered as \leaders-like fill:
// a horizontal rule (\hrulefill) or a row of dots (\dotfill) spanning its set
// width. leaderNone (the zero value) is ordinary glue that paints nothing.
type glueLeader uint8

const (
	leaderNone glueLeader = iota
	leaderRule
	leaderDots
)

// fillGlue returns 0pt plus 1fill (order-2 infinite stretch), the glue LaTeX's
// \fill uses; \hfill, \hrulefill and \dotfill are all built on it.
func fillGlue() glueSpec { return glueSpec{stretch: unity, stretchOrder: 2} }

type penaltyNode struct{ penalty int } // \penalty
type ruleNode struct {                 // \hrule/\vrule
	width, height, depth          int
	widthRun, heightRun, depthRun bool // a "running" dimension follows the box
}
type charNode struct { // a set character from the current font (metrics in sp)
	ch                   rune
	width, height, depth int
	srcLine              int    // 1-based source line this glyph came from (0 = unknown)
	size                 int    // font size (px/pt) this glyph was set at (0 = base)
	color                uint32 // 0xRRGGBB fill colour (0 = default black)
}
type discNode struct{ penalty int } // a discretionary hyphen point (Liang)

func (kernNode) isNode()    {}
func (glueNode) isNode()    {}
func (penaltyNode) isNode() {}
func (ruleNode) isNode()    {}
func (charNode) isNode()    {}
func (discNode) isNode()    {}
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
	srcLine              int     // 1-based source line (first stamped child; 0 = unknown)
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
		case charNode:
			natural += c.width
			if c.height > h {
				h = c.height
			}
			if c.depth > d {
				d = c.depth
			}
		case mathNode:
			natural += c.width
			if c.height > h {
				h = c.height
			}
			if c.depth > d {
				d = c.depth
			}
		case imageNode:
			natural += c.width
			if c.height > h {
				h = c.height
			}
			if c.depth > d {
				d = c.depth
			}
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
		case frameNode:
			natural += c.width()
			if c.height() > h {
				h = c.height()
			}
			if c.depth() > d {
				d = c.depth()
			}
		case decoNode:
			natural += c.width()
			if c.height() > h {
				h = c.height()
			}
			if c.depth() > d {
				d = c.depth()
			}
		case transformNode:
			natural += c.width()
			if c.height() > h {
				h = c.height()
			}
			if c.depth() > d {
				d = c.depth()
			}
		case linkNode:
			natural += c.width()
			if c.height() > h {
				h = c.height()
			}
			if c.depth() > d {
				d = c.depth()
			}
		case internalLinkNode:
			natural += c.width()
			if c.height() > h {
				h = c.height()
			}
			if c.depth() > d {
				d = c.depth()
			}
		}
	}
	b := &boxNode{kind: hbox, width: packWidth(natural, mode, target), height: h, depth: d, list: list, srcLine: firstSrcLine(list)}
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
		case charNode: // unusual in a vlist; treat like a small box
			if c.width > w {
				w = c.width
			}
			x += d + c.height
			d = c.depth
		case mathNode: // display math on the vertical list
			if c.width > w {
				w = c.width
			}
			x += d + c.height
			d = c.depth
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
		case frameNode:
			if c.width() > w {
				w = c.width()
			}
			x += d + c.height()
			d = c.depth()
		case decoNode:
			if c.width() > w {
				w = c.width()
			}
			x += d + c.height()
			d = c.depth()
		case transformNode:
			if c.width() > w {
				w = c.width()
			}
			x += d + c.height()
			d = c.depth()
		case linkNode:
			if c.width() > w {
				w = c.width()
			}
			x += d + c.height()
			d = c.depth()
		case internalLinkNode:
			if c.width() > w {
				w = c.width()
			}
			x += d + c.height()
			d = c.depth()
		}
	}
	b := &boxNode{kind: vbox, width: w, height: packWidth(x, mode, target), depth: d, list: list, srcLine: firstSrcLine(list)}
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
			case "vtop":
				return e.makeVtop()
			case "vsplit":
				return e.doVsplit()
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
	t, ok := e.getXToken()
	if !ok {
		return &boxNode{kind: kind} // malformed: empty box
	}
	rt := t
	if c, isChar := e.implicitChar(t); isChar {
		rt = c // accept \bgroup as the box's opening brace
	}
	if !(rt.cat == catBegin && !rt.cs_) {
		e.back(t)
		return &boxNode{kind: kind} // malformed: empty box
	}
	e.beginGroupKind(boxGroup)
	list := e.buildBoxList()
	e.endGroup()
	if kind == hbox {
		return hpackSP(list, mode, target)
	}
	return vpackSP(list, mode, target)
}

// makeVtop builds a \vtop: the same contents as a \vbox, but the reference point
// is at the top, so the box height is the height of its first box/rule and the
// depth is everything below (TeX §1087).
func (e *Engine) makeVtop() *boxNode {
	b := e.makeBox(vbox)
	total := b.height + b.depth
	firstH := 0
	for _, n := range b.list {
		switch c := n.(type) {
		case *boxNode:
			firstH = c.height
		case ruleNode:
			if !c.heightRun {
				firstH = c.height
			}
		default:
			continue // skip leading glue/kern
		}
		break
	}
	b.height = firstH
	b.depth = total - firstH
	return b
}

// buildBoxList builds a material list until the box's matching '}' (consumed),
// turning node-producing primitives into nodes and executing everything else via
// the shared execCS dispatch. Nested { … } groups are handled with a depth count
// so an inner '}' does not end the box (its own group is begun/ended locally).
func (e *Engine) buildBoxList() []node {
	// Inside a box we are in restricted horizontal mode: keep inPar set so a
	// character-emitting command (an accent, \char, …) appends to the current
	// list rather than opening a fresh paragraph with a \parindent box.
	savedInPar := e.inPar
	e.inPar = true
	defer func() { e.inPar = savedInPar }()
	var list []node
	depth := 0
	for e.err == nil {
		t, ok := e.getXToken()
		if !ok {
			return list
		}
		if c, isChar := e.implicitChar(t); isChar {
			t = c // \egroup closes the box, \bgroup opens a nested group
		}
		if !t.cs_ {
			switch t.cat {
			case catEnd:
				if k, open := e.curGroupKind(); open && k == semiSimpleGroup {
					// A \begingroup inside the box is still open, so this brace is not
					// the box's closer. Report it and close the group anyway — see
					// closeBrace for why the engine does not yet delete the brace the
					// way TeX does.
					e.groupError("Extra }, or forgotten \\endgroup.")
				}
				if depth == 0 {
					return list // the box's own closing brace (caller ends its group)
				}
				depth--
				e.endGroup()
			case catBegin:
				depth++
				e.beginGroup()
			case catLetter, catOther, catParam:
				// catParam here is a stray literal '#' (a ## reduced by scanBody that
				// was not consumed as a parameter char by a nested \def); typeset it.
				if e.curFont != nil {
					list = e.appendChar(list, t.ch)
				}
			case catSpace:
				if e.curFont != nil {
					list = append(list, glueNode{spec: e.curFont.spaceSP()})
				}
			case catMath:
				src, display := e.scanMathSource()
				list = append(list, e.makeMath(src, display))
			}
			continue
		}
		if n, isNode := e.boxNodeFor(t); isNode {
			if n != nil {
				list = append(list, n)
			}
			continue
		}
		// A command executed here (e.g. an accent like \'e) emits horizontal
		// material by appending to e.parList. Capture anything it added so the
		// glyph joins this box's list instead of leaking to the outer paragraph.
		mark := len(e.parList)
		if !e.execCS(t) {
			return list
		}
		if len(e.parList) > mark {
			list = append(list, e.parList[mark:]...)
			e.parList = e.parList[:mark]
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
	case "hspace", "vspace":
		e.scanOptStar()
		return glueNode{spec: glueSpec{width: e.readBraceDimen()}}, true
	case "hrulefill":
		return glueNode{spec: fillGlue(), leader: leaderRule}, true
	case "dotfill":
		return glueNode{spec: fillGlue(), leader: leaderDots}, true
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
	case "rule":
		return e.doRuleNode(), true
	case "underline":
		return e.makeDeco('u'), true
	case "sout":
		return e.makeDeco('s'), true
	case "textoverline":
		return e.makeDeco('o'), true
	case "phantom":
		return e.makePhantom(phantomFull), true
	case "hphantom":
		return e.makePhantom(phantomH), true
	case "vphantom":
		return e.makePhantom(phantomV), true
	case "smash":
		return e.makeSmash(), true
	case "scalebox":
		return e.doScalebox(), true
	case "reflectbox":
		return e.doReflectbox(), true
	case "resizebox":
		return e.doResizebox(), true
	case "rotatebox":
		return e.doRotatebox(), true
	case "penalty":
		return penaltyNode{penalty: e.scanInt()}, true
	case "char":
		n := e.scanInt()
		if c, ok := e.charNodeFor(rune(n)); ok {
			return c, true
		}
		return nil, true
	case " ": // control space inside a box
		if e.curFont != nil {
			return glueNode{spec: e.curFont.spaceSP()}, true
		}
		return nil, true
	case "hbox":
		return e.makeBox(hbox), true
	case "vbox":
		return e.makeBox(vbox), true
	case "vtop":
		return e.makeVtop(), true
	case "box":
		i := e.scanInt()
		b := e.getBox(i)
		e.setBox(i, nil)
		return boxOrNil(b)
	case "copy":
		return boxOrNil(cloneBox(e.getBox(e.scanInt())))
	case "raise":
		d := e.scanDimen()
		return boxOrNil(e.scanShiftedBox(-d))
	case "lower":
		d := e.scanDimen()
		return boxOrNil(e.scanShiftedBox(d))
	case "makebox":
		return boxOrNil(e.doMakebox())
	case "raisebox":
		return boxOrNil(e.doRaisebox())
	case "usebox":
		return boxOrNil(e.doUsebox())
	}
	return nil, false
}

// boxOrNil returns (box, true) as a node-producer result, using an untyped nil
// node when the box is void so buildBoxList appends nothing.
func boxOrNil(b *boxNode) (node, bool) {
	if b == nil {
		return nil, true
	}
	return b, true
}

// scanShiftedBox reads a box and applies a shift amount (positive = down).
func (e *Engine) scanShiftedBox(shift int) *boxNode {
	b := e.scanBox()
	if b != nil {
		b.shift = shift
	}
	return b
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

// setBoxScoped is \setbox: an ordinary assignment, so a group restores the
// register when it closes. Measured against a real TeX — \setbox0 inside { }
// leaves box 0 as it was outside. tikz relies on it: a node voids \tikz@figbox
// inside the group that collects its text, and every node already accumulated
// there has to survive that.
func (e *Engine) setBoxScoped(i int, b *boxNode, global bool) {
	if i < 0 || i >= 256 {
		return
	}
	if global {
		e.forgetSaved(10, i, "")
	} else if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 10, idx: i, oldbox: e.box[i]})
	}
	e.box[i] = b
}

// setBox is the unscoped assignment, used where TeX itself does not restore: a
// register read with \box or \unhbox is voided for good, not until the end of
// the group that read it. Measured the same way.
func (e *Engine) setBox(i int, b *boxNode) {
	if i < 0 || i >= 256 {
		return
	}
	e.forgetSaved(10, i, "")
	e.box[i] = b
}

// boxRegIndex reads a box-register reference: either a \newbox/\newsavebox-defined
// control sequence (mBoxRef, e.g. amsart's \abstractbox) or a bare register number
// (\setbox0). Without it \setbox\abstractbox would misparse and leak the '=' that
// follows the register.
func (e *Engine) boxRegIndex() int {
	e.skipOptSpace()
	if t, ok := e.getXToken(); ok {
		if t.cs_ {
			if m := e.eq[t.cs]; m != nil && m.kind == mBoxRef {
				return m.code
			}
		}
		e.back(t)
	}
	return e.scanInt()
}

// doSetbox handles \setbox<n>=<box>.
func (e *Engine) doSetbox(global bool) {
	i := e.boxRegIndex()
	e.scanEquals()
	e.setBoxScoped(i, e.scanBox(), global)
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
