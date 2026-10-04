// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// acmart's teaserfigure is the full-width figure above the title. It was
// undefined, so its \caption took the path above. tectonic prints "Figure 1: A
// teaser caption here" for this document, so it numbers with the FIGURE counter —
// which is what is asserted here, together with the counter being SHARED (the
// following figure is 2, not 1).
func TestAcmartTeaserFigureNumbersAsAFigure(t *testing.T) {
	e, err := compile([]byte(`\documentclass{acmart}\begin{document}\title{T}`+
		`\begin{teaserfigure}\caption{TEASER}\end{teaserfigure}`+
		`\begin{figure}\caption{AFTER}\end{figure}BODY\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	for _, want := range []string{"Figure1:TEASER", "Figure2:AFTER"} {
		if !strings.Contains(got, want) {
			t.Errorf("the page carries %q, which is missing %q", got, want)
		}
	}
	if strings.Contains(got, "by1") {
		t.Errorf("the page carries %q, which still leaks the \\advance keyword", got)
	}
}
