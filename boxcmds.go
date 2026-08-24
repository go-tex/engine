// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's box-placement and save-box commands, built on the
// same hpack/alignList machinery as \framebox (see boxframe.go) and the box
// registers used by \setbox/\box/\copy (see stomach.go):
//
//   \makebox[width][pos]{content}  an hbox of the given width (natural if no
//                                  [width]), content aligned l/c/r (default c)
//                                  with fil glue — \framebox without the frame.
//   \raisebox{lift}{content}       the content hbox shifted up by lift (a dimen;
//                                  negative lowers). \raisebox{lift}[ht][dp]{…}
//                                  additionally overrides the box's reported
//                                  height/depth so surrounding lines see that
//                                  vertical extent regardless of the content.
//   \newsavebox{\name}             allocate a \box register and bind \name as its
//                                  handle (an mBoxRef, the box analogue of the
//                                  mCountRef a \newcount hands out).
//   \sbox{\name}{content}          store \hbox{content} in \name's register.
//   \savebox{\name}[w][pos]{…}     store \makebox[w][pos]{…} in \name's register.
//   \usebox{\name}                 place a copy of \name's register inline.
//
// Box-register assignment (\sbox/\savebox, like the engine's \setbox) is global:
// setBox writes the register with no group save/restore, so a stored box survives
// the enclosing group — matching this engine's existing \setbox behaviour. The
// \newsavebox handle is likewise defined globally, so a preamble allocation is
// visible everywhere. \usebox places a *copy* (cloneBox), so the register is left
// intact and can be reused any number of times.

// loadBoxCmds registers the box-placement and save-box primitives. It is called at
// the end of loadStomach so the commands share the box-building machinery.
func (e *Engine) loadBoxCmds() {
	e.prim("makebox", func(e *Engine) { e.place(e.doMakebox()) })
	e.prim("raisebox", func(e *Engine) { e.place(e.doRaisebox()) })
	e.prim("newsavebox", func(e *Engine) { e.doNewsavebox() })
	e.prim("sbox", func(e *Engine) { e.doSbox() })
	e.prim("savebox", func(e *Engine) { e.doSavebox() })
	e.prim("usebox", func(e *Engine) { e.place(e.doUsebox()) })
}

// doMakebox implements \makebox[width][pos]{content}: with no [width] it packs the
// content at its natural width; with [width] it packs to that width, the content
// aligned l/c/r (default c) via fil glue — \framebox (see doFramebox) without the
// surrounding frame.
func (e *Engine) doMakebox() *boxNode {
	width, hasWidth := e.scanOptBracketDimen()
	pos := e.scanOptBracketPos()
	list, _ := e.grabHboxList()
	if hasWidth {
		return hpackSP(alignList(list, pos), packTo, width)
	}
	return hpackSP(list, packNatural, 0)
}

// doRaisebox implements \raisebox{lift}{content} and \raisebox{lift}[ht][dp]{…}:
// the content is packed as a natural hbox and shifted up by lift (shift is
// positive-downward, so shift = -lift; a negative lift lowers). When the optional
// [ht]/[dp] are given, the box's reported height/depth are overridden to those
// values so surrounding material sees the stated vertical extent regardless of the
// content's own metrics.
func (e *Engine) doRaisebox() *boxNode {
	lift := e.readBraceDimen()
	ht, hasHt := e.scanOptBracketDimen()
	dp, hasDp := e.scanOptBracketDimen()
	list, _ := e.grabHboxList()
	b := hpackSP(list, packNatural, 0)
	b.shift = -lift
	if hasHt {
		b.height = ht
	}
	if hasDp {
		b.depth = dp
	}
	return b
}

// doNewsavebox implements \newsavebox{\name}: it allocates the next free \box
// register and binds \name as its handle (an mBoxRef, whose code is the register
// index). Re-allocating an existing handle is a no-op (its register is kept), and
// the binding is global so a preamble \newsavebox is visible everywhere.
func (e *Engine) doNewsavebox() {
	name := e.readBraceCSName()
	if name == "" {
		return
	}
	if m := e.eq[name]; m != nil && m.kind == mBoxRef {
		return // already allocated: reuse the register
	}
	if e.allocBox >= 256 {
		return // out of box registers
	}
	e.define(name, &meaning{kind: mBoxRef, code: e.allocBox}, true)
	e.allocBox++
}

// doSbox implements \sbox{\name}{content}: it stores \hbox{content} (packed at its
// natural width) in \name's register. The {content} group is always consumed to
// keep the input in sync, even when \name is not a valid save-box handle.
func (e *Engine) doSbox() {
	reg, ok := e.readBoxHandle()
	list, _ := e.grabHboxList()
	if ok {
		e.setBox(reg, hpackSP(list, packNatural, 0))
	}
}

// doSavebox implements \savebox{\name}[w][pos]{content}: it stores the result of
// \makebox[w][pos]{content} in \name's register. The \makebox arguments are always
// consumed, even when \name is not a valid save-box handle.
func (e *Engine) doSavebox() {
	reg, ok := e.readBoxHandle()
	b := e.doMakebox()
	if ok {
		e.setBox(reg, b)
	}
}

// doUsebox implements \usebox{\name}: it returns a copy (cloneBox) of \name's
// register so the stored box survives and can be reused. A void register or an
// invalid handle yields a nil box, which place drops.
func (e *Engine) doUsebox() *boxNode {
	reg, ok := e.readBoxHandle()
	if !ok {
		return nil
	}
	return cloneBox(e.getBox(reg))
}

// readBraceCSName reads a {\cs} group and returns the control-sequence name it
// contains (the first cs token), or "" if the group is absent or holds no cs. The
// group's tokens are read without expansion so a save-box handle is not executed.
func (e *Engine) readBraceCSName() string {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return ""
	}
	cs := ""
	for {
		u, ok := e.getNext()
		if !ok || (u.cat == catEnd && !u.cs_) {
			break
		}
		if u.cs_ && cs == "" {
			cs = u.cs
		}
	}
	return cs
}

// readBoxHandle reads a {\name} group and resolves \name to its \box register
// index. The second result is false when the group is absent or \name is not a
// save-box handle (an mBoxRef defined by \newsavebox).
func (e *Engine) readBoxHandle() (int, bool) {
	cs := e.readBraceCSName()
	if cs == "" {
		return -1, false
	}
	if m := e.eq[cs]; m != nil && m.kind == mBoxRef {
		return m.code, true
	}
	return -1, false
}

// \unhbox<n> unpacks a box register onto the list being built, instead of
// placing the box as a single item: the register's contents become part of the
// surrounding list and the register is voided. \unhcopy leaves the register
// alone, and \unvbox / \unvcopy are the vertical pair.
//
// This is how a file accumulates material a piece at a time —
// \setbox0=\hbox{\unhbox0 <more>} appends to box 0 without nesting a box inside
// a box, which would trap the contents at a fixed width. pgf builds a path's
// nodes exactly that way, so while these consumed the register number and threw
// the contents away, every node on a path but the last one disappeared.
//
// TeX refuses to unpack a vbox onto a horizontal list or the reverse; the engine
// contributes nothing in that case, which is what the refusal amounts to here.
func (e *Engine) installUnbox() {
	e.prim("unhbox", func(e *Engine) { e.unbox(hbox, false) })
	e.prim("unhcopy", func(e *Engine) { e.unbox(hbox, true) })
	e.prim("unvbox", func(e *Engine) { e.unbox(vbox, false) })
	e.prim("unvcopy", func(e *Engine) { e.unbox(vbox, true) })
}

func (e *Engine) unbox(want boxKind, keep bool) {
	i := e.boxRegIndex()
	b := e.getBox(i)
	if b == nil || b.kind != want {
		return
	}
	list := b.list
	if keep {
		list = cloneBox(b).list
	} else {
		e.setBox(i, nil)
	}
	for _, n := range list {
		e.place(n)
	}
}
