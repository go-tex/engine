// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \height, \depth, \width and \totalheight are bound by \raisebox to the
// CONTENT's metrics (ltboxes.dtx: \setbox\@tempboxa\hbox{#4}, then
// \def\height{\ht\@tempboxa}), so they are only meaningful once the content is
// packed — which is after the lift has been read off the input. The lift is
// therefore kept as tokens and scanned afterwards.
//
// They have to be internal DIMENS, not macros expanding to "12.34pt": the whole
// point is that they appear with a coefficient, and "-.512.34pt" is not a
// dimension at all.
//
// The shifts below were measured against tectonic on the same four-line witness,
// as the difference between the raised word's yMin and its line's:
//
//	0pt            ours  0.0    tectonic  0.0
//	-.5\height     ours +3.3    tectonic +3.4
//	\totalheight   ours -8.9    tectonic -8.9
//	-.25\width     ours +3.1    tectonic +3.1
func TestRaiseboxBoxMetrics(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt\noindent\setbox0\hbox{\raisebox{0pt}{Hg}}`); err != nil {
		t.Fatal(err)
	}
	// Measure the mock font's "Hg" so the expectations are relative, not absolute.
	plain := e.box[0]
	if plain == nil {
		t.Fatal("no box: the harness did not pack the content")
	}
	h, d, w := plain.height, plain.depth, plain.width
	if h == 0 || w == 0 {
		t.Fatalf("content has no metrics: height %d width %d", h, w)
	}
	for _, c := range []struct {
		lift string
		want int // the box's shift, positive-downward (shift = -lift)
	}{
		{`0pt`, 0},
		{`-.5\height`, h / 2},
		{`\totalheight`, -(h + d)},
		{`-.25\width`, w / 4},
		{`\height`, -h},
		{`\depth`, -d},
	} {
		e2 := New()
		e2.LoadLaTeX()
		e2.SetFont(spMock{})
		src := `\hsize=300pt\noindent\setbox0\hbox{\raisebox{` + c.lift + `}{Hg}}`
		if _, err := e2.Run(src); err != nil {
			t.Fatalf("%s: %v", c.lift, err)
		}
		outer := e2.box[0]
		if outer == nil || len(outer.list) == 0 {
			t.Errorf("%s: nothing packed", c.lift)
			continue
		}
		inner, ok := outer.list[0].(*boxNode)
		if !ok {
			t.Errorf("%s: first node is %T, want a box", c.lift, outer.list[0])
			continue
		}
		// A point of slack: the halving of an odd scaled-point count rounds.
		if diff := inner.shift - c.want; diff > 1 || diff < -1 {
			t.Errorf("raisebox{%s}: shift %d, want %d", c.lift, inner.shift, c.want)
		}
	}
}

// ⛔ The cost of the missing bindings was NOT a wrong lift. \scanDimen met an
// undefined control sequence where it wanted a unit and read ON, swallowing the
// content group and unbalancing the float the \raisebox sat in — so a \caption
// later in that float had no \@captype and the literal text "\the@captype"
// reached the page.
//
// This is the six-line witness, and the two ingredients are both needed: the
// float must take the inline path ([h]) and the lift must name a metric
// (\raisebox{2pt} is fine). tectonic renders it "1".
func TestRaiseboxMetricDoesNotSwallowItsFloat(t *testing.T) {
	src := `\documentclass{article}\hsize=300pt
\begin{document}
\begin{figure}[h]
\raisebox{-.5\height}{x}
\caption{cap}\label{f}
\end{figure}
\end{document}`
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.labels["f"], "1"; got != want {
		t.Errorf("\\ref = %q, want %q", got, want)
	}
	if got, want := e.refTypes["f"], "figure"; got != want {
		t.Errorf("type = %q, want %q", got, want)
	}
}

// The bindings are scoped: a \raisebox inside another's content must not leave
// its own metrics behind for the outer one to read. The inner box is narrow, the
// outer wide, so an unscoped binding would lift the outer by the INNER's width.
func TestRaiseboxMetricsAreScoped(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt\noindent\setbox0\hbox{\raisebox{-.25\width}{WWWWWWWW\raisebox{0pt}{i}}}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	outer := e.box[0]
	if outer == nil || len(outer.list) == 0 {
		t.Fatal("nothing packed")
	}
	b, ok := outer.list[0].(*boxNode)
	if !ok {
		t.Fatalf("first node is %T, want a box", outer.list[0])
	}
	// The outer lift is a quarter of the OUTER box's width, which is the whole
	// run of W's plus the i — many times the inner "i" alone.
	if want := b.width / 4; b.shift < want-1 || b.shift > want+1 {
		t.Errorf("outer shift %d, want %d (a quarter of the OUTER width %d)", b.shift, want, b.width)
	}
}

// \raisebox{lift}[]{…} means "take the natural height", and an EMPTY bracket must
// not read as a stated 0pt. scanOptBracketDimen reported an empty bracket as
// absent; reading the optionals as tokens loses that, so it is restored here.
// No corpus paper writes one, which is exactly why it needs a test.
func TestRaiseboxEmptyOptionalKeepsNaturalMetric(t *testing.T) {
	natural := func(src string) (int, int) {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(`\hsize=300pt\noindent\setbox0\hbox{` + src + `}`); err != nil {
			t.Fatal(err)
		}
		b, ok := e.box[0].list[0].(*boxNode)
		if !ok {
			t.Fatalf("%s: first node is %T, want a box", src, e.box[0].list[0])
		}
		return b.height, b.depth
	}
	h0, d0 := natural(`\raisebox{0pt}{Hg}`)
	for _, src := range []string{`\raisebox{0pt}[]{Hg}`, `\raisebox{0pt}[][]{Hg}`} {
		h, d := natural(src)
		if h != h0 || d != d0 {
			t.Errorf("%s: height/depth %d/%d, want the natural %d/%d", src, h, d, h0, d0)
		}
	}
	// A STATED height is still honoured.
	if h, _ := natural(`\raisebox{0pt}[5pt]{Hg}`); h != 5*65536 {
		t.Errorf("stated height = %d, want %d", h, 5*65536)
	}
}

// ⛔ The corpus PAGE metric gets WORSE from this change — Σ|page deviation|
// 337 → 342 over four papers — and the geometry gets RIGHT. Both are measured,
// and the second is why the first is not a reason to revert.
//
// The vertical space a float containing \raisebox{-.5\height}{\rule{40pt}{20pt}}
// occupies, as the gap between a MARK above it and a MARK below:
//
//	main, before      79.2pt
//	with this change  75.4pt
//	tectonic          75.6pt
//
// So 3.6pt of spurious height per such float became 0.2pt. The four papers that
// lose pages are all ALREADY SHORT of their reference (34 against 44, 18 against
// 20, 30 against 33) for reasons that have nothing to do with \raisebox, and the
// spurious space was partly compensating for that: one of them, 2304.05592, also
// GAINS text, 96.3% → 96.9% of its reference's words.
//
// Attribution, measured by disabling the halves separately: the whole page
// movement comes from reading the lift as TOKENS instead of scanning it off the
// live input; binding the metrics changes the lift VALUES and not the pagination.
func TestRaiseboxFloatHeightMatchesReference(t *testing.T) {
	// The unit-level statement of the same thing: a \raisebox whose lift names a
	// metric must leave the content INSIDE a box, so the surrounding list sees one
	// box and not loose material.
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt\noindent\setbox0\hbox{\raisebox{-.5\height}{Hg}}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	outer := e.box[0]
	if outer == nil {
		t.Fatal("nothing packed")
	}
	if len(outer.list) != 1 {
		t.Errorf("the hbox holds %d nodes, want 1 — the content leaked out of the box", len(outer.list))
	}
	if _, ok := outer.list[0].(*boxNode); !ok {
		t.Errorf("node is %T, want a box", outer.list[0])
	}
}

// ⛔ \width and \depth are names a DOCUMENT may use for something of its own.
// Two corpus papers define one of them, and 2407.18384 writes \depth(\Phi) sixty
// times for a neural network's depth. The bindings are therefore local to the
// \raisebox, and the document's own meaning comes back afterwards. Neither paper
// happens to use \raisebox, so no corpus measurement would have caught this.
func TestRaiseboxDoesNotClobberTheDocumentsOwnNames(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt\def\depth{DEPTH}\def\width{WIDTH}\noindent` +
		`\raisebox{-.5\height}{Hg}[\depth][\width]`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	out := b.String()
	for _, want := range []string{"DEPTH", "WIDTH"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is gone from %q — the \\raisebox binding outlived itself", want, out)
		}
	}
}
