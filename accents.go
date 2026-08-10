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
