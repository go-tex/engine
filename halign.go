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
// An implicit brace (\bgroup) is a left brace here, as §403 tests the command
// code and not the character. It was held back while the alignment it opens could
// not be built — the entries were collected and expanded before they ran, so
// IEEEtran's author block lost its names — and is accepted now that they run live
// (go-tex/engine#602). Anything else TALLIES the refusal: an alignment dropped
// whole is exactly the kind of silent loss every other field of Diagnostics exists
// to make visible.
func (e *Engine) scanLeftBrace(prim string) bool {
	for {
		t, ok := e.getXToken()
		if !ok {
			e.noteAlignmentAbandoned(prim)
			return false
		}
		orig := t
		if c, isChar := e.implicitChar(t); isChar {
			t = c // \bgroup is a left brace (§403 tests the command code, not the character)
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
		e.back(orig)
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
		kind := e.alignPeek()
		if kind == peekEnd {
			break
		}
		if kind == peekNoalign {
			items = append(items, alignItem{vmat: e.noalignBody()})
			continue
		}
		row, ended := e.liveRow(templates)
		if row != nil {
			items = append(items, alignItem{row: row})
		}
		if ended {
			break
		}
	}
	e.contribute(e.assembleAlignment(templates, items))
}

type peekKind int

const (
	peekRow peekKind = iota
	peekNoalign
	peekEnd
)

// alignPeek is tex.web §785 (align_peek): at the start of a row it expands up to
// the first token that is not blank, not \relax and not a macro call, skips a
// \crcr, and then decides — \noalign opens vertical material, a right brace ends
// the alignment, anything else is the first token of a row, left on the input for
// the entry to read. The row is NOT collected here or anywhere: its entries are
// executed as they are read (liveRow).
func (e *Engine) alignPeek() peekKind {
	for {
		t, ok := e.getXToken()
		if !ok {
			return peekEnd
		}
		orig := t
		if c, isChar := e.implicitChar(t); isChar {
			t = c // \egroup is a right brace, as \bgroup is a left one
		}
		switch {
		case !t.cs_ && t.cat == catSpace:
			continue
		case t.cs_ && (t.cs == "relax" || t.cs == "crcr"):
			continue
		case t.cs_ && t.cs == "noalign":
			return peekNoalign
		case !t.cs_ && t.cat == catEnd:
			return peekEnd
		}
		e.back(orig)
		return peekRow
	}
}

// alignItem is one thing in an alignment's vertical list: either a row of cells
// or the material a \noalign contributed between two rows. Exactly one field is
// set.
type alignItem struct {
	row  [][]node
	vmat []node
}

// noalignBody reads the group of a \noalign alignPeek has just met, returning the
// vertical list it builds.
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
func (e *Engine) noalignBody() []node {
	e.skipOptSpace()
	b, ok := e.getXToken()
	if !ok {
		return nil
	}
	if c, isChar := e.implicitChar(b); isChar {
		b = c // \bgroup opens the group just as { does
	}
	if b.cs_ || b.cat != catBegin {
		e.back(b) // \noalign without a group: nothing to contribute
		return nil
	}
	// TeX makes the material a group of its own (tex.web §785 opens one), so a
	// font or glue change inside a \noalign stops at it.
	e.beginGroupKind(boxGroup)
	list := e.buildVBoxList()
	e.endGroup()
	return list
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
	// ⛔ The template is copied UNEXPANDED. tex.web §782 (get_preamble_token) reads
	// it with get_token and expands once only the token after \span; \tabskip is
	// performed as the assignment it is. Reading it with
	// getXToken expanded every macro in the u and v parts ONCE, at preamble time,
	// so a \gdef or \let in a cell met a frozen copy of the macro it names
	// (go-tex/engine#602: `\halign{#\a\cr \gdef\a{NEW}x\cr}` typeset OLD).
	//
	// depth is align_state's brace part: §783-784 end a u or v part only on an & or
	// \cr met with align_state=-1000000, i.e. with the braces BALANCED, so one inside braces belongs to the
	// template.
	depth := 0
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		if t.noexp {
			t.noexp = false
		}
		for t.cs_ && t.cs == "span" && (e.meaningOf(t) == nil || e.meaningOf(t).kind == mPrim) {
			// §782: the token after \span "will be expanded once" — one step of
			// expansion, then the next token is taken as it comes.
			// At end of input x is the zero token, which the template drops with the
			// unfinished column the next getNext of this loop ends.
			x, _ := e.getNext()
			if m := e.meaningOf(x); x.cs_ && m != nil && m.kind == mMacro {
				e.expandMacro(m)
				x, _ = e.getNext()
			} else if x.cs_ && m != nil && m.kind != mPrim {
				e.back(x)
				x, _ = e.getXToken()
			}
			t = x
		}
		if t.cs_ && t.cs == "tabskip" {
			if m := e.meaningOf(t); m != nil && m.kind == mSkipRef {
				e.execCS(t) // §782: \tabskip=<glue> is executed, not copied
				continue
			}
		}
		// §783: spaces are eliminated from the beginning of a u template.
		if !seenHash && !t.cs_ && t.cat == catSpace && len(cur.before) == 0 {
			continue
		}
		orig := t // the template keeps the token as written
		if c, isChar := e.implicitChar(t); isChar {
			t = c
		}
		inside := depth > 0
		if !t.cs_ {
			switch t.cat {
			case catBegin:
				depth++
			case catEnd:
				if depth > 0 {
					depth--
				}
			}
		}
		if inside || (!t.cs_ && t.cat == catBegin) {
			if seenHash {
				cur.after = append(cur.after, orig)
			} else {
				cur.before = append(cur.before, orig)
			}
			continue
		}
		t = orig
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

// liveRow runs one row's entries (separated by &, ended by \cr), each executed as
// it is read — nothing is collected first, so an assignment in one entry is in
// force for the macros that follow it (tex.web §785-791). ended is true when the
// alignment's closing brace or end of input was reached.
func (e *Engine) liveRow(tpl alignTemplates) ([][]node, bool) {
	var cells [][]node
	for col := 0; ; col++ {
		list, term, ok := e.liveCell(tpl.at(col))
		cells = append(cells, list)
		switch {
		case !ok:
			// No & or \cr ended the entry: the alignment's own closing brace did
			// (tex.web §1132 would say "Missing \cr inserted"), or the input ran out.
			return cells, true
		case !term.cs_: // &
			continue
		default: // \cr, \crcr
			return cells, false
		}
	}
}

// endTemplateCS and uEndCS are private control sequences that never reach a
// document: the first closes the v part of a template (tex.web's \endtemplate,
// §780), the second marks where the u part ends.
const (
	endTemplateCS = "\x00endtemplate"
	uEndCS        = "\x00uend"
)

// alignCtx is the state of the alignment entry being read, align_state of
// tex.web §324 and §342. It counts braces where the TOKENS ARE READ, not where
// groups are executed, because that is what lets an entry end from inside a
// group: IEEEtran closes its author cell with
//
//	{\ifnum0=`}\fi … \ifnum0=`{\fi}\cr          (IEEEtran.cls:3377-3383)
//
// where the backquoted } is read and counted although no group closes, so the
// \cr that follows meets state 0 while the cell's { is still open.
type alignCtx struct {
	state int // align_state: 1000000 while the u part is read, then 0 at entry level
	after []tok
	term  tok  // the & or \cr that ended the entry
	done  bool // the v part was inserted and its \endtemplate reached
	outer *alignCtx
}

// unread undoes the count of a token that is put back on the input.
func (c *alignCtx) unread(t tok) {
	if t.cs_ {
		return
	}
	switch t.cat {
	case catBegin:
		c.state--
	case catEnd:
		c.state++
	}
}

// getNextAligned is getNext while an alignment entry is being read: it keeps
// align_state (§324) and, when an & or \cr arrives at state 0, inserts the v part
// of the template in its place (§342, §789) and remembers the token that ended
// the entry.
func (e *Engine) getNextAligned() (tok, bool) {
	for {
		t, ok := e.getNextRaw()
		if !ok {
			return t, false
		}
		c := e.align // never nil here: getNext routes to this only while an entry is read
		if t.cs_ {
			switch t.cs {
			case uEndCS:
				c.state = 0 // §324: the end of the u part resets align_state
				continue
			case "cr", "crcr":
				if c.state == 0 && !c.done {
					if m := e.meaningOf(t); m == nil || m.kind == mPrim {
						e.insertV(c, t)
						continue
					}
				}
			}
			return t, true
		}
		switch t.cat {
		case catBegin:
			c.state++
		case catEnd:
			c.state--
		case catAlign:
			if c.state == 0 && !c.done {
				e.insertV(c, t)
				continue
			}
		}
		return t, true
	}
}

// insertV is §789: the token that ended the entry is kept aside and the v part
// of the template, closed by \endtemplate, is read in its place.
func (e *Engine) insertV(c *alignCtx, t tok) {
	c.term = t
	c.done = true
	v := make([]tok, 0, len(c.after)+1)
	v = append(v, c.after...)
	v = append(v, tok{cs_: true, cs: endTemplateCS})
	e.push(v)
	c.state = 1000000
}

// liveCell executes one alignment entry on the live input, tex.web §788-791: the u
// part is pushed ahead of the input, the ordinary box loop reads the entry, and
// the scanner ends it (getNextAligned) when an & or \cr arrives with align_state
// 0. ok is false when nothing ended the entry but the alignment's closing brace.
func (e *Engine) liveCell(t colTemplate) (list []node, term tok, ok bool) {
	c := &alignCtx{state: 1000000, after: t.after, outer: e.align}
	e.align = c
	e.beginGroupKind(boxGroup)
	open := len(e.groups)
	u := make([]tok, 0, len(t.before)+1)
	u = append(u, t.before...)
	u = append(u, tok{cs_: true, cs: uEndCS})
	e.push(u)
	list = e.buildBoxList()
	// A group the entry left open ends with it; TeX reports it, the tolerant
	// engine closes it so the nesting below stays right.
	for len(e.groups) > open {
		e.endGroup()
	}
	e.endGroup()
	e.align = c.outer
	return list, c.term, c.done
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
