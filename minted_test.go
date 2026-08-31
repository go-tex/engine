// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The minted environment sets its body verbatim (minted's own no-Pygments
// fallback) — braces, $, %, # and backslashes are ordinary characters — while its
// [options] and {language} are consumed, not typeset. Without the handler the head
// and body would leak into the running text.
//
// Assertions avoid interior spaces (mvlText reads glyph nodes, not the verbatim
// inter-word glue) and avoid frame= (a frameNode is not walked by mvlText).
func TestMintedEnvironmentVerbatim(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "Before\n" +
		"\\begin{minted}[linenos]{python}\n" +
		"def f(x):\n" +
		"    return(x*2)#{braces}$math$100%\n" +
		"\\end{minted}\n" +
		"After"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)

	// The code is present verbatim (the special characters survived as literals).
	for _, want := range []string{"def", "f(x):", "return(x*2)", "{braces}", "$math$", "100%"} {
		if !strings.Contains(txt, want) {
			t.Errorf("minted body lost %q: %q", want, txt)
		}
	}
	// The head — options and language — did NOT leak into the text.
	for _, garbage := range []string{"linenos", "python"} {
		if strings.Contains(txt, garbage) {
			t.Errorf("minted leaked head token %q into the text: %q", garbage, txt)
		}
	}
	if !strings.Contains(txt, "Before") || !strings.Contains(txt, "After") {
		t.Errorf("surrounding prose lost: %q", txt)
	}
}

// \mintinline[opts]{language}{code} (and the equivalent \mint) set code inline in
// the tt font, consuming the language argument. The braced and the |delimiter|
// forms both work.
func TestMintinlineForms(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`A \mintinline{c}{int_y=3;} B \mint{ruby}|puts(42)| C`); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"int_y=3;", "puts(42)", "A", "B", "C"} {
		if !strings.Contains(txt, want) {
			t.Errorf("inline code lost %q: %q", want, txt)
		}
	}
	for _, garbage := range []string{"ruby"} {
		if strings.Contains(txt, garbage) {
			t.Errorf("inline minted leaked its language %q: %q", garbage, txt)
		}
	}
}

// lstlisting is unchanged by the shared-helper extraction: its body still sets
// verbatim with the [numbers=left] gutter.
func TestLstlistingStillWorksAfterExtraction(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{lstlisting}[numbers=left]\n" +
		"alpha\n" +
		"beta\n" +
		"\\end{lstlisting}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"alpha", "beta", "1", "2"} {
		if !strings.Contains(txt, want) {
			t.Errorf("lstlisting lost %q: %q", want, txt)
		}
	}
}
