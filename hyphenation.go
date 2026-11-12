// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bufio"
	"bytes"
	_ "embed"
	"strings"
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

// newEnglishHyphenator builds a Hyphenator preloaded with the embedded US-English
// patterns. Comment lines (a leading %) and blanks are skipped; every other token
// is one Liang pattern.
func newEnglishHyphenator() *hyphenator {
	h := newHyphenator()
	sc := bufio.NewScanner(bytes.NewReader(enUSPatterns))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
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

// activeHyphenator returns the Hyphenator to use for the current paragraph, or nil
// when the text must not be hyphenated. A document's own \patterns always win. With
// no document patterns, English (\language 0) gets the embedded default set, built
// once and cached; any other \language gets nothing, since no patterns are embedded
// for it and guessing would break at the wrong places. Either way the live
// \lefthyphenmin / \righthyphenmin decide the affix limits, as in real TeX.
func (e *Engine) activeHyphenator() *hyphenator {
	h := e.hyph
	if h == nil {
		if e.namedInt("language") != 0 {
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
// maxHyphenatableLength is TeX's own ceiling on the length of a word it will
// try to hyphenate (tex.web §891). Longer words are set unhyphenated.
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
		// gather a word: chars plus interior kerns that sit between two chars
		var wordNodes []node
		var letters []rune
		j := i
		for j < len(list) {
			if c, ok := list[j].(charNode); ok {
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
		// ⛔ TeX DOES NOT HYPHENATE A WORD LONGER THAN max_hyphenatable_length
		// (tex.web §891: "if l > max_hyphenatable_length then goto done1"), and
		// neither does this — which is faithfulness and, here, also a bound.
		//
		// Liang's algorithm looks up every substring of the word, so its cost is
		// QUADRATIC in the word's length. A document with no space in it hands this
		// one enormous "word": profiled on a macro that expands exponentially,
		// 98.6% of the engine's time was inside Hyphenator.Points, hashing pattern
		// lookups, and the run never ended. A real word is tens of letters; the
		// longest paragraph in the whole fidelity corpus is 8_881 nodes and no word
		// in it comes near this limit.
		if len(letters) <= maxHyphenatableLength {
			for _, t := range h.Points(string(letters)) {
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
