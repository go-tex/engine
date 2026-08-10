// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "math"

// This file is the page builder: it splits the main vertical list into pages no
// taller than \vsize using TeX's cost-based rule — at each legal breakpoint the
// cost is the page badness plus the breakpoint penalty, the page fires at the
// least-cost break once it would overflow, a forced break (\penalty ≤ −10000)
// fires immediately, and \penalty ≥ 10000 forbids a break. Glue/penalty/kern at
// a break is discarded. \vsplit reuses the simpler height-limited splitVList.
// (Insertions, \topskip and a token \output routine are future work.)

// vContribution is a node's contribution to the running page height (sp).
func vContribution(n node) int {
	switch c := n.(type) {
	case *boxNode:
		return c.height + c.depth
	case ruleNode:
		return c.height + c.depth
	case kernNode:
		return c.width
	case glueNode:
		return c.spec.width
	case charNode:
		return c.height + c.depth
	case mathNode:
		return c.height + c.depth
	}
	return 0
}

// Pages splits the main vertical list into pages using TeX's cost-based page
// builder (§1005): at each legal breakpoint it forms a cost = page badness +
// penalty, breaks at the least-cost point once the page would overflow, honours a
// forced break (\penalty ≤ −10000) immediately, and never breaks at \penalty ≥
// 10000. Each page is vpacked at natural height. (Insertions/\topskip are future.)
func (e *Engine) Pages() []*boxNode {
	var pages []*boxNode
	list := e.mvl
	for start := 0; start < len(list); {
		end := e.findPageBreak(list, start)
		if page := trimTrailingGlue(list[start:end]); len(page) > 0 {
			pages = append(pages, vpackSP(page, packNatural, 0))
		}
		start = skipDiscardable(list, end)
		if end <= start-1 { // safety: always make progress
			start = end + 1
		}
		if end == len(list) {
			break
		}
	}
	return pages
}

// findPageBreak returns the exclusive end index of the page beginning at start.
func (e *Engine) findPageBreak(list []node, start int) int {
	h := 0
	best := -1
	least := math.Inf(1)
	for i := start; i < len(list); i++ {
		if ok, pen := legalPageBreak(list, i, start); ok {
			if pen <= -10000 {
				return i // forced break
			}
			b := pageBadness(h, e.vsize)
			if math.IsInf(b, 1) { // already overfull at a legal break
				if best >= 0 {
					return best
				}
				return i
			}
			if c := b + float64(pen); c <= least {
				least, best = c, i
			}
		}
		h += vContribution(list[i])
		if h > e.vsize && best >= 0 {
			return best
		}
	}
	return len(list)
}

// legalPageBreak reports whether index i is a legal page breakpoint and its
// penalty: a penalty node (< 10000), or glue that follows box-like material.
func legalPageBreak(list []node, i, start int) (bool, int) {
	switch n := list[i].(type) {
	case penaltyNode:
		if n.penalty >= 10000 {
			return false, 0
		}
		return true, n.penalty
	case glueNode:
		if i > start && isBoxLike(list[i-1]) {
			return true, 0
		}
	}
	return false, 0
}

func isBoxLike(n node) bool {
	switch n.(type) {
	case *boxNode, ruleNode, charNode, mathNode:
		return true
	}
	return false
}

// pageBadness measures how far a page of height h falls short of \vsize: 0 when
// exactly full, up to 10000 when empty, and +Inf when overfull.
func pageBadness(h, vsize int) float64 {
	if h > vsize {
		return math.Inf(1)
	}
	if vsize <= 0 {
		return 0
	}
	f := float64(vsize-h) / float64(vsize)
	return 10000 * f * f * f
}

// skipDiscardable advances past glue/penalty/kern at a page break (discarded).
func skipDiscardable(list []node, i int) int {
	for i < len(list) {
		switch list[i].(type) {
		case glueNode, penaltyNode, kernNode:
			i++
		default:
			return i
		}
	}
	return i
}

// trimTrailingGlue drops trailing glue/penalty/kern from a page's material.
func trimTrailingGlue(page []node) []node {
	for len(page) > 0 {
		switch page[len(page)-1].(type) {
		case glueNode, penaltyNode, kernNode:
			page = page[:len(page)-1]
		default:
			return page
		}
	}
	return page
}

// splitVList splits a vertical list at the first legal break that keeps the
// height within vsize, returning the taken part and the remainder (with a leading
// glue/kern discarded, as at a page break).
func splitVList(list []node, vsize int) ([]node, []node) {
	h := 0
	for i, n := range list {
		c := vContribution(n)
		_, isGlue := n.(glueNode)
		_, isKern := n.(kernNode)
		if h+c > vsize && h > 0 {
			if isGlue || isKern {
				return list[:i], list[i+1:]
			}
			return list[:i], list[i:]
		}
		h += c
	}
	return list, nil
}

// doVsplit handles \vsplit<n> to <dimen>: split box register n's vbox to the
// given height, returning the top part (packed to that height) and leaving the
// remainder in register n. Returns nil if the register is void or the syntax is
// malformed.
func (e *Engine) doVsplit() *boxNode {
	n := e.scanInt()
	if !e.scanKeyword("to") {
		return nil
	}
	h := e.scanDimen()
	src := e.getBox(n)
	if src == nil {
		return nil
	}
	top, rest := splitVList(src.list, h)
	if len(rest) == 0 {
		e.setBox(n, nil)
	} else {
		e.setBox(n, vpackSP(rest, packNatural, 0))
	}
	return vpackSP(top, packTo, h)
}

// RenderPages renders each page of the main vertical list to its own SVG string.
func (e *Engine) RenderPages(margin float64) []string {
	pages := e.Pages()
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = renderBoxSVG(p, margin, e.curFont)
	}
	return out
}
