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
	l := newTextLayer()
	(&textRun{}).flush(l) // nothing accumulated
	if l.String() != "" {
		t.Errorf("empty run emitted %q", l.String())
	}

	l = newTextLayer()
	tr := &textRun{}
	tr.addSpace() // before any word: no separator is owed
	tr.addChar('a', 1, 2, 0)
	tr.flush(l)
	if got := l.String(); !strings.Contains(got, `font-size="10"`) {
		t.Errorf("unsized run should fall back to 10pt: %s", got)
	}
	if tr.words != nil || tr.space || tr.size != 0 {
		t.Errorf("emit must reset the run: %+v", tr)
	}

	l = newTextLayer()
	zero := &textRun{}
	zero.addChar('a', 5, 0, 11) // a zero-advance character
	zero.flush(l)
	if got := l.String(); strings.Contains(got, "textLength") {
		t.Errorf("a zero-width word must not be pinned: %s", got)
	}

	l = newTextLayer()
	blank := &textRun{words: []textWord{{x: 1}, {x: 2, runes: []rune("b")}}}
	blank.flush(l) // a word carrying no runes is skipped, the next one still emits
	if got := l.String(); !strings.Contains(got, ">b</tspan>") {
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

	l := newTextLayer()
	l.cur = textCursor{live: true, baseline: 100, endX: 400, size: 10, lastRune: '-'}
	run := &textRun{baseline: 112, size: 10}
	run.addChar('a', 70, 5, 10)
	run.flush(l)
	if strings.Contains(l.String(), "> <tspan") {
		t.Errorf("a hyphenated break must not gain a space: %s", l.String())
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
		// The formula now speaks its own source, so the words no longer join
		// across it — they join across what it says.
		{"inline math", `The mass is $E = mc^2$ exactly.`, "The mass is E = mc^2 exactly."},
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

// The whole page is ONE <text>, and that is the property search depends on: a
// browser's find does not match a phrase spanning two <text> elements, whatever
// their text content says. Measured in Chrome — the same words in two <text>
// are unfindable while the same words as two <tspan> of one <text> are found —
// so this is the assertion that keeps the page searchable across formulas,
// table cells and line breaks.
func TestPageIsOneTextElement(t *testing.T) {
	doc := `\documentclass{article}\begin{document}
Words before. The rest mass is $E = mc^2$ exactly, and a table:
\begin{tabular}{ll}alpha & beta\\ gamma & delta\end{tabular}
Words after, on another line entirely.
\end{document}`
	pages, _, err := CompileToSVGPagesDiag([]byte(doc), Options{Size: 11, Lenient: true})
	if err != nil || len(pages) == 0 {
		t.Fatalf("compile: %v (%d pages)", err, len(pages))
	}
	svg := string(pages[0])
	if n := strings.Count(svg, "<text"); n != 1 {
		t.Errorf("the page has %d <text> elements; a phrase cannot be found across two", n)
	}
	got := textLayerContent(svg)
	for _, want := range []string{
		"The rest mass is E = mc^2 exactly", // across a formula, which now speaks
		"alpha beta",                        // across two table cells
		"Words after, on another",           // across a line break
	} {
		if !strings.Contains(got, want) {
			t.Errorf("layer does not carry %q:\n%s", want, got)
		}
	}
}

// The text layer is written BEFORE the outlines, so a selection's background
// paints behind the glyphs instead of hiding the words it highlights.
func TestTextLayerPrecedesTheGlyphGroup(t *testing.T) {
	pages, _, err := CompileToSVGPagesDiag([]byte(`\documentclass{article}\begin{document}ab\end{document}`),
		Options{Size: 11, Lenient: true})
	if err != nil || len(pages) == 0 {
		t.Fatalf("compile: %v", err)
	}
	svg := string(pages[0])
	text, path := strings.Index(svg, "<text"), strings.Index(svg, "<path")
	if text < 0 || path < 0 {
		t.Fatalf("expected both a text layer and an outline")
	}
	if text > path {
		t.Errorf("<text> at %d comes after <path> at %d; the highlight would cover the glyphs", text, path)
	}
}

// Text under a matrix is PLACED by that matrix, so it cannot join the page's one
// <text>: it gets its own, inside the same transform. A rotated caption that
// joined the page chunk would be laid out at the wrong place entirely.
func TestTransformedTextGetsItsOwnChunk(t *testing.T) {
	l := newTextLayer()
	plain := &textRun{baseline: 10, size: 10}
	plain.addChar('a', 0, 5, 10)
	plain.flush(l)

	l.pushTransform("matrix(0,1,-1,0,0,0)")
	rot := &textRun{baseline: 20, size: 10}
	rot.addChar('b', 0, 5, 10)
	rot.flush(l)
	l.popTransform()

	after := &textRun{baseline: 30, size: 10}
	after.addChar('c', 0, 5, 10)
	after.flush(l)

	out := l.String()
	if n := strings.Count(out, "<text"); n != 3 {
		t.Errorf("expected three chunks (before, transformed, after), got %d: %s", n, out)
	}
	if !strings.Contains(out, `<g transform="matrix(0,1,-1,0,0,0)"><text`) {
		t.Errorf("the transformed chunk must sit inside its own <g>: %s", out)
	}
	// The cursor is reset across the boundary: a rotated caption is not a
	// continuation of the text beside it.
	if strings.Index(out, ">b</tspan>") < strings.Index(out, `<g transform`) {
		t.Errorf("the rotated run escaped its transform: %s", out)
	}
}

// A run can end holding a space: a source-line break closes the <g data-l>
// group mid-output-line, and the glue that separated the two words is still
// pending. That is exact information, and it has to outrank the geometric
// guess — justification can shrink an inter-word space below the gap a kern
// would occupy (2.5pt against a 2.75pt threshold, in the paragraph that found
// this), so the guess alone glued "every word" into "everyword".
func TestPendingSpaceOutranksTheGap(t *testing.T) {
	l := newTextLayer()
	first := &textRun{baseline: 100, size: 11}
	first.addChar('a', 0, 5, 11)
	first.addSpace() // glue arrived, then the group closed
	first.flush(l)
	if !l.cur.owed {
		t.Fatal("a run that ended holding glue must say so")
	}

	// The next run starts closer than the geometric threshold would accept.
	second := &textRun{baseline: 100, size: 11}
	second.addChar('b', 7, 5, 11)
	second.flush(l)

	if !strings.Contains(l.String(), "</tspan> <tspan") {
		t.Errorf("the pending space was dropped: %s", l.String())
	}
}

// A run that ended because something interrupted it owes nothing, and the
// geometric rule decides as before.
func TestNoPendingSpaceLeavesTheGapInCharge(t *testing.T) {
	l := newTextLayer()
	first := &textRun{baseline: 100, size: 11}
	first.addChar('a', 0, 5, 11) // no glue: a box interrupted it
	first.flush(l)
	if l.cur.owed {
		t.Fatal("a run interrupted without glue owes no space")
	}
	second := &textRun{baseline: 100, size: 11}
	second.addChar('b', 5.5, 5, 11) // abutting: a footnote marker, not a word
	second.flush(l)
	if strings.Contains(l.String(), "</tspan> <tspan") {
		t.Errorf("an abutting run must not gain a space: %s", l.String())
	}
}

// End to end: a paragraph whose SOURCE wraps mid-sentence reads as one sentence,
// justified spacing and all.
func TestSourceNewlineDoesNotGlueWords(t *testing.T) {
	doc := "\\documentclass{article}\\begin{document}\n" +
		"This paragraph is blitted pixels on a canvas, and every\nword of it can be found.\n" +
		"\\end{document}"
	pages, _, err := CompileToSVGPagesDiag([]byte(doc), Options{Size: 11, Lenient: true})
	if err != nil || len(pages) == 0 {
		t.Fatalf("compile: %v", err)
	}
	if got := textLayerContent(string(pages[0])); !strings.Contains(got, "every word of it can be found") {
		t.Errorf("layer = %q", got)
	}
}

// A formula's outlines carry no characters either, so the layer says what the
// formula IS: the source the author typed, and would type again to search for
// it. A spoken form is a separate, language-dependent problem.
func TestFormulaSourceIsSearchable(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"inline", `The mass is $E = mc^2$ exactly.`, "The mass is E = mc^2 exactly."},
		{"display", `A formula: \[ \sum_{i=1}^{n} x_i \]`, `\sum_{i=1}^{n} x_i`},
	}
	for _, c := range cases {
		doc := `\documentclass{article}\begin{document}` + c.src + `\end{document}`
		pages, _, err := CompileToSVGPagesDiag([]byte(doc), Options{Size: 11, Lenient: true})
		if err != nil || len(pages) == 0 {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := textLayerContent(string(pages[0])); !strings.Contains(got, c.want) {
			t.Errorf("%s: layer = %q, want it to contain %q", c.name, got, c.want)
		}
	}
}

// The tokeniser writes a space after every control sequence. That space is
// dropped where TeX never needed it and kept where it did — and the author's own
// spacing is never touched.
func TestCollapseSpace(t *testing.T) {
	cases := map[string]string{
		`\sum _{i=1}^{n}`: `\sum_{i=1}^{n}`, // the tokeniser's, before a non-letter
		`\alpha b`:        `\alpha b`,       // needed: the name would run on
		`E = mc^2`:        `E = mc^2`,       // the author's own, before a non-letter
		"  a \n b  ":      "a b",            // runs squeezed, ends trimmed
		`\frac {a}{b}`:    `\frac{a}{b}`,
		"":                "",
		` `:               "",
	}
	for in, want := range cases {
		if got := collapseSpace(in); got != want {
			t.Errorf("collapseSpace(%q) = %q, want %q", in, got, want)
		}
	}
	if endsControlSequence([]rune("abc")) {
		t.Error("plain letters do not end a control sequence")
	}
	if endsControlSequence([]rune(`\`)) {
		t.Error("a lone backslash names nothing")
	}
}

// A phrase with no width, or no text, contributes nothing rather than an
// unplaceable element.
func TestAddPhraseRefusesTheUnplaceable(t *testing.T) {
	l := newTextLayer()
	l.addPhrase("", 0, 10, 100, 10)
	l.addPhrase("x", 0, 0, 100, 10)
	l.addPhrase("   ", 0, 10, 100, 10)
	if got := l.String(); got != "" {
		t.Errorf("expected nothing, got %q", got)
	}
}

// A big display equation gets a layer size matching what it covers, so a
// selection over it is the size of the formula rather than of the body text.
func TestMathLayerSize(t *testing.T) {
	if got := mathLayerSize(mathNode{height: 20 * unity, depth: 4 * unity}); got != 24 {
		t.Errorf("mathLayerSize = %v, want 24", got)
	}
	if got := mathLayerSize(mathNode{}); got != 10 {
		t.Errorf("a zero-height formula must fall back to a readable size, got %v", got)
	}
}
