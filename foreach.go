// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strconv"

// This file implements \foreach — pgffor's loop — for the one shape documents use
// outside a picture.
//
// Inside a tikzpicture the question does not arise: the emulated build gobbles the
// whole environment body (doGobbleEnv in primitives.go), so a \foreach there never
// ran either way. What is left is the document-level loop, and the corpus says
// what that costs. arXiv 2312.05895 ends with
//
//	\foreach \x in {1,...,12}{%
//	  \clearpage
//	  \includepdf[pages={\x,{}}]{SI.pdf}
//	}
//
// and lost its entire twelve-page supplement: fifteen pages where its reference
// has twenty-seven, the paper rendering at 56% of its length — for TWO undefined
// control sequences, \foreach and \x. Nothing else was missing: \includepdf and
// pdfpages are already here, and the loop is all that stood between them and the
// pages.
//
// Scope, settled by counting rather than by guessing: all 55 \foreach occurrences
// in the 154-paper corpus are the single-variable form `\foreach \v in`. The
// multi-variable form (\foreach \x/\y in …) and the optional keys
// (\foreach [count=\i] …) are deliberately NOT implemented. A \foreach that is not
// the supported shape is handed back to skipUndefined untouched, exactly as before
// this file existed, so the census keeps reporting it rather than quietly doing
// half of it.

// doForeach runs \foreach \v in {<list>}{<body>}.
//
// The body is run once per value with \v defined to it, inside a group, which is
// what \@forloop does for \@for (kernelhelpers.go) and what pgffor does here.
func (e *Engine) doForeach() {
	m := e.markInput()

	v, ok := e.scanForeachVar()
	if !ok {
		e.restoreInput(m)
		e.skipUndefined("foreach")
		return
	}
	items, ok := e.scanForeachList()
	if !ok {
		e.restoreInput(m)
		e.skipUndefined("foreach")
		return
	}
	body := e.readBraceToksRaw()
	if body == nil {
		// pgffor also accepts a body ending at a semicolon, which is the tikz path
		// form and only ever appears inside a picture.
		e.restoreInput(m)
		e.skipUndefined("foreach")
		return
	}

	var out []tok
	for _, val := range items {
		out = append(out, chTok('{', catBegin), csTok("def"), v, chTok('{', catBegin))
		out = append(out, val...)
		out = append(out, chTok('}', catEnd))
		out = append(out, body...)
		out = append(out, chTok('}', catEnd))
	}
	e.push(out)
}

// scanForeachVar reads the loop variable and the `in` that follows it.
//
// The variable is read WITHOUT expansion: it is the name being defined, and a
// document that has already used \x for something else would otherwise have its
// own macro expanded here instead of rebound. pgffor reads it the same way, as an
// undelimited parameter.
func (e *Engine) scanForeachVar() (tok, bool) {
	e.skipOptSpace()
	v, ok := e.getNext()
	if !ok || !v.cs_ {
		return tok{}, false
	}
	e.skipOptSpace()
	for _, want := range []rune{'i', 'n'} {
		t, ok := e.getNext()
		if !ok || t.cs_ || t.ch != want {
			return tok{}, false
		}
	}
	return v, true
}

// scanForeachList reads {<list>} and returns one token list per value, with the
// `a,...,b` ranges filled in.
func (e *Engine) scanForeachList() ([][]tok, bool) {
	toks := e.readBraceToksRaw()
	if toks == nil {
		return nil, false
	}
	return expandForeachRanges(splitTopLevelCommas(toks))
}

// splitTopLevelCommas cuts a token list at the commas that are not inside a group,
// trimming the spaces around each piece. An empty list yields no items.
func splitTopLevelCommas(toks []tok) [][]tok {
	var items [][]tok
	var cur []tok
	depth := 0
	for _, t := range toks {
		switch {
		case !t.cs_ && t.cat == catBegin:
			depth++
		case !t.cs_ && t.cat == catEnd:
			depth--
		case depth == 0 && !t.cs_ && t.ch == ',' && t.cat == catOther:
			items = append(items, trimSpaceToks(cur))
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	if last := trimSpaceToks(cur); len(last) > 0 {
		items = append(items, last)
	}
	return items
}

// plainText is an item's characters, or "" when it holds anything that is not one
// (a control sequence, a group) — so a value that is not a plain number can never
// be mistaken for one.
func plainText(toks []tok) string {
	var b []rune
	for _, t := range toks {
		if t.cs_ || t.cat == catBegin || t.cat == catEnd {
			return ""
		}
		b = append(b, t.ch)
	}
	return string(b)
}

// numTok renders an integer as the character tokens a value is made of.
func numTok(n int) []tok {
	var out []tok
	for _, r := range strconv.Itoa(n) {
		c := catOther
		if r == '-' {
			c = catOther
		}
		out = append(out, chTok(r, c))
	}
	return out
}

// expandForeachRanges replaces an item that is exactly `...` by the values it
// stands for.
//
// pgffor reads the step from what came before: {1,...,12} steps by one, and
// {1,3,...,11} steps by two, because the two values before the dots set it. The
// bound itself is left to the item that follows, which is emitted normally.
// A range whose ends are not plain integers, or whose step cannot reach its bound,
// is not one this understands, and the whole \foreach is handed back.
func expandForeachRanges(items [][]tok) ([][]tok, bool) {
	var out [][]tok
	for i, it := range items {
		if plainText(it) != "..." {
			out = append(out, it)
			continue
		}
		if i+1 >= len(items) || len(out) == 0 {
			return nil, false
		}
		from, err1 := strconv.Atoi(plainText(out[len(out)-1]))
		to, err2 := strconv.Atoi(plainText(items[i+1]))
		if err1 != nil || err2 != nil {
			return nil, false
		}
		step := 1
		if len(out) >= 2 {
			if prev, err := strconv.Atoi(plainText(out[len(out)-2])); err == nil {
				step = from - prev
			}
		}
		if step == 0 || (to-from)*step < 0 {
			return nil, false
		}
		// A range is bounded by its own ends, so it cannot run away; the guard is
		// on the arithmetic above, not on a count.
		for n := from + step; (step > 0 && n < to) || (step < 0 && n > to); n += step {
			out = append(out, numTok(n))
		}
	}
	return out, true
}
