// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \cleardoublepage was undefined, so lenient mode skipped it and a book's parts
// and chapters did not start a new page AT ALL — 23 missing breaks in one
// 333-page thesis of the corpus, which came out 19 pages short of its reference.
//
// LaTeX's is \clearpage plus, in TWOSIDE, a blank page when the next one would be
// even. The engine decides page numbers in the page builder and not in the mouth,
// so \ifodd\c@page cannot be asked from a macro; the blank page is omitted and the
// command reduces to \clearpage. Against tectonic on the same three-word document:
//
//	[oneside]{book}   reference 3 pages, ours 3   exact
//	{book} (twoside)  reference 5 pages, ours 3   the two blanks are what is missing
func TestCleardoublepageStartsAPage(t *testing.T) {
	src := `\documentclass[oneside]{book}\begin{document}` +
		`Un.\cleardoublepage Deux.\cleardoublepage Trois.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(e.Pages()); got != 3 {
		t.Errorf("%d pages, want 3 — the reference sets this document in 3", got)
	}
	if n := e.skippedCS["cleardoublepage"]; n != 0 {
		t.Errorf("\\cleardoublepage was skipped %d times; it is defined now", n)
	}
}
