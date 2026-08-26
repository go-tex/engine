// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "golang.org/x/text/unicode/norm"

// This file implements TeX's accent commands (\'e → é, \c c → ç, \^o → ô, …). The
// output fonts carry precomposed accented glyphs, so rather than overlay an
// accent box, the engine combines the base letter with the accent's Unicode
// combining mark and normalises to the precomposed form (NFC). This gives correct
// results for every base/accent pair the font provides, essential for French and
// other Latin-script languages.

// combiningMark maps an accent command name to its Unicode combining diacritic.
var combiningMark = map[string]rune{
	"'":  0x0301, // acute      \'
	"`":  0x0300, // grave      \`
	"^":  0x0302, // circumflex \^
	"\"": 0x0308, // diaeresis  \"
	"~":  0x0303, // tilde      \~
	"=":  0x0304, // macron     \=
	".":  0x0307, // dot above  \.
	"u":  0x0306, // breve      \u
	"v":  0x030C, // caron      \v
	"H":  0x030B, // double acute \H
	"r":  0x030A, // ring       \r
	"c":  0x0327, // cedilla    \c
	"k":  0x0328, // ogonek     \k
	"b":  0x0331, // bar under  \b
	"d":  0x0323, // dot under  \d
}

// doAccent typesets base char accented by the given accent command: it reads the
// (raw) following base letter and typesets the precomposed character, falling back
// to the base letter when no precomposed form exists.
func (e *Engine) doAccent(accent string) {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok {
		return
	}
	if t.cat == catBegin && !t.cs_ {
		// A LaTeX text accent takes one argument, and \^{a} braces its base
		// letter. Read the balanced group, accent its first token and re-inject
		// the rest so it typesets normally. Consuming the group's close brace is
		// what matters: left in the stream it closes an enclosing group (a
		// \raisebox box, a tabular cell) and the rest of the document is silently
		// swallowed — the eptcs \publicationstatus "C\^{a}mpeanu" truncation.
		group := e.readGroupToks()
		if len(group) == 0 {
			return // \^{}: an empty argument accents nothing.
		}
		t = group[0]
		if len(group) > 1 {
			e.push(group[1:])
		}
	}
	base := t.ch
	if t.cs_ {
		// \i / \j (dotless) as accent bases: use the dotted letter for composition.
		switch t.cs {
		case "i":
			base = 'i'
		case "j":
			base = 'j'
		default:
			e.back(t)
			return
		}
	}
	e.startChar(precompose(accent, base))
}

// readGroupToks reads the balanced {…} whose opening brace has already been
// consumed and returns the tokens inside it, excluding the outer braces but
// keeping any nested braces so the returned run stays balanced. It stops at the
// matching close brace (which it consumes) or at end of input.
func (e *Engine) readGroupToks() []tok {
	var toks []tok
	depth := 0
	for {
		u, ok := e.getNext()
		if !ok {
			break
		}
		if !u.cs_ && u.cat == catEnd {
			if depth == 0 {
				break // the matching close brace of the whole group
			}
			depth--
		} else if !u.cs_ && u.cat == catBegin {
			depth++
		}
		toks = append(toks, u)
	}
	return toks
}

// precompose combines a base rune with an accent's combining mark and returns the
// NFC-normalised single rune, or the base rune when there is no precomposed form.
func precompose(accent string, base rune) rune {
	mark, ok := combiningMark[accent]
	if !ok {
		return base
	}
	r := []rune(norm.NFC.String(string(base) + string(mark)))
	if len(r) == 1 {
		return r[0]
	}
	return base
}
