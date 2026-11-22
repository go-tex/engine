// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// xstring's \IfInteger{<number>}{<true>}{<false>} and \IfDecimal{<number>}{<true>}{<false>}
// (xstring.tex:533-558). Nine papers of the 154-paper corpus reach \IfInteger, through
// a class that tests its own options with it; undefined, the command was skipped and
// BOTH branches were typeset, since each is an ordinary braced group.
//
// The real macro is loaded when a paper ships xstring.sty and the engine runs it to
// the end without an undefined command, but it answers wrongly: measured on
// \IfInteger{42}{int}{notint} it sets "int" AND "notint", where tectonic sets "int".
// Its test is \afterassignment\xs_defafterinteger\integerpart… read back through
// \expandafter\xs_testdot\afterinteger\_nil, which depends on the remainder of a
// number scan being delivered as tokens. The decision it reaches is small and fully
// described by the source, so it is made here, as the same two tests:
//
//	\xs_formatnumber  prefixes "0" (after a single leading + or -), so "" is "0X"
//	                  and ".5" is "0.5": the integer part may be empty, a lone sign
//	                  or an empty string is never a number
//	\xs_testdot       an integer part, then optionally "." or "," and a decimal part
//	                  that must be non-empty (xstring.tex:578-584 turns an empty one
//	                  into "0X"): "5." is NOT decimal
//	\IfInteger        additionally needs the decimal part to be zero (:540), so
//	                  "2.0" is an integer — the case xstring.tex:1152 records as a bug
//
// The first argument is expanded, as xstring does with \xs_expand_and_assign.
func (e *Engine) doIfNumberKind(integer bool) {
	num := e.grabUndelimited()
	yes := e.grabUndelimited()
	no := e.grabUndelimited()
	if xstringIsNumber(e.expandToString(num), integer) {
		e.push(yes)
	} else {
		e.push(no)
	}
}

// xstringIsNumber is the decision \IfDecimal (integer false) or \IfInteger (true) takes.
func xstringIsNumber(s string, integer bool) bool {
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	rest := s[i:]
	if rest == "" {
		// An integer part alone. An empty string, or a lone sign, is "0X" to xstring.
		return i > 0
	}
	if rest[0] != '.' && rest[0] != ',' {
		return false
	}
	dec := rest[1:]
	if dec == "" {
		return false
	}
	for j := 0; j < len(dec); j++ {
		if dec[j] < '0' || dec[j] > '9' {
			return false
		}
	}
	if integer {
		return strings.Trim(dec, "0") == ""
	}
	return true
}
