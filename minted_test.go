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

// minted's \newminted{lang}{opts} is how a paper gets a code environment of its
// own: it defines <lang>code. One arXiv paper writes \newminted{jl}{…} and then 28
// jlcode blocks — and one of those blocks holds a lone $ in a shell path, which was
// enough to swallow the rest of the paper (#225).
func TestNewmintedDefinesItsEnvironment(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := "\\newminted{jl}{fontsize=\\footnotesize}\n" +
		"AVANT\n\\begin{jlcode}\npath = \"$w/x\"\n\\end{jlcode}\nAPRES\\par"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "APRES") {
		t.Errorf("the lone $ in the code swallowed the document: %q", txt)
	}
	if !strings.Contains(strings.ReplaceAll(txt, " ", ""), `path="$w/x"`) {
		t.Errorf("the code line is not set verbatim: %q", txt)
	}
}

// The optional argument names the environment instead of deriving it.
func TestNewmintedOptionalNameWins(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := "\\newminted[julia]{jl}{}\n\\begin{julia}\ncode_ici\n\\end{julia}\nAPRES\\par"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "code_ici") || !strings.Contains(txt, "APRES") {
		t.Errorf("the named environment did not set its body: %q", txt)
	}
}

// \newmintinline gives \<lang>inline, the inline sibling.
func TestNewmintinlineDefinesItsCommand(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\newmintinline{jl}{}avant \jlinline|code_ici()| apres\par`); err != nil {
		t.Fatal(err)
	}
	if txt := strings.ReplaceAll(mvlText(e.mvl), " ", ""); !strings.Contains(txt, "code_ici()") {
		t.Errorf("the inline command lost its code: %q", txt)
	}
}
