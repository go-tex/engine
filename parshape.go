// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \parshape, and the \linewidth register LaTeX's list machinery drives it from.
//
// ⛔ A LIST'S MARGIN IS NOT A \leftskip. latex.ltx:11473, inside \@trivlist, sets
//
//	\leftskip \z@skip
//
// and the indentation comes from the last line of \list (latex.ltx:11453):
//
//	\advance\linewidth -\rightmargin
//	\advance\linewidth -\leftmargin
//	\advance\@totalleftmargin \leftmargin
//	\parshape \@ne \@totalleftmargin \linewidth
//
// So a quote is indented by a PARAGRAPH SHAPE, not by glue on each line. Reaching the
// same geometry through \leftskip would put a value in a register LaTeX keeps at zero
// there, and a document that reads \the\leftskip inside a list would see it.
//
// tex.web §16676-16689 gives the semantics:
//
//	If |par_shape_ptr| is not null, it points to a $(2n+1)$-word record in |mem|,
//	where the |info| in the first word contains the value of |n|, and the other $2n$
//	words contain the left margins and line lengths for the first |n| lines of the
//	paragraph; the specifications for line |n| apply to all subsequent lines.
//
// ⚠ Only n = 1 is applied here, and that is enough for every list: \list emits
// \parshape \@ne and nothing else. The breaker takes ONE measure (breakSegment in
// paragraph.go), so honouring a per-line shape would mean changing its signature and
// its interior; a shape with n > 1 is therefore RECORDED as unimplemented rather than
// approximated by one of its pairs, because approximating it would put lines 1…n-1 at
// a width the document did not ask for while reporting nothing.

// recordUnimplemented notes a construction the engine read but does not honour, so
// -report-skipped can name it. The map is created on demand.
//
// ⛔ The lazy creation is the point: skippedCS is nil on an Engine built by New()
// alone, and writing to a nil map PANICS. A \parshape with two lines brought the
// engine down that way — found by a test, which is the only reason it is not a crash
// in somebody's document. Every other caller happens to run after the map exists,
// which is exactly why this was invisible.
func (e *Engine) recordUnimplemented(what string) {
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	e.skippedCS[what]++
}

// parshapePair is one line's left margin and length, in scaled points.
type parshapePair struct {
	indent int
	length int
}

// doParshape implements \parshape = <n> <2n dimens>.
//
// tex.web's scanner reads the count first and then 2n dimens; n <= 0 clears the shape
// (tex.web:8512-8513 reads it back as 0). The dimens are read even when n is 0 is not
// the case — TeX reads exactly 2n of them — so a malformed specification consumes what
// it says it will and nothing more.
func (e *Engine) doParshape() {
	e.scanEquals()
	n := e.scanInt()
	if n <= 0 {
		e.setParshape(nil, false)
		return
	}
	pairs := make([]parshapePair, 0, n)
	for i := 0; i < n; i++ {
		indent := e.scanDimen()
		length := e.scanDimen()
		pairs = append(pairs, parshapePair{indent: indent, length: length})
	}
	if n > 1 {
		// Recorded, not approximated. -report-skipped surfaces it the way every other
		// known gap in this engine is surfaced.
		e.recordUnimplemented(`\parshape with more than one line`)
	}
	e.setParshape(pairs, false)
}

// setParshape assigns the shape, scoped like the eqtb entry it is: \parshape lives at
// par_shape_loc in TeX's equivalents table (tex.web:4754), so a shape set inside a
// group is gone at its end — which is exactly what makes \begin{quote}…\end{quote}
// restore the enclosing measure without anyone restoring it by hand.
func (e *Engine) setParshape(pairs []parshapePair, global bool) {
	if global {
		e.forgetSaved(saveParshape, 0, "")
	} else if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: saveParshape, oldshape: e.parshape})
	}
	e.parshape = pairs
}

// parshapeMeasure reports the left margin and line length a paragraph must be set at,
// and whether a shape is in force at all.
//
// ⚠ The caller must fall back to \hsize less the two skips when this returns false:
// that is the normal setting (tex.web §16688, "par_shape_ptr=null, hang_after=1,
// hang_indent=0").
func (e *Engine) parshapeMeasure() (indent, length int, ok bool) {
	if len(e.parshape) == 0 {
		return 0, 0, false
	}
	p := e.parshape[0]
	if p.length <= 0 {
		return 0, 0, false // a zero-length shape would set every line empty
	}
	return p.indent, p.length, true
}

// setLineWidth assigns \linewidth.
//
// latex.ltx:11414 declares it `\newdimen\linewidth`: its own register, which \list
// decrements by both margins before handing it to \parshape. It was \let to \hsize
// here, and that is the same conflation \columnwidth had — except worse, because
// \list's `\advance\linewidth -\leftmargin` would then shrink \hsize ITSELF, and the
// enclosing text would come out narrowed by every list it contained.
//
// ⚠ Zero means "nobody has stated it yet", and then it reads as the current measure —
// which is what the \let gave, so a document that never reaches \begin{document}'s
// recompute behaves as before rather than seeing a line width of 0pt.
func (e *Engine) setLineWidth(v int, global bool) {
	e.setEngineDimen(saveLineWidth, &e.lineWidthReg, v, global)
}

// lineWidthValue is \linewidth.
func (e *Engine) lineWidthValue() int {
	if e.lineWidthReg > 0 {
		return e.lineWidthReg
	}
	return e.hsize
}
