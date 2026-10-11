// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file is TeX's SPACE FACTOR: the state that makes the space after a
// sentence wider than the space after a word, and the space after a comma
// stretch less than an ordinary one.
//
// ⛔⛔ It was not modelled at all. \sfcode was scanned and thrown away
// ("No space-factor model yet"), \frenchspacing and \nonfrenchspacing were empty
// macros, and every interword space was the font's plain one. Measured against
// tectonic on three sentences — the same words with commas and with periods:
//
//	            commas   periods   difference
//	tectonic     88.02    91.34     +3.32bp
//	this engine  74.56    74.56      0.00
//
// Three sentence boundaries, so the reference adds 1.107bp each — and cmr10's
// \fontdimen7 (extra_space) is 1.111pt = 1.107bp. The number is that font
// parameter, once per boundary, not an approximation of anything.
//
// It is not a cosmetic 3bp either: §1043 below scales stretch AND shrink at every
// factor other than 1000, so a comma's space is wrong too, and spacing feeds the
// line breaker — which makes this a document-wide bias in how much fits on a
// line (go-tex/engine#610).

// defaultSfcode is INITEX's table (tex.web §232, lines 4838 and 4850): every
// character is 1000, and an UPPERCASE letter is 999 — which is what makes a period
// after a capital not end a sentence, and the whole reason \@ exists.
//
// The engine is Unicode where TeX is 8-bit, so the rule is applied to the rune
// rather than to a 256-entry array.
func defaultSfcode(r rune) int {
	if r >= 'A' && r <= 'Z' {
		return 999
	}
	return 1000
}

// sfcodeOf is the space-factor code of a character: what a document assigned, or
// INITEX's default.
func (e *Engine) sfcodeOf(r rune) int {
	if v, ok := e.sfcode[r]; ok {
		return v
	}
	return defaultSfcode(r)
}

// setSfcode performs \sfcode<char>=<int>, group-scoped like every other table
// assignment.
func (e *Engine) setSfcode(r rune, v int, global bool) {
	if e.sfcode == nil {
		e.sfcode = map[rune]int{}
	}
	if !global && len(e.groups) > 0 {
		old, had := e.sfcode[r]
		if !had {
			old = defaultSfcode(r)
		}
		// oldi, not oldc: oldc is a `cat`, which is a small integer type — an sfcode
		// of 3000 came back as 184 through it, 3000 mod 256, and the test for the
		// scoping is what caught that.
		e.save = append(e.save, saveItem{kind: saveSfcode, idx: int(r), oldi: old})
	}
	e.sfcode[r] = v
}

// adjustSpaceFactor is tex.web's adjust_space_factor macro (§1034), verbatim:
//
//	main_s := sf_code(cur_chr)
//	if main_s = 1000 then space_factor := 1000
//	else if main_s < 1000 then begin if main_s > 0 then space_factor := main_s end
//	else if space_factor < 1000 then space_factor := 1000
//	else space_factor := main_s
//
// The middle clause is the one worth reading twice: an sfcode of 0 — latex.ltx
// gives ")", "'" and "]" one — leaves the factor ALONE, so "end.)" still ends a
// sentence. And the last two say the factor never climbs from below 1000 straight
// past it, which is why "NASA." does not get sentence spacing.
func (e *Engine) adjustSpaceFactor(r rune) {
	s := e.sfcodeOf(r)
	switch {
	case s == 1000:
		e.spaceFactor = 1000
	case s < 1000:
		if s > 0 {
			e.spaceFactor = s
		}
	case e.spaceFactor < 1000:
		e.spaceFactor = 1000
	default:
		e.spaceFactor = s
	}
}

// spaceGlueForFactor is tex.web's app_space (§1043): the interword glue once the
// space factor is taken into account.
//
//	if space_factor >= 2000:  width += extra_space
//	stretch = stretch × space_factor / 1000
//	shrink  = shrink  × 1000 / space_factor
//
// ⚠ Two parts of §1043 are NOT here, and saying which keeps the next reader from
// assuming they are: the \xspaceskip branch (taken when the factor is ≥ 2000 and
// that skip is non-zero) and \spaceskip as the base glue. Both are declared in
// this engine as \newskip registers rather than real glue parameters, so there is
// nothing to read; a document that sets them is unaffected today and would be
// wrongly affected by half an implementation.
func (e *Engine) spaceGlueForFactor(f fontFace) glueSpec {
	g := e.spaceGlueOf(f)
	sf := e.spaceFactor
	if sf <= 0 {
		return g
	}
	if sf >= 2000 {
		g.width += e.extraSpaceOf(f)
	}
	// xnOverD is the engine's existing TeX §107 helper, quotient and remainder; the
	// remainder is dropped exactly as TeX drops it here (§1043 assigns the quotient).
	g.stretch, _ = xnOverD(g.stretch, sf, 1000)
	g.shrink, _ = xnOverD(g.shrink, 1000, sf)
	return g
}

// extraSpaceOf is the font's \fontdimen7, the amount added at the end of a
// sentence (tex.web §10886: "additional space at end of sentence").
//
// A document that assigns \fontdimen7 gets exactly that, like \fontdimen2/3/4
// already do. An OpenType face carries no such parameter, so the default is a
// MODEL rather than a measurement: Computer Modern's text fonts set it to a third
// of the interword space (cmr10: space 3.333pt, extra_space 1.111pt), and
// spaceSP's own shrink is already space/3, so this reproduces CM's ratio at every
// size. The witness in #610 is the check on it: 1.107bp per boundary is what the
// reference adds, and what this gives.
func (e *Engine) extraSpaceOf(f fontFace) int {
	if v, ok := e.fontDimens[f][7]; ok {
		return v
	}
	g := e.spaceGlueOf(f)
	return g.width / 3
}
