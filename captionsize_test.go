// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

func captionGlyphSizes(t *testing.T, preamble string) map[rune]int {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(scaleMock{px: 10})
	src := `\noindent A` + preamble + `\begin{figure}\caption{Z}\end{figure}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return glyphSizes(e)
}

// article.cls's \@makecaption sets "#1: #2" at the CURRENT size (l.482) — there is
// no \small in it. We hard-coded \small for every caption, which is the minority
// rule: of the 200 arXiv corpus papers, 149 never redefine \@makecaption (so
// article.cls's rule is theirs) and 19 redefine it at the current size.
//
// Measured directly against tectonic rather than inferred from the class: with the
// SAME word in the body and in the caption, the reference gives it width 66.01 in
// both. Ours was a factor 10/9 narrower in the caption — the signature of \small.
func TestCaptionIsSetAtTheCurrentSize(t *testing.T) {
	sizes := captionGlyphSizes(t, "")
	if sizes['A'] != 10 {
		t.Fatalf("body glyph size = %d, want the 10 the mock declares", sizes['A'])
	}
	if sizes['Z'] != sizes['A'] {
		t.Errorf("caption glyph size = %d, want the body's %d", sizes['Z'], sizes['A'])
	}
}

// \captionfont is caption.sty's own name for the caption size — its small,
// footnotesize and scriptsize options are literally \def\captionfont{\small} and
// friends. Empty by default, so a document that wants a smaller caption (48 of the
// 200 corpus papers do, through a class or through the caption package) has
// somewhere to say so and is honoured.
func TestCaptionFontHookIsHonoured(t *testing.T) {
	sizes := captionGlyphSizes(t, `\renewcommand\captionfont{\small}`)
	if sizes['A'] != 10 {
		t.Fatalf("body glyph size = %d, want 10", sizes['A'])
	}
	if sizes['Z'] != 9 {
		t.Errorf("caption glyph size = %d with \\captionfont=\\small, want 9", sizes['Z'])
	}
}
