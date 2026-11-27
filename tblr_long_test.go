// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"reflect"
	"strings"
	"testing"
)

func runLongtblr(t *testing.T, pre, outer, inner string, rows int) (*Engine, string, int) {
	t.Helper()
	src := `\documentclass{article}
\usepackage{tabularray}
\begin{document}
` + pre + `
\begin{longtblr}` + outer + `{` + inner + `}
HEADCELL & HDX\\
` + tblrRows(rows) + `\end{longtblr}
\end{document}`
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return e, pageText(e), len(e.Pages())
}

// A longtblr with rowhead=1 repeats its first row at the top of every page and
// draws "Continued on next page" at the foot of every page but the last
// (tabularray.sty:3035, 4750, 4761-4772, 5579-5593). Before, the header row was
// set once and the foot never: 2407.18384's Notation table took 4 pages where the
// reference has 5.
func TestLongtblrRepeatsRowheadAndContinues(t *testing.T) {
	_, txt, n := runLongtblr(t, "", "", `colspec = {ll}, rowhead = 1`, 90)
	if n < 3 {
		t.Fatalf("pages = %d, want at least 3 for the witness", n)
	}
	if got := strings.Count(txt, "HEADCELL"); got != n {
		t.Errorf("header row appears %d times on %d pages, want once per page", got, n)
	}
	if got := strings.Count(txt, "Continued"); got != n-1 {
		t.Errorf("continuation foot appears %d times on %d pages, want %d", got, n, n-1)
	}
}

// Without rowhead the first row stays where it is.
func TestLongtblrWithoutRowheadDoesNotRepeat(t *testing.T) {
	_, txt, n := runLongtblr(t, "", "", `colspec = {ll}`, 90)
	if got := strings.Count(txt, "HEADCELL"); got != 1 || n < 2 {
		t.Errorf("header appears %d times on %d pages, want once on several", got, n)
	}
}

// \DefTblrTemplate{contfoot-text}{default}{} empties the foot (tabularray.sty:4750).
func TestLongtblrFootTemplateEmptied(t *testing.T) {
	_, txt, _ := runLongtblr(t, `\DefTblrTemplate{contfoot-text}{default}{}`, "", `colspec = {ll}, rowhead = 1`, 90)
	if strings.Contains(txt, "Continued") {
		t.Errorf("the emptied foot template still draws: %q", txt)
	}
}

// \SetTblrTemplate{contfoot}{empty} removes the foot (tabularray.sty:4753).
func TestLongtblrFootSetEmpty(t *testing.T) {
	_, txt, _ := runLongtblr(t, `\SetTblrTemplate{contfoot}{empty}`, "", `colspec = {ll}`, 90)
	if strings.Contains(txt, "Continued") {
		t.Errorf("an empty contfoot still draws: %q", txt)
	}
}

// A replacement text for the foot is used (\DefTblrTemplate{contfoot-text}{default}{..}).
func TestLongtblrFootTextReplaced(t *testing.T) {
	_, txt, _ := runLongtblr(t, `\DefTblrTemplate{contfoot-text}{default}{Onwards}`, "", `colspec = {ll}`, 90)
	if !strings.Contains(txt, "Onwards") || strings.Contains(txt, "Continued") {
		t.Errorf("replacement foot text not used: %q", txt)
	}
}

// caption= is set above the first page and again, with "(Continued)", above the
// next ones (tabularray.sty:4819-4883, 4920-4951), and label= names the table
// counter stepped by \__tblr_build_table_label_entry: (5326-5341).
func TestLongtblrCaptionAndLabel(t *testing.T) {
	e, txt, n := runLongtblr(t, "", `[caption = {Zebra}, label = {tab:z}]`, `colspec = {ll}`, 90)
	if got := strings.Count(txt, "Zebra"); got != n {
		t.Errorf("caption appears %d times on %d pages", got, n)
	}
	if got := strings.Count(txt, "(Continued)"); got != n-1 {
		t.Errorf("(Continued) appears %d times on %d pages, want %d", got, n, n-1)
	}
	if got := e.labels["tab:z"]; got != "1" {
		t.Errorf("label tab:z = %q, want \"1\"", got)
	}
}

// Notation's templates (\DefTblrTemplate{caption|conthead|capcont}{default}{}) leave
// no caption anywhere, and label=none steps no counter.
func TestLongtblrCaptionTemplatesEmptied(t *testing.T) {
	pre := `\DefTblrTemplate{caption}{default}{}
\DefTblrTemplate{conthead}{default}{}
\DefTblrTemplate{capcont}{default}{}`
	_, txt, _ := runLongtblr(t, pre, `[caption = {Zebra}]`, `colspec = {ll}`, 90)
	if strings.Contains(txt, "Zebra") || strings.Contains(txt, "Continued)") {
		t.Errorf("emptied caption templates still draw: %q", txt)
	}
}

func TestLongtblrCaptionOnlyContinuedEmptied(t *testing.T) {
	_, txt, n := runLongtblr(t, `\DefTblrTemplate{conthead}{default}{}`, `[caption = {Zebra}]`, `colspec = {ll}`, 90)
	if strings.Contains(txt, "(Continued)") || strings.Count(txt, "Zebra") != n {
		t.Errorf("conthead emptied: caption %d times on %d pages, text %q", strings.Count(txt, "Zebra"), n, txt)
	}
}

func TestLongtblrLabelNoneStepsNothing(t *testing.T) {
	e, _, _ := runLongtblr(t, "", `[label = none]`, `colspec = {ll}`, 3)
	if got := e.getCount(e.eq["c@table"].code); got != 0 {
		t.Errorf("table counter = %d after label=none, want 0", got)
	}
	e, _, _ = runLongtblr(t, "", `[label = {tab:q}]`, `colspec = {ll}`, 3)
	if got := e.getCount(e.eq["c@table"].code); got != 1 {
		t.Errorf("table counter = %d after label=, want 1", got)
	}
}

// A table that starts on a partly filled page takes presep (18pt) first and fills
// what is left; one that does not fit there at all starts the next page.
func TestLongtblrAfterText(t *testing.T) {
	var text strings.Builder
	for i := 0; i < 40; i++ {
		text.WriteString("Line of text.\n\n")
	}
	_, _, n := runLongtblr(t, text.String(), "", `colspec = {ll}, rowhead = 1`, 5)
	if n < 2 {
		t.Errorf("pages = %d, want the table after 40 paragraphs to need a second page", n)
	}
}

func TestTblrPaginate(t *testing.T) {
	type c struct {
		name                                            string
		h                                               []int
		fixedFirst, fixedNext, footH, capFirst, capNext int
		partial                                         bool
		want                                            [][2]int
		wantFresh                                       bool
	}
	ten := []int{10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
	for _, tc := range []c{
		{name: "all fit", h: []int{10, 10}, fixedFirst: 5, footH: 3, capFirst: 100, capNext: 100, want: [][2]int{{0, 2}}},
		{name: "last page has no foot", h: []int{10, 10, 10}, footH: 5, capFirst: 30, capNext: 30, want: [][2]int{{0, 3}}},
		{name: "foot reserved", h: ten, footH: 5, capFirst: 35, capNext: 35, want: [][2]int{{0, 3}, {3, 6}, {6, 9}, {9, 10}}},
		{name: "fixed charged first", h: ten, fixedFirst: 10, footH: 5, capFirst: 35, capNext: 35, want: [][2]int{{0, 2}, {2, 5}, {5, 8}, {8, 10}}},
		{name: "fixed charged next", h: ten, fixedNext: 10, footH: 5, capFirst: 35, capNext: 35, want: [][2]int{{0, 3}, {3, 5}, {5, 7}, {7, 9}, {9, 10}}},
		{name: "first page smaller", h: ten, footH: 5, capFirst: 15, capNext: 35, want: [][2]int{{0, 1}, {1, 4}, {4, 7}, {7, 10}}},
		{name: "starts fresh", h: []int{10}, footH: 0, capFirst: 5, capNext: 100, partial: true, want: [][2]int{{0, 1}}, wantFresh: true},
		{name: "forced progress", h: []int{50, 50}, capFirst: 10, capNext: 10, want: [][2]int{{0, 1}, {1, 2}}},
		{name: "empty", h: nil, capFirst: 1, capNext: 1, want: [][2]int{{0, 0}}},
	} {
		got, fresh := tblrPaginate(tc.h, tc.fixedFirst, tc.fixedNext, tc.footH, tc.capFirst, tc.capNext, tc.partial)
		if !reflect.DeepEqual(got, tc.want) || fresh != tc.wantFresh {
			t.Errorf("%s: spans %v fresh %v, want %v %v", tc.name, got, fresh, tc.want, tc.wantFresh)
		}
	}
}

func TestTblrRowGroups(t *testing.T) {
	row := func() node { return &boxNode{kind: hbox, height: 10 * 65536} }
	rule := ruleNode{height: 65536}
	list := []node{rule, row(), rule, row(), rule, row(), rule, row(), rule}
	head, body, foot := tblrRowGroups(list, 1, 1)
	if len(head) != 3 || len(body) != 2 || len(foot) != 2 {
		t.Errorf("head %d body %d foot %d nodes, want 3, 2 units, 2", len(head), len(body), len(foot))
	}
	if h, b, f := tblrRowGroups(list, 0, 0); len(h) != 0 || len(b) != 4 || len(f) != 0 {
		t.Errorf("no head/foot: %d %d %d", len(h), len(b), len(f))
	}
	if h, b, f := tblrRowGroups(list, 9, 9); len(h) != len(list) || len(b) != 0 || len(f) != 0 {
		t.Errorf("clamped: %d %d %d", len(h), len(b), len(f))
	}
	if h, b, f := tblrRowGroups(list, -3, -3); len(h) != 0 || len(b) != 4 || len(f) != 0 {
		t.Errorf("negative: %d %d %d", len(h), len(b), len(f))
	}
	if got := tblrHeight([]node{rule, row()}); got != 11*65536 {
		t.Errorf("height = %d", got)
	}
}

func TestParseTblrOuterAndRows(t *testing.T) {
	cap, label := parseTblrOuter(tokenizeTeX(`caption = {A = B}, label = {tab:x}, entry = {x}`))
	if tokString(cap) != "A = B" || label != "tab:x" {
		t.Errorf("caption %q label %q", tokString(cap), label)
	}
	if _, l := parseTblrOuter(tokenizeTeX(`label = none`)); l != "none" {
		t.Errorf("label = %q, want none", l)
	}
	if c, l := parseTblrOuter(nil); c != nil || l != "" {
		t.Errorf("empty outer gave %q %q", tokString(c), l)
	}
	sp := parseTblrInner(tokenizeTeX(`colspec = {ll}, rowhead = 2, rowfoot = 3`))
	if sp.rowhead != 2 || sp.rowfoot != 3 {
		t.Errorf("rowhead %d rowfoot %d, want 2 3", sp.rowhead, sp.rowfoot)
	}
}

// The column letters map as tabularray.sty defines them: S is centred (6970, in
// the siunitx library), Q is l, t is p (2472), X is l without a width (2486).
func TestNormalizeTblrColspecLetters(t *testing.T) {
	if got := tokString(normalizeTblrColspec(tokenizeTeX("SQtX"), false)); got != "clpl" {
		t.Errorf("SQtX = %q, want clpl", got)
	}
	if got := tokString(normalizeTblrColspec(tokenizeTeX("X"), true)); got != "X" {
		t.Errorf("X with a width = %q, want X", got)
	}
}

// Only longtblr repeats a header and draws the continuation foot; tblr and
// talltblr leave the rows alone.
func TestTblrAndTalltblrDoNotRepeatRowhead(t *testing.T) {
	for _, env := range []string{"tblr", "talltblr"} {
		src := `\documentclass{article}
\usepackage{tabularray}
\begin{document}
\begin{` + env + `}{colspec = {ll}, rowhead = 1}
HEADCELL & HDX\\
` + tblrRows(90) + `\end{` + env + `}
\end{document}`
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		txt := pageText(e)
		if strings.Count(txt, "HEADCELL") != 1 || strings.Contains(txt, "Continued") {
			t.Errorf("%s: header %d times, foot %v", env, strings.Count(txt, "HEADCELL"), strings.Contains(txt, "Continued"))
		}
	}
}

func TestPlaceLongNilBox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.placeLong(nil, tblrSpec{}, nil) // must not panic
}

// Inside a box a longtblr is packed whole, on the line, like a tblr.
func TestLongtblrInsideABoxStaysInline(t *testing.T) {
	a := boxHeightOf(t, `x \begin{longtblr}{ll}a&b\\c&d\\\end{longtblr}`)
	b := boxHeightOf(t, `x\par\begin{longtblr}{ll}a&b\\c&d\\\end{longtblr}`)
	if a >= b {
		t.Errorf("longtblr in a box height %d, want less than %d", a, b)
	}
}

// An empty longtblr body must not break placement.
func TestLongtblrEmptyBody(t *testing.T) {
	if n := pagesOf(t, `\documentclass{article}
\usepackage{tabularray}
\begin{document}
\begin{longtblr}{ll}
\end{longtblr}
\end{document}`); n < 1 {
		t.Errorf("pages = %d", n)
	}
}

func wrappedBlocks(e *Engine) int {
	wrapped := 0
	for _, p := range e.Pages() {
		for _, top := range p.list {
			tb, ok := top.(*boxNode)
			if !ok {
				continue
			}
			for _, n := range tb.list {
				if b, ok := n.(*boxNode); ok && len(b.list) > 0 {
					if _, g := b.list[0].(glueNode); g {
						wrapped++
					}
				}
			}
		}
	}
	return wrapped
}

// Under \centering every page block after the first carries the skips.
func TestLongtblrKeepsCentering(t *testing.T) {
	for _, tc := range []struct {
		pre  string
		want func(int) bool
	}{
		{`\centering`, func(n int) bool { return n >= 2 }},
		{``, func(n int) bool { return n == 0 }},
	} {
		e, _, pages := runLongtblr(t, tc.pre, "", `colspec = {ll}, rowhead = 1`, 90)
		if got := wrappedBlocks(e); !tc.want(got) || pages < 3 {
			t.Errorf("%q: %d wrapped blocks on %d pages", tc.pre, got, pages)
		}
	}
}
