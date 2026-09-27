// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// \IfEqCase{<test>}{{<case>}{<code>}…}[<else>] and \IfStrEqCase, xstring's string
// switches (xstring.tex:620-645):
//
//	\def\IfEqCase{\xs_ifstar{\xs_IfStringCase{\IfEq*}}{\xs_IfStringCase\IfEq}}
//	\long\def\xs_IfStringCase#1#2#3{… reads two syntactic units at a time, compares,
//	  runs the matching code and EATS the optional argument; with the list exhausted it
//	  runs the <else> instead …}
//
// It has to be resolved HERE, on the maths source string, and not as a TeX macro. The
// maths layer hands go-tex/math a SOURCE STRING and renderMathResolvingMacros expands
// parameterless macros against text: it can match neither xstring's delimited scan
// (##3\_nil) nor a conditional, so a faithful TeX transcription is unreachable from a
// formula — the same wall that made \textcolor a primitive (engine#454), one step further
// along, because here the answer is not "strip it" but "pick a branch".
//
// Corpus paper 2308.09839 ships porousmedia-macros.sty, which does \usepackage{xstring}
// and then
//
//	\newcommand{\tensor}[2]{\IfEqCase*{#1}{{0}{#2}{1}{\boldsymbol{#2}}
//	  {2}{\boldsymbol{#2}}{4}{\textbf{\sffamily{#2}}}}[Did not match any given case!!]}
//
// \tensor is used throughout its formulas, so one undefined command cost 33 EQUATIONS —
// the largest single entry in the dropped-equation channel.
//
// The comparison is textual, which is what both spellings reduce to for a case list of
// small integers or words. \IfEq's numeric coercion (1 = 1.0) is NOT emulated: a case list
// that relies on it would fall through to the <else>, which is the branch xstring itself
// runs when nothing matches, so the formula still renders.
func (e *Engine) resolveMathIfEqCase(src, name string) (string, bool) {
	if name != "IfEqCase" && name != "IfStrEqCase" {
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
		rest = strings.TrimPrefix(rest, "*") // the starred form differs only in expansion
		rest = strings.TrimLeft(rest, " ")
		args, consumed, ok := parseMathArgs(rest, 2)
		if !ok {
			out.WriteString(needle) // leave it for go-tex/math to reject
			src = src[i+len(needle):]
			continue
		}
		rest = rest[consumed:]
		// The optional <else> is consumed whether or not a case matched: xstring's
		// \xs_IfStringCase_ii eats it on a hit, and runs it on a miss.
		fallback := ""
		if a, r, okOpt := takeMathOptArg(rest); okOpt {
			fallback, rest = a, r
		}
		out.WriteString(pickEqCase(strings.TrimSpace(args[0]), args[1], fallback))
		src = rest
		changed = true
	}
	out.WriteString(src)
	return out.String(), changed
}

// pickEqCase reads {<case>}{<code>} pairs out of a case list and returns the code of the
// first case equal to test, or the fallback.
//
// A list with an odd number of groups is xstring's own error case; the trailing group is
// dropped rather than guessed at, and the fallback answers.
func pickEqCase(test, cases, fallback string) string {
	for {
		c, rest, ok := takeFirstBracedGroup(cases)
		if !ok {
			return fallback
		}
		code, rest2, ok := takeFirstBracedGroup(rest)
		if !ok {
			return fallback
		}
		if strings.TrimSpace(c) == test {
			return code
		}
		cases = rest2
	}
}

// takeFirstBracedGroup returns the contents of the leading {…} of s and what follows it,
// counting nested braces. Leading spaces are skipped, because a case list is written out
// over several lines.
func takeFirstBracedGroup(s string) (group, rest string, ok bool) {
	s = strings.TrimLeft(s, " \t\n")
	if !strings.HasPrefix(s, "{") {
		return "", s, false
	}
	depth := 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], true
			}
		}
	}
	return "", s, false
}
