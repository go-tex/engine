// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's framed boxes: \fbox{content} and
// \framebox[width][pos]{content}. Both compose their content as an hbox and draw a
// rule of thickness \fboxrule around it, leaving \fboxsep between the content and
// the frame. For this milestone \fboxsep and \fboxrule are fixed at their LaTeX
// defaults (3pt and 0.4pt); they are not yet settable registers. The frame is
// monochrome (black), matching the rest of the engine's single-colour output.
//
// \framebox's first optional argument forces the content-box width; the second
// ([pos] = l/c/r, default c) aligns the content within that width using fil glue.
// A frameNode carries the packed inner hbox and the two thicknesses; its outer
// dimensions add sep+rule on every side (2*(sep+rule) to the width, sep+rule to
// the height and to the depth), so it packs and paints like any other box item.

const (
	fboxSep  = 3 * unity   // \fboxsep: gap between content and frame (3pt)
	fboxRule = defaultRule // \fboxrule: frame line thickness (0.4pt)
)

// frameNode is a framed box: the content packed as an hbox, surrounded by a rule
// of thickness rule with sep of clearance on every side. It sits on the content's
// baseline, the frame extending sep+rule above the content and sep+rule below.
type frameNode struct {
	inner     *boxNode
	sep, rule int
	bg        uint32 // background fill colour (0xRRGGBB; 0 = none) — \colorbox/\fcolorbox
	ruleColor uint32 // frame colour (0xRRGGBB; 0 = black) — \fcolorbox
}

func (frameNode) isNode() {}

// width, height and depth are the frame's outer reference-point dimensions (sp):
// the inner box grown by sep+rule on each side.
func (fr frameNode) width() int  { return fr.inner.width + 2*(fr.sep+fr.rule) }
func (fr frameNode) height() int { return fr.inner.height + fr.sep + fr.rule }
func (fr frameNode) depth() int  { return fr.inner.depth + fr.sep + fr.rule }

// doFbox implements \fbox{content}: it packs the content at its natural width and
// wraps it in a frame at the default thicknesses.
func (e *Engine) doFbox() frameNode {
	list, _ := e.grabHboxList()
	return frameNode{inner: hpackSP(list, packNatural, 0), sep: fboxSep, rule: fboxRule}
}

// doFramebox implements \framebox[width][pos]{content}: with no [width] it behaves
// like \fbox; with [width] it packs the content to that width, aligned l/c/r per
// the optional [pos] (default c) using fil glue.
func (e *Engine) doFramebox() frameNode {
	width, hasWidth := e.scanOptBracketDimen()
	pos := e.scanOptBracketPos()
	list, _ := e.grabHboxList()
	var inner *boxNode
	if hasWidth {
		inner = hpackSP(alignList(list, pos), packTo, width)
	} else {
		inner = hpackSP(list, packNatural, 0)
	}
	return frameNode{inner: inner, sep: fboxSep, rule: fboxRule}
}

// alignList surrounds a content list with fil glue so hpacking it "to" a wider
// target aligns the content: 'l' pads on the right, 'r' on the left, 'c' (the
// default) on both sides.
func alignList(list []node, pos byte) []node {
	fil := func() node { return glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}} }
	switch pos {
	case 'l':
		return append(append([]node{}, list...), fil())
	case 'r':
		return append([]node{fil()}, list...)
	default: // 'c'
		return append(append([]node{fil()}, list...), fil())
	}
}

// grabHboxList reads a braced {content} group and returns its material as a node
// list, built in its own group (like the body of an \hbox). The second result is
// false when no '{' follows (the token is backed out and an empty list returned).
func (e *Engine) grabHboxList() ([]node, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return nil, false
	}
	e.beginGroupKind(boxGroup)
	list := e.buildBoxList()
	e.endGroup()
	return list, true
}

// scanOptBracketDimen reads an optional [dimen] and returns its value and whether
// one was present (a bare [] or absent bracket yields (0, false) — matching
// \framebox's "no explicit width" case).
func (e *Engine) scanOptBracketDimen() (int, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 0, false
	}
	if t.cs_ || t.ch != '[' {
		e.back(t)
		return 0, false
	}
	// An empty bracket [] means "natural width": peek for an immediate ']'.
	u, ok := e.getXToken()
	if ok && !u.cs_ && u.ch == ']' {
		return 0, false
	}
	if ok {
		e.back(u)
	}
	d := e.scanDimen()
	if c, ok := e.getXToken(); ok && !(!c.cs_ && c.ch == ']') {
		e.back(c)
	}
	return d, true
}

// scanOptBracketPos reads an optional [pos] alignment letter (l/c/r) and returns
// it, defaulting to 'c' when the bracket or a recognised letter is absent.
func (e *Engine) scanOptBracketPos() byte {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 'c'
	}
	if t.cs_ || t.ch != '[' {
		e.back(t)
		return 'c'
	}
	pos := byte('c')
	for {
		u, ok := e.getNext()
		if !ok || (!u.cs_ && u.ch == ']') {
			break
		}
		if !u.cs_ && (u.ch == 'l' || u.ch == 'c' || u.ch == 'r') {
			pos = byte(u.ch)
		}
	}
	return pos
}
