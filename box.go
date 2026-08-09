// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file is TeX's box model and the packers hpack/vpack, plus the paragraph
// builder that turns a horizontal list into a vertical list of line boxes using
// the Knuth–Plass breakpoints. It is the part of the stomach that produces the
// box tree a driver (DVI/PDF) will later ship.

// Node is an element of a packed box tree.
type Node interface{ dims() (w, h, d float64) }

// Char is a set glyph (a leaf box with width/height/depth).
type Char struct {
	R       rune
	W, H, D float64
}

func (c Char) dims() (float64, float64, float64) { return c.W, c.H, c.D }

// Rule is a filled rectangle (e.g. a fraction bar or \hrule).
type Rule struct{ W, H, D float64 }

func (r Rule) dims() (float64, float64, float64) { return r.W, r.H, r.D }

// SetGlue is glue after packing: Set is the actual width it was stretched or
// shrunk to.
type SetGlue struct {
	W, Stretch, Shrink float64
	Set                float64
}

func (g SetGlue) dims() (float64, float64, float64) { return g.Set, 0, 0 }

// HBox is a horizontal box: its list is set left-to-right on a common baseline.
type HBox struct {
	W, H, D float64
	GlueSet float64 // adjustment ratio applied to the glue (for reference)
	List    []Node
}

func (b HBox) dims() (float64, float64, float64) { return b.W, b.H, b.D }

// VBox is a vertical box: its list is stacked; the reference point is the
// baseline of the last box (TeX's \vbox).
type VBox struct {
	W, H, D float64
	List    []Node
}

func (b VBox) dims() (float64, float64, float64) { return b.W, b.H, b.D }

// hpack packs a horizontal list. If toWidth is nil the box takes its natural
// width; otherwise glue is set (stretched/shrunk) to reach *toWidth. Height and
// depth are the max over the list's boxes.
func hpack(list []Node, toWidth *float64) HBox {
	var natural, stretch, shrink, h, d float64
	for _, n := range list {
		if g, ok := n.(SetGlue); ok {
			natural += g.W // natural (unset) width
			stretch += g.Stretch
			shrink += g.Shrink
			continue
		}
		w, nh, nd := n.dims()
		natural += w
		if nh > h {
			h = nh
		}
		if nd > d {
			d = nd
		}
	}
	target := natural
	set := 0.0
	if toWidth != nil {
		target = *toWidth
		set = ratio(natural, stretch, shrink, target)
	}
	out := make([]Node, len(list))
	for i, n := range list {
		if g, ok := n.(SetGlue); ok {
			g.Set = g.W + glueDelta(set, g.Stretch, g.Shrink)
			out[i] = g
		} else {
			out[i] = n
		}
	}
	return HBox{W: target, H: h, D: d, GlueSet: set, List: out}
}

// glueDelta is the width added to (or removed from) a glue item for adjustment
// ratio r: stretch when r>0, shrink when r<0.
func glueDelta(r, stretch, shrink float64) float64 {
	if r >= 0 {
		if isInf(r) {
			return 0
		}
		return r * stretch
	}
	return r * shrink // r<0 → negative → shrink
}

func isInf(r float64) bool { return r > 1e300 || r < -1e300 }

// vpack stacks a vertical list of boxes separated by baselineskip glue. The
// resulting VBox's height is the total from the top to the last baseline, and
// its depth is the last box's depth (TeX's \vbox reference point).
func vpack(lines []Node, baselineskip float64) VBox {
	var w, total float64
	out := make([]Node, 0, len(lines)*2)
	var lastDepth float64
	for i, ln := range lines {
		lw, lh, ld := ln.dims()
		if lw > w {
			w = lw
		}
		if i > 0 {
			gap := baselineskip - lastDepth - lh
			if gap < 0 {
				gap = 0
			}
			out = append(out, SetGlue{W: gap, Set: gap})
			total += gap
		}
		out = append(out, ln)
		total += lh + ld
		lastDepth = ld
	}
	return VBox{W: w, H: total - lastDepth, D: lastDepth, List: out}
}

// Paragraph is a fully built paragraph: a vertical box of line boxes.
type Paragraph struct {
	Box   VBox
	Lines []Line
}

// BuildParagraph runs Knuth–Plass over items, packs each resulting line to
// lineWidth with hpack, and stacks the line boxes with vpack. lineHeight and
// lineDepth are used for boxes lacking explicit metrics; baselineskip sets the
// inter-line spacing.
func BuildParagraph(items []Item, lineWidth, tolerance, linePenalty, baselineskip float64) (Paragraph, bool) {
	lines, ok := KnuthPlass(items, lineWidth, tolerance, linePenalty)
	if !ok {
		return Paragraph{}, false
	}
	var lineBoxes []Node
	for _, ln := range lines {
		var hl []Node
		for i := ln.Start; i < ln.End; i++ {
			it := items[i]
			switch it.Kind {
			case KBox:
				hl = append(hl, Char{W: it.Width, H: lineHeightFor(it), D: 0})
			case KGlue:
				hl = append(hl, SetGlue{W: it.Width, Stretch: it.Stretch, Shrink: it.Shrink})
			}
		}
		w := lineWidth
		lineBoxes = append(lineBoxes, hpack(hl, &w))
	}
	return Paragraph{Box: vpack(lineBoxes, baselineskip), Lines: lines}, true
}

// lineHeightFor is a placeholder box height until real font metrics arrive
// (fonts are the next stage); a box carries no height field yet, so use a unit.
func lineHeightFor(it Item) float64 {
	_ = it
	return 1
}
