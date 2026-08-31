// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// jss.cls Code environments set their bodies verbatim, and CodeChunk is a
// transparent wrapper: the input and output lines keep their own line structure
// instead of collapsing into a single run of prose the way the undefined
// environments did. (Assertions avoid interior spaces — mvlText reads glyph nodes,
// not the verbatim inter-word glue.)
func TestJSSCodeEnvironmentsVerbatim(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "Before\n" +
		"\\begin{CodeChunk}\n" +
		"\\begin{CodeInput}\n" +
		"R> x <- c(1, 2, 3)\n" +
		"R> mean(x)\n" +
		"\\end{CodeInput}\n" +
		"\\begin{CodeOutput}\n" +
		"[1] 2\n" +
		"\\end{CodeOutput}\n" +
		"\\end{CodeChunk}\n" +
		"After"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)

	// The code and output survive verbatim (special characters as literals).
	for _, want := range []string{"c(1,2,3)", "mean(x)", "[1]2", "Before", "After"} {
		if !strings.Contains(txt, want) {
			t.Errorf("Code environment lost %q: %q", want, txt)
		}
	}
	// The environment/format machinery did not leak into the text.
	for _, garbage := range []string{"CodeInput", "CodeOutput", "CodeChunk", "Verbatim", "fontshape"} {
		if strings.Contains(txt, garbage) {
			t.Errorf("jss Code leaked %q into the text: %q", garbage, txt)
		}
	}
}

// A bare \begin{Code}…\end{Code} block (the third jss verbatim environment) is set
// verbatim on its own, and an optional fancyvrb [options] head is accepted.
func TestJSSCodeBareBlock(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{Code}[numbers=left]\n" +
		"alpha(1)\n" +
		"beta(2)\n" +
		"\\end{Code}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"alpha(1)", "beta(2)"} {
		if !strings.Contains(txt, want) {
			t.Errorf("Code block lost %q: %q", want, txt)
		}
	}
	if strings.Contains(txt, "numbers") {
		t.Errorf("Code block leaked its [options]: %q", txt)
	}
}
