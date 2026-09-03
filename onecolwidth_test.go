// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// Under the GOTEX_FLOATS faithful mode, a one-column figure sized to a fraction of
// \textwidth reserves real space: the graphics-option parser evaluates 0.9\textwidth
// as a genuine dimension (≈ 0.9 of the text width), as it already does in two-column
// mode, instead of the 0.9pt a raw-text scan produced by dropping the \textwidth
// control sequence. With the flag off, one-column keeps the legacy sub-point read, so
// the tuned single-column class baselines are untouched (default byte-identical).
func TestOneColumnFaithfulFigureWidth(t *testing.T) {
	uri := pngDataURI(t, 100, 100) // square, so height tracks width
	run := func(faithful bool) (imageNode, *Engine) {
		if faithful {
			t.Setenv("GOTEX_FLOATS", "1")
		} else {
			t.Setenv("GOTEX_FLOATS", "0") // opt out: the legacy raw-text read
		}
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		src := `\documentclass{article}\begin{document}` +
			`First paragraph to settle the measure.\par` +
			`\noindent\includegraphics[width=0.9\textwidth]{` + uri + `}` +
			`\end{document}`
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		im, ok := firstImage(e.mvl)
		if !ok {
			t.Fatal("no imageNode placed")
		}
		return im, e
	}

	// Faithful mode: 0.9\textwidth is evaluated as a real dimension (\textwidth is the
	// single column measure in one column).
	on, e := run(true)
	if want := 9 * e.hsize / 10; !within(on.width, want, want/50) {
		t.Errorf("faithful one-column figure width = %d sp, want ≈0.9·textwidth = %d", on.width, want)
	}

	// Default: the legacy raw-text read keeps it sub-point, so the single-column corpus
	// baseline is unchanged.
	off, _ := run(false)
	if off.width > unity {
		t.Errorf("default one-column figure width = %d sp, expected the legacy sub-point size", off.width)
	}
}
