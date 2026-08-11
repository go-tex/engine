// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the phantom and smash box commands. \phantom{x} reserves the
// space of x without drawing it; \hphantom keeps only its width, \vphantom only its
// height and depth. \smash{x} draws x but reports zero height and depth, so the
// surrounding lines ignore its vertical extent. They are built on the same hbox
// machinery as \mbox: the content is packed to measure it, then made invisible
// (empty list) or zero-dimensioned as required. No new node type is needed.

// phantomKind selects which dimensions of the measured content survive.
type phantomKind byte

const (
	phantomFull phantomKind = 'f' // \phantom: width, height and depth
	phantomH    phantomKind = 'h' // \hphantom: width only
	phantomV    phantomKind = 'v' // \vphantom: height and depth only
)

// makePhantom reads {content}, measures it as an hbox and returns an EMPTY box (so
// nothing is drawn) carrying the requested subset of the content's dimensions.
func (e *Engine) makePhantom(kind phantomKind) *boxNode {
	m := hpackSP(e.grabHboxListOnly(), packNatural, 0)
	b := &boxNode{kind: hbox}
	if kind == phantomFull || kind == phantomH {
		b.width = m.width
	}
	if kind == phantomFull || kind == phantomV {
		b.height = m.height
		b.depth = m.depth
	}
	return b
}

// makeSmash reads {content} and returns it packed as an hbox but with zero height
// and depth, so it overlaps its neighbours vertically without affecting line
// spacing (TeX's \smash, the vertical=both case).
func (e *Engine) makeSmash() *boxNode {
	b := hpackSP(e.grabHboxListOnly(), packNatural, 0)
	b.height = 0
	b.depth = 0
	return b
}

// grabHboxListOnly is grabHboxList without the "present?" flag, for callers that
// always want a (possibly empty) list.
func (e *Engine) grabHboxListOnly() []node {
	list, _ := e.grabHboxList()
	return list
}
