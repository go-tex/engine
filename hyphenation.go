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

// hyphenateList returns a copy of a paragraph's horizontal list with discretionary
// hyphen nodes inserted at every legal break inside each word. A word is a maximal
// run of characters (font kerns between them are kept as interior material).
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
		for _, t := range h.Points(string(letters)) {
			breakAfter[t] = true
		}
		seen := 0
		for _, wn := range wordNodes {
			out = append(out, wn)
			if _, ok := wn.(charNode); ok {
				seen++
				if breakAfter[seen] {
					out = append(out, discNode{penalty: e.hyphenpenalty})
				}
			}
		}
		i = j
	}
	return out
}
