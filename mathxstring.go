// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strconv"
	"strings"
)

// xstring's \IfEqCase and \IfStrEqCase are a string-comparison SWITCH: they take a
// test string and a list of {case}{code} pairs, and expand to the code of the first
// case that matches. 33 equations on one arXiv paper of the reference corpus were
// dropped by one of them, which is the largest single item in the dropped-equation
// census.
//
// The real macros are deep expansion machinery — \xs_testcase recursing over \_nil
// delimiters, \xs_ifstar, \xs_testopt (xstring.tex:626-645) — and none of it can be
// expanded textually. What CAN be done textually is the switch's decision, and the
// bibliography is what showed that this is enough. xstring.tex:588-596 gives the
// comparison exactly:
//
//	\xs_TestEqual   exact token-list equality first (\xs_ifx), else …
//	\xs_IfStrEqFalse_ii   … if BOTH arguments are decimals, \ifdim#1pt=#2pt,
//	                      otherwise false
//
// So \IfEqCase compares as strings and falls back to NUMERIC equality when both
// sides are decimals; \IfStrEqCase (\xs_IfStrEqFalse_i) compares as strings only.
// That difference is why the two are not folded together here.
//
// The star, per xstring.tex:235-256, swaps \xs_expand_and_assign for
// \xs_expand_and_detokenize: the starred form compares the arguments' printed
// characters. A go-tex/math source string is already scanned text, so the starred
// behaviour is what this reproduces; the unstarred form's token-level comparison is
// not available here and is treated the same way. The corpus paper writes the starred
// form, and its selector is always a literal digit — \tensor{1}{…} 58 times and
// \tensor{2}{…} 55 — so the distinction does not arise in practice.
func (e *Engine) resolveXStringCase(src, name string) (string, bool) {
	numeric := false
	switch name {
	case "IfEqCase":
		numeric = true
	case "IfStrEqCase":
	default:
		return src, false
	}
	if !e.pkgRequested["xstring"] {
		return src, false
	}
	needle := "\\" + name + " "
	var out strings.Builder
	changed := false
	for {
		i := strings.Index(src, needle)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := strings.TrimLeft(src[i+len(needle):], " ")
		rest = strings.TrimPrefix(rest, "*")
		args, consumed, ok := parseMathArgs(rest, 2)
		if !ok {
			// Malformed: left verbatim so go-tex/math reports it, rather than this
			// silently eating the switch and whatever followed.
			out.WriteString(needle)
			src = src[i+len(needle):]
			continue
		}
		rest = rest[consumed:]
		// The optional [else] branch, which xstring reads with \xs_testopt.
		other := ""
		if a, r, ok := takeMathOptArg(rest); ok {
			other, rest = a, r
		}
		out.WriteString(xstringCasePick(args[0], args[1], other, numeric))
		src = rest
		changed = true
	}
	if !changed {
		return src, false
	}
	out.WriteString(src)
	return out.String(), true
}

// xstringCasePick walks the {case}{code} list and returns the first match's code, or
// the else branch.
//
// A list that does not come in PAIRS stops the walk and falls to the else branch
// rather than pairing a case with the next case's code: a trailing odd group is a
// malformed switch, and guessing which half it is would put the wrong material on the
// page silently.
func xstringCasePick(test, list, other string, numeric bool) string {
	for {
		pair, consumed, ok := parseMathArgs(list, 2)
		if !ok {
			return other
		}
		if xstringEq(test, pair[0], numeric) {
			return pair[1]
		}
		list = list[consumed:]
	}
}

// xstringEq is \xs_TestEqual: string equality, then numeric equality when BOTH sides
// are decimals and the caller asked for it (xstring.tex:588-596).
func xstringEq(a, b string, numeric bool) bool {
	if a == b {
		return true
	}
	if !numeric {
		return false
	}
	x, errA := strconv.ParseFloat(strings.TrimSpace(a), 64)
	y, errB := strconv.ParseFloat(strings.TrimSpace(b), 64)
	return errA == nil && errB == nil && x == y
}
