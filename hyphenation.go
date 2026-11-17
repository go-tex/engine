// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bufio"
	"bytes"
	_ "embed"
	"strings"
	"unicode"
)

// The Liang algorithm itself now lives in github.com/go-typeset/hyphenation (see
// libs.go). What stays here is what is TeX-specific: the \patterns primitive
// that loads a pattern file, the default US-English pattern set the engine ships
// so English text hyphenates out of the box, and the walk that turns a word's
// break points into discretionary nodes in the paragraph list.

// enUSPatterns holds the standard American-English Liang patterns
// (hyph-en-us / ushyphmax.tex), one per line, with a preserved licence header.
// It is embedded so hyphenation needs no runtime file access.
//
//go:embed hyph_en_us_patterns.txt
var enUSPatterns []byte

// extraPatternsMarker is the line ushyphmax.tex puts between the two halves of
// this file. Everything before it is hyphen.tex — Knuth's patterns, which
// hyph-en-us.tex ships and which LaTeX loads for \language0. Everything after it
// is ushyphmax's own additions, which a LaTeX document does NOT get.
const extraPatternsMarker = "% hyphen.tex patterns end here"

// newEnglishHyphenator builds a Hyphenator preloaded with the embedded US-English
// patterns. Comment lines (a leading %) and blanks are skipped; every other token
// is one Liang pattern.
//
// ⛔⛔ AND IT STOPS AT THE MARKER, BECAUSE THE REFERENCE'S SET STOPS THERE.
//
// The file is ushyphmax.tex: hyphen.tex's 4447 patterns, then 492 more from the
// US word list. LaTeX's language.dat loads hyph-en-us.tex, which is the FIRST
// half only, so loading all of it hyphenated differently from the reference
// wherever one of the extra patterns applies — and an extra pattern wins by
// Liang's rule whenever its value is higher, including when it is EVEN and
// therefore FORBIDS a break the first half allows.
//
// Measured, not reasoned. `every` on tectonic 0.17.0 at \hsize=18pt:
//
//	reference   ev-/ery
//	here        every, overhanging the margin by 5pt
//
// because `.ev2` (an addition) outranks `ev1er` (hyphen.tex) at the same
// position and 2 is even. Loading only the first half gives `ev-ery` and leaves
// `incomprehensibilities`, `overwhelm`, `paragraph`, `photography`,
// `typographical` and `democracy` at exactly the points they already had.
//
// It was the last divergence on #768's witness: with it, our five lines are the
// reference's five.
//
// ⚠ The extra patterns are KEPT IN THE FILE, after the marker, rather than
// deleted: the file is a verbatim extract and its provenance is the licence
// header. What changes is which half is loaded.
func newEnglishHyphenator() *hyphenator {
	h := newHyphenator()
	sc := bufio.NewScanner(bytes.NewReader(enUSPatterns))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, extraPatternsMarker) {
			break
		}
		if line == "" || strings.HasPrefix(line, "%") {
			continue
		}
		for _, p := range strings.Fields(line) {
			h.AddPattern(p)
		}
	}
	return h
}

// namedInt reads a TeX named integer parameter (\language, \lefthyphenmin, …)
// from the count register it was bound to, falling back to plain TeX's default if
// it is somehow unbound.
func (e *Engine) namedInt(name string) int {
	if m := e.eq[name]; m != nil && m.kind == mCountRef && m.code >= 0 && m.code < len(e.count) {
		return e.count[m.code]
	}
	return texIntParams[name]
}

// curLang maps \language to TeX's internal cur_lang, which is NOT the same
// number. tex.web:18261-18263 sends every value outside 0..255 to language
// ZERO rather than treating it as a language of its own:
//
//	@d set_cur_lang==if language<=0 then cur_lang:=0
//	  else if language>255 then cur_lang:=0
//	  else cur_lang:=language
//
// So \language=-1 and \language=300 select English here and ARE hyphenated,
// where reading the register directly hyphenated neither.
//
// ⚠ An in-range \language is a different question and deliberately unchanged:
// \language=5 names patterns this engine embeds none of, and returning nil
// there is a choice — hyphenating one language with another's patterns breaks
// words where no reader would accept it — not this defect.
func (e *Engine) curLang() int {
	if lang := e.namedInt("language"); lang > 0 && lang <= 255 {
		return lang
	}
	return 0
}

// activeHyphenator returns the Hyphenator to use for the current paragraph, or nil
// when the text must not be hyphenated. A document's own \patterns always win. With
// no document patterns, English (cur_lang 0) gets the embedded default set, built
// once and cached; any other language gets nothing, since no patterns are embedded
// for it and guessing would break at the wrong places. Either way the live
// \lefthyphenmin / \righthyphenmin decide the affix limits, as in real TeX.
func (e *Engine) activeHyphenator() *hyphenator {
	h := e.hyph
	if h == nil {
		if e.curLang() != 0 {
			return nil
		}
		if e.enHyph == nil {
			e.enHyph = newEnglishHyphenator()
		}
		h = e.enHyph
	}
	h.SetMins(e.namedInt("lefthyphenmin"), e.namedInt("righthyphenmin"))
	return h
}

// doPatterns handles \patterns{p1 p2 …}: load space-separated Liang patterns into
// the document's own hyphenator, which then takes precedence over the default set.
func (e *Engine) doPatterns() {
	if e.hyph == nil {
		e.hyph = newHyphenator()
	}
	e.skipOptSpace()
	if t, ok := e.getXToken(); !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return
	}
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			e.hyph.AddPattern(cur.String())
			cur.Reset()
		}
	}
	for {
		t, ok := e.getNext()
		if !ok || (t.cat == catEnd && !t.cs_) {
			break
		}
		if t.cat == catSpace && !t.cs_ {
			flush()
			continue
		}
		if !t.cs_ {
			cur.WriteRune(t.ch)
		}
	}
	flush()
}

// hyphenPenalty is what a discretionary hyphen costs. It reads the REGISTER, so a
// document that writes \hyphenpenalty=10000 to suppress hyphenation is obeyed —
// the field alone could only ever hold the default, since \hyphenpenalty resolves
// to a \newcount here and an assignment goes to the register.
func (e *Engine) hyphenPenalty() int {
	if m := e.eq["hyphenpenalty"]; m != nil && m.kind == mCountRef {
		return e.count[m.code]
	}
	return e.hyphenpenalty
}

// hyphenateList returns a copy of a paragraph's horizontal list with discretionary
// hyphen nodes inserted at every legal break inside each word. A word is a maximal
// run of characters (font kerns between them are kept as interior material).
// maxHyphenatableLength is the capacity of TeX's own hu/hc buffers, which hold
// the word being hyphenated (tex.web:17602). A longer word is hyphenated in its
// first 63 letters, not left unhyphenated: see the citation at the use site.
const maxHyphenatableLength = 63

// precededByGlue reports whether the character at i begins a word TeX would
// consider for hyphenation: one reached from a glue node.
//
// ⚠ IT STEPS OVER KERNS AND WHATSITS, which is what §896-§899 do on the way
// from the glue to the first letter — a font kern or a \write between the space
// and the word does not make the word unhyphenatable. It does NOT step over a
// box, maths or a discretionary: those are exactly the cases §891:17476 names
// as ending the scan, and stepping over them would reintroduce the defect this
// exists to remove.
func precededByGlue(list []node, i int) bool {
	for k := i - 1; k >= 0; k-- {
		switch list[k].(type) {
		case glueNode:
			return true
		case kernNode, specialNode:
			continue // transparent on the way from the glue to the letter
		default:
			return false
		}
	}
	return false // nothing precedes it: the start of the list is not a glue
}

// lcCodeOf is \lccode, as the hyphenation pass asks it: tex.web:17601 is
//
//	if lc_code(c)=0 then goto done3
//
// so a character whose \lccode is zero is NOT a letter and ends the word being
// gathered (§891:17480-17483 defines a letter the same way). This engine gathered
// every maximal run of charNodes instead, which puts the punctuation INSIDE the
// word: "word," was hyphenated as a five-letter word and matched patterns against
// a string no dictionary contains. That is most words in most paragraphs, not an
// edge case.
//
// ⚠ A DELIBERATE DIVERGENCE FROM 8-BIT tex.web, STATED. INITEX gives \lccode only
// to a-z and A-Z, so `é` would be a non-letter and every French word would be cut
// at its accents — worse than the defect being fixed. XeTeX and LuaTeX, which is
// what tectonic is, give every Unicode letter an \lccode, and that is what this
// follows: an explicit table entry wins, else a Unicode letter is its own
// lowercase, else zero. go-tex/engine#765 item (3).
//
// ⚠ NOT DONE HERE: feeding the hyphenator the \lccode-LOWERED letters. The
// library lowercases with strings.ToLower, so a document that remaps \lccode to
// make one character hyphenate as another is still unobeyed. The letter TEST is
// what moves ordinary text; that mapping is a separate change.
func (e *Engine) lcCodeOf(r rune) int {
	if v, ok := e.lccode[r]; ok {
		return v
	}
	if unicode.IsLetter(r) {
		return int(unicode.ToLower(r))
	}
	return 0
}

// gatherWord collects the word that starts at list[i]: its characters, plus the
// interior kerns that sit between two of them, and returns the index one past it.
//
// ⛔ A CHARACTER WHOSE \lccode IS ZERO IS NOT A LETTER AND ENDS THE WORD
// (tex.web:17601, `if lc_code(c)=0 then goto done3`). That is what keeps the
// comma out of "word," and the apostrophe out of "don't"; see lcCodeOf for the
// one place the rule is decided and for the Unicode divergence it states.
//
// It is a function of its own because the rule is worth testing directly: the
// obvious page-level witness for it does not discriminate (lccodeletter_test.go
// says why), so what has to be asserted is where the word ends.
func (e *Engine) gatherWord(list []node, i int) (wordNodes []node, letters []rune, j int) {
	for j = i; j < len(list); {
		if c, ok := list[j].(charNode); ok {
			if e.lcCodeOf(c.ch) == 0 {
				break
			}
			letters = append(letters, c.ch)
			wordNodes = append(wordNodes, list[j])
			j++
			continue
		}
		if _, ok := list[j].(kernNode); ok && j+1 < len(list) {
			if _, ok2 := list[j+1].(charNode); ok2 {
				wordNodes = append(wordNodes, list[j])
				j++
				continue
			}
		}
		break
	}
	return wordNodes, letters, j
}

func (e *Engine) hyphenateList(list []node) []node {
	h := e.activeHyphenator()
	if h == nil {
		return list
	}
	out := make([]node, 0, len(list))
	i := 0
	for i < len(list) {
		if _, ok := list[i].(charNode); !ok {
			out = append(out, list[i])
			i++
			continue
		}
		// ⛔ A WORD IS HYPHENATED ONLY WHERE A GLUE PRECEDES IT. tex.web §894 and
		// §896 start the hyphenation scan at a glue node: "cur_p points to a glue
		// node" is the precondition, not a detail of how the loop is written.
		//
		// This engine hyphenated every maximal run of characters whatever sat on
		// either side, which differs from the reference in two ways that both move
		// line counts:
		//
		//   * A PARAGRAPH'S FIRST WORD IS NEVER HYPHENATED BY TeX, because what
		//     precedes it is the \parindent box. We hyphenated it, on every
		//     paragraph of every document.
		//   * Nor is a word after a box, after maths, or after a user's own
		//     discretionary (§891:17476). We hyphenated those too.
		//
		// The list's very start is the same case: nothing precedes it, so nothing
		// is glue, so TeX would not hyphenate there either.
		if !precededByGlue(list, i) {
			out = append(out, list[i])
			i++
			continue
		}
		wordNodes, letters, j := e.gatherWord(list, i)
		// ⛔⛔ A NON-LETTER WHERE A WORD WAS EXPECTED IS NOT A WORD, AND SAYING
		// NOTHING HERE HANGS THE LOOP. The gather above now stops BEFORE the first
		// character when its \lccode is zero, so wordNodes is empty and j is still
		// i — and `i = j` below is then `i = i`. The whole suite went from 15 s to
		// a 600 s timeout on exactly that, which is the shape a liveness defect
		// takes when every other test still passes.
		if len(wordNodes) == 0 {
			out = append(out, list[i])
			i++
			continue
		}
		breakAfter := map[int]bool{}
		// ⛔ A FONT WHOSE \hyphenchar IS NEGATIVE IS NEVER HYPHENATED (tex.web
		// §896: `if hyf_char<0 then goto done1`). tulmtt.fd:51 sets exactly that
		// for the typewriter family, which is how the reference stops code and
		// identifiers being broken across lines — measured: the same word at a
		// 72pt measure breaks in the roman face and does not in \texttt.
		//
		// The word is still EMITTED below; only its break points are withheld.
		if !e.wordIsHyphenatable(wordNodes) {
			out = append(out, wordNodes...)
			i = j
			continue
		}
		// ⛔ THIS USED TO CITE A LINE tex.web DOES NOT CONTAIN. It read: «TeX does
		// not hyphenate a word longer than max_hyphenatable_length (tex.web §891:
		// "if l > max_hyphenatable_length then goto done1")». The identifier
		// `max_hyphenatable_length` does not occur ANYWHERE in tex.web — grepped
		// over all 25010 lines, with `hn=63` at :17602 and 44 hits for
		// "hyphenate" as positive controls — so that quote came from some other
		// engine and was attributed to Knuth. It is also the opposite of what
		// :17602 does; see the citation at the use site below.
		//
		// What remains true, and is the REASON for the ceiling rather than its
		// authority: Liang's algorithm looks up every substring of the word, so
		// its cost is
		// QUADRATIC in the word's length. A document with no space in it hands this
		// one enormous "word": profiled on a macro that expands exponentially,
		// 98.6% of the engine's time was inside Hyphenator.Points, hashing pattern
		// lookups, and the run never ended. A real word is tens of letters; the
		// longest paragraph in the whole fidelity corpus is 8_881 nodes and no word
		// in it comes near this limit.
		// ⛔ AN EXCEPTION REPLACES THE PATTERNS, IT DOES NOT ADD TO THEM
		// (tex.web §937). A word in \hyphenation{…} breaks only where its entry
		// says — including nowhere, for an entry written with no hyphen at all
		// — and the patterns are never consulted for it. Consulting both would
		// hand a document that spelled a word out the very breaks it wrote the
		// entry to prevent.
		if pts, isException := e.hyphExcept.lookup(string(letters)); isException {
			// ⛔⛔ AND THE MINIMA APPLY TO AN EXCEPTION TOO. tex.web:18145-18160 is
			// one block: the exception table is consulted first and a hit does
			// `goto found` — to the SAME label the pattern scan falls through to,
			// and `found` is where the limits are imposed:
			//
			//	found: for j:=0 to l_hyf-1 do hyf[j]:=0;
			//	       for j:=0 to r_hyf-1 do hyf[hn-j]:=0
			//
			// So \hyphenation{} says where a break MAY go, not that one must, and
			// \lefthyphenmin/\righthyphenmin still refuse the ones too close to
			// either end. hyphexception.go's own header has said exactly this
			// since it was written; the code below it did not do it, and the
			// breaks went straight into breakAfter. The comment was right and the
			// code was wrong, which is the harder way round to notice.
			//
			// ⛔ CLIPPED HERE AND NOT WHEN THE ENTRY IS RECORDED, because TeX
			// imposes them when it HYPHENATES. A document may write
			// \hyphenation{…} in a package and set \lefthyphenmin later, or change
			// it per language mid-document; clipping at `add` time would freeze
			// whichever value happened to be in force when the entry was read.
			lmin, rmin := h.Mins()
			for _, t := range clipToMins(pts, len(letters), lmin, rmin) {
				breakAfter[t] = true
			}
		} else {
			// ⛔ TeX TRUNCATES, it does not refuse. tex.web:17602's
			// `if hn=63 then goto done3` stops COLLECTING letters at 63;
			// done3 (:17615) is that loop's normal exit — the same label a
			// font change or a non-letter reaches — and `hyphenate` runs
			// right after it (:17550-17552). The only length refusal is
			// :17637, `if hn<l_hyf+r_hyf then goto done1`, for a word too
			// SHORT. :17548 settles it: `if l_hyf+r_hyf>63 then goto done1`
			// refuses only when the MINIMUMS alone exceed 63, which makes 63
			// a buffer capacity and not a policy against long words.
			//
			// So a 110-letter word is hyphenated in its first 63 letters, and
			// \righthyphenmin counts back from the PREFIX rather than from the
			// real end of the word — TeX will break inside a long word.
			//
			// ⭐ The DoS bound this ceiling was added for is untouched: Points
			// still sees at most 63 runes, so Liang's quadratic cost stays
			// O(63²) per word however long the word is.
			word := letters
			if len(word) > maxHyphenatableLength {
				word = word[:maxHyphenatableLength]
			}
			for _, t := range h.Points(string(word)) {
				breakAfter[t] = true
			}
		}
		seen := 0
		for _, wn := range wordNodes {
			out = append(out, wn)
			if _, ok := wn.(charNode); ok {
				seen++
				if breakAfter[seen] {
					out = append(out, discNode{penalty: e.hyphenPenalty(), pre: "-"})
				}
			}
		}
		i = j
	}
	return out
}
