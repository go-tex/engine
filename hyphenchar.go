// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \hyphenchar<font>: the character TeX inserts at a discretionary break in that
// font, and — when it is NEGATIVE — the instruction not to hyphenate in that
// font at all.
//
// ⛔ tex.web §896 IS A SINGLE LINE AND IT DECIDES WHOLE PARAGRAPHS:
//
//	if hyf_char<0 then goto done1
//
// A font whose \hyphenchar is negative is never hyphenated. That is how TeX and
// LaTeX stop code, identifiers and file names being broken across lines, and it
// is not a nicety: a reader cannot tell a hyphen the typesetter inserted from
// one the identifier contains.
//
// ⭐ MEASURED AGAINST tectonic 0.17.0, the same word at a 72pt measure in two
// faces, the roman one being the control:
//
//	Word incom-            <- roman: hyphenated
//	prehensibilities
//	here.
//	   Word                <- \texttt: NOT hyphenated, the line overruns
//	incomprehensibilities
//	here.
//
// The reference sets it in the font-shape declaration: tulmtt.fd:51 is
// `\hyphenchar \font\m@ne`. This engine has no .fd files, so the Go binding
// where the typewriter face is installed is the same place.
//
// ⚠ WHAT IS NOT DONE HERE. \the\hyphenchar\font is not readable — that is the
// same gap as \the\fontdimen and \fontname, filed as #762 — and
// \defaulthyphenchar is still an accept-and-ignore parameter, so a face with no
// explicit value hyphenates rather than consulting it. Both are deliberate: a
// face that has never been told anything behaves exactly as it did before this
// change, so the only behaviour that moves is the one measured above.

// hyphenCharOf returns the \hyphenchar in force for a face.
//
// ⛔ AN UNSET FACE PERMITS HYPHENATION, and does NOT fall back to
// \defaulthyphenchar. TeX's INITEX default for that parameter is -1 and
// latex.ltx:523 raises it to `\- ; this engine carries the INITEX value in
// texparams.go and nothing raises it, so consulting it would switch
// hyphenation OFF for every face in every document. Reading a parameter that
// is only half-implemented is worse than not reading it: it would look like
// fidelity and silently stop every break in the corpus.
func (e *Engine) hyphenCharOf(f fontFace) int {
	if f == nil {
		return hyphenCharPermitted
	}
	if v, ok := e.hyphenChars[hyphenCharKey(f)]; ok {
		return v
	}
	return hyphenCharPermitted
}

// hyphenCharKey identifies the thing \hyphenchar belongs to.
//
// ⛔ THE FONT PROGRAM, NOT THE FACE AT A SIZE. A face is re-made whenever the
// size changes — \small inside \texttt produces a different *OpenTypeFont over
// the same file — so keying on the face would lose the setting at every size
// switch. Keyed on the face, `\hyphenchar\font=-1` then read back 45 in a test
// here, because the document's current face was no longer the object the
// binding had been made on.
//
// The reference reaches the same place from the other side: tulmtt.fd sets
// `\hyphenchar\font\m@ne` in the loading hook of EVERY size of the family, so
// the value follows the family however the size moves. Keying on the program
// is that, expressed once.
//
// A face with no identifiable program (a test mock) keys on itself, which is
// the old behaviour and is right for something that cannot be re-faced.
func hyphenCharKey(f fontFace) any {
	if p, ok := f.(faceProgram); ok {
		return p.faceProgram()
	}
	return f
}

// hyphenCharPermitted is the value an unset face reports: TeX's own `\-, which
// is what latex.ltx:523 puts in \defaulthyphenchar.
const hyphenCharPermitted = '-'

// setHyphenChar records \hyphenchar<font>=<number> for a face.
func (e *Engine) setHyphenChar(f fontFace, v int) {
	if f == nil {
		return
	}
	if e.hyphenChars == nil {
		e.hyphenChars = map[any]int{}
	}
	e.hyphenChars[hyphenCharKey(f)] = v
}

// doHyphenChar implements the \hyphenchar primitive: \hyphenchar<font>=<number>.
//
// The font is read the way TeX reads one (tex.web §577, scan_font_ident): a
// control sequence whose meaning is a font, or \font for the current one.
func (e *Engine) doHyphenChar() {
	f, ok := e.scanFontIdent()
	e.scanEquals()
	v := e.scanInt()
	if !ok {
		return // a font we could not resolve: the value is consumed, nothing is set
	}
	e.setHyphenChar(f, v)
}

// scanFontIdent reads a font reference: \font (the current face) or a control
// sequence bound to one by \newfont / the engine's own family bindings.
func (e *Engine) scanFontIdent() (fontFace, bool) {
	t, ok := e.getXToken()
	if !ok {
		return nil, false
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil {
			if m.kind == mPrim && m.name == "font" {
				return e.curFont, e.curFont != nil
			}
			if m.kind == mFont && m.font != nil {
				return m.font, true
			}
		}
	}
	e.back(t)
	return nil, false
}

// wordIsHyphenatable reports whether the characters gathered for a word may be
// hyphenated at all, which is a property of their FONT.
//
// ⚠ IT ASKS THE FIRST CHARACTER'S FACE. tex.web §896 reads hyf_char from the
// font of the first letter it scans and does not re-read it mid-word, because a
// word whose font changes in the middle is not one word to TeX either (§891:
// "characters that do not have the same font as c1 are nonletters"). This
// engine does not yet split a word at a font change — a separate gap, recorded
// in #765 — so asking the first face is both what TeX does and the best this
// can do until that one is closed.
func (e *Engine) wordIsHyphenatable(wordNodes []node) bool {
	for _, n := range wordNodes {
		if c, ok := n.(charNode); ok {
			return e.hyphenCharOf(c.face) >= 0
		}
	}
	return true
}
