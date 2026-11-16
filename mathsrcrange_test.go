// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"strings"
	"testing"
)

// ⛔ A LINE CANNOT BE EDITED.
//
// #750 gave a formula the line it opened on, which is what a click-to-scroll
// needs. It is not what EDITING an equation needs: "which characters are this
// formula" has no answer in a line number, and two formulas on one line are
// indistinguishable by it. go-tex/go-tex.github.io#235 asks for the second
// thing and this is its producer half.
//
// Columns are 0-based and counted in RUNES, which is what both ends already
// speak — the engine reads the document as []rune and the editor addresses its
// caret as "the 0-based rune column on its line". The question "rune or byte?"
// was open in #750 and is closed by reading the two declarations.

const twoOnOneLine = `\documentclass{article}
\begin{document}
Mass $E=mc^2$ and energy $e=h\nu$ on one line.
\end{document}`

// mathRanges walks the built document and returns each formula's source range,
// keyed by its source text.
func mathRanges(t *testing.T, src string) map[string][4]int {
	t.Helper()
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][4]int{}
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case mathNode:
				out[c.src] = [4]int{c.srcLine, c.srcCol, c.srcEndLine, c.srcEndCol}
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	return out
}

// ⛔ THE TEST THE LINE STAMP CANNOT PASS. Two formulas on one line share a
// line number exactly; if the range were still line-only, both would carry the
// same stamp and neither could be selected. Their COLUMNS must differ, and the
// second must start after the first has ended.
func TestTwoFormulasOnOneLineAreToldApart(t *testing.T) {
	got := mathRanges(t, twoOnOneLine)
	if len(got) != 2 {
		t.Fatalf("expected two formulas, got %d: %v", len(got), got)
	}
	var ranges [][4]int
	for _, r := range got {
		if r[0] != 3 || r[2] != 3 {
			t.Errorf("a formula on line 3 is stamped %v", r)
		}
		ranges = append(ranges, r)
	}
	a, b := ranges[0], ranges[1]
	if a[1] > b[1] {
		a, b = b, a
	}
	if a[1] == b[1] {
		t.Fatalf("both formulas start at the same column: %v and %v", a, b)
	}
	// ⛔ And they do not overlap: the second opens after the first closes, or
	// a consumer selecting one would swallow the other.
	if b[1] <= a[3] {
		t.Errorf("the second formula opens at column %d, which is not past the first's close at %d", b[1], a[3])
	}
}

// The range's two ends are the DELIMITERS' own positions, and the body lies
// strictly between them. Stated against the document's real columns rather
// than "nonzero", because a stamp that was always 0 would pass that.
func TestTheRangeIsTheDelimitersOwnPositions(t *testing.T) {
	got := mathRanges(t, twoOnOneLine)
	// Line 3 is `Mass $E=mc^2$ and energy $e=h\nu$ on one line.`
	//            0123456789...
	// The first `$` is at column 5 and its closing `$` at column 12.
	want := [4]int{3, 5, 3, 12}
	found := false
	for _, r := range got {
		if r == want {
			found = true
		}
	}
	if !found {
		t.Errorf("no formula is stamped %v (the first `$` at column 5, its close at 12); got %v", want, got)
	}
}

// ⛔⛔ THE TEST #750 COULD NOT FAIL, AND THE DEFECT IT HID.
//
// #750 stamped the catMath branch of the stomach's main loop, with a comment
// promising that "a display spanning several lines belongs to the one the
// author would go looking for it at, not to its last". A probe placed in that
// branch never fires: `$…$` reaches the maths layer through doMath and `\[…\]`
// through doDelimitedMath, and neither goes past it. What made #750's tests
// pass was makeMath's default — srcLine read AFTER the scan — which equals the
// opening line for a formula on ONE line, and every fixture in that change was
// one line.
//
// Measured before this fix: a display opening on line 5 and closing on line 8
// was stamped line 8. The comment described behaviour the code did not have,
// and no fixture crossed a line boundary to notice.
func TestAMultiLineDisplayCarriesTheLineItOpenedOn(t *testing.T) {
	const doc = `\documentclass{article}
\begin{document}
Before.

\[
  a^2 + b^2
  = c^2
\]

After.
\end{document}`
	got := mathRanges(t, doc)
	if len(got) != 1 {
		t.Fatalf("expected one formula, got %d: %v", len(got), got)
	}
	for src, r := range got {
		// `\[` opens on line 5; `\]` closes on line 8.
		if r[0] != 5 {
			t.Errorf("formula %q is stamped as opening on line %d, want 5 (it ends on 8)", src, r[0])
		}
		if r[2] != 8 {
			t.Errorf("formula %q is stamped as closing on line %d, want 8", src, r[2])
		}
	}
}

// ⛔ And the rendered page, which is what the playground reads. A formula's
// group carries data-m beside data-l.
func TestTheRenderedFormulaCarriesItsRange(t *testing.T) {
	pages, err := CompileToSVGPages([]byte(twoOnOneLine), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
	re := regexp.MustCompile(`data-m="(\d+):(\d+)-(\d+):(\d+)"`)
	ms := re.FindAllStringSubmatch(pages[0], -1)
	if len(ms) != 2 {
		t.Fatalf("expected two data-m attributes on the page, got %d:\n%s", len(ms), excerpt(pages[0]))
	}
	// Each one sits on a group that also carries a line, so a consumer reading
	// either attribute finds the other.
	if n := strings.Count(pages[0], `data-l=`); n < 2 {
		t.Errorf("only %d data-l groups: data-m must not replace the line stamp", n)
	}
}

// ⛔ A formula with no stamp carries NO attribute, not a zero one. "0:0-0:0" is
// a well-formed position and a consumer would act on it.
func TestAnUnstampedFormulaCarriesNoRangeAttribute(t *testing.T) {
	if got := mathRangeAttr(mathNode{}); got != "" {
		t.Errorf("an unstamped formula rendered %q, want nothing", got)
	}
	if got := mathRangeAttr(mathNode{srcLine: 4, srcCol: 2}); got != "" {
		t.Errorf("a half-stamped formula rendered %q, want nothing", got)
	}
	if got := mathRangeAttr(mathNode{srcLine: 4, srcCol: 2, srcEndLine: 4, srcEndCol: 9}); got != ` data-m="4:2-4:9"` {
		t.Errorf("a stamped formula rendered %q", got)
	}
}

func excerpt(s string) string {
	if len(s) > 600 {
		return s[:600] + "…"
	}
	return s
}
