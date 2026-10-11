// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements \halign, TeX's horizontal alignment (tables). The
// preamble gives each column a template split at # into "before" and "after"
// token lists; rows are entries separated by & and ended by \cr. Each column's
// width is the maximum of its cells' natural widths, every cell is then repacked
// to that width, and the rows are stacked into a vbox. Simplifications for now:
// no \tabskip glue, no \omit/\span, and a cell may not contain a bare
// top-level {…} group (use \hbox{…} for grouped content) — matching the box
// builder's brace handling.

// colTemplate is one preamble column: text before and after the entry (#).
type colTemplate struct{ before, after []tok }

// scanLeftBrace reads the MANDATORY left brace that opens a primitive's group,
// after the spaces and \relax tokens tex.web §403's scan_left_brace reads past.
//
// ⛔ \halign returned SILENTLY when no brace followed, so
//
//	\halign\bgroup&\relax\hfil …##\relax\hfil …\cr      (IEEEtran.cls:4701)
//
// abandoned the whole alignment and let its body — the PREAMBLE included — be
// typeset as ordinary text. A real paper's title page then carried a stray "#"
// in front of its author list, and the two commands the preamble leaks,
// \noalign and \everycr, were reported as undefined on a path where they are
// nothing of the kind (go-tex/engine#602). \hbox and \vbox accept the implicit
// brace; this is the one caller that does not, measured on probes of all four.
//
// ⛔⛔ The implicit brace is DELIBERATELY NOT ACCEPTED YET, and that is a measured
// decision rather than an oversight. Accepting it enters IEEEtran's author
// alignment, which this engine cannot yet build: the page then reads "0.511mm"
// where the reference reads "A Name", because the template's v-part carries a
// rule spec we mis-scan, and \everycr is still undefined. Trading a cosmetic "#"
// for the author names themselves is a worse page, and content loss outranks a
// stray character. What is missing is listed in #602; the brace is the LAST of
// those to flip, not the first.
//
// So this reads §403's spaces and \relax, still requires a LITERAL {, and
// TALLIES the refusal — an alignment dropped whole is exactly the kind of silent
// loss every other field of Diagnostics exists to make visible, and nothing
// reported it for as long as it has been happening.
func (e *Engine) scanLeftBrace(prim string) bool {
	for {
		t, ok := e.getXToken()
		if !ok {
			e.noteAlignmentAbandoned(prim)
			return false
		}
		if !t.cs_ {
			switch t.cat {
			case catBegin:
				return true
			case catSpace:
				continue
			}
		} else if t.cs == "relax" {
			continue // §403 reads past \relax, as it does past a space
		}
		e.back(t)
		e.noteAlignmentAbandoned(prim)
		return false
	}
}

// noteAlignmentAbandoned records an alignment dropped for want of its brace.
func (e *Engine) noteAlignmentAbandoned(prim string) {
	if e.alignAbandoned == nil {
		e.alignAbandoned = map[string]int{}
	}
	e.alignAbandoned[`\`+prim]++
}

// doHalign parses and lays out an \halign{…}. The result vbox is contributed to
// the current vertical list.
func (e *Engine) doHalign() {
	e.endParagraph()
	if !e.scanLeftBrace("halign") {
		return
	}
	cols, loopFrom := e.parsePreamble()
	templates := alignTemplates{cols: cols, loopFrom: loopFrom}
	var items []alignItem
	for {
		for {
			vmat, ok := e.scanNoalign()
			if !ok {
				break
			}
			items = append(items, alignItem{vmat: vmat})
		}
		row, ended := e.parseRow(templates)
		if row != nil {
			items = append(items, alignItem{row: row})
		}
		if ended {
			break
		}
	}
	e.contribute(e.assembleAlignment(templates, items))
}

// alignItem is one thing in an alignment's vertical list: either a row of cells
// or the material a \noalign contributed between two rows. Exactly one field is
// set.
type alignItem struct {
	row  [][]node
	vmat []node
}

// scanNoalign consumes a \noalign{…} if one is next, returning the vertical list
// its group builds.
//
// tex.web §785: \noalign may appear only where a row could begin — at the very
// start of an alignment or just after a \cr — and its argument is VERTICAL mode
// material that goes into the enclosing vertical list, between the rows. TikZ
// stacks the lines of a node with align= exactly this way, one \halign whose
// rows are the lines and whose \noalign{\vskip …} is the leading between them
// (tikz.code.tex, \tikz@align@end@check), so a node reading
//
//	\node [align=center] {one \\ two};
//
// used to leave \noalign undefined. That is not merely a lost skip: the group
// after it then reached parseRow as a bare top-level {…}, which a cell may not
// hold, and the rest of the document went with it.
func (e *Engine) scanNoalign() ([]node, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return nil, false
	}
	if !t.cs_ || t.cs != "noalign" {
		e.back(t)
		return nil, false
	}
	e.skipOptSpace()
	b, ok := e.getXToken()
	if !ok {
		return nil, true
	}
	if c, isChar := e.implicitChar(b); isChar {
		b = c // \bgroup opens the group just as { does
	}
	if b.cs_ || b.cat != catBegin {
		e.back(b) // \noalign without a group: nothing to contribute
		return nil, true
	}
	// TeX makes the material a group of its own (tex.web §785 opens one), so a
	// font or glue change inside a \noalign stops at it.
	e.beginGroupKind(boxGroup)
	list := e.buildVBoxList()
	e.endGroup()
	return list, true
}

// parsePreamble reads the template row (up to \cr), returning one entry per
// column split at the # placeholder.
func (e *Engine) parsePreamble() ([]colTemplate, int) {
	var cols []colTemplate
	var cur colTemplate
	seenHash := false
	loopFrom := -1
	flush := func() {
		cols = append(cols, cur)
		cur = colTemplate{}
		seenHash = false
	}
	for {
		t, ok := e.getXToken()
		if !ok {
			break
		}
		switch {
		case t.cs_ && (t.cs == "cr" || t.cs == "crcr"):
			flush()
			return cols, loopFrom
		case !t.cs_ && t.cat == catAlign:
			// tex.web §786, in the u_j scanner: a & met while the template is still
			// EMPTY and no loop has been set does NOT separate a column — it marks
			// the template that FOLLOWS as the one to repeat ("cur_loop"), which is
			// how \halign{&T\cr …} gives every column the same template.
			//
			// ⛔ Read as a separator it produced an EMPTY column 0, and every cell
			// then landed one column to the right of its template. IEEEtran's author
			// block is written exactly this way (IEEEtran.cls:4701).
			if loopFrom < 0 && !seenHash && len(cur.before) == 0 && len(cur.after) == 0 {
				loopFrom = len(cols)
				continue
			}
			flush()
		case !t.cs_ && t.cat == catParam:
			seenHash = true
		case seenHash:
			cur.after = append(cur.after, t)
		default:
			cur.before = append(cur.before, t)
		}
	}
	return cols, loopFrom
}

// alignTemplates is a preamble that may repeat: a periodic one (tex.web §786)
// lengthens itself from loopFrom whenever a row asks for a column it does not
// have (§793's "Lengthen the preamble periodically").
type alignTemplates struct {
	cols     []colTemplate
	loopFrom int // -1 when the preamble is not periodic
}

// at returns the template for column j, extending a periodic preamble as TeX
// does. A row that overruns a NON-periodic preamble gets an empty template,
// which is where TeX says "Extra alignment tab has been changed to \cr".
func (t alignTemplates) at(j int) colTemplate {
	if j < len(t.cols) {
		return t.cols[j]
	}
	if t.loopFrom < 0 || t.loopFrom >= len(t.cols) {
		return colTemplate{}
	}
	period := len(t.cols) - t.loopFrom
	return t.cols[t.loopFrom+(j-t.loopFrom)%period]
}

// parseRow reads one row's cells (separated by &, ended by \cr). ended is true
// when the alignment's closing brace or end of input was reached.
func (e *Engine) parseRow(tpl alignTemplates) ([][]node, bool) {
	var cells [][]node
	var content []tok
	col, depth := 0, 0
	build := func() {
		t := tpl.at(col)
		toks := append([]tok{}, t.before...)
		toks = append(toks, content...)
		toks = append(toks, t.after...)
		cells = append(cells, e.buildCellHList(toks))
		content = nil
		col++
	}
	for {
		t, ok := e.getXToken()
		if !ok {
			if len(content) > 0 || col > 0 {
				build()
			}
			return cells, true
		}
		// An implicit brace is a brace here, as it is in the stomach and as
		// align_state counts it in tex.web. Without this the \egroup that closes an
		// alignment opened with \halign\bgroup was read as ORDINARY CELL MATERIAL,
		// so the row never ended and the alignment ran to end of input.
		if c, isChar := e.implicitChar(t); isChar {
			t = c
		}
		switch {
		case depth == 0 && t.cs_ && (t.cs == "cr" || t.cs == "crcr"):
			build()
			return cells, false
		case depth == 0 && !t.cs_ && t.cat == catEnd: // alignment's closing }
			if len(content) > 0 || col > 0 {
				build()
			}
			return cells, true
		case depth == 0 && !t.cs_ && t.cat == catAlign:
			build()
		default:
			if !t.cs_ && t.cat == catBegin {
				depth++
			} else if !t.cs_ && t.cat == catEnd {
				depth--
			}
			content = append(content, t)
		}
	}
}

// buildCellHList builds a cell's horizontal list from its full token sequence
// (template before + entry + after) in isolation.
func (e *Engine) buildCellHList(toks []tok) []node {
	saved := e.noBase
	e.noBase = true
	seq := make([]tok, 0, len(toks)+1)
	seq = append(seq, toks...)
	seq = append(seq, chTok('}', catEnd)) // sentinel to terminate buildBoxList
	e.push(seq)
	// The sentinel closes a real group, as the cell's own braces would: TeX makes
	// every alignment entry a group (tex.web §791 — the u-part and v-part of a
	// template are inserted inside braces), so a font or colour switch in a cell
	// stops at the cell. Without the group the sentinel was a STRAY brace, which
	// the stomach reports the moment any \begingroup is open — and \begin{env} now
	// opens one.
	e.beginGroupKind(boxGroup)
	list := e.buildBoxList()
	e.endGroup()
	e.noBase = saved
	return list
}

// buildCellVList builds a collected token list as VERTICAL material, the way
// scanNoalign builds a \noalign read straight from the input. tabular collects its
// whole body as tokens before building any of it, so its \noalign cannot be read
// from the stream and is replayed here instead.
//
// The group is real, as tex.web §785's is: a \vskip, a font or a colour set inside
// the \noalign stops at it.
func (e *Engine) buildCellVList(toks []tok) []node {
	seq := make([]tok, 0, len(toks)+1)
	seq = append(seq, toks...)
	seq = append(seq, chTok('}', catEnd)) // sentinel to terminate buildVBoxList
	e.push(seq)
	e.beginGroupKind(boxGroup)
	list := e.buildVBoxList()
	e.endGroup()
	return list
}

// assembleAlignment computes column widths and stacks the repacked rows into a
// vbox.
func (e *Engine) assembleAlignment(tpl alignTemplates, items []alignItem) *boxNode {
	// A periodic preamble lengthens itself, so the column count is the widest ROW
	// and not the preamble's length. Taking len(cols) dropped every column past
	// the first on \halign{&T\cr a&b&c\cr}.
	ncol := len(tpl.cols)
	for _, it := range items {
		if n := len(it.row); n > ncol {
			ncol = n
		}
	}
	colw := make([]int, ncol)
	for _, it := range items {
		for j, cell := range it.row {
			if j < ncol {
				if w := hpackSP(cell, packNatural, 0).width; w > colw[j] {
					colw[j] = w
				}
			}
		}
	}
	var vlist []node
	// The rows are a vertical list, so they take interline glue like any other
	// (tex.web §679, interlineGlue in paragraph.go). Stacked at their natural
	// height they grew 6.63pt per row against the reference's 12.00pt — the
	// height of a line of text rather than \baselineskip.
	//
	// No paper in the 200-document arXiv reference corpus exercises this: all 282
	// of its \halign occurrences sit in .sty/.cls files, which this engine
	// substitutes rather than executes, and none in a document body. The fix rests
	// on the direct witness above, not on a corpus measurement, which is 0 pages,
	// 0 PDFs and 0 ink changed.
	prevDepth := ignoreDepth
	for _, it := range items {
		if it.row == nil {
			// \noalign material joins the vertical list as it stands. Glue and
			// penalties do not change prev_depth, and a box inside it already took
			// its own interline glue while buildVBoxList ran, so the running depth
			// is left alone here.
			vlist = append(vlist, it.vmat...)
			continue
		}
		var rowNodes []node
		for j, cell := range it.row {
			width := 0
			if j < ncol {
				width = colw[j]
			}
			rowNodes = append(rowNodes, hpackSP(cell, packTo, width))
		}
		b := hpackSP(rowNodes, packNatural, 0)
		if g, ok := e.interlineGlue(prevDepth, b.height); ok {
			vlist = append(vlist, g)
		}
		vlist = append(vlist, b)
		prevDepth = b.depth
	}
	return vpackSP(vlist, packNatural, 0)
}
