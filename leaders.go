// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \leaders, \cleaders and \xleaders <box or rule><glue> fill the glue's width with
// copies of a box (or with a rule). tex.web §149 binds them as one command with
// three subtypes; §1078 reads the box-or-rule and then the glue; §626-§628 (hlist_out)
// and §635-§637 (vlist_out) set them.
//
//	\leaders   ALIGNED: tiles sit on a grid anchored at the left edge of the enclosing
//	           box, so the dots of two lines line up (§626: cur_h = left_edge +
//	           leader_wd * ((cur_h - left_edge) div leader_wd), then one tile more if
//	           that fell before the glue); a tile is set only if it fits whole.
//	\cleaders  CENTERED: lq whole tiles, the leftover lr = width - lq*leader_wd split
//	           in two, half before the first tile and half after the last (§627).
//	\xleaders  EXPANDED: the leftover is spread between the tiles, lx = lr div (lq+1)
//	           in each of the lq+1 gaps (§628).
//
// A rule leader paints one rule across the whole glue (§624-§625), with the rule's
// own height and depth (a running dimension takes the enclosing box's).
//
// ⚠ Only HORIZONTAL leaders paint. A \leaders followed by \vskip keeps the glue's
// space (the page is the same height) and paints nothing in it.

// leaderMode is the layout of the tiles inside a leader's glue.
type leaderMode uint8

const (
	leadersAligned  leaderMode = iota // \leaders
	leadersCentered                   // \cleaders
	leadersExpanded                   // \xleaders
)

// leaderTileOrigins returns the left edge of each tile, in pt. edge is the left edge
// of the enclosing box, x the glue's start, w its set width, tw one tile's width.
func leaderTileOrigins(mode leaderMode, edge, x, w, tw float64) []float64 {
	if tw <= 0 { // a non-positive w gives lq <= 0 below, and dotLeaderOrigins refuses it itself
		return nil
	}
	if mode == leadersAligned {
		return dotLeaderOrigins(edge, x, w, tw)
	}
	lq := int(w/tw + 1e-9)
	if lq <= 0 {
		return nil
	}
	lr := w - float64(lq)*tw
	cur, lx := x, 0.0
	if mode == leadersCentered {
		cur += lr / 2
	} else {
		lx = lr / float64(lq+1)
		cur += (lr - float64(lq-1)*lx) / 2
	}
	out := make([]float64, 0, lq)
	for i := 0; i < lq; i++ {
		out = append(out, cur)
		cur += tw + lx
	}
	return out
}

// leaderTileWidth is the width a tile advances: a box's width, 0 for a rule (a rule
// leader is painted as one rule, not tiled).
func leaderTileWidth(tile node) int {
	if b, ok := tile.(*boxNode); ok && b != nil {
		return b.width
	}
	return 0
}

// installLeaders registers the three primitives.
func (e *Engine) installLeaders() {
	for name, mode := range map[string]leaderMode{
		"leaders": leadersAligned, "cleaders": leadersCentered, "xleaders": leadersExpanded,
	} {
		mode := mode
		e.prim(name, func(e *Engine) { e.doLeaders(mode) })
	}
}

// doLeaders reads <box or rule> then <glue> and places the leader glue.
func (e *Engine) doLeaders(mode leaderMode) {
	var tile node
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return
	}
	if m := e.meaningOf(t); t.cs_ && m != nil && m.kind == mPrim && (m.name == "hrule" || m.name == "vrule") {
		tile = e.scanRule(m.name == "hrule")
	} else {
		e.back(t)
		b := e.scanBox()
		if b == nil {
			return // tex.web §1078: "A <box> was supposed to be here" -- the glue is left to be read as itself
		}
		tile = b
	}
	e.skipOptSpace()
	g, ok := e.getXToken()
	if !ok {
		return
	}
	gm := e.meaningOf(g)
	if !g.cs_ || gm == nil || gm.kind != mPrim {
		e.back(g)
		return // "Leaders not followed by proper glue": the tile is dropped
	}
	var spec glueSpec
	horizontal := true
	switch gm.name {
	case "hskip":
		spec = e.scanGlue()
	case "hfil":
		spec = glueSpec{stretch: unity, stretchOrder: 1}
	case "hfill":
		spec = glueSpec{stretch: unity, stretchOrder: 2}
	case "hss":
		spec = glueSpec{stretch: unity, stretchOrder: 1, shrink: unity, shrinkOrder: 1}
	case "vskip":
		spec, horizontal = e.scanGlue(), false
	case "vfil":
		spec, horizontal = glueSpec{stretch: unity, stretchOrder: 1}, false
	case "vfill":
		spec, horizontal = glueSpec{stretch: unity, stretchOrder: 2}, false
	case "vss":
		spec, horizontal = glueSpec{stretch: unity, stretchOrder: 1, shrink: unity, shrinkOrder: 1}, false
	default:
		e.back(g)
		return
	}
	n := glueNode{spec: spec, tile: tile, lmode: mode}
	if horizontal {
		e.placeHGlueNode(n)
	} else {
		e.contribute(glueNode{spec: spec})
	}
}
