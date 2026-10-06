// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// listings' caption= and label= are CONTENT, not styling. The caption is
// numbered by listings' own \c@lstlisting and set ABOVE the block
// (listings.sty's default captionpos=t), and label= points at that number.
// Both were parsed away and dropped, so every \ref to a labelled listing
// printed "??" — 21 of them over 8 corpus papers, against 4 in the references.
//
// Every expectation is tectonic's, from the witness this was built on:
//
//	TOP
//	Listing 1: a listing, with a comma
//	code here
//	MID
//	Listing 2: no braces
//	other
//	A: 1 | listing 1 | Listing 1 | 2
func TestLstlistingCaptionAndLabel(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{lstlisting}[caption={a listing, with a comma},label={l:x}]
code here
\end{lstlisting}
\begin{lstlisting}[caption=no braces,label=l:y]
other
\end{lstlisting}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"l:x": "1", "l:y": "2"} {
		if got := e.labels[k]; got != want {
			t.Errorf("label[%q] = %q, want %q", k, got, want)
		}
	}
	// The reference TYPE is the counter's — "lstlisting" — and cleveref's own
	// alias is what turns it into a listing for \cref.
	if got, want := e.refTypes["l:x"], "lstlisting"; got != want {
		t.Errorf("type = %q, want %q", got, want)
	}
	if got, want := e.crefOne("l:x", false), "listing 1"; got != want {
		t.Errorf("cref = %q, want %q", got, want)
	}
	if got, want := e.crefOne("l:x", true), "Listing 1"; got != want {
		t.Errorf("Cref = %q, want %q", got, want)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	out := b.String()
	// The comma inside braces belongs to the caption. A plain strings.Split on
	// "," cut it in half and read "with a comma}" as another key — harmless while
	// the caption was ignored, and wrong the moment it is typeset.
	for _, want := range []string{"Listing1:alisting,withacomma", "Listing2:nobraces"} {
		if !strings.Contains(strings.ReplaceAll(out, " ", ""), want) {
			t.Errorf("caption %q missing from %q", want, out)
		}
	}
	// A literal tilde must not reach the page: the first version wrote
	// "Listing~1:" because the tie was a character token, not \nobreakspace.
	if strings.Contains(out, "~") {
		t.Errorf("a literal tilde reached the page: %q", out)
	}
}

// No caption=, no number: listings numbers only captioned listings, so a bare
// lstlisting must not step the counter — otherwise two unrelated blocks shift
// every later listing's number.
func TestLstlistingWithoutCaptionDoesNotNumber(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{lstlisting}
nothing to see
\end{lstlisting}
\begin{lstlisting}[caption=first numbered,label=l]
x
\end{lstlisting}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.labels["l"], "1"; got != want {
		t.Errorf("number = %q, want %q — an uncaptioned listing stepped the counter", got, want)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if strings.Count(b.String(), "Listing") != 1 {
		t.Errorf("captions: %q, want exactly one", b.String())
	}
}

// splitLstOptions splits on commas at BRACE LEVEL ZERO only.
func TestSplitLstOptions(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"a=1,b=2", []string{"a=1", "b=2"}},
		{"caption={x, y},label=z", []string{"caption={x, y}", "label=z"}},
		{"caption={a,b,c}", []string{"caption={a,b,c}"}},
		{"", []string{""}},
		{"numbers=left", []string{"numbers=left"}},
		// an unbalanced brace must not swallow the rest as one field
		{"caption={x,label=y", []string{"caption={x,label=y"}},
	} {
		got := splitLstOptions(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitLstOptions(%q) = %q, want %q", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitLstOptions(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// The aliases cleveref installs itself: a counter named with a package's internal
// spelling reports the type a reader knows it by (cleveref.sty:3123-3135).
func TestCrefTypeAliasesFromCleveref(t *testing.T) {
	for in, want := range map[string]string{
		"lstlisting":      "listing",
		"algocf":          "algorithm",
		"lstnumber":       "line",
		"algocfline":      "line",
		"AlgoLine":        "line",
		"IEEEsubequation": "subequation",
		"figure":          "figure", // untouched
		"theorem":         "theorem",
	} {
		if got := crefTypeFallback(in); got != want {
			t.Errorf("crefTypeFallback(%q) = %q, want %q", in, got, want)
		}
	}
}

// A TeX comment inside the option list runs to the end of the line and takes the
// newline with it — so it eats the key that follows. The block is read as RAW
// bytes, so the comment arrives intact; 2208.11395 writes
//
//	basicstyle=\tiny, %or \small or \footnotesize etc.
//	caption={"Task-pool", …},
//
// and its caption landed inside a segment whose key was
// "%or \small or \footnotesize etc." — no caption, no number, and the \ref to it
// printed "??". An escaped \% is not a comment.
func TestStripTeXComments(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a=1, %note\nb=2", "a=1, b=2"},
		{"a=1", "a=1"},
		{"a=1 %to the end", "a=1 "},
		{`a=100\%, b=2`, `a=100\%, b=2`},
		{"%whole line\nb=2", "b=2"},
		{"", ""},
	} {
		if got := stripTeXComments(c.in); got != c.want {
			t.Errorf("stripTeXComments(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// End to end: the caption after a comment is read, numbered and referable.
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\hsize=300pt\n\\begin{lstlisting}[\n  numbers=left, %or small\n  caption={apres un commentaire},\n  label=l\n]\nx\n\\end{lstlisting}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.labels["l"], "1"; got != want {
		t.Errorf("label = %q, want %q", got, want)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(strings.ReplaceAll(b.String(), " ", ""), "Listing1:apresuncommentaire") {
		t.Errorf("caption missing from %q", b.String())
	}
}
