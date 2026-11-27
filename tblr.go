// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strconv"
	"strings"
)

// This file implements the tabularray package's tblr, longtblr and talltblr
// environments:
//
//	\begin{tblr}[outer]{inner} … \end{tblr}
//
// tabularray is written in expl3, which this engine does not run, so an
// environment of its was an unknown one whose body lenient mode set as running
// text — preamble keys and all: the source of 2407.18384's Notation table
// printed "captiondefault contheaddefault capcontdefault [ caption = …, label =
// … ] colspec = |t0.15|m0.53|b0.23|, rowhead = 1, hlines, …" in the middle of the
// page, and the table, five pages in the reference, took three.
//
// What is reproduced is the table through the machinery tabular already has: the
// body syntax (& and \\) is tabular's, so the inner specification is reduced to
// what that machinery takes — a column specification, the hlines/vlines keys
// that draw every rule, and a width for X columns. The rest of the inner keys
// (row{..}, cell{..}, colors) restyle rows and are read and dropped. rowhead and
// rowfoot, and the outer [caption=…, label=…], are used by longtblr (placeLong).
//
// The column letters are tabularray's (tabularray.sty §"Column types"): l c r,
// the paragraph columns t m b (t is p), X and Q, with an optional [..] of
// per-column keys, and |. Per-column keys are dropped, t/m/b become paragraph
// columns of the width in their braces, and X is a flexible column only when
// the table has a width= to fill.

// tblrSpec is what the inner specification of a tblr environment reduces to.
type tblrSpec struct {
	colspec []tok // the column specification, as tabular's scanColSpec reads it
	width   []tok // value of width=, nil when absent
	hlines  bool  // hlines: a rule above every row and below the last
	vlines  bool  // vlines: a rule between every pair of columns and at both edges
	rowhead int   // rowhead=n: the first n rows repeat at the top of every page of a longtblr
	rowfoot int   // rowfoot=n: the last n rows repeat at the bottom of every page
}

// tblrStyle is what \DefTblrTemplate and \SetTblrTemplate changed. Only what the
// continuation head and foot depend on is kept: off says a template was emptied
// (so the head or foot it draws is not drawn), text holds a replacement body.
type tblrStyle struct {
	off  map[string]bool
	text map[string][]tok
}

// tblrIsCh reports whether t is the plain character r.
func tblrIsCh(t tok, r rune) bool { return !t.cs_ && t.ch == r && t.cat != catSpace }

// tblrTrim drops leading and trailing space tokens.
func tblrTrim(ts []tok) []tok {
	for len(ts) > 0 && !ts[0].cs_ && ts[0].cat == catSpace {
		ts = ts[1:]
	}
	for len(ts) > 0 && !ts[len(ts)-1].cs_ && ts[len(ts)-1].cat == catSpace {
		ts = ts[:len(ts)-1]
	}
	return ts
}

// tblrUnbrace removes one pair of enclosing braces, as tabularray does for a
// key's value.
func tblrUnbrace(ts []tok) []tok {
	ts = tblrTrim(ts)
	if len(ts) >= 2 && !ts[0].cs_ && ts[0].cat == catBegin && !ts[len(ts)-1].cs_ && ts[len(ts)-1].cat == catEnd {
		depth := 0
		for i, t := range ts {
			if t.cs_ {
				continue
			}
			switch t.cat {
			case catBegin:
				depth++
			case catEnd:
				depth--
				if depth == 0 && i < len(ts)-1 {
					return ts // the first brace closes early: two groups, not one
				}
			}
		}
		return tblrTrim(ts[1 : len(ts)-1])
	}
	return ts
}

// tblrSplit splits ts at the top-level occurrences of sep, where top level means
// outside {} and [].
func tblrSplit(ts []tok, sep rune) [][]tok {
	var out [][]tok
	var cur []tok
	brace, brack := 0, 0
	for _, t := range ts {
		if !t.cs_ {
			switch {
			case t.cat == catBegin:
				brace++
			case t.cat == catEnd:
				brace--
			case t.ch == '[' && brace == 0:
				brack++
			case t.ch == ']' && brace == 0:
				brack--
			case t.ch == sep && brace == 0 && brack == 0 && t.cat != catSpace:
				out = append(out, cur)
				cur = nil
				continue
			}
		}
		cur = append(cur, t)
	}
	return append(out, cur)
}

// parseTblrInner reduces the inner specification of a tblr environment. A
// specification with no key=value at its top level is a bare column
// specification, {X[l]X[r]}, which tabularray accepts in place of colspec=.
func parseTblrInner(inner []tok) tblrSpec {
	var sp tblrSpec
	pieces := tblrSplit(inner, ',')
	keyed := false
	for _, p := range pieces {
		if len(tblrSplit(p, '=')) > 1 {
			keyed = true
		}
	}
	if !keyed {
		sp.colspec = tblrTrim(inner)
		return sp
	}
	for _, p := range pieces {
		p = tblrTrim(p)
		kv := tblrSplit(p, '=')
		key := ""
		for _, t := range tblrTrim(kv[0]) {
			if !t.cs_ && t.cat != catSpace {
				key += string(t.ch)
			}
		}
		var val []tok
		if len(kv) > 1 {
			// Everything after the first '=': a value may itself hold '=' inside braces
			// only, which tblrSplit has kept whole, so rejoin any remainder.
			for i, part := range kv[1:] {
				if i > 0 {
					val = append(val, chTok('=', catOther))
				}
				val = append(val, part...)
			}
			val = tblrUnbrace(val)
		}
		switch key {
		case "colspec":
			sp.colspec = val
		case "width":
			sp.width = val
		case "hlines":
			sp.hlines = true
		case "vlines":
			sp.vlines = true
		case "rowhead":
			sp.rowhead, _ = strconv.Atoi(trimSpaceString(tokString(val)))
		case "rowfoot":
			sp.rowfoot, _ = strconv.Atoi(trimSpaceString(tokString(val)))
		}
	}
	return sp
}

// tblrGroupEnd returns the index just past the balanced {..} group starting at
// ts[i], or len(ts) when it does not close.
func tblrGroupEnd(ts []tok, i int) int {
	depth := 0
	for ; i < len(ts); i++ {
		if ts[i].cs_ {
			continue
		}
		switch ts[i].cat {
		case catBegin:
			depth++
		case catEnd:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(ts)
}

// normalizeTblrColspec rewrites a tabularray column specification into the
// array-package one scanColSpec reads. hasWidth says the table has a width= for
// an X column to fill; without one X is an l column.
func normalizeTblrColspec(spec []tok, hasWidth bool) []tok {
	var out []tok
	for i := 0; i < len(spec); {
		t := spec[i]
		if t.cs_ {
			out = append(out, t)
			i++
			continue
		}
		switch {
		case t.cat == catBegin: // @{…}, >{…}, <{…}, !{…} and p's width: kept whole
			j := tblrGroupEnd(spec, i)
			out = append(out, spec[i:j]...)
			i = j
		case t.ch == '*' && t.cat != catSpace: // *{n}{spec}: n copies of spec
			j := i + 1
			for j < len(spec) && !spec[j].cs_ && spec[j].cat == catSpace {
				j++
			}
			if j >= len(spec) || spec[j].cat != catBegin {
				out = append(out, t)
				i++
				break
			}
			k := tblrGroupEnd(spec, j)
			n, _ := strconv.Atoi(trimSpaceString(tokString(spec[j+1 : k-1])))
			l := k
			for l < len(spec) && !spec[l].cs_ && spec[l].cat == catSpace {
				l++
			}
			if l >= len(spec) || spec[l].cat != catBegin {
				i = k
				break
			}
			m := tblrGroupEnd(spec, l)
			body := normalizeTblrColspec(spec[l+1:m-1], hasWidth)
			for ; n > 0 && len(out) < 4096; n-- {
				out = append(out, body...)
			}
			i = m
		case t.cat == catLetter || t.cat == catOther:
			ch := t.ch
			switch ch {
			case 't':
				out = append(out, chTok('p', catLetter))
			case 'Q':
				out = append(out, chTok('l', catLetter))
			case 'S':
				out = append(out, chTok('c', catLetter)) // tabularray.sty:6970: Q[si={..}, c]
			case 'X':
				if hasWidth {
					out = append(out, t)
				} else {
					out = append(out, chTok('l', catLetter))
				}
			default:
				out = append(out, t)
			}
			i++
			// The per-column keys of a column letter: X[1,l], Q[wd=2cm], c[co=1].
			if isTblrColumnLetter(ch) && i < len(spec) && tblrIsCh(spec[i], '[') {
				for i < len(spec) && !tblrIsCh(spec[i], ']') {
					i++
				}
				if i < len(spec) {
					i++
				}
			}
		default:
			out = append(out, t)
			i++
		}
	}
	return out
}

func isTblrColumnLetter(r rune) bool {
	switch r {
	case 'l', 'c', 'r', 'p', 'm', 'b', 't', 'X', 'Q', 'S':
		return true
	}
	return false
}

// tokString renders tokens as the characters they hold, control sequences
// included, for the few places a specification is read as text.
func tokString(ts []tok) string {
	s := ""
	for _, t := range ts {
		if t.cs_ {
			s += "\\" + t.cs
		} else {
			s += string(t.ch)
		}
	}
	return s
}

func trimSpaceString(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// doTblr typesets \begin{env}[outer]{inner} … \end{env} for env in tblr,
// longtblr, talltblr. A table longer than a page is split row by row, as
// longtable's is; a longtblr also repeats its rowhead rows and draws the
// continuation foot on every page but the last (placeLong).
func (e *Engine) doTblr(env string) {
	outer, _ := e.scanOptBracketToks() // caption=, label=; entry= is dropped
	caption, label := parseTblrOuter(outer)
	sp := parseTblrInner(e.readBraceToksRaw())
	width := 0
	if len(sp.width) > 0 {
		e.push(append(append([]tok{chTok('{', catBegin)}, sp.width...), chTok('}', catEnd)))
		width = e.readBraceDimen()
	}
	e.push(append(append([]tok{chTok('{', catBegin)}, normalizeTblrColspec(sp.colspec, width > 0)...), chTok('}', catEnd)))
	aligns, pwidths, vrules := e.scanColSpec()
	if sp.vlines {
		for i := range vrules {
			vrules[i] = true
		}
	}
	items := e.collectTabularBody(env)
	if sp.hlines {
		items = tblrHlines(items)
	}
	if width > 0 {
		e.resolveXWidths(width, aligns, pwidths, vrules, items)
	}
	// A longtblr carries a \caption, and it is a table's (see doLongtable).
	e.beginGroup()
	e.define("@captype", &meaning{kind: mMacro, body: stringToToks("table")}, false)
	// tabularray pads every row by rowsep = 2pt above and below (its default),
	// which the strut scaled by \arraystretch reproduces: (baselineskip + 4pt) / baselineskip.
	if e.baselineskip > 0 {
		f := 1 + float64(4*65536)/float64(e.baselineskip)
		e.define("arraystretch", &meaning{kind: mMacro, body: stringToToks(strconv.FormatFloat(f, 'f', 4, 64))}, false)
	}
	box := e.buildTabularBox(aligns, pwidths, vrules, items)
	e.endGroup()
	if env != "longtblr" {
		e.placeSplit(box)
		return
	}
	// \__tblr_build_table_label_entry: (tabularray.sty:5326-5341) steps the table
	// counter for every longtblr unless label=none, and \labels it when label= is
	// not empty.
	var capToks []tok
	if label != "none" {
		e.push(tokenizeTeX("{table}"))
		e.doRefstepcounter()
		if label != "" {
			e.push(tokenizeTeX("{" + label + "}"))
			e.doLabel()
		}
		if len(caption) > 0 && !e.tblr.off["caption"] {
			capToks = append(stringToToks("Table "), csTok("thetable"))
			capToks = append(append(capToks, stringToToks(": ")...), caption...)
		}
	}
	e.placeLong(box, sp, capToks)
}

// parseTblrOuter reads the outer [caption=…, label=…] of a longtblr. label is the
// label key as text, "none" included.
func parseTblrOuter(outer []tok) (caption []tok, label string) {
	for _, p := range tblrSplit(outer, ',') {
		kv := tblrSplit(tblrTrim(p), '=')
		if len(kv) < 2 {
			continue
		}
		var val []tok
		for i, part := range kv[1:] {
			if i > 0 {
				val = append(val, chTok('=', catOther))
			}
			val = append(val, part...)
		}
		val = tblrUnbrace(val)
		switch strings.TrimSpace(tokString(kv[0])) {
		case "caption":
			caption = val
		case "label":
			label = strings.TrimSpace(tokString(val))
		}
	}
	return caption, label
}

// tblrHlines draws the rules hlines asks for: one above every row and one below
// the last, except where the body already put a rule.
func tblrHlines(items []tabItem) []tabItem {
	out := make([]tabItem, 0, 2*len(items)+1)
	prevRule := false
	for _, it := range items {
		rule := it.hline || it.cline || it.brule || it.bcmid
		if !rule && !it.noalign && !prevRule {
			out = append(out, tabItem{hline: true})
		}
		out = append(out, it)
		prevRule = rule
	}
	if len(out) > 0 && !prevRule {
		out = append(out, tabItem{hline: true})
	}
	return out
}

// tblrGobble reads and drops the arguments of one of tabularray's styling
// commands: an optional [..] then groups mandatory {..}. They configure what the
// engine does not model (templates, per-cell spans and colours), and read raw
// they would reach the page as the text of their arguments.
func (e *Engine) tblrGobble(groups int) {
	e.scanOptBracketToks()
	for i := 0; i < groups; i++ {
		e.readBraceToksRaw()
	}
}

// tblrTemplate reads \DefTblrTemplate{name}{template}{body} (def) or
// \SetTblrTemplate{name}{template} and records what the continuation head and foot
// depend on. A body that is empty, set as the "default" template, or the "empty"
// template chosen for a name, switches that name off (Notation's
// \DefTblrTemplate{capcont}{default}{}); a non-empty default body replaces the
// text of the name (tabularray.sty:4750, 4753, 4761).
func (e *Engine) tblrTemplate(def bool) {
	e.scanOptBracketToks()
	name := strings.TrimSpace(tokString(e.readBraceToksRaw()))
	tmpl := strings.TrimSpace(tokString(e.readBraceToksRaw()))
	var body []tok
	if def {
		body = tblrTrim(e.readBraceToksRaw())
	}
	if e.tblr.off == nil {
		e.tblr.off = map[string]bool{}
		e.tblr.text = map[string][]tok{}
	}
	switch {
	case def && tmpl == "default" && len(body) == 0:
		e.tblr.off[name] = true
	case def && tmpl == "default":
		e.tblr.off[name] = false
		e.tblr.text[name] = body
	case !def && tmpl == "empty":
		e.tblr.off[name] = true
	case !def:
		e.tblr.off[name] = false
	}
}

// tblrRowGroups cuts the rows of a table into units — a row and the rules below
// it, the first unit also taking any rule above — and returns the rowhead first
// units, the rowfoot last ones and the body between.
func tblrRowGroups(list []node, nh, nf int) (head []node, body [][]node, foot []node) {
	var units [][]node
	var cur []node
	hasRow := false
	for _, n := range list {
		if _, ok := n.(*boxNode); ok {
			if hasRow {
				units = append(units, cur)
				cur = nil
			}
			hasRow = true
		}
		cur = append(cur, n)
	}
	if len(cur) > 0 {
		units = append(units, cur)
	}
	nh = max(0, min(nh, len(units)))
	nf = max(0, min(nf, len(units)-nh))
	for _, u := range units[:nh] {
		head = append(head, u...)
	}
	for _, u := range units[len(units)-nf:] {
		foot = append(foot, u...)
	}
	return head, units[nh : len(units)-nf], foot
}

// tblrHeight is the height plus depth of a vertical list of boxes and rules.
func tblrHeight(list []node) int {
	t := 0
	for _, n := range list {
		h, d, _ := vHeightDepth(n)
		t += h + d
	}
	return t
}

// tblrPaginate decides which body units go on which page of a longtblr, as
// \__tblr_get_next_table_rows: and \__tblr_check_table_page_break: do
// (tabularray.sty:5496-5574). h holds the unit heights; fixedFirst and fixedNext
// the height of what every page carries besides its body rows (the caption or
// continued caption, the repeated rowhead and rowfoot); footH the continuation
// foot, drawn on every page but the last; capFirst and capNext what a page has room
// for. A page takes units while the head, the units and the foot fit; the last page
// has no foot, so it takes what is left if that fits without one. partial says the
// first page is already partly filled: when not even one unit fits there, fresh is
// true and the table starts on a page of its own.
func tblrPaginate(h []int, fixedFirst, fixedNext, footH, capFirst, capNext int, partial bool) (spans [][2]int, fresh bool) {
	if len(h) == 0 {
		return [][2]int{{0, 0}}, false
	}
	i := 0
	room, fixed := capFirst, fixedFirst
	for first := true; ; first = false {
		if i >= len(h) {
			return spans, fresh
		}
		total := 0
		for _, x := range h[i:] {
			total += x
		}
		if fixed+total <= room {
			return append(spans, [2]int{i, len(h)}), fresh
		}
		n, used := 0, 0
		for i+n < len(h) && fixed+used+h[i+n]+footH <= room {
			used += h[i+n]
			n++
		}
		if n == 0 && first && partial {
			fresh, room = true, capNext
			continue
		}
		n = max(n, 1)
		spans = append(spans, [2]int{i, i + n})
		i += n
		room, fixed = capNext, fixedNext
	}
}

// tblrPageFill is how much of the current page the vertical list built so far
// fills, \pagetotal of tabularray.sty:5417: the pages already complete are cut off
// the list exactly as paginateSingleList cuts them, and what is left is measured.
func (e *Engine) tblrPageFill() int {
	list := e.mvl
	start := skipDiscardable(list, 0)
	for start < len(list) {
		end := e.findPageBreak(list, start)
		if end >= len(list) {
			break
		}
		next := skipDiscardable(list, end)
		if next <= start {
			next = start + 1
		}
		start = next
	}
	p := pageSoFar{total: e.topskipHeightAt(list, start)}
	maxDepth := e.pageMaxDepth()
	for _, n := range list[start:] {
		p.add(n, maxDepth)
	}
	return p.total + p.depth
}

// tblrFixedGlue is a vertical skip of a fixed size.
func tblrFixedGlue(w int) node { return glueNode{spec: glueSpec{width: w}} }

// tblrLine sets toks as one line of the table's width: centred (a caption) or
// flushed right (the continuation foot).
func (e *Engine) tblrLine(toks []tok, width int, right bool) *boxNode {
	box := e.typesetFieldToHbox(toks)
	fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
	switch {
	case box.width >= width:
		return box
	case right:
		return hpackSP([]node{fil, box}, packTo, width)
	}
	return hpackSP([]node{fil, box, fil}, packTo, width)
}

// placeLong contributes a longtblr page by page. Each page is one box: the caption
// (the first page's, or its "(Continued)" form), the rowhead rows, the rows that
// fit, the rowfoot rows and, on every page but the last, the continuation foot —
// the layout \__tblr_build_page_table: builds (tabularray.sty:5579-5593) — and a
// forced break separates the pages, as \TblrNewPage does. presep and postsep
// (1.5\bigskipamount, tabularray.sty:2568-2569) surround the table, presep only
// when the page is not empty.
func (e *Engine) placeLong(b *boxNode, sp tblrSpec, capToks []tok) {
	if b == nil {
		return
	}
	if k, open := e.curGroupKind(); open && k == boxGroup {
		e.place(b)
		return
	}
	e.endParagraph()
	head, body, rowfoot := tblrRowGroups(b.list, sp.rowhead, sp.rowfoot)
	boxH := func(x *boxNode) int { return x.height + x.depth }
	headsep, footsep, presep, postsep := ptToSP(6), ptToSP(6), ptToSP(18), ptToSP(18)

	capFirst := e.tblrLine(capToks, b.width, false)
	var capNext *boxNode
	if len(capToks) > 0 && !e.tblr.off["capcont"] {
		toks := append([]tok{}, capToks...)
		if !e.tblr.off["conthead"] {
			text := stringToToks("(Continued)")
			if t, ok := e.tblr.text["conthead"]; ok {
				text = t
			}
			toks = append(append(toks, stringToToks(" ")...), text...)
		}
		capNext = e.tblrLine(toks, b.width, false)
	}
	if len(capToks) == 0 {
		capFirst = nil
	}
	var footBox *boxNode
	if !e.tblr.off["contfoot"] && !e.tblr.off["contfoot-text"] {
		text := stringToToks("Continued on next page")
		if t, ok := e.tblr.text["contfoot-text"]; ok {
			text = t
		}
		footBox = e.tblrLine(text, b.width, true)
	}
	capH := func(c *boxNode) int {
		if c == nil {
			return 0
		}
		return boxH(c) + headsep
	}
	rowsH := tblrHeight(head) + tblrHeight(rowfoot)
	footH := 0
	if footBox != nil {
		footH = footsep + boxH(footBox)
	}

	h := make([]int, len(body))
	for i, u := range body {
		h[i] = tblrHeight(u)
	}
	vsize := e.effectiveVsize()
	used := e.tblrPageFill()
	partial := used > 0
	capRoom := vsize
	if partial {
		capRoom = vsize - used - presep
	}
	spans, fresh := tblrPaginate(h, capH(capFirst)+rowsH, capH(capNext)+rowsH, footH, capRoom, vsize, partial)

	var blocks []*boxNode
	for k, s := range spans {
		var l []node
		c := capFirst
		if k > 0 {
			c = capNext
		}
		if c != nil {
			l = append(l, c, tblrFixedGlue(headsep))
		}
		l = append(l, head...)
		for _, u := range body[s[0]:s[1]] {
			l = append(l, u...)
		}
		l = append(l, rowfoot...)
		if k < len(spans)-1 && footBox != nil {
			l = append(l, tblrFixedGlue(footsep), footBox)
		}
		blocks = append(blocks, vpackSP(l, packNatural, 0))
	}

	skip := e.leftskip.stretchOrder > 0 || e.rightskip.stretchOrder > 0
	wrap := func(n node) node {
		if !skip {
			return n
		}
		return hpackSP([]node{glueNode{spec: e.leftskip}, n, glueNode{spec: e.rightskip}}, packTo, e.hsize)
	}
	switch {
	case fresh:
		e.mvl = append(e.mvl, penaltyNode{penalty: -10000})
	case partial:
		e.mvl = append(e.mvl, tblrFixedGlue(presep))
		e.prevDepth = ignoreDepth
	}
	for k, blk := range blocks {
		if k == 0 {
			e.contribute(blk)
			continue
		}
		e.mvl = append(e.mvl, penaltyNode{penalty: -10000}, wrap(blk))
	}
	e.mvl = append(e.mvl, tblrFixedGlue(postsep))
	e.prevDepth = blocks[len(blocks)-1].depth
}

// placeSplit contributes an assembled table to the vertical list row by row, so
// the page builder can break between rows. The table was one box, and a box taller
// than the page is not split: a longtable of 60 rows kept one page of them and
// lost the rest (tectonic: 2 pages). Between rows the real package allows a
// break (longtable.dtx sets \penalty on each \\ and keeps it off before an
// \hline), which is a penalty of 0 after a row or a rule that a row follows.
//
// Each row is a box of the table's own width and each rule is full width, so
// under \centering the skips wrap them one by one exactly as contribute wraps a
// lone box. No interline glue goes between rows: a row carries its own strut, and
// the one vbox it replaces had none either.
func (e *Engine) placeSplit(b *boxNode) {
	if b == nil {
		return
	}
	// Inside a box (a cell, a \parbox) the material is packed whole, never paged.
	if k, open := e.curGroupKind(); len(b.list) == 0 || (open && k == boxGroup) {
		e.place(b)
		return
	}
	e.endParagraph()
	skip := e.leftskip.stretchOrder > 0 || e.rightskip.stretchOrder > 0
	wrap := func(n node) node {
		if !skip {
			return n
		}
		return hpackSP([]node{glueNode{spec: e.leftskip}, n, glueNode{spec: e.rightskip}}, packTo, e.hsize)
	}
	first := true
	prevWasRowOrRule := false
	for _, n := range b.list {
		switch c := n.(type) {
		case *boxNode:
			if c == nil {
				continue
			}
			if prevWasRowOrRule {
				e.mvl = append(e.mvl, penaltyNode{penalty: 0})
			}
			if first {
				e.contribute(c)
			} else {
				e.mvl = append(e.mvl, wrap(c))
			}
			prevWasRowOrRule = true
		case ruleNode:
			if first {
				e.contribute(c)
			} else if skip {
				e.mvl = append(e.mvl, wrap(c))
			} else {
				e.mvl = append(e.mvl, c)
			}
			// A break may follow a rule, not precede it: the rule stays with its row.
			prevWasRowOrRule = false
			e.mvl = append(e.mvl, penaltyNode{penalty: 0})
		default:
			e.mvl = append(e.mvl, n)
		}
		first = false
	}
	e.prevDepth = b.depth
}
