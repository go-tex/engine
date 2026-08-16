// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements eTeX's expression primitives, \numexpr and \dimexpr.
// They are not a convenience: a package that computes — a drawing package
// converting every coordinate from points to big points, a length calculation in
// a class file — reaches for them on every value, so without them such a package
// runs but produces nonsense. e-TeX has been the default engine under LaTeX for
// two decades, so real-world sources assume them.
//
//	<expr>   → <term> { (+|-) <term> }
//	<term>   → <factor> { (*|/) <factor> }
//	<factor> → ( <expr> ) | <quantity>
//
// The expression is terminated by an optional \relax (consumed, as e-TeX does).
// Arithmetic is exact: the operands are accumulated in 64-bit, so the scaling
// idiom a*b/c keeps its full intermediate product, and division rounds to the
// nearest integer with ties away from zero (e-TeX's rule).
//
// A \dimexpr's factors are dimensions, but a factor with no unit is an integer
// coefficient, so both `\dimexpr 2pt*3\relax` and `\dimexpr 3*2pt\relax` are
// read the way a source expects. The result of either primitive can be used
// wherever its type is wanted — as an internal quantity (\count0=\numexpr…,
// \hsize=\dimexpr…), inside another expression, or printed with \the.

// scanExpr evaluates an eTeX expression. dimen selects the type: an integer
// expression (\numexpr) or a dimension in scaled points (\dimexpr).
func (e *Engine) scanExpr(dimen bool) int {
	v := e.scanExprSum(dimen)
	e.skipOptSpace()
	// A trailing \relax terminates the expression and is absorbed; anything else
	// belongs to whatever follows.
	if t, ok := e.getXToken(); ok {
		if !(t.cs_ && e.isPrim(t.cs, "relax")) {
			e.back(t)
		}
	}
	return int(v)
}

// scanExprSum reads a sum of terms.
func (e *Engine) scanExprSum(dimen bool) int64 {
	sum, _ := e.scanExprTerm(dimen)
	for {
		e.skipOptSpace()
		t, ok := e.getXToken()
		if !ok {
			return sum
		}
		switch {
		case t.is('+', catOther):
			v, _ := e.scanExprTerm(dimen)
			sum += v
		case t.is('-', catOther):
			v, _ := e.scanExprTerm(dimen)
			sum -= v
		default:
			e.back(t)
			return sum
		}
	}
}

// scanExprTerm reads a product/quotient of factors, reporting whether the term
// carries the expression's own type (a dimension, for \dimexpr) or is a bare
// integer coefficient.
func (e *Engine) scanExprTerm(dimen bool) (int64, bool) {
	v, typed := e.scanExprFactor(dimen)
	for {
		e.skipOptSpace()
		t, ok := e.getXToken()
		if !ok {
			return v, typed
		}
		switch {
		case t.is('*', catOther):
			w, wTyped := e.scanExprFactor(dimen)
			v *= w
			typed = typed || wTyped
		case t.is('/', catOther):
			w, _ := e.scanExprFactor(dimen)
			v = divRound(v, w)
		default:
			e.back(t)
			return v, typed
		}
	}
}

// scanExprFactor reads one factor: a parenthesised subexpression, or a quantity
// of the expression's type. In a \dimexpr a unitless number is an integer
// coefficient (typed = false), which is how `3*2pt` and `2pt*3` both work.
func (e *Engine) scanExprFactor(dimen bool) (int64, bool) {
	e.skipOptSpace()
	sign := e.scanSign()
	e.skipOptSpace()
	if t, ok := e.getXToken(); ok {
		if t.is('(', catOther) {
			v := e.scanExprSum(dimen)
			e.skipOptSpace()
			if u, ok := e.getXToken(); ok && !u.is(')', catOther) {
				e.back(u) // unbalanced: leave it for the caller rather than eat it
			}
			return int64(sign) * v, dimen
		}
		e.back(t)
	}
	if !dimen {
		return int64(sign) * int64(e.scanInt()), true
	}
	v, unit := e.scanDimenOrCoefficient()
	return int64(sign) * int64(v), unit
}

// scanDimenOrCoefficient reads a \dimexpr factor: a dimension when a unit
// follows, otherwise the bare integer that multiplies one.
func (e *Engine) scanDimenOrCoefficient() (int, bool) {
	// An internal dimension (a register, \hsize, a nested \dimexpr…) is always a
	// dimension; scanDimenValue backs out and returns 0 when the next token is
	// not one, in which case the factor is a plain number.
	t, ok := e.getXToken()
	if !ok {
		return 0, true
	}
	e.back(t)
	if t.cs_ {
		v, _ := e.scanDimenValue(false)
		return v, true
	}
	// A number: it is a dimension only when a unit follows it.
	intPart, frac := e.scanDecimalSP()
	if e.upcomingUnit() {
		return e.applyUnit(intPart, frac), true
	}
	return intPart, false
}

// upcomingUnit reports whether the input begins with a unit of measure — a unit
// keyword, or an internal dimension used as one (TeX's <factor><internal dimen>,
// as in .5\hsize) — without consuming anything.
func (e *Engine) upcomingUnit() bool {
	var buf []tok
	restore := func() {
		for k := len(buf) - 1; k >= 0; k-- {
			e.back(buf[k])
		}
	}
	e.skipOptSpace()
	var letters []rune
	for len(letters) < 2 {
		t, ok := e.getXToken()
		if !ok {
			restore()
			return false
		}
		buf = append(buf, t)
		if t.cs_ {
			restore()
			return e.isInternalDimen(t)
		}
		letters = append(letters, lower(t.ch))
	}
	restore()
	u := string(letters)
	if u == "pt" || u == "sp" || u == "em" || u == "ex" || u == "tr" { // "true<unit>"
		return true
	}
	_, ok := unitRatio[u]
	return ok
}

// isInternalDimen reports whether a control sequence denotes a dimension, so it
// can serve as the unit of a preceding factor.
func (e *Engine) isInternalDimen(t tok) bool {
	m := e.eq[t.cs]
	if m == nil {
		return false
	}
	switch m.kind {
	case mDimenRef, mSkipRef:
		return true
	case mPrim:
		switch m.name {
		case "dimen", "skip", "wd", "ht", "dp", "hsize", "vsize", "parindent",
			"baselineskip", "leftskip", "rightskip", "dimexpr":
			return true
		}
	}
	return false
}

// divRound divides, rounding to the nearest integer with ties away from zero —
// e-TeX's rounding for the / operator. Division by zero yields zero (e-TeX
// reports an error and uses zero).
func divRound(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	neg := (a < 0) != (b < 0)
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	q := (2*a + b) / (2 * b)
	if neg {
		return -q
	}
	return q
}

// isPrim reports whether a control sequence currently means the named primitive.
func (e *Engine) isPrim(cs, name string) bool {
	m := e.eq[cs]
	return m != nil && m.kind == mPrim && m.name == name
}
