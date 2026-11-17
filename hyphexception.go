// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// \hyphenation{as-so-ciate as-so-ciates} — the exception dictionary.
//
// ⛔ IT WAS NOT IMPLEMENTED AT ALL, and a document that wrote it got the
// patterns' answer instead of its own. The word a author spells out is
// precisely the one the patterns get wrong — that is why they wrote it down —
// so the silence fell on exactly the cases it mattered for. 170 of the 2935
// corpus papers use \hyphenation (5.8%), and it is one of the gaps #765 names.
//
// # What TeX does
//
// tex.web §934-940: the exception dictionary is consulted FIRST, and a word
// found there hyphenates ONLY where its entry says, never where the patterns
// would. §937 is explicit that the patterns are not applied to an exception:
// the entry replaces them rather than adding to them. The Appendix H rule a
// reader meets is the same one — "\hyphenation gives the complete list of
// permissible breaks".
//
// \lefthyphenmin and \righthyphenmin still apply, as they do to any word: an
// exception says where a break MAY go, not that one must.
type hyphExceptions map[string][]int

// add records one \hyphenation entry: as-so-ciate becomes the key associate
// with breaks after 2 and 4 letters.
//
// ⚠ THE KEY IS THE WORD WITHOUT ITS HYPHENS, lowercased. TeX lowercases through
// \lccode, which this engine does not yet track (#765 names that separately);
// strings.ToLower is what it uses instead, and the difference shows for an
// alphabet whose case mapping a document has redefined. Said here rather than
// left for a reader to discover.
func (x hyphExceptions) add(entry string) {
	var word strings.Builder
	var breaks []int
	letters := 0
	for _, r := range entry {
		if r == '-' {
			// a break AFTER the letters seen so far; a leading or doubled
			// hyphen would record a break after none, which TeX cannot use and
			// which lefthyphenmin refuses anyway
			if letters > 0 {
				breaks = append(breaks, letters)
			}
			continue
		}
		word.WriteRune(r)
		letters++
	}
	if word.Len() == 0 {
		return
	}
	// ⛔ AN ENTRY WITH NO HYPHEN IS NOT NOTHING: `\hyphenation{photography}`
	// says the word may NOT be broken at all, and recording an empty slice is
	// what makes the lookup below say so. Skipping it would hand the word back
	// to the patterns — the opposite of what was asked.
	if breaks == nil {
		breaks = []int{}
	}
	x[strings.ToLower(word.String())] = breaks
}

// clipToMins drops the breaks \lefthyphenmin and \righthyphenmin refuse from an
// exception's list: n is the word's length in letters, and a break after t
// letters survives only while lmin <= t <= n-rmin.
//
// ⛔⛔ IT IS THE SAME RULE THE PATTERNS GET, AND tex.web APPLIES IT IN ONE PLACE
// FOR BOTH. :18145-18160 is a single block: the exception table is consulted
// first and a hit does `goto found` — the SAME label the pattern scan falls
// through to — and `found` is where the limits are imposed:
//
//	found: for j:=0 to l_hyf-1 do hyf[j]:=0;
//	       for j:=0 to r_hyf-1 do hyf[hn-j]:=0
//
// The header above has said so since it was written. The code did not: lookup
// returned the entry's breaks and the caller wrote them straight into its break
// set, so an exception escaped the minima. ⚠ The comment was RIGHT and the code
// was WRONG, which is the harder way round to find — a reader checking the rule
// meets it stated correctly and moves on.
//
// ⛔ The caller applies this when it HYPHENATES, not when the entry is recorded.
// A document may write \hyphenation{…} in a package and set \lefthyphenmin
// afterwards, or change it per language mid-document; clipping at `add` time
// would freeze whichever value happened to be in force when the entry was read.
func clipToMins(pts []int, n, lmin, rmin int) []int {
	out := pts[:0:0] // never alias the stored entry: it is reused for every occurrence
	for _, t := range pts {
		if t < lmin || t > n-rmin {
			continue
		}
		out = append(out, t)
	}
	return out
}

// lookup returns the breaks an exception prescribes, and whether the word is
// one. The bool is what distinguishes "no breaks allowed" from "not an
// exception"; a nil slice would conflate them.
func (x hyphExceptions) lookup(word string) ([]int, bool) {
	if x == nil {
		return nil, false
	}
	b, ok := x[strings.ToLower(word)]
	return b, ok
}

// doHyphenation handles \hyphenation{w1 w2 …}, in the shape doPatterns reads
// \patterns{…}: a braced, space-separated list, taken as characters rather
// than as tokens so that a control sequence inside it is skipped rather than
// expanded into the word.
func (e *Engine) doHyphenation() {
	if e.hyphExcept == nil {
		e.hyphExcept = hyphExceptions{}
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
			e.hyphExcept.add(cur.String())
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
