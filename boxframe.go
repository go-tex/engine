// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

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

// doTightFrame implements \gotex@tightframe{content}: \fbox with \fboxsep set to
// zero, which is what LaTeX's \frame is ("\leavevmode\fboxsep\z@\fbox{#1}",
// ltboxes). A picture's \framebox(60,40) must draw its rule ON the 60x40 box the
// author declared, not 3pt outside it.
func (e *Engine) doTightFrame() frameNode {
	list, _ := e.grabHboxList()
	return frameNode{inner: hpackSP(list, packNatural, 0), sep: 0, rule: fboxRule}
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
// it, defaulting to 'c' when the bracket or a recognised letter is absent. This is
// the HORIZONTAL set, for \makebox and \framebox.
//
// ⛔ It is not the one minipage, parbox, tabular and subfigure want: those take
// t/c/b, a VERTICAL anchor, and they all called this reader — which does not
// recognise t or b and so returned 'c' for every one of them. alignParbox's 't'
// and 'b' branches were therefore unreachable from any caller, carefully written
// and never run. Measured on \hbox{\begin{minipage}[…]{60pt}AAA\\BBB\\CCC\end{minipage}},
// \ht/\dp against tectonic:
//
//	       tectonic          before            after
//	[c]   18.19/13.19   17.865/12.865   17.865/12.865
//	[t]    7.16/24.22   17.865/12.865     7.16/24.22
//	[b]   31.16/00.22   17.865/12.865    31.16/00.22
//
// Two readers rather than a union of letters, so the next caller cannot pick the
// wrong set by accident: a union would have accepted [t] in a \makebox and [l] in
// a minipage, each silently defaulting somewhere downstream.
func (e *Engine) scanOptBracketPos() byte {
	return e.scanOptBracketLetter("lcr")
}

// scanOptBracketVPos reads the VERTICAL [pos] letter (t/c/b) that minipage,
// parbox, tabular and subfigure take, defaulting to 'c' — which is what latex.ltx
// documents for all four, and what alignParbox centres on the math axis.
func (e *Engine) scanOptBracketVPos() byte {
	return e.scanOptBracketLetter("tcb")
}

// scanOptBracketLetter reads an optional [x] whose letter is one of accept.
func (e *Engine) scanOptBracketLetter(accept string) byte {
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
		if !u.cs_ && u.ch < 0x80 && strings.ContainsRune(accept, u.ch) {
			pos = byte(u.ch)
		}
	}
	return pos
}
