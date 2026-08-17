// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// placeins's \FloatBarrier recurses "\newpage\FloatBarrier" until the float lists
// are empty. The engine has no float mechanism, so those lists (\@deferlist,
// \@dbldeferlist, \@botlist) must still be DEFINED as empty — otherwise the test
// \edef\@tempa{...}\ifx\@tempa\@empty never matches, and the barrier emits one
// forced page break per turn: a 5000-page (maxPages) runaway on an ordinary
// article that loads placeins and gives \section a \FloatBarrier. This reproduces
// the placeins shape with a bundled .sty and checks pagination stays bounded.
func TestFloatBarrierDoesNotExplodePages(t *testing.T) {
	// placeins's real \FloatBarrier body: when the float lists are non-empty it
	// enters the \@fltovf/\if@firstcolumn recursion and, with \if@firstcolumn not
	// true, loops on "\null\newpage\FloatBarrier" without end. Only the leading
	// \ifx\@tempa\@empty short-circuit — which needs the float lists to expand to
	// nothing — keeps it a no-op.
	withTempDir(t, map[string]string{
		"floatbar.sty": `\def\@fb@botlist{\@botlist}\def\@fb@topbarrier{}` +
			`\def\FloatBarrier{\par\begingroup \let\@elt\relax` +
			`\edef\@tempa{\@fb@botlist\@deferlist\@dbldeferlist}` +
			`\ifx\@tempa\@empty\else` +
			`\ifx\@fltovf\relax\if@firstcolumn \clearpage\else \null\newpage\FloatBarrier \fi` +
			`\else \newpage \let\@fltovf\relax \FloatBarrier` +
			`\fi\fi \endgroup \@fb@topbarrier }`,
	}, func() {
		src := `\documentclass{article}\usepackage{floatbar}\begin{document}` +
			"Body text.\\FloatBarrier More body.\n\\end{document}"
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		if n := len(e.Pages()); n > 5 {
			t.Errorf("\\FloatBarrier exploded pagination into %d pages; the float lists must be empty so it no-ops", n)
		}
	})
}
