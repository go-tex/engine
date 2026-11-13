// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// What to do when no feasible set of breaks exists.
//
// ⛔⛔ THE OLD ANSWER WAS "ONE LINE, NOTHING LOST", AND IT LOST THE PARAGRAPH.
// `lines = []Line{{Start: 0, End: len(list)}}` keeps every node, so nothing is
// dropped in the sense of content — and the reader gets one line running
// hundreds of points off the page. Measured: a 45pt measure, an indented
// paragraph, and the whole of
//
//	Typographical incomprehensibilities overwhelm every paragraph.
//
// came out as a single line 254.47pt past the margin. Everything was there and
// none of it was readable.
//
// ⛔ AND #766 IS WHAT MADE IT REACHABLE. tex.web §894/§896 hyphenate only after
// a glue, so a paragraph's first word — preceded by the \parindent box — is
// never hyphenated. That is right, and the reference does it. But it removed
// the break this engine had been relying on: with `Typographical` unbreakable
// and only 30pt left on the indented first line, NO feasible solution exists,
// the breaker fails, and the fallback fired.
//
// ⭐ WHAT THE REFERENCE DOES THERE, measured on the same document:
//
//	    Typographical        <- its own line, OVERFULL, overhanging the margin
//	incompre-
//	hensibil-
//	ities over-
//	whelm ev-
//	ery para-
//	graph.
//
// It sets the unbreakable word as an overfull line and carries on normally.
// That is #768's mechanism from the other side: TeX will leave a line overfull,
// and this engine's breaker cannot produce one, so where TeX overhangs we have
// nothing to fall back to.
//
// ⚠ THIS IS A MITIGATION, NOT THAT FIX. Making the breaker able to return an
// overfull line is #768 and lives partly in go-typeset/linebreak. What this
// does is make the failure mode resemble the reference instead of destroying
// the paragraph: break greedily, at the last legal point that fits, and let a
// line that cannot fit overhang alone.

// greedyLines breaks a horizontal list at the last legal point that fits on
// each line, and puts an item that cannot fit anywhere on a line of its own.
//
// It is deliberately NOT Knuth-Plass: this runs only where Knuth-Plass found
// nothing, so there is no optimum to approximate — the job is to produce
// something a reader can read.
func greedyLines(list []node, lineWidthPt float64) []Line {
	target := ptToSP(lineWidthPt)
	var out []Line
	start := 0
	lastFit := -1 // the most recent legal breakpoint that still fitted
	for i := 0; i <= len(list); i++ {
		if i == len(list) {
			if start < i {
				out = append(out, Line{Start: start, End: i})
			}
			break
		}
		w := hpackSP(list[start:i+1], packNatural, 0).width
		if legalGreedyBreak(list, i) && w <= target {
			lastFit = i
		}
		if w > target {
			switch {
			case lastFit > start:
				// ⛔ ONLY A GLUE IS SWALLOWED BY THE BREAK. The first version
				// advanced past the node at lastFit unconditionally, which is
				// right for the space at a word boundary and DROPS A CHARACTER
				// at a hyphenation point — the node there is the letter after
				// the discretionary. It came out as
				//
				//	incom / rehensibil / ties / helm / raph.
				//
				// losing the p, the i, the w and the g: content destroyed, which
				// is worse than the unreadable line this exists to replace.
				out = append(out, Line{Start: start, End: lastFit})
				// The break node itself — the glue that was the word space, or
				// the discretionary whose hyphen the previous line just drew —
				// does not begin the next line.
				start = lastFit + 1
			default:
				// ⛔ NOTHING FITS, SO THE WHOLE UNBREAKABLE RUN OVERHANGS — it is
				// NOT cut at the character where the width ran out. Doing that
				// put `Typogr` and `aphical` on separate lines with no hyphen
				// between them: a break TeX would never make, in the middle of a
				// word, silently presented as if it were one.
				//
				// The reference sets the whole word on one overfull line and
				// carries on, which is what this does: run to the next legal
				// break, or to the end.
				end := len(list)
				for k := i; k < len(list); k++ {
					if legalGreedyBreak(list, k) {
						end = k
						break
					}
				}
				if end <= start {
					end = start + 1
				}
				out = append(out, Line{Start: start, End: end})
				start = end + 1
				if start > len(list) {
					start = len(list)
				}
			}
			lastFit = -1
			i = start - 1 // re-scan from the new line's first item
		}
	}
	if len(out) == 0 && len(list) > 0 {
		out = append(out, Line{Start: 0, End: len(list)})
	}
	return out
}

// legalGreedyBreak reports whether a line may END AT index i — that is, whether
// Line{Start, End: i} is a break this engine knows how to render.
//
// ⛔ THE INDEX POINTS AT THE BREAK NODE, NOT PAST IT, and the first version had
// it one off. paragraph.go:122 emits a hyphen when `list[ln.End]` IS the
// discretionary; with End one further along, the disc fell inside the line and
// no hyphen was drawn. The text came out as
//
//	incom / prehensi / bilities
//
// broken at the right places with the hyphens missing, which reads as a word
// split for no reason.
func legalGreedyBreak(list []node, i int) bool {
	if i <= 0 || i >= len(list) {
		return false
	}
	switch list[i].(type) {
	case glueNode, discNode:
		return true
	}
	return false
}
