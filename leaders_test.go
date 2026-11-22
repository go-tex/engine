// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"strings"
	"testing"
)

// tex.web §626-§628 on one glue: edge 0, glue from 3 to 28 (w 25), tile 10.
func TestLeaderTileOrigins(t *testing.T) {
	for _, c := range []struct {
		name string
		mode leaderMode
		want []float64
	}{
		// aligned: grid 0,10,20,30; the first tile at 10 is the first one at or after
		// x=3, and the one at 20 would end at 30, past the glue's end at 28: not set.
		{"aligned", leadersAligned, []float64{10}},
		// centered: lq=2, lr=5, start 3+2.5.
		{"centered", leadersCentered, []float64{5.5, 15.5}},
		// expanded: lq=2, lr=5, lx=5/3, start 3+(5-5/3)/2, step 10+5/3.
		{"expanded", leadersExpanded, []float64{3 + (5-5.0/3)/2, 3 + (5-5.0/3)/2 + 10 + 5.0/3}},
	} {
		got := leaderTileOrigins(c.mode, 0, 3, 25, 10)
		if len(got) != len(c.want) {
			t.Fatalf("%s: %v, want %v", c.name, got, c.want)
		}
		for i := range got {
			if math.Abs(got[i]-c.want[i]) > 1e-6 {
				t.Errorf("%s: tile %d at %v, want %v", c.name, i, got[i], c.want[i])
			}
		}
	}
	// A zero-width tile or glue sets nothing (and must not divide by zero).
	for _, m := range []leaderMode{leadersAligned, leadersCentered, leadersExpanded} {
		if got := leaderTileOrigins(m, 0, 0, 10, 0); got != nil {
			t.Errorf("mode %d, zero tile: %v", m, got)
		}
		if got := leaderTileOrigins(m, 0, 0, 0, 10); got != nil {
			t.Errorf("mode %d, zero glue: %v", m, got)
		}
	}
	if got := leaderTileOrigins(leadersCentered, 0, 0, 9, 10); got != nil {
		t.Errorf("a tile wider than the glue must not be set, got %v", got)
	}
}

// The primitives exist and paint: a box tile once per fit, a rule across the glue.
func TestLeadersPaint(t *testing.T) {
	count := func(body string) (paths, rects int) {
		e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		s := strings.Join(e.RenderPages(e.renderMargin(0)), "")
		return strings.Count(s, "<path"), strings.Count(s, "<rect")
	}
	base, baseRects := count(`\noindent\hbox to 100pt{\hfil}`)
	for _, m := range []string{`\leaders`, `\cleaders`, `\xleaders`} {
		p, _ := count(`\noindent\hbox to 100pt{` + m + `\hbox to 10pt{x\hss}\hfil}`)
		if p-base != 10 {
			t.Errorf("%s: %d tiles, want 10", m, p-base)
		}
	}
	// 95pt: the aligned grid fits 9 whole tiles, cleaders 9 as well.
	p, _ := count(`\noindent\hbox to 95pt{\cleaders\hbox to 10pt{x\hss}\hfil}`)
	if p-base != 9 {
		t.Errorf("cleaders in 95pt: %d tiles, want 9", p-base)
	}
	_, r := count(`\noindent\hbox to 100pt{\leaders\hrule height 1pt\hfil}`)
	if r-baseRects != 1 {
		t.Errorf("a rule leader paints one rule, got %d", r-baseRects)
	}
}

// Malformed leaders (tex.web §1078): no box, or no glue after it, drops the leader
// and what follows is read as itself; input ending inside one is not a crash.
func TestLeadersMalformed(t *testing.T) {
	text := func(body string) string {
		e, err := compile([]byte(`\documentclass{article}\begin{document}`+body), Options{Lenient: true})
		if err != nil {
			t.Fatalf("compile %q: %v", body, err)
		}
		return strings.Join(strings.Fields(stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))), " ")
	}
	for _, c := range []struct{ body, want string }{
		{`\leaders QQ\end{document}`, "QQ"},                          // no box: the text survives
		{`\leaders\hbox{x}\relax QQ\end{document}`, "QQ"},            // box, no glue
		{`\leaders\hbox{x}QQ\end{document}`, "QQ"},                   // box, a letter instead of glue
		{`\noindent AA\leaders\hbox{x}\hfil BB\end{document}`, "AA"}, // well formed still works
	} {
		if got := text(c.body); !strings.Contains(got, c.want) {
			t.Errorf("%q: %q lacks %q", c.body, got, c.want)
		}
	}
	for _, body := range []string{`\leaders`, `\leaders\hbox{x}`} { // EOF inside the leader
		text(body)
	}
}
