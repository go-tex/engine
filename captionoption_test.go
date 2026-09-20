// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// caption.sty's small/footnotesize/scriptsize options are literally
// \def\captionfont{\small} and friends, so honouring them is just defining the hook
// \caption already reads. The texmf tree carries fonts only, the real caption.sty
// never loads, and our stub gobbles the options — so without this a document that
// asks for a smaller caption got the body size article.cls prescribes for everyone
// who does NOT ask. 11 of the 200 corpus papers ask through this option form.
func TestCaptionPackageSizeOptionIsHonoured(t *testing.T) {
	for _, c := range []struct {
		opts string
		want int
	}{
		{`[font=footnotesize]`, 8},
		{`[font=small]`, 9},
		{`[small]`, 9},
		{`[font={small}]`, 9},
		{`[labelfont=bf]`, 10}, // not a size: the body size stands
		{``, 10},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(scaleMock{px: 10})
		src := `\usepackage` + c.opts + `{caption}` +
			`\noindent A\begin{figure}\caption{Z}\end{figure}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%s: %v", c.opts, err)
		}
		sizes := glyphSizes(e)
		if sizes['A'] != 10 {
			t.Fatalf("%s: body glyph = %d, want 10", c.opts, sizes['A'])
		}
		if sizes['Z'] != c.want {
			t.Errorf("\\usepackage%s{caption}: caption glyph = %d, want %d",
				c.opts, sizes['Z'], c.want)
		}
	}
}
