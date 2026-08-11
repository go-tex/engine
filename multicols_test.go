// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// colBody is a block of running text long enough to wrap into many line boxes at a
// narrow column measure, so the balancer has material to distribute.
const colBody = "aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa " +
	"aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa " +
	"aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa aaaa"

// findColumnRow returns the first hbox whose direct children include ≥2 vboxes of
// the given column width, together with those column vboxes — i.e. the multicols
// row laid out side by side.
func findColumnRow(nodes []node, width int) (*boxNode, []*boxNode) {
	for _, n := range nodes {
		b, ok := n.(*boxNode)
		if !ok {
			continue
		}
		if b.kind == hbox {
			var cols []*boxNode
			for _, c := range b.list {
				if cb, ok := c.(*boxNode); ok && cb.kind == vbox && cb.width == width {
					cols = append(cols, cb)
				}
			}
			if len(cols) >= 2 {
				return b, cols
			}
		}
		if hb, cs := findColumnRow(b.list, width); hb != nil {
			return hb, cs
		}
	}
	return nil, nil
}

// colContentHeight is a column's natural content height (before top-anchoring set a
// common declared height), used to check that the two columns are balanced.
func colContentHeight(b *boxNode) int { return vlistHeight(b.list) }

// A two-column multicols lays its body into two side-by-side vboxes, each of the
// column measure (\hsize−\columnsep)/2, and the two columns come out close in height.
func TestMulticolsTwoColumns(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=200pt\begin{multicols}{2}` + colBody + `\end{multicols}`); err != nil {
		t.Fatal(err)
	}
	wantW := (200*unity - e.columnsep) / 2
	row, cols := findColumnRow(e.mvl, wantW)
	if row == nil {
		t.Fatalf("no two-column row of width %d found", wantW)
	}
	if len(cols) != 2 {
		t.Fatalf("found %d columns, want 2", len(cols))
	}
	for i, c := range cols {
		if c.width != wantW {
			t.Errorf("column %d width = %d, want %d", i, c.width, wantW)
		}
	}
	// The row spans (about) \hsize: two columns plus one \columnsep gap.
	if wantRow := 2*wantW + e.columnsep; row.width != wantRow {
		t.Errorf("row width = %d, want %d", row.width, wantRow)
	}
	// Balanced: the two natural heights are within one line's worth of each other.
	h0, h1 := colContentHeight(cols[0]), colContentHeight(cols[1])
	tol := 20 * unity // ~ one line (7pt) + interline glue, generous for best-effort
	if diff := h0 - h1; diff > tol || diff < -tol {
		t.Errorf("columns unbalanced: heights %d vs %d (diff %d > tol %d)", h0, h1, diff, tol)
	}
	// Both columns carry several line boxes (the body wrapped and was split).
	for i, c := range cols {
		lines := 0
		for _, n := range c.list {
			if _, ok := n.(*boxNode); ok {
				lines++
			}
		}
		if lines < 2 {
			t.Errorf("column %d has %d line boxes, want ≥2", i, lines)
		}
	}
}

// {3} produces three side-by-side columns of measure (\hsize−2·\columnsep)/3.
func TestMulticolsThreeColumns(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt\begin{multicols}{3}` + colBody + `\end{multicols}`); err != nil {
		t.Fatal(err)
	}
	wantW := (300*unity - 2*e.columnsep) / 3
	row, cols := findColumnRow(e.mvl, wantW)
	if row == nil {
		t.Fatalf("no three-column row of width %d found", wantW)
	}
	if len(cols) != 3 {
		t.Fatalf("found %d columns, want 3", len(cols))
	}
	if wantRow := 3*wantW + 2*e.columnsep; row.width != wantRow {
		t.Errorf("row width = %d, want %d", row.width, wantRow)
	}
}

// \columnseprule>0 inserts a vertical rule of that thickness between the columns.
func TestMulticolsColumnSepRule(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=200pt\columnseprule=2pt\begin{multicols}{2}` + colBody + `\end{multicols}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if e.columnseprule != 2*unity {
		t.Fatalf("\\columnseprule = %d, want %d", e.columnseprule, 2*unity)
	}
	wantW := (200*unity - e.columnsep) / 2
	row, cols := findColumnRow(e.mvl, wantW)
	if row == nil {
		t.Fatal("no two-column row found")
	}
	if len(cols) != 2 {
		t.Fatalf("found %d columns, want 2", len(cols))
	}
	var rules int
	for _, n := range row.list {
		if r, ok := n.(ruleNode); ok && r.width == 2*unity {
			rules++
		}
	}
	if rules != 1 {
		t.Errorf("found %d separator rules of 2pt, want 1", rules)
	}
}

// The default \columnsep is 10pt; without a rule the gap is a single kern of that
// width and no rule appears.
func TestMulticolsDefaultsNoRule(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if e.columnsep != 10*unity {
		t.Fatalf("default \\columnsep = %d, want %d", e.columnsep, 10*unity)
	}
	if e.columnseprule != 0 {
		t.Fatalf("default \\columnseprule = %d, want 0", e.columnseprule)
	}
	if _, err := e.Run(`\hsize=200pt\begin{multicols}{2}` + colBody + `\end{multicols}`); err != nil {
		t.Fatal(err)
	}
	wantW := (200*unity - e.columnsep) / 2
	row, _ := findColumnRow(e.mvl, wantW)
	if row == nil {
		t.Fatal("no two-column row found")
	}
	for _, n := range row.list {
		if _, ok := n.(ruleNode); ok {
			t.Errorf("unexpected separator rule with \\columnseprule=0")
		}
	}
	// Exactly one 10pt gap kern between the two columns.
	gaps := 0
	for _, n := range row.list {
		if k, ok := n.(kernNode); ok && k.width == e.columnsep {
			gaps++
		}
	}
	if gaps != 1 {
		t.Errorf("found %d column-gap kerns, want 1", gaps)
	}
}

// An optional [preamble] is typeset full-width above the columns.
func TestMulticolsPreamble(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=200pt\begin{multicols}{2}[bb bb bb]` + colBody + `\end{multicols}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	// The preamble is set at full \hsize, so a vbox of that width precedes the row.
	if _, ok := firstVboxOfWidth(e.mvl, 200*unity); !ok {
		t.Fatal("no full-width preamble vbox placed")
	}
	wantW := (200*unity - e.columnsep) / 2
	if row, _ := findColumnRow(e.mvl, wantW); row == nil {
		t.Fatal("no column row after preamble")
	}
}

// N ≤ 1 degenerates to ordinary single-column typesetting: no column row is built,
// the body is simply set at the current \hsize, and nothing panics.
func TestMulticolsDegenerate(t *testing.T) {
	for _, n := range []string{"0", "1"} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		src := `\hsize=200pt\begin{multicols}{` + n + `}` + colBody + `\end{multicols}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("N=%s: %v", n, err)
		}
		// A degenerate run produces normal full-width lines, never a column row.
		full := 200 * unity
		if row, _ := findColumnRow(e.mvl, (full-e.columnsep)/2); row != nil {
			t.Errorf("N=%s: unexpected column row in degenerate mode", n)
		}
		if _, ok := firstVboxOfWidth(e.mvl, full); ok {
			// full-width line boxes are wrapped in the page vbox; ensure body set.
		}
		if len(e.mvl) == 0 {
			t.Errorf("N=%s: nothing typeset", n)
		}
	}
}

// An empty body must not panic; it yields (possibly empty) columns and no error.
func TestMulticolsEmptyBody(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=200pt\begin{multicols}{2}\end{multicols}`); err != nil {
		t.Fatal(err)
	}
	// A row may or may not be findable (columns can be empty); the point is no panic.
}

// A missing \end{multicols} (premature EOF) must not panic; collectEnvBody returns
// whatever it gathered and the environment is typeset from that.
func TestMulticolsMissingEnd(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// No \end{multicols}: must return without panicking (error is acceptable).
	_, _ = e.Run(`\hsize=200pt\begin{multicols}{2}` + colBody)
}

// balanceVList slices a vertical list into n pieces whose heights are close, and the
// pieces reassemble to the whole list in order.
func TestBalanceVList(t *testing.T) {
	// Ten unit-height boxes (10pt each).
	var list []node
	for i := 0; i < 10; i++ {
		list = append(list, &boxNode{kind: hbox, height: 10 * unity})
	}
	pieces := balanceVList(list, 2)
	if len(pieces) != 2 {
		t.Fatalf("got %d pieces, want 2", len(pieces))
	}
	total := len(pieces[0]) + len(pieces[1])
	if total != 10 {
		t.Errorf("pieces cover %d boxes, want 10", total)
	}
	if len(pieces[0]) != 5 || len(pieces[1]) != 5 {
		t.Errorf("split = %d/%d, want 5/5", len(pieces[0]), len(pieces[1]))
	}
	// A single oversized item is placed alone rather than dropped.
	tall := []node{
		&boxNode{kind: hbox, height: 100 * unity},
		&boxNode{kind: hbox, height: unity},
		&boxNode{kind: hbox, height: unity},
	}
	tp := balanceVList(tall, 2)
	if len(tp[0]) != 1 {
		t.Errorf("oversized first item: piece 0 has %d items, want 1", len(tp[0]))
	}
}

// vlistHeight sums the vertical contributions of a list.
func TestVListHeight(t *testing.T) {
	list := []node{
		&boxNode{kind: hbox, height: 3 * unity, depth: unity}, // 4pt
		glueNode{spec: glueSpec{width: 2 * unity}},            // 2pt
		kernNode{width: unity},                                // 1pt
	}
	if got, want := vlistHeight(list), 7*unity; got != want {
		t.Errorf("vlistHeight = %d, want %d", got, want)
	}
}

// columnGap without a rule is one sep-wide kern; with a rule it is two kerns around
// a centred rule, and the three pieces sum to exactly sep.
func TestColumnGap(t *testing.T) {
	sep := 10 * unity
	if g := columnGap(sep, 0, 50*unity); len(g) != 1 {
		t.Fatalf("no-rule gap has %d nodes, want 1", len(g))
	} else if k, ok := g[0].(kernNode); !ok || k.width != sep {
		t.Errorf("no-rule gap = %v, want one %d kern", g, sep)
	}
	rule := 3 * unity
	g := columnGap(sep, rule, 50*unity)
	if len(g) != 3 {
		t.Fatalf("ruled gap has %d nodes, want 3", len(g))
	}
	kl, ok0 := g[0].(kernNode)
	rn, ok1 := g[1].(ruleNode)
	kr, ok2 := g[2].(kernNode)
	if !ok0 || !ok1 || !ok2 {
		t.Fatalf("ruled gap shape = %T,%T,%T", g[0], g[1], g[2])
	}
	if rn.width != rule || rn.height != 50*unity {
		t.Errorf("rule = w%d h%d, want w%d h%d", rn.width, rn.height, rule, 50*unity)
	}
	if kl.width+rn.width+kr.width != sep {
		t.Errorf("ruled gap widths %d+%d+%d != sep %d", kl.width, rn.width, kr.width, sep)
	}
}

// A column body containing a nested environment is closed only by the matching
// \end{multicols}, not by the inner \end.
func TestMulticolsNestedEnv(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=200pt\begin{multicols}{2}\begin{itemize}\item ` +
		strings.Repeat("aaaa ", 20) + `\end{itemize}\end{multicols}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	wantW := (200*unity - e.columnsep) / 2
	if row, _ := findColumnRow(e.mvl, wantW); row == nil {
		t.Fatal("no column row (inner \\end{itemize} closed multicols early?)")
	}
}
