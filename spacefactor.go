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

// nonZeroSkip reads a glue parameter and reports whether it is SET, which is
// TeX's `<> zero_glue` test (tex.web:20328-20330): a parameter all of whose
// three components are zero is unset, and the caller falls back.
//
// ⛔ It is not "width > 0". A \spaceskip of `0pt plus 2pt` IS set — it is a space
// that can only grow — and a width test would throw it away. The float skips had
// the same conflation in the other direction (#815).
func (e *Engine) nonZeroSkip(name string) (glueSpec, bool) {
	g := e.namedSkip(name)
	if g.width == 0 && g.stretch == 0 && g.shrink == 0 {
		return glueSpec{}, false
	}
	return g, true
}

// spaceGlueForFactor is tex.web's app_space (§1043): the interword glue once the
// space factor is taken into account.
//
//	if space_factor >= 2000:  width += extra_space
//	stretch = stretch × space_factor / 1000
//	shrink  = shrink  × 1000 / space_factor
//
// ⛔⛔ AND BOTH HALVES OF §1043 ARE HERE NOW, because half of it is worse than
// none — which is what the note this replaces said, and it was right while only
// half could be written. tex.web:20358-20370, verbatim:
//
//	if (space_factor>=2000)and(xspace_skip<>zero_glue) then
//	  q:=new_param_glue(xspace_skip_code)
//	else begin if space_skip<>zero_glue then main_p:=space_skip
//	  else <Find the glue specification...>;
//	  main_p:=new_spec(main_p);
//	  <Modify the glue specification in main_p according to the space factor>;
//
// Two rules, and they are NOT the same rule:
//
//   - \xspaceskip is taken VERBATIM — whole, unscaled, and only when the factor
//     is at least 2000. No \fontdimen7 is added to it and the factor does not
//     touch its stretch or shrink.
//   - \spaceskip replaces the FONT's space as the base, and is then scaled by
//     the factor exactly as the font's would have been, extra_space included
//     (§1044 adds extra_space(cur_font) whatever the base is).
//
// ⛔ ZERO IS "UNSET", NOT A WIDTH. :20328-20330 says TeX compares the glue
// parameter against zero_glue by POINTER — "it is simple to test whether a glue
// parameter is zero or not" — so a \spaceskip of 0pt plus 0pt minus 0pt means
// "nobody set this" and the font's space is used. Reading a zero as a deliberate
// zero-width space is the same conflation #815 found in the float skips, in the
// other direction.
//
// ⚠ MEASURED COST OF THE SILENCE, before this existed: a line-breaking fixture
// that set \spaceskip=4pt plus 8pt minus 2pt was handed the font's 3pt plus 1.5,
// and eighteen configurations of it said \looseness had no effect. It has one.
// See #826.
func (e *Engine) spaceGlueForFactor(f fontFace) glueSpec {
	sf := e.spaceFactor
	// The \xspaceskip branch is tested FIRST and returns at once: it is not a
	// base to be modified.
	if sf >= 2000 {
		if x, ok := e.nonZeroSkip("xspaceskip"); ok {
			return x
		}
	}
	g, ok := e.nonZeroSkip("spaceskip")
	if !ok {
		g = e.spaceGlueOf(f)
	}
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
