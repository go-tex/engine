package engine

// eso-pic's shipout pictures: material drawn at the paper's lower-left corner on a
// page as it ships, BEHIND the body (the BG layer) or over it (FG and the plain
// \AddToShipoutPicture).
//
// ⛔ WHAT THE REFERENCE DOES, read in eso-pic.sty and measured against tectonic:
//   - \AddToShipoutPicture{...} adds to a list that EVERY later page draws;
//   - the starred form draws on the CURRENT page only (the page it is shipped
//     with), and is gone on the next;
//   - \ClearShipoutPicture empties the list.
//
// This engine paginates after reading, with no shipout hook, so the picture rides
// the vertical list as a node (the way \thispagestyle and \mark do): a node lands on
// the page it was written on by construction, and the page assembler lifts it out.
// Material written in the PREAMBLE has no page to land on: a plain picture goes
// into shipPicPre, a starred one into shipPicPreOnce (page 1 only).
//
// Pages() may run many times, so the per-run state (shipPics, shipPicSeen,
// shipPicFirst) is rebuilt from the preamble's at the top of each run.
//
// The picture box is a zero-size picture environment, so its reference point is the
// origin of the picture coordinates; both renderers put that point at the paper's
// lower-left corner (x = 0, baseline = paper height).

// pageOverlay is one picture drawn on a page, behind (fg false) or over (fg true).
type pageOverlay struct {
	box *boxNode
	fg  bool
}

// shipPicNode carries one \AddToShipoutPicture* call through page breaking, or a
// \ClearShipoutPicture (clear true, box nil).
type shipPicNode struct {
	box   *boxNode
	star  bool
	fg    bool
	clear bool
	id    int
}

func (shipPicNode) isNode() {}

// doShipoutPic is \gotex@shipoutpic{star}{bg|fg|clear}<box number>, the hook the
// eso-pic commands of the TeX prelude end in.
func (e *Engine) doShipoutPic() {
	star := e.readBraceName() == "1"
	layer := e.readBraceName()
	n := shipPicNode{star: star, clear: layer == "clear", fg: layer == "fg"}
	if !n.clear {
		src := e.getBox(e.scanInt())
		if src == nil {
			return
		}
		n.box = cloneBox(src)
	}
	e.shipPicSeq++
	n.id = e.shipPicSeq
	if e.inBody {
		e.contribute(n)
		return
	}
	switch {
	case n.clear:
		e.shipPicPre, e.shipPicPreOnce = nil, nil
	case n.star:
		e.shipPicPreOnce = append(e.shipPicPreOnce, pageOverlay{box: n.box, fg: n.fg})
	default:
		e.shipPicPre = append(e.shipPicPre, pageOverlay{box: n.box, fg: n.fg})
	}
}

// resetShipPics starts a pagination run from the preamble's state.
func (e *Engine) resetShipPics() {
	e.shipPics = append([]pageOverlay(nil), e.shipPicPre...)
	e.shipPicSeen = map[int]bool{}
	e.shipPicFirst = true
}

// takeShipPics lifts the shipout-picture nodes out of a vertical list, in order.
func takeShipPics(vlist []node) (pics []shipPicNode, rest []node) {
	found := false
	for _, n := range vlist {
		if _, ok := n.(shipPicNode); ok {
			found = true
			break
		}
	}
	if !found {
		return nil, vlist
	}
	rest = make([]node, 0, len(vlist))
	for _, n := range vlist {
		if sp, ok := n.(shipPicNode); ok {
			pics = append(pics, sp)
			continue
		}
		rest = append(rest, n)
	}
	return pics, rest
}

// attachShipPics gives a finished page box its overlays: the persistent list, as it
// stands after this page's own plain pictures joined it, then this page's starred
// ones. The first page assembled also carries the preamble's starred pictures.
func (e *Engine) attachShipPics(b *boxNode, pics []shipPicNode) {
	if b == nil {
		return
	}
	if e.shipPicSeen == nil {
		e.resetShipPics()
	}
	var once []pageOverlay
	if e.shipPicFirst {
		e.shipPicFirst = false
		once = append(once, e.shipPicPreOnce...)
	}
	for _, p := range pics {
		if e.shipPicSeen[p.id] {
			continue
		}
		e.shipPicSeen[p.id] = true
		switch {
		case p.clear:
			e.shipPics = nil
			if p.star {
				once = nil
			}
		case p.star:
			once = append(once, pageOverlay{box: p.box, fg: p.fg})
		default:
			e.shipPics = append(e.shipPics, pageOverlay{box: p.box, fg: p.fg})
		}
	}
	b.overlays = append(append([]pageOverlay(nil), e.shipPics...), once...)
}
