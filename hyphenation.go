// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// This file implements Liang's hyphenation algorithm — the method TeX uses to
// find legal break points inside words. Patterns (loaded with \patterns) carry
// odd/even priority digits between letters; for a word, the maximum value at each
// inter-letter position decides whether a break is allowed there (odd = allowed),
// subject to \lefthyphenmin / \righthyphenmin. Allowed points become discretionary
// nodes in the paragraph list; when a line breaks at one, a hyphen is appended.

// hyphenator holds the loaded patterns and the min-affix limits.
type hyphenator struct {
	pat  map[string][]int
	lmin int // \lefthyphenmin: min letters before the first hyphen
	rmin int // \righthyphenmin: min letters after the last hyphen
}

func newHyphenator() *hyphenator {
	return &hyphenator{pat: map[string][]int{}, lmin: 2, rmin: 3}
}

// addPattern parses one Liang pattern (e.g. "a1bc3d" or ".ach4") into its letter
// key and inter-letter value array (length = letters+1).
func (h *hyphenator) addPattern(p string) {
	var letters strings.Builder
	var vals []int
	pending := 0
	haveDigit := false
	for _, r := range p {
		if r >= '0' && r <= '9' {
			pending = int(r - '0')
			haveDigit = true
			continue
		}
		vals = append(vals, pending)
		letters.WriteRune(r)
		pending, haveDigit = 0, false
	}
	vals = append(vals, pending)
	_ = haveDigit
	h.pat[letters.String()] = vals
}

// points returns the break positions in word: each value t means a hyphen is
// allowed after the first t letters (so between word[t-1] and word[t]).
func (h *hyphenator) points(word string) []int {
	w := "." + strings.ToLower(word) + "."
	n := len(w)
	val := make([]int, n+1)
	for i := 0; i < n; i++ {
		for j := i + 1; j <= n; j++ {
			v, ok := h.pat[w[i:j]]
			if !ok {
				continue
			}
			for k := 0; k < len(v); k++ {
				if i+k < len(val) && v[k] > val[i+k] {
					val[i+k] = v[k]
				}
			}
		}
	}
	// The augmented word has a leading '.', so original letter t sits at w[t+1];
	// the break after t letters uses val[t+1].
	L := len([]rune(word))
	var pts []int
	for t := h.lmin; t <= L-h.rmin; t++ {
		if val[t+1]%2 == 1 {
			pts = append(pts, t)
		}
	}
	return pts
}

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
			e.hyph.addPattern(cur.String())
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
		for _, t := range e.hyph.points(string(letters)) {
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
