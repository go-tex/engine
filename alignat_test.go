// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// alignat(*) is amsmath's align with the number of column PAIRS stated and no
// inter-column space (amsmath.sty:1750-1759):
//
//	\newenvironment{alignat}{\start@align\z@\st@rredfalse}{\endalign}
//
// \start@align reads the {n} itself, and n only decides the spacing between pairs —
// the content and the numbering are align's. Undefined, the environment resolved to
// \relax, its COLUMN COUNT was typeset as a digit on the page and its body was set as
// prose with no numbering:
//
//	main       AVANT 2 a = b c = d e = f g = h APRES
//	this       AVANT a = b c = d (1) e = f g = h (2) APRES
//	reference  AVANT a=b e=f c=d g=h APRES (1) (2)
//
// Three corpus papers, four displays. Reusing align's column model is what flalign
// already does a few lines above, for the same reason.
func TestAlignatIsAlignWithAStatedColumnCount(t *testing.T) {
	for _, c := range []struct {
		name, env string
		numbered  bool
	}{
		{"alignat", "alignat", true},
		{"alignat*", "alignat*", false},
	} {
		src := `\documentclass{article}\usepackage{amsmath}\begin{document}BEFORE` +
			`\begin{` + c.env + `}{2}` + "\n" +
			`AAA &= BBB & CCC &= DDD` + "\n" +
			`\end{` + c.env + `}` + `AFTER\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("%s: compile: %v", c.name, err)
		}
		if got := e.Diagnostics().UndefinedEnvs[c.env]; got != 0 {
			t.Errorf("%s still reported undefined (%d)", c.name, got)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: equation dropped: %v", c.name, e.mathDropped)
		}
		svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
		// The column count is an ARGUMENT, not content: it must not reach the page.
		// "2" alone is too common to search for, so the witness surrounds it.
		if strings.Contains(svg, "BEFORE2") || strings.Contains(svg, "2AAA") {
			t.Errorf("%s: the column count was typeset", c.name)
		}
		for _, want := range []string{"BEFORE", "AAA", "DDD", "AFTER"} {
			if !strings.Contains(svg, want) {
				t.Errorf("%s: %q is not on the page", c.name, want)
			}
		}
	}
}
