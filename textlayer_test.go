// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A page whose glyphs are outlines carries its characters in an invisible <text>
// layer, so the document can still be searched, selected and read aloud. These
// tests pin the layer's contract; the proof that a real browser finds and copies
// it lives in the go-tex.github.io playground's browser suite.

// End-to-end: the layer names the words, in order, with the spaces between them.
func TestTextLayerCarriesTheWords(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(scaleMock{px: 10})
	if _, err := e.Run(`\noindent ab cd`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPage(2)
	if got := textLayerContent(svg); got != "ab cd" {
		t.Errorf("text layer = %q, want %q", got, "ab cd")
	}
}

// The layer is written BEFORE the outlines it describes, so a selection's
// background paints behind the glyphs instead of hiding them.
func TestTextLayerPrecedesTheOutlines(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(scaleMock{px: 10})
	if _, err := e.Run(`\noindent ab`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPage(2)
	text, path := strings.Index(svg, "<text"), strings.Index(svg, "<path")
	if text < 0 || path < 0 {
		t.Fatalf("expected both a <text> layer and a <path> outline: %s", svg)
	}
	if text > path {
		t.Errorf("<text> at %d comes after <path> at %d; the highlight would cover the glyphs", text, path)
	}
}

// A glyph the font cannot draw contributes no outline, so it contributes no
// searchable text either: the layer describes what is on the page.
func TestTextLayerSkipsUndrawnGlyphs(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{}) // glyphPathAt returns "" for every rune
	if _, err := e.Run(`\noindent ab`); err != nil {
		t.Fatal(err)
	}
	if svg := e.RenderPage(2); strings.Contains(svg, "<text") {
		t.Errorf("undrawn glyphs must not enter the text layer: %s", svg)
	}
}

// A word the ligature program folded is searchable by the letters a reader types.
func TestExpandLigature(t *testing.T) {
	cases := []struct {
		in   rune
		want string
	}{
		{ligFF, "ff"}, {ligFI, "fi"}, {ligFL, "fl"},
		{ligFFI, "ffi"}, {ligFFL, "ffl"},
		{'a', "a"}, {enDash, "–"}, {rsQuote, "’"},
	}
	for _, c := range cases {
		if got := string(expandLigature(c.in)); got != c.want {
			t.Errorf("expandLigature(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEscapeXMLText(t *testing.T) {
	if got := escapeXMLText("plain"); got != "plain" { // the no-escape fast path
		t.Errorf("escapeXMLText(plain) = %q", got)
	}
	if got := escapeXMLText(`a&b<c>d`); got != `a&amp;b&lt;c&gt;d` {
		t.Errorf("escapeXMLText = %q", got)
	}
}

func TestCharSizePt(t *testing.T) {
	if got := charSizePt(charNode{size: 17}, scaleMock{px: 10}); got != 17 {
		t.Errorf("explicit size = %v, want 17", got)
	}
	if got := charSizePt(charNode{}, scaleMock{px: 10}); got != 10 {
		t.Errorf("font size = %v, want 10", got)
	}
}

// The accumulator's own edges: an empty run writes nothing, a leading space is
// not a word separator, an unsized run falls back to a readable size and a
// zero-advance word is emitted without a meaningless textLength.
func TestTextRunEdges(t *testing.T) {
	var sb strings.Builder
	(&textRun{}).emit(&sb, &textCursor{}) // nothing accumulated
	if sb.String() != "" {
		t.Errorf("empty run emitted %q", sb.String())
	}

	sb.Reset()
	tr := &textRun{}
	tr.addSpace() // before any word: no separator is owed
	tr.addChar('a', 1, 2, 0)
	tr.emit(&sb, &textCursor{})
	if got := sb.String(); !strings.Contains(got, `font-size="10"`) {
		t.Errorf("unsized run should fall back to 10pt: %s", got)
	}
	if tr.words != nil || tr.space || tr.size != 0 {
		t.Errorf("emit must reset the run: %+v", tr)
	}

	sb.Reset()
	zero := &textRun{}
	zero.addChar('a', 5, 0, 11) // a zero-advance character
	zero.emit(&sb, &textCursor{})
	if got := sb.String(); strings.Contains(got, "textLength") {
		t.Errorf("a zero-width word must not be pinned: %s", got)
	}

	sb.Reset()
	blank := &textRun{words: []textWord{{x: 1}, {x: 2, runes: []rune("b")}}}
	blank.emit(&sb, &textCursor{}) // a word carrying no runes is skipped, the next one still emits
	if got := sb.String(); !strings.Contains(got, ">b</tspan>") {
		t.Errorf("word after an empty one lost: %s", got)
	}
}

// textLayerContent returns everything the invisible layers of an SVG spell out,
// the way a browser's text content or a screen reader would read the page.
func textLayerContent(svg string) string {
	var out strings.Builder
	rest := svg
	for {
		i := strings.Index(rest, "<text")
		if i < 0 {
			break
		}
		rest = rest[i:]
		j := strings.Index(rest, "</text>")
		if j < 0 {
			break
		}
		out.WriteString(stripTags(rest[:j]))
		rest = rest[j+len("</text>"):]
	}
	return strings.TrimSpace(out.String())
}

// stripTags drops every element tag, leaving the character data between them.
func stripTags(s string) string {
	var out strings.Builder
	for {
		i := strings.Index(s, "<")
		if i < 0 {
			out.WriteString(s)
			return out.String()
		}
		out.WriteString(s[:i])
		j := strings.Index(s[i:], ">")
		if j < 0 {
			return out.String()
		}
		s = s[i+j+1:]
	}
}

// The boundary between two runs is geometric: it exists where a reader can see
// a gap. Deciding from the kind of node that interrupted, or from the kind of
// glue around it, gets it wrong in both directions.
func TestTextCursorWantsSpace(t *testing.T) {
	cases := []struct {
		name          string
		cur           textCursor
		x, base, size float64
		want          bool
	}{
		{"first text on the page", textCursor{}, 10, 100, 10, false},
		{"continues where it stopped", textCursor{live: true, baseline: 100, endX: 20, size: 10}, 20, 100, 10, false},
		{"a kern-sized gap is not a space", textCursor{live: true, baseline: 100, endX: 20, size: 10}, 22, 100, 10, false},
		{"a visible gap is", textCursor{live: true, baseline: 100, endX: 20, size: 10}, 26, 100, 10, true},
		{"a new line, back to the margin", textCursor{live: true, baseline: 100, endX: 400, size: 10}, 70, 112, 10, true},
		{"a superscript keeps moving right", textCursor{live: true, baseline: 100, endX: 20, size: 10}, 20, 96, 7, false},
		{"the gap scales with the larger size", textCursor{live: true, baseline: 100, endX: 20, size: 24}, 25, 100, 10, false},
	}
	for _, c := range cases {
		if got := c.cur.wantsSpace(c.x, c.base, c.size); got != c.want {
			t.Errorf("%s: wantsSpace = %v, want %v", c.name, got, c.want)
		}
	}
}

// A word TeX broke across lines keeps its hyphen and gains no space, so
// "hyphen-ated" does not become "hyphen- ated".
func TestNoSpaceAfterAHyphenatedBreak(t *testing.T) {
	for _, r := range []rune{'-', 0x2010, 0x2011} {
		if !endsWithHyphen(r) {
			t.Errorf("%q must count as a break hyphen", r)
		}
	}
	if endsWithHyphen('a') {
		t.Error("an ordinary letter is not a break hyphen")
	}

	var sb strings.Builder
	cur := &textCursor{live: true, baseline: 100, endX: 400, size: 10, lastRune: '-'}
	run := &textRun{baseline: 112, size: 10}
	run.addChar('a', 70, 5, 10)
	run.emit(&sb, cur)
	if strings.Contains(sb.String(), "> a") {
		t.Errorf("a hyphenated break must not gain a space: %s", sb.String())
	}
}

// A run of nothing leaves the cursor where it was: an empty element must not
// teleport the next run's boundary decision.
func TestAdvanceIgnoresEmptyWords(t *testing.T) {
	cur := &textCursor{live: true, baseline: 5, endX: 9, size: 10}
	(&textRun{baseline: 50, words: []textWord{{x: 1}}}).advance(cur)
	if cur.baseline != 5 || cur.endX != 9 {
		t.Errorf("an empty run moved the cursor: %+v", cur)
	}
}

// End to end, over the real engine: the constructs that used to glue.
func TestWordBoundariesAcrossInterruptions(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"inline math", `The mass is $E = mc^2$ exactly.`, "The mass is exactly."},
		{"table cells", `\begin{tabular}{ll}alpha & beta\\ gamma & delta\end{tabular}`, "alpha beta gamma delta"},
		{"a boxed word", `a \fbox{boxed} word.`, "a boxed word."},
		{"a link", `see \href{http://x}{here} now.`, "see here now."},
		{"a footnote marker", `Text\footnote{a note}.`, "Text1."},
		{"an inline font change", `un\textbf{break}able works.`, "unbreakable works."},
		{"a list", `\begin{itemize}\item first \item second\end{itemize}`, "first"},
	}
	for _, c := range cases {
		doc := `\documentclass{article}\begin{document}` + c.src + `\end{document}`
		pages, _, err := CompileToSVGPagesDiag([]byte(doc), Options{Size: 11, Lenient: true})
		if err != nil || len(pages) == 0 {
			t.Errorf("%s: %v (%d pages)", c.name, err, len(pages))
			continue
		}
		if got := textLayerContent(string(pages[0])); !strings.Contains(got, c.want) {
			t.Errorf("%s: layer = %q, want it to contain %q", c.name, got, c.want)
		}
	}
}
