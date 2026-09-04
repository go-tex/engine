// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// mvlBoxes returns the boxes contributed directly to a vertical list (the line
// boxes of an unframed listing, or the single wrapper box of a framed one).
func mvlBoxes(nodes []node) []*boxNode {
	var out []*boxNode
	for _, n := range nodes {
		if b, ok := n.(*boxNode); ok {
			out = append(out, b)
		}
	}
	return out
}

// findFrames collects every frameNode reachable through the box tree.
func findFrames(nodes []node) []frameNode {
	var out []frameNode
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case frameNode:
				out = append(out, c)
				walk(c.inner.list)
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(nodes)
	return out
}

// firstChar returns the first charNode reached in a box (depth-first), or 0.
func firstChar(b *boxNode) rune {
	for _, n := range b.list {
		switch c := n.(type) {
		case charNode:
			return c.ch
		case *boxNode:
			if r := firstChar(c); r != 0 {
				return r
			}
		}
	}
	return 0
}

// A lstlisting block sets each raw line literally: leading indentation survives as
// fixed-width space kerns, {, }, $, %, \ are ordinary characters, blank lines are
// kept, and the line count matches the source.
func TestLstlistingBlock(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{lstlisting}\n" +
		"  a{b}$c%d\\e\n" + // 2 leading spaces; literal { } $ % \
		"\n" + // a blank line, preserved
		"second()\n" +
		"\\end{lstlisting}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := mvlText(e.mvl), `a{b}$c%d\esecond()`; got != want {
		t.Errorf("lstlisting typeset %q, want %q", got, want)
	}
	boxes := mvlBoxes(e.mvl)
	if len(boxes) != 3 { // three content lines (incl. the blank one)
		t.Fatalf("lstlisting produced %d line boxes, want 3", len(boxes))
	}
	// The two leading spaces became two space-width kerns at the head of line 1.
	space := spMock{}.spaceSP().width
	for i := 0; i < 2; i++ {
		k, ok := boxes[0].list[i].(kernNode)
		if !ok {
			t.Fatalf("line 1 node %d = %T, want kernNode (indentation)", i, boxes[0].list[i])
		}
		if k.width != space {
			t.Errorf("indent kern %d width = %d, want %d", i, k.width, space)
		}
	}
	// No frame requested → no frameNode anywhere.
	if fs := findFrames(e.mvl); len(fs) != 0 {
		t.Errorf("unframed lstlisting produced %d frames, want 0", len(fs))
	}
}

// numbers=left prepends a right-aligned line number to every line; numbers=none
// (and the default) prepend nothing.
func TestLstlistingLineNumbers(t *testing.T) {
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
	boxes := mvlBoxes(e.mvl)
	if len(boxes) != 2 {
		t.Fatalf("got %d line boxes, want 2", len(boxes))
	}
	if r := firstChar(boxes[0]); r != '1' {
		t.Errorf("line 1 starts with %q, want '1' (line number)", r)
	}
	if r := firstChar(boxes[1]); r != '2' {
		t.Errorf("line 2 starts with %q, want '2' (line number)", r)
	}
	if got, want := mvlText(e.mvl), "1alpha2beta"; got != want {
		t.Errorf("numbered listing text = %q, want %q", got, want)
	}

	// numbers=none: no leading digit, the code starts directly.
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run("\\begin{lstlisting}[numbers=none]\nalpha\n\\end{lstlisting}"); err != nil {
		t.Fatal(err)
	}
	if r := firstChar(mvlBoxes(e2.mvl)[0]); r != 'a' {
		t.Errorf("numbers=none line starts with %q, want 'a'", r)
	}
}

// frame=single wraps the whole block in a single frameNode whose inner vbox holds
// every code line.
func TestLstlistingFrame(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{lstlisting}[frame=single]\n" +
		"one\n" +
		"two\n" +
		"\\end{lstlisting}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	frames := findFrames(e.mvl)
	if len(frames) != 1 {
		t.Fatalf("frame=single produced %d frames, want 1", len(frames))
	}
	fr := frames[0]
	if fr.rule != fboxRule || fr.sep != fboxSep {
		t.Errorf("frame rule/sep = %d/%d, want %d/%d", fr.rule, fr.sep, fboxRule, fboxSep)
	}
	if fr.inner.kind != vbox {
		t.Errorf("frame inner kind = %d, want vbox", fr.inner.kind)
	}
	inner := mvlBoxes(fr.inner.list)
	if len(inner) != 2 {
		t.Errorf("framed block holds %d line boxes, want 2", len(inner))
	}
	// The framed block sits on the page as a single wrapper box.
	if got := len(mvlBoxes(e.mvl)); got != 1 {
		t.Errorf("framed block contributed %d top-level boxes, want 1", got)
	}
	// The two code lines carry their glyphs inside the frame's inner vbox.
	if got, want := mvlText(inner[0].list)+mvlText(inner[1].list), "onetwo"; got != want {
		t.Errorf("framed listing text = %q, want %q", got, want)
	}
}

// frame=single AND numbers=left combine: a framed block whose lines carry numbers.
// The option bracket is written after a space (\begin{lstlisting} [...]) to
// exercise the leading-space skip of the raw option scanner, and \baselineskip is
// forced tiny so the interline gap in the framed vbox hits its \lineskip floor.
func TestLstlistingFrameAndNumbers(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\baselineskip=0pt\\begin{lstlisting} [frame=single,numbers=left]\n" +
		"x\n" +
		"y\n" +
		"\\end{lstlisting}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	frames := findFrames(e.mvl)
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	inner := mvlBoxes(frames[0].inner.list)
	if len(inner) != 2 {
		t.Fatalf("framed block holds %d lines, want 2", len(inner))
	}
	if r := firstChar(inner[0]); r != '1' {
		t.Errorf("framed line 1 starts with %q, want '1'", r)
	}
	// With \baselineskip 0, the vbox interline glue is the \lineskip floor (>0).
	g, ok := frames[0].inner.list[1].(glueNode)
	if !ok || g.spec.width <= 0 {
		t.Errorf("interline node = %#v, want a positive \\lineskip glue", frames[0].inner.list[1])
	}
}

// Unknown keys (language, caption, basicstyle, …) are accepted and ignored: the
// block still renders, with no frame and no numbers. language= is deliberately not
// colourised (highlighting is out of scope).
func TestLstlistingUnknownKeysIgnored(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{lstlisting}[language=Go,caption=Hi,basicstyle=x]\n" +
		"code\n" +
		"more\n" +
		"\\end{lstlisting}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := len(mvlBoxes(e.mvl)); got != 2 {
		t.Errorf("got %d line boxes, want 2", got)
	}
	if fs := findFrames(e.mvl); len(fs) != 0 {
		t.Errorf("unknown-key listing produced %d frames, want 0", len(fs))
	}
	if r := firstChar(mvlBoxes(e.mvl)[0]); r != 'c' { // 'code', no line number prefixed
		t.Errorf("line starts with %q, want 'c'", r)
	}
}

// \lstinline sets delimited text literally inline, like \verb: specials are
// ordinary, and both the char-delimiter and the brace forms work, with or without
// an optional [opts].
func TestLstinline(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\noindent\lstinline|a\b{}$|`, `a\b{}$`},        // char delimiter
		{`\noindent\lstinline{a b c}`, `abc`},            // brace form (spaces are kerns)
		{`\noindent\lstinline[language=Go]|x%y|`, `x%y`}, // with ignored options
		{`\lstinline|z|`, `z`},                           // starts its own paragraph in vertical mode
	}
	for _, c := range cases {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(c.src); err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		if got := mvlText(e.mvl); got != c.want {
			t.Errorf("%q typeset %q, want %q", c.src, got, c.want)
		}
	}
}

// A missing \end{lstlisting}, empty [] options and an empty body must not panic.
func TestLstlistingMalformed(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// No closing \end: the rest of the input is taken as the body.
	if _, err := e.Run("\\begin{lstlisting}[]\nunterminated\ncode"); err != nil {
		t.Fatal(err)
	}
	if len(mvlBoxes(e.mvl)) != 2 {
		t.Errorf("unterminated listing produced %d boxes, want 2", len(mvlBoxes(e.mvl)))
	}

	// Empty body between \begin and \end: one (empty) line box, no panic.
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run("\\begin{lstlisting}\n\\end{lstlisting}"); err != nil {
		t.Fatal(err)
	}
}

// \lstinline with no delimiter at end of input, and a listing/inline run with no
// bound font, return gracefully rather than panicking.
func TestLstEdgeCases(t *testing.T) {
	// \lstinline at the very end of the input: nothing after the command.
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\lstinline`); err != nil {
		t.Fatal(err)
	}

	// No font bound (SetFont never called): both forms return early, no boxes.
	e2 := New()
	e2.LoadLaTeX()
	if _, err := e2.Run("\\begin{lstlisting}\nx\n\\end{lstlisting}"); err != nil {
		t.Fatal(err)
	}
	if len(mvlBoxes(e2.mvl)) != 0 {
		t.Errorf("no-font listing produced %d boxes, want 0", len(mvlBoxes(e2.mvl)))
	}
	e3 := New()
	e3.LoadLaTeX()
	if _, err := e3.Run(`\noindent\lstinline|x|`); err != nil {
		t.Fatal(err)
	}
}

// parseLstOptions honours numbers/frame and ignores everything else, treating a
// literal "none" (and a valueless key) as off.
func TestParseLstOptions(t *testing.T) {
	cases := []struct {
		in                 string
		wantNum, wantFrame bool
	}{
		{"", false, false},
		{"  ", false, false},
		{"numbers=left", true, false},
		{"numbers=none", false, false},
		{"numbers", false, false}, // key with no value → off
		{"frame=single", false, true},
		{"frame=none", false, false},
		{"language=Go,caption=Hi", false, false}, // unknown keys
		{" numbers = left , frame = single ", true, true},
		{"numbers=right,frame=lines", true, true}, // any non-none value → on
	}
	for _, c := range cases {
		o := parseLstOptions(c.in)
		if o.numbers != c.wantNum || o.frame != c.wantFrame {
			t.Errorf("parseLstOptions(%q) = {num:%v frame:%v}, want {num:%v frame:%v}",
				c.in, o.numbers, o.frame, c.wantNum, c.wantFrame)
		}
	}
}

// Verbatim glyph source-line stamping carries over to lstlisting (click-to-source):
// \begin is line 1, the two code lines are 2 and 3.
func TestLstlistingSourceLines(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{lstlisting}\nalpha\nbeta\n\\end{lstlisting}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	got := charLine(e.mvl)
	if got['a'] != 2 {
		t.Errorf("glyph 'a' source line = %d, want 2", got['a'])
	}
	if got['b'] != 3 {
		t.Errorf("glyph 'b' source line = %d, want 3", got['b'])
	}
}

// A verbatim environment inside a minipage is not read from the character buffer:
// the minipage captured its whole body as tokens first, so the buffer's cursor is
// already past \end{minipage}. Reading it there copied the document that FOLLOWS —
// \end{minipage}, the text after it and \end{document} all vanished into the
// listing, which was then printed at the end of the page.
func TestListingInsideMinipageKeepsTheDocument(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass{article}\begin{document}` +
		"\\begin{minipage}{200pt}\n\\begin{lstlisting}\ncode ici\n\\end{lstlisting}\n\\end{minipage}\n" +
		`APRES\par\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "APRES") {
		t.Errorf("the text after the minipage was swallowed: %q", txt)
	}
	if strings.Contains(txt, `\end{document}`) {
		t.Errorf("\\end{document} was typeset as text: %q", txt)
	}
	if !strings.Contains(strings.ReplaceAll(txt, " ", ""), "codeici") {
		t.Errorf("the listing's own content is missing: %q", txt)
	}
}

// The same environment OUTSIDE any captured body still reads the character buffer,
// where the body is verbatim to the character — indentation included.
func TestListingOutsideACapturedBodyStaysVerbatim(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass{article}\begin{document}` +
		"\\begin{lstlisting}\n    indente\n\\end{lstlisting}\n" +
		`APRES\par\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "indente") || !strings.Contains(txt, "APRES") {
		t.Errorf("plain listing lost content: %q", txt)
	}
}
