// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// calc.sty makes a LENGTH ARGUMENT an arithmetic expression. Without it, TeX's
// <dimen> scanning stops after one value, so
//
//	\begin{minipage}[t]{\textwidth-57.625mm}
//
// reads \textwidth, stops, and leaves "-57.625mm" in the input — where it is
// TYPESET, as visible text at the top of the body. Measured on sagej.cls:318, which
// is how arXiv 2406.08646 and 2302.04180 begin: the minipage took the full
// \textwidth instead of 57.625mm less, and the page opened with the characters
// "-57.625mm". 1085 papers of the arXiv pool load calc, 16 of the measured 154.
//
// ⛔ This is DELIBERATELY not applied to TeX's own <dimen> scanning, only to the
// LaTeX length arguments calc itself patches. calc redefines \setlength,
// \addtolength, \settowidth and the \@defaultunits path that LaTeX's length
// arguments go through; it does NOT touch \dimen0=…, and making that calc-aware
// would change what `\hskip 2pt -3pt` means in every document that happens to load
// the package.
//
// The grammar is calc.dtx's own (required/tools/calc.dtx):
//
//	<expr>   ::= <term> | <expr> ('+'|'-') <term>
//	<term>   ::= <factor> | <term> ('*'|'/') <integer>
//	<factor> ::= <dimen> | '(' <expr> ')'
//
// The right operand of * and / is an INTEGER, not a dimen — a real factor needs
// \real{…} or \ratio{…}{…}, which are recorded as unimplemented rather than
// guessed at (see calcInteger). Parentheses must be category 12, which is what the
// documentation requires too.

// calcLoaded reports whether calc's arithmetic is in force.
//
// ⛔ pkgRequested, not loadedPackages. The second is populated only when a .sty is
// actually found and run, and there is no calc.sty here — the arithmetic is the
// emulation. The first records every package the document ASKED for, which is
// exactly the question: a document that says \usepackage{calc} means its length
// arguments to be expressions whether or not this engine has the file. A class that
// \RequirePackage{calc}s it counts the same; the registry does not distinguish.
//
// Measured: against loadedPackages this returned false for every corpus paper and
// the whole change was inert.
func (e *Engine) calcLoaded() bool { return e.pkgRequested["calc"] }

// calcLeadDimen reads the first factor when it is a PARENTHESISED expression, which
// TeX's own <dimen> scanner cannot start on.
//
// ⛔ The tail hook alone is not enough. A caller scans its first value before asking
// for a tail, so "(100pt+20pt)*2" met scanDimen, which reads no "(" — it returned
// zero and the whole expression was left in the input and typeset. Measured: the
// length came out 0.0pt with "(100pt+20pt)*2" printed beside it.
func (e *Engine) calcLeadDimen() (int, bool) {
	if !e.calcLoaded() {
		return 0, false
	}
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 0, false
	}
	if t.cs_ || t.ch != '(' {
		e.back(t)
		return 0, false
	}
	e.back(t)
	return e.calcFactor(), true
}

// calcTailDimen finishes a length argument whose first factor has already been
// scanned. With calc absent it returns that value untouched, which is TeX's own
// behaviour and what every document without the package must keep.
func (e *Engine) calcTailDimen(first int) int {
	if !e.calcLoaded() {
		return first
	}
	return e.calcExprTail(e.calcTermTail(first))
}

// calcExprTail reads { ('+'|'-') <term> } onto an accumulated value.
func (e *Engine) calcExprTail(acc int) int {
	for {
		op, ok := e.calcOp('+', '-')
		if !ok {
			return acc
		}
		t := e.calcTermTail(e.calcFactor())
		if op == '+' {
			acc += t
		} else {
			acc -= t
		}
	}
}

// calcTermTail reads { ('*'|'/') <integer> } onto a factor already in hand. It
// binds tighter than + and -, so "a + b*2" multiplies b and not the sum.
func (e *Engine) calcTermTail(v int) int {
	for {
		op, ok := e.calcOp('*', '/')
		if !ok {
			return v
		}
		n, ok := e.calcInteger()
		if !ok {
			return v // an operand this engine cannot read: leave the value alone
		}
		switch {
		case op == '*':
			v *= n
		case n != 0:
			v /= n
		}
	}
}

// calcFactor reads a <dimen> or a parenthesised <expr>.
func (e *Engine) calcFactor() int {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 0
	}
	if !t.cs_ && t.ch == '(' {
		v := e.calcExprTail(e.calcTermTail(e.calcFactor()))
		e.skipOptSpace()
		if c, ok := e.getXToken(); ok && (c.cs_ || c.ch != ')') {
			e.back(c) // unbalanced: the caller's own scan will meet it
		}
		return v
	}
	e.back(t)
	return e.scanDimen()
}

// calcOp reads one of the two operators if it is next, and nothing otherwise.
//
// ⛔ It must put back what it read. These scanners sit inside a braced argument
// whose closing brace the caller is about to consume, so a token taken here and
// dropped would desynchronise the whole argument — which is the failure mode that
// makes a length argument eat the text after it.
func (e *Engine) calcOp(a, b rune) (rune, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 0, false
	}
	if !t.cs_ && (t.ch == a || t.ch == b) {
		return t.ch, true
	}
	e.back(t)
	return 0, false
}

// calcInteger reads the right operand of * or /. calc allows an integer, \real{…}
// or \ratio{…}{…}; the two real forms are NOT implemented and are recorded as
// skipped so the census names them, rather than returning a number that would
// silently mis-size a box.
func (e *Engine) calcInteger() (int, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 0, false
	}
	if t.cs_ {
		switch t.cs {
		case "real", "ratio":
			e.recordSkippedLength("calc:\\" + t.cs)
			e.readBraceName() // consume the first argument
			if t.cs == "ratio" {
				e.readBraceName()
			}
			return 0, false
		}
	}
	e.back(t)
	return e.scanInt(), true
}
