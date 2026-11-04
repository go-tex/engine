// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \predisplaysize and the choice between the long and the short display skips.
//
// ⛔⛔ THE SHORT SKIPS WERE NEVER SELECTED, and that is 13.95pt of surplus on every
// display. TeX picks between two pairs (tex.web:22578-22585):
//
//	if (d+s <= pre_display_size) or l then   {not enough clearance}
//	  begin g1:=above_display_skip_code; g2:=below_display_skip_code; end
//	else
//	  begin g1:=above_display_short_skip_code; g2:=below_display_short_skip_code; end
//
// — the SHORT pair when the line before the display ends to the LEFT of where the
// display begins, so the two do not collide and little space is needed between them.
// This engine always took the long pair. Measured baseline to baseline across one
// display between two text lines, \abovedisplayskip 10pt against
// \abovedisplayshortskip 0pt and \belowdisplayskip 10pt against
// \belowdisplayshortskip 6pt:
//
//	                au-dessus   en dessous
//	tectonic            11.96       17.93
//	nous, avant         22.00       22.00
//
// 10pt + 4pt = the 13.95pt excess, entirely. On a corpus of 154 arXiv papers a probe
// of 40 displays cost one page more than the reference (go-tex/engine#683).

// preDisplaySize computes TeX's \predisplaysize for the line that precedes a display:
// how far the characters of that final line reach to the right of the reference point,
// plus two ems (tex.web:21781-21813).
//
//	v := shift_amount(just_box) + 2*quad(cur_font);  w := -max_dimen
//	for each node p of the line:
//	    d := its natural width
//	    if p is VISIBLE (char, box, rule): v += d; w := v
//	    else if p is glue that stretches or shrinks INFINITELY: v := max_dimen
//	    else v += d
//	    if v = max_dimen at a visible node: w := max_dimen, stop
//
// ⛔ THE RUNNING SUM STOPS AT THE LAST VISIBLE NODE, not at the end of the list. A
// line ends with \parfillskip — `0pt plus 1fil` — and trailing glue is not something
// the text "reaches"; counting it would make every line look full-width and select
// the long skips always, which is exactly the defect this is here to fix.
//
// ⛔ AND ONLY *INFINITE* STRETCH POISONS THE SUM. Ordinary interword glue stretches
// finitely and must NOT set max_dimen: a justified line would then always report
// max_dimen and, again, always take the long skips. The reference confirms it —
// tectonic reports \predisplaysize as 48.37pt and 93.77pt on two ordinary lines, both
// finite, where a poisoned sum would print 16383.99998pt.
func preDisplaySize(line *boxNode, shift, quad int) int {
	const maxDimen = 1073741823 // 2^30 - 1 sp, TeX's \maxdimen
	if line == nil {
		return -maxDimen
	}
	v, w := shift+2*quad, -maxDimen
	for _, n := range line.list {
		switch c := n.(type) {
		case charNode:
			if v >= maxDimen {
				return maxDimen
			}
			v += c.width
			w = v
		case *boxNode:
			if c == nil {
				continue
			}
			if v >= maxDimen {
				return maxDimen
			}
			v += c.width
			w = v
		case ruleNode:
			if v >= maxDimen {
				return maxDimen
			}
			v += c.width
			w = v
		case glueNode:
			if c.spec.stretchOrder > 0 || c.spec.shrinkOrder > 0 {
				v = maxDimen
				continue
			}
			if v < maxDimen {
				v += c.spec.width
			}
		case kernNode:
			if v < maxDimen {
				v += c.width
			}
		case mathNode:
			if v >= maxDimen {
				return maxDimen
			}
			v += c.width
			w = v
		}
	}
	return w
}

// naturalContentWidth is the width a box's contents occupy before any glue is set —
// the sum of every item's natural width. A display box is packed to \hsize with
// stretchable glue on each side, so this is what says where the formula itself sits.
func naturalContentWidth(b *boxNode) int {
	if b == nil {
		return 0
	}
	w := 0
	for _, n := range b.list {
		switch c := n.(type) {
		case charNode:
			w += c.width
		case *boxNode:
			if c != nil {
				w += c.width
			}
		case ruleNode:
			w += c.width
		case glueNode:
			w += c.spec.width
		case kernNode:
			w += c.width
		case mathNode:
			w += c.width
		}
	}
	return w
}

// displayLeftEdge is TeX's d+s for a display: where the formula starts, measured from
// the left of the text block. The display box is packed to \hsize around a centring
// glue on each side, so the formula's left edge is half of what the packing had to
// distribute.
//
// ⚠ This reads the CENTRED case, which is what \[…\] and equation produce here. A
// display shifted by \displayindent (fleqn, and the alignment displays) does not
// centre, and would need its own left edge rather than this halving — which is why
// the caller passes the box rather than this function reaching for e.hsize.
func displayLeftEdge(b *boxNode) int {
	if b == nil {
		return 0
	}
	slack := b.width - naturalContentWidth(b)
	if slack <= 0 {
		return 0
	}
	return slack / 2
}

// lastLinePreDisplaySize is \predisplaysize for the line the main vertical list ends
// with — the last line of the paragraph a display has just interrupted.
//
// ⛔ WITH NO SUCH LINE IT IS -\maxdimen, which is what TeX sets for `\noindent$$`
// (tex.web:21761-21763, "if head=tail then pop_nest; w:=-max_dimen"). That makes every
// left edge "far enough" and selects the SHORT skips — the opposite of a safe default,
// so it is spelled out here rather than left to a zero value.
func (e *Engine) lastLinePreDisplaySize() int {
	const maxDimen = 1073741823
	for i := len(e.mvl) - 1; i >= 0; i-- {
		if b, ok := e.mvl[i].(*boxNode); ok && b != nil {
			quad := 0
			if e.curFont != nil {
				quad = e.curFont.sizePt() * unity
			}
			return preDisplaySize(b, b.shift, quad)
		}
	}
	return -maxDimen
}

// useShortDisplaySkips applies tex.web:22580. The long pair is the answer whenever
// the previous line reaches as far as the display's left edge — including the case
// of no previous line at all, where \predisplaysize is -\maxdimen and every edge is
// "far enough", and the case of an equation number on the left (l), which this engine
// does not set.
func useShortDisplaySkips(preSize, leftEdge int) bool {
	return leftEdge > preSize
}
