// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// folios typesets src with a plain page style and returns the text of each page; the
// body text is "Q", so what follows it is the folio.
func folios(t *testing.T, src string) []string {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.vsize = 200 * unity
	if _, err := e.Run(`\pagestyle{plain}` + src); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range e.Pages() {
		var b strings.Builder
		collectChars(p.list, &b)
		out = append(out, strings.TrimPrefix(b.String(), "Q"))
	}
	return out
}

// \setcounter{page}{5} mid-document: the page it falls on takes 5 and the next one 6.
// tectonic prints 1, 5, 6 for this source (latex.ltx:10816 \countdef\c@page; the
// folio is read when the page ships).
func TestSetcounterPageResetsTheFolio(t *testing.T) {
	got := folios(t, `Q\newpage\setcounter{page}{5}Q\newpage Q`)
	want := []string{"1", "5", "6"}
	if len(got) != len(want) {
		t.Fatalf("%d pages %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("page %d prints %q, want %q (all: %q)", i+1, got[i], want[i], got)
		}
	}
}

// \pagenumbering{roman} then {arabic}: latex.ltx:10818-10820 resets \c@page to 1 and
// swaps \thepage, so the folios read i ii, then 1 2 — the last style must not win for
// the pages before it (it did, when the format was one engine-wide field).
func TestPagenumberingResetsAndKeepsEachStyle(t *testing.T) {
	got := folios(t, `Q\clearpage\pagenumbering{roman}Q\newpage Q\clearpage\pagenumbering{arabic}Q\newpage Q`)
	want := []string{"1", "i", "ii", "1", "2"}
	if len(got) != len(want) {
		t.Fatalf("%d pages %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("page %d prints %q, want %q (all: %q)", i+1, got[i], want[i], got)
		}
	}
}

// A write that opens the list alone (what article.cls leaves: \pagenumbering{arabic}
// before \onecolumn) must neither ship a page of its own nor open a column region.
func TestPageCounterWriteAloneShipsNoPage(t *testing.T) {
	got := folios(t, `\setcounter{page}{7}\onecolumn Q`)
	if len(got) != 1 || got[0] != "7" {
		t.Errorf("pages %q, want one page printing 7", got)
	}
}
