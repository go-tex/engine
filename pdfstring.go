// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// This file renders a title-like token list (a section heading, or \@title/\@author
// for hyperref's pdfusetitle) as clean plain text, for a PDF bookmark /Title and the
// document information dictionary. It is the job LaTeX's \pdfstringdef does: strip
// the markup a heading may carry — font switches, \textbf-style wrappers, \thanks —
// and resolve accents to real Unicode letters, so "R\'esum\'e" and a literal "Résumé"
// both become "Résumé" rather than a title full of backslashes.
//
// It walks the RAW (unexpanded) token list. Expanding first is wrong here: symbol
// macros like \ss expand to "\char223\relax" (see format.go) and \i to a dotless-i
// char code, so the accent base and the letter are both lost. Instead each control
// sequence is handled by name — accents via precompose (accents.go), font switches
// dropped, \thanks-style commands gobbling their argument — and a param-less user
// macro is expanded on demand so a \newcommand in a title still contributes its text.

// textCommandReplacements maps a control sequence a title can carry to the plain
// text it stands for. \ss, \oe, the \LaTeX logo and the escaped specials all live
// here; a name that is none of these, nor an accent, font switch or gobble command,
// is dropped (its braced argument, if any, is still walked).
var textCommandReplacements = map[string]string{
	"LaTeX": "LaTeX", "LaTeXe": "LaTeX2e", "TeX": "TeX", "eTeX": "e-TeX",
	"&": "&", "_": "_", "%": "%", "#": "#", "$": "$", "{": "{", "}": "}",
	"textbackslash": `\`, "textasciitilde": "~", "textasciicircum": "^",
	"ss": "ß", "ae": "æ", "oe": "œ", "aa": "å", "o": "ø", "l": "ł",
	"AE": "Æ", "OE": "Œ", "AA": "Å", "O": "Ø", "L": "Ł",
	"i": "ı", "j": "ȷ", // dotless i/j standing alone; as an accent base they are handled first
	"dag": "†", "ddag": "‡", "pounds": "£", "copyright": "©", "S": "§", "P": "¶",
	"dots": "…", "ldots": "…", "textellipsis": "…",
	"textbar": "|", "textless": "<", "textgreater": ">",
	"textquotedblleft": "“", "textquotedblright": "”",
	"textquoteleft": "‘", "textquoteright": "’",
	"textemdash": "—", "textendash": "–",
	// spacing commands become a single space (collapsed later)
	"nobreakspace": " ", " ": " ", ",": " ", ";": " ", ":": " ",
	"quad": " ", "qquad": " ", "enspace": " ", "thinspace": " ",
}

// fontSwitches are control sequences that only change the font, size or shape. In
// plain text they are dropped, keeping the text they governed.
var fontSwitches = map[string]bool{
	"bfseries": true, "mdseries": true, "itshape": true, "scshape": true,
	"slshape": true, "upshape": true, "normalfont": true, "rmfamily": true,
	"sffamily": true, "ttfamily": true, "em": true, "normalsize": true,
	"bf": true, "it": true, "sl": true, "sc": true, "tt": true, "rm": true, "sf": true,
	"tiny": true, "scriptsize": true, "footnotesize": true, "small": true,
	"large": true, "Large": true, "LARGE": true, "huge": true, "Huge": true,
	"boldmath": true, "unboldmath": true, "selectfont": true,
	"protect": true, "relax": true, "noindent": true, "ignorespaces": true,
}

// gobbleCommands drop themselves AND their following braced argument, so a title's
// \thanks{…} or \footnote{…} contributes nothing (as it does not in a bookmark).
var gobbleCommands = map[string]bool{
	"thanks": true, "footnote": true, "footnotemark": true,
	"label": true, "index": true, "nonumberline": true,
}

// tokensToPlainText renders a raw title token list as clean plain text, collapsing
// runs of whitespace.
func (e *Engine) tokensToPlainText(toks []tok) string {
	var b strings.Builder
	e.appendPlain(&b, toks, 0)
	return strings.Join(strings.Fields(b.String()), " ")
}

// appendPlain walks toks (raw), writing plain text into b. depth bounds recursion
// into expanded user macros.
func (e *Engine) appendPlain(b *strings.Builder, toks []tok, depth int) {
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if !t.cs_ {
			switch t.cat {
			case catBegin, catEnd, catMath:
				// structural: drop
			case catSpace:
				b.WriteByte(' ')
			case catActive:
				if t.ch == '~' {
					b.WriteByte(' ') // a tie is a space in plain text
				} else {
					b.WriteRune(t.ch)
				}
			default:
				b.WriteRune(t.ch)
			}
			continue
		}
		switch {
		case isAccentCS(t.cs):
			if base, adv, ok := accentBase(toks[i+1:]); ok {
				b.WriteRune(precompose(t.cs, base))
				i += adv
			}
		case gobbleCommands[t.cs]:
			i = skipOneGroup(toks, i+1) - 1 // also drop the {argument}
		case fontSwitches[t.cs]:
			// drop, keep the governed text
		case t.cs == "char":
			if r, adv := charValue(toks[i+1:]); adv > 0 {
				b.WriteRune(r)
				i += adv
			}
		case t.cs == `\` || t.cs == "newline" || t.cs == "par":
			b.WriteByte(' ')
		default:
			if r, ok := textCommandReplacements[t.cs]; ok {
				b.WriteString(r)
			} else if m := e.eq[t.cs]; m != nil && m.kind == mMacro && len(m.params) == 0 && depth < 8 {
				e.appendPlain(b, m.body, depth+1) // a param-less user macro contributes its text
			}
			// else unknown: drop it (never emit a literal \name)
		}
	}
}

// isAccentCS reports whether a control-sequence name is a text accent (\', \`, \^,
// \c, \v …) with an entry in the combining-mark table (see accents.go).
func isAccentCS(name string) bool {
	_, ok := combiningMark[name]
	return ok
}

// accentBase reads the base an accent applies to from the tokens following it — a
// braced {x} or the single next token — returning the base rune, how many tokens to
// advance past, and ok=false when there is no usable base (\'{} or end of list).
func accentBase(rest []tok) (base rune, advance int, ok bool) {
	if len(rest) == 0 {
		return 0, 0, false
	}
	if rest[0].cat == catBegin && !rest[0].cs_ {
		r, adv, ok := singleBase(rest[1:])
		if !ok {
			return 0, 0, false
		}
		if adv < len(rest[1:]) && rest[1+adv].cat == catEnd && !rest[1+adv].cs_ {
			return r, 1 + adv + 1, true // { base }
		}
		return 0, 0, false
	}
	return singleBase(rest)
}

// singleBase yields the base rune for an accent from the head of rest: a character
// token gives its rune; \i / \j give the dotted letters they stand in for (so an
// accent composes, matching doAccent); \char<n> gives its code point.
func singleBase(rest []tok) (rune, int, bool) {
	if len(rest) == 0 {
		return 0, 0, false
	}
	t := rest[0]
	if t.cs_ {
		switch t.cs {
		case "i":
			return 'i', 1, true
		case "j":
			return 'j', 1, true
		case "char":
			if r, adv := charValue(rest[1:]); adv > 0 {
				return r, 1 + adv, true
			}
		}
		return 0, 0, false
	}
	return t.ch, 1, true
}

// charValue parses a TeX \char argument at the head of rest — decimal digits, or a
// "hex, 'octal or `char prefix — and returns the code point and how many tokens it
// spans (including a trailing \relax or space). advance is 0 when nothing parses.
func charValue(rest []tok) (r rune, advance int) {
	if len(rest) == 0 {
		return 0, 0
	}
	i, base := 0, 10
	switch {
	case rest[0].is('"', catOther):
		base, i = 16, 1
	case rest[0].is('\'', catOther):
		base, i = 8, 1
	case rest[0].is('`', catOther):
		if len(rest) >= 2 {
			c := rest[1].ch
			if rest[1].cs_ && len(rest[1].cs) == 1 {
				c = rune(rest[1].cs[0])
			}
			return c, skipTrailer(rest, 2)
		}
		return 0, 0
	}
	start, n := i, int64(0)
	for i < len(rest) && !rest[i].cs_ {
		d, ok := digitVal(rest[i].ch, base)
		if !ok {
			break
		}
		n = n*int64(base) + int64(d)
		i++
	}
	if i == start {
		return 0, 0
	}
	return rune(n), skipTrailer(rest, i)
}

// digitVal returns the value of a digit rune in the given base (10 or 16), false if
// it is not one.
func digitVal(r rune, base int) (int, bool) {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0'), r-'0' < rune(base)
	case base == 16 && r >= 'a' && r <= 'f':
		return int(r-'a') + 10, true
	case base == 16 && r >= 'A' && r <= 'F':
		return int(r-'A') + 10, true
	}
	return 0, false
}

// skipTrailer advances past a \char number's optional trailing \relax or space.
func skipTrailer(rest []tok, i int) int {
	if i < len(rest) {
		if rest[i].cs_ && rest[i].cs == "relax" {
			return i + 1
		}
		if !rest[i].cs_ && rest[i].cat == catSpace {
			return i + 1
		}
	}
	return i
}

// skipOneGroup returns the index just past the braced group starting at toks[j], or
// j itself when toks[j] is not an opening brace (nothing to skip).
func skipOneGroup(toks []tok, j int) int {
	if j >= len(toks) || toks[j].cs_ || toks[j].cat != catBegin {
		return j
	}
	depth := 0
	for i := j; i < len(toks); i++ {
		if toks[i].cs_ {
			continue
		}
		switch toks[i].cat {
		case catBegin:
			depth++
		case catEnd:
			if depth--; depth == 0 {
				return i + 1
			}
		}
	}
	return len(toks)
}

// macroPlainText renders a macro's body (\@title, \@author) as plain text for PDF
// metadata, or "" when the name is undefined or is not a macro.
func (e *Engine) macroPlainText(name string) string {
	m := e.eq[name]
	if m == nil || m.kind != mMacro {
		return ""
	}
	return e.tokensToPlainText(m.body)
}
