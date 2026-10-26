// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// prunePageTop is tex.web's prune_page_top (18857-18893): it returns the list with
// every glue, kern and penalty node before the first box or rule DELETED, and a
// \splittopskip glue put in front of that box — « designed so that the topmost
// baseline will be at distance split_top_skip from the top, whenever this is possible
// without backspacing » (18860-18862).
//
//	while p<>null do case type(p) of
//	  hlist_node,vlist_node,rule_node:  <Insert glue for split_top_skip and set p:=null>
//	  whatsit_node,mark_node,ins_node:  prev_p:=p; p:=link(prev_p)
//	  glue_node,kern_node,penalty_node: <delete p>
//
// ⛔ THIS IS WHAT A SPLIT REMAINDER WAS MISSING ENTIRELY, and \vsplit is the only
// caller TeX has for it besides the output routine's insertions. Without it the
// remainder keeps the vertical material that the break discarded — paragraph glue, a
// section's \vskip, a widow penalty — so the next column or page starts lower than it
// should, by an amount nothing in the document declares. Ours dropped exactly ONE
// glue or kern, and only when the break happened to fall on it.
//
// ⛔ And the two errors ran in OPPOSITE directions, which is why neither showed up in
// a page count: the leftover glue pushed the column's first baseline DOWN, while the
// missing \splittopskip left it UP by up to 10pt. A measurement of the first baseline
// alone could read either as the other's absence.
//
// This engine has no mark node; a specialNode is TeX's whatsit and a footnoteNode its
// insertion, so both are kept and stepped over rather than deleted. The switch is
// written out instead of going through vHeightDepth's three kinds, because the kinds
// disagree here: a penalty contributes NOTHING to a page's height and yet is DELETED
// by pruning, so one classification cannot serve both.
func (e *Engine) prunePageTop(list []node) []node {
	out := make([]node, 0, len(list)+1)
	for i, n := range list {
		switch n.(type) {
		case glueNode, kernNode, penaltyNode:
			continue // deleted (tex.web:18879-18881)
		case specialNode, footnoteNode:
			out = append(out, n) // whatsit / insertion: kept (tex.web:18877)
			continue
		}
		// A box or a rule: the glue goes in front of it and the walk stops there,
		// everything after it untouched (tex.web:18888-18893).
		if g, _, ok := e.splitTopskipGlueFor(list[i:]); ok {
			out = append(out, g)
		}
		return append(out, list[i:]...)
	}
	// No box or rule at all: the glue has nothing to sit above, and what is left is
	// whatever whatsits and insertions were in the list. TeX returns the same thing —
	// link(temp_head) after the whole walk — and inserts no glue.
	return out
}
