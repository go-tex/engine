package engine

import (
	"strings"
	"unicode/utf8"
)

// expandIfNextCharInMathSource resolves LaTeX's one-token look-ahead inside a maths
// source string.
//
// \@ifnextchar <t>{yes}{no} skips spaces, compares the NEXT token with <t> using
// \ifx, and expands to yes or no without consuming that token (latex.ltx:1602-1618).
// It is the kernel's whole optional-argument protocol: \@testopt, \@protected@testopt,
// \@ifstar and \@dblarg are all written on top of it.
//
// The maths path is a separate, STRING-level expander that never runs the gullet, so
// an \@ifnextchar carried into a formula by a macro body arrived at go-tex/math as a
// bare name and the equation was dropped whole. Measured on 999 arXiv papers: 290
// equations over 6 papers, the fifth-largest trigger in the census (#466). The six
// callers have nothing in common — a paper's own \widebar, springer's \@spnthm, a
// bra-ket \k@t, \@slashbox — but the MECHANISM is one, which is what makes a fix here
// general rather than six paper-local patches.
//
// The decision is taken with ifxEqual, the same comparison \ifx itself uses, so a
// control sequence \let to the character it peeks at compares equal here exactly as it
// does in the stomach. Where the look-ahead cannot be read — a missing argument, an
// unbalanced group — the name is left standing so the census reports it, rather than a
// branch being guessed: an equation that renders the wrong half is invisible to every
// channel, where a refused one is counted.
func (e *Engine) expandIfNextCharInMathSource(src, name string) (string, bool) {
	var out strings.Builder
	changed := false
	for {
		i := findMathCS(src, name)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := src[i+1+len(name):]
		args, consumed, ok := parseMathArgs(rest, 3)
		if !ok || args[0] == "" {
			// Leave this occurrence for go-tex/math to reject. An empty first
			// argument is malformed input, not "no token to match".
			out.WriteString("\\" + name)
			src = rest
			continue
		}
		// \@xifnch loops while the peeked token is \@sptoken, so the spaces before
		// the look-ahead ARE consumed; the token itself is only peeked and stays.
		after := strings.TrimLeft(rest[consumed:], " ")
		branch := args[2]
		if e.mathLookaheadMatches(after, args[0]) {
			branch = args[1]
		}
		out.WriteString(branch)
		src = after
		changed = true
	}
	if !changed {
		return "", false
	}
	out.WriteString(src)
	return out.String(), true
}

// mathLookaheadMatches reports whether the token at the head of a maths source string
// is \ifx-equal to the one the look-ahead is asking about.
//
// An empty string means the formula ends here, so what TeX would actually peek is the
// math shift that closed it — never the [ or ^ or macro name a look-ahead asks about.
// Answering "no" there is the same answer TeX gives, not a default.
func (e *Engine) mathLookaheadMatches(after, want string) bool {
	next, ok := e.mathTokenFromSource(after)
	if !ok {
		return false
	}
	target, ok := e.mathTokenFromSource(want)
	if !ok {
		return false
	}
	return e.ifxEqual(target, next)
}

// mathTokenFromSource reads the ONE token at the head of a maths source string, in
// the encoding writeMathCS produces: a control sequence as \name (letters and @), a
// control symbol as \ plus one rune, anything else as a single rune carrying the
// engine's own catcode for it — the same catcode the target token is given, so the
// two are compared on equal terms.
func (e *Engine) mathTokenFromSource(s string) (tok, bool) {
	if s == "" {
		return tok{}, false
	}
	if s[0] == '\\' {
		j := 1
		for j < len(s) && isMathCSLetter(s[j]) {
			j++
		}
		if j == 1 {
			r, w := utf8.DecodeRuneInString(s[1:])
			if w == 0 {
				return tok{}, false
			}
			return tok{cs: string(r), cs_: true}, true
		}
		return tok{cs: s[1:j], cs_: true}, true
	}
	r, _ := utf8.DecodeRuneInString(s)
	return tok{ch: r, cat: e.catOf(r)}, true
}

// findMathCS locates \name in a maths source string at a control-WORD boundary: the
// name must not run on into another name character, or a search for \@ifnextchar
// would also match the start of \@ifnextcharX.
func findMathCS(src, name string) int {
	needle := "\\" + name
	for i := 0; ; {
		j := strings.Index(src[i:], needle)
		if j < 0 {
			return -1
		}
		j += i
		k := j + len(needle)
		if k >= len(src) || !isMathCSLetter(src[k]) {
			return j
		}
		i = k
	}
}

// isMathCSLetter reports whether a byte continues a control-sequence name in a maths
// source string. The kernel makes @ a letter while a class or package is being read
// (latex.ltx's \makeatletter), and every name this path meets — \@ifnextchar itself,
// \@spnthm, \k@t — is written with it, so @ counts here where isMathLetter, which
// answers about TeX's letters, does not.
func isMathCSLetter(c byte) bool {
	return isMathLetter(c) || c == '@'
}
