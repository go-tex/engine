// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Material between pgf's invisibility pair keeps its metrics and draws nothing:
// that is what beamer means by covered, and why the page does not move as the
// overlay steps arrive.
func TestCoveredGlyphsKeepTheirSpaceAndDrawNothing(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`ab\pgfsys@begininvisible cd\pgfsys@endinvisible ef\par`); err != nil {
		t.Fatal(err)
	}
	var seen []struct {
		ch      rune
		covered bool
	}
	var walk func([]node)
	walk = func(l []node) {
		for _, n := range l {
			switch c := n.(type) {
			case charNode:
				seen = append(seen, struct {
					ch      rune
					covered bool
				}{c.ch, c.covered})
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	got := map[rune]bool{}
	for _, s := range seen {
		got[s.ch] = s.covered
	}
	for _, r := range "ab" {
		if got[r] {
			t.Errorf("%q was marked covered outside the pair", r)
		}
	}
	for _, r := range "cd" {
		if !got[r] {
			t.Errorf("%q inside the pair was not marked covered", r)
		}
	}
	for _, r := range "ef" {
		if got[r] {
			t.Errorf("%q after \\pgfsys@endinvisible is still covered", r)
		}
	}
	if e.coveringDepth != 0 {
		t.Errorf("covering depth = %d after the pair closed, want 0", e.coveringDepth)
	}
}

// \pgfsys@endinvisible with no pair open must not drive the depth negative (a
// document may close one the engine never saw opened).
func TestUnmatchedEndInvisibleIsHarmless(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\pgfsys@endinvisible x\par`); err != nil {
		t.Fatal(err)
	}
	if e.coveringDepth != 0 {
		t.Errorf("covering depth = %d, want 0", e.coveringDepth)
	}
}

// End to end with the REAL class: a frame with two \pause's makes three pages that
// reveal their material one step at a time. The emulation never opens an
// invisibility group, so this skips where the class is not installed rather than
// passing for the wrong reason.
func TestPauseRevealsOneStepAtATime(t *testing.T) {
	tree := os.Getenv("GOTEX_TEXMF")
	if tree == "" {
		tree = "/Users/Shared/gotex/measure/texmf"
	}
	if _, err := os.Stat(filepath.Join(tree, "beamer.cls")); err != nil {
		t.Skip("no real beamer.cls under GOTEX_TEXMF: the emulation has no overlays to cover")
	}
	t.Setenv("GOTEX_TEXMF", tree)
	src := `\documentclass{beamer}\begin{document}` +
		`\begin{frame}{Etapes}un\pause deux\pause trois\end{frame}` +
		`\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 3 {
		t.Fatalf("pages = %d, want 3 (one per overlay step)", len(pages))
	}
	drawn := func(p string) int { return strings.Count(p, "<path") }
	if !(drawn(pages[0]) < drawn(pages[1]) && drawn(pages[1]) < drawn(pages[2])) {
		t.Errorf("each step must draw more than the one before: %d, %d, %d",
			drawn(pages[0]), drawn(pages[1]), drawn(pages[2]))
	}
}
