// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// The Liang algorithm itself now lives in github.com/go-tex/hyphenation (see
// libs.go). What stays here is what is TeX-specific: the \patterns primitive
// that loads a pattern file, and the walk that turns a word's break points into
// discretionary nodes in the paragraph list.

// doPatterns handles \patterns{p1 p2 …}: load space-separated Liang patterns.
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
	if e.hyph == nil {
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
		for _, t := range e.hyph.Points(string(letters)) {
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
