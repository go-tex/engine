// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the longtable environment: a tabular that may run over
// several pages. Undefined, \begin{longtable} was an unknown environment whose body
// lenient mode set as running text — over the 157-paper arXiv corpus the 9 papers
// that use one carry 43 pages of page-count error between them.
//
// What is reproduced is the TABLE, not its pagination: the rows are assembled into
// one tabular box through the machinery tabular/tabularx already use, so column
// specs, rules, \\, &, \multicolumn and booktabs behave as they do there. A
// longtable that fits a page is then identical to the real one; a longer one is a
// single tall box the page builder splits like any other oversized box.
//
// It depends on readColumnWidth (tabular.go): Pandoc, which generates most of the
// corpus's longtables, writes computed column widths that must be consumed whole or
// the spec unbalances and the table is swallowed.

// longtableMarks are the row-group terminators longtable adds to tabular's syntax:
//
//	first head rows \endfirsthead   — the head shown on the FIRST page
//	repeat head rows \endhead       — the head repeated on every LATER page
//	foot rows        \endfoot       — the foot on every page but the last
//	last foot rows   \endlastfoot   — the foot on the LAST page
var longtableMarks = map[string]bool{
	"endfirsthead": true, "endhead": true, "endfoot": true, "endlastfoot": true,
}

// doLongtable typesets \begin{longtable}[pos]{spec} … \end{longtable}. The optional
// [pos] is a whole-box placement the engine does not model, scanned and discarded as
// doTabular does with [t]/[b]/[c].
func (e *Engine) doLongtable() {
	e.scanOptBracketToks()
	aligns, pwidths, vrules := e.scanColSpec()
	items := e.collectTabularBody("longtable")
	e.place(e.buildTabularBox(aligns, pwidths, vrules, longtableRows(items)))
}

// longtableRows reduces a collected longtable body to the rows a single-page
// rendering shows. It KEEPS every row except the two that are provably duplicates:
// the \endhead group when an \endfirsthead group also exists, and the \endfoot group
// when an \endlastfoot group also exists — set as one table there is exactly one
// first page and one last page, so emitting both would print the header twice.
//
// Dropping only proven duplicates is deliberate. An earlier version assumed the
// marker groups always precede the body — which is how longtable is normally written
// — and treated every run before a marker as a head or foot; on a paper whose
// markers do not sit at the front that threw away body rows. A table that loses rows
// is worse than a table set as running text, so the rule is subtractive and
// order-independent.
func longtableRows(items []tabItem) []tabItem {
	var hasFirstHead, hasLastFoot bool
	for _, it := range items {
		switch it.ltmark {
		case "endfirsthead":
			hasFirstHead = true
		case "endlastfoot":
			hasLastFoot = true
		}
	}
	out := make([]tabItem, 0, len(items))
	var run []tabItem
	for _, it := range items {
		if it.ltmark == "" {
			run = append(run, it)
			continue
		}
		dup := (it.ltmark == "endhead" && hasFirstHead) || (it.ltmark == "endfoot" && hasLastFoot)
		if !dup {
			out = append(out, run...)
		}
		run = nil
	}
	return append(out, run...) // rows after the last marker: the body
}
