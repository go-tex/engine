// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \parshape, tex.web:23275-23286, which is the whole of its scanner:
//
//	set_shape: begin scan_optional_equals; scan_int; n:=cur_val;
//	  if n<=0 then p:=null
//	  else  begin p:=get_node(2*n+1); info(p):=n;
//	    for j:=1 to n do
//	      begin scan_normal_dimen;
//	      mem[p+2*j-1].sc:=cur_val; {indentation}
//	      scan_normal_dimen;
//	      mem[p+2*j].sc:=cur_val; {width}
//	      end;
//	    end;
//	  define(par_shape_loc,shape_ref,p);
//	  end;
//
// Three rules fall out of those six lines, and each is a way to get it wrong:
//
//   - ⛔ n<=0 TURNS THE SHAPE OFF, AND THE EARLY RETURN IS A PANIC GUARD. TeX
//     needs the test because `get_node(2*n+1)` is nonsense for n<=0; here the
//     equivalent is `make([]Shape, 0, n)`, which PANICS on a negative capacity.
//     So `\parshape=-1` in a document — a plain integer a document may write —
//     crashes the engine without this line. Measured by removing it:
//     `\parshape=0` is unharmed (the pair loop is bounded by n and runs zero
//     times either way) and `\parshape=-1` panics.
//     ⚠ An earlier draft of this comment claimed the guard stops the scanner
//     eating the document's next two dimensions. It does not: the loop cannot
//     over-read, because its bound IS n. The mutation said so.
//   - ⛔ EACH PAIR IS INDENT THEN WIDTH, in that order.
//   - ⛔ AND THE LAST PAIR APPLIES TO EVERY LATER LINE (:16680-16681), which is
//     go-typeset/linebreak's ParShape rather than this file's problem — but it
//     is why a shape of two pairs is a complete answer for a paragraph of nine
//     lines.
//
// ⭐ AND \parshape WINS OVER \hangindent, which is not obvious and is settled at
// :16706: `if par_shape_ptr=null then (hang_indent branch) else (parshape
// branch)`. A document setting both gets the shape, and \hangindent is not
// consulted at all.
func (e *Engine) doParShape() {
	e.scanEquals() // tex.web's scan_optional_equals: `\parshape=3` and `\parshape 3`
	n := e.scanInt()
	if n <= 0 {
		// Off, and nothing further is scanned.
		e.parShape = nil
		return
	}
	shape := make([]Shape, 0, n)
	for j := 0; j < n; j++ {
		indent := spToPt(e.scanDimen())
		width := spToPt(e.scanDimen())
		shape = append(shape, Shape{Indent: indent, Width: width})
	}
	e.parShape = shape
}
