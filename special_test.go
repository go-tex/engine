package engine

import (
	"bytes"
	"compress/zlib"
	"io"
	"strings"
	"testing"

	texmath "github.com/go-tex/math"
)

// testFont is the built-in OFL font as an embeddable face, so a PDF test needs no
// system font (and runs on every CI platform).
func testFont(t *testing.T) fontFace {
	t.Helper()
	f, err := NewOpenTypeFont(texmath.DefaultFont(), 10)
	if err != nil {
		t.Fatalf("default font: %v", err)
	}
	return f
}

// pdfContent decodes every stream in a PDF and returns the concatenated text, so
// a test can assert on the actual imaging operators the driver emitted rather
// than merely that a PDF was produced.
func pdfContent(t *testing.T, pdf []byte) string {
	t.Helper()
	return strings.Join(pdfStreams(t, pdf), "\n")
}

// pdfStreams decodes every stream in a PDF, one entry per stream.
func pdfStreams(t *testing.T, pdf []byte) []string {
	t.Helper()
	var out []string
	rest := pdf
	for {
		i := bytes.Index(rest, []byte("stream"))
		if i < 0 {
			break
		}
		body := rest[i+len("stream"):]
		body = bytes.TrimPrefix(bytes.TrimPrefix(body, []byte("\r")), []byte("\n"))
		j := bytes.Index(body, []byte("endstream"))
		if j < 0 {
			break
		}
		raw := body[:j]
		rest = body[j:]
		if zr, err := zlib.NewReader(bytes.NewReader(raw)); err == nil {
			if dec, err := io.ReadAll(zr); err == nil {
				out = append(out, string(dec))
				continue
			}
		}
		out = append(out, string(raw))
	}
	return out
}

// firstSpecial returns the first specialNode reachable in a node tree.
func firstSpecial(nodes []node) (specialNode, bool) {
	for _, n := range nodes {
		switch c := n.(type) {
		case specialNode:
			return c, true
		case *boxNode:
			if s, ok := firstSpecial(c.list); ok {
				return s, true
			}
		}
	}
	return specialNode{}, false
}

// \special stores its expanded text as a whatsit in the current list.
func TestSpecialExpandsAndPlaces(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\def\w{7}\special{gotex:<rect width="\w"/>}`); err != nil {
		t.Fatal(err)
	}
	s, ok := firstSpecial(e.mvl)
	if !ok {
		t.Fatal("no specialNode placed")
	}
	if want := `gotex:<rect width="7"/>`; s.text != want {
		t.Errorf("special text = %q, want %q", s.text, want)
	}
}

// A \special contributes no width, height or depth to the box carrying it: the
// hbox it rides in measures exactly as it would without the special.
func TestSpecialHasNoDimensions(t *testing.T) {
	withSpecial := New()
	withSpecial.SetFont(spMock{})
	if _, err := withSpecial.Run(`\setbox0=\hbox{A\special{gotex:<rect/>}B}`); err != nil {
		t.Fatal(err)
	}
	plain := New()
	plain.SetFont(spMock{})
	if _, err := plain.Run(`\setbox0=\hbox{AB}`); err != nil {
		t.Fatal(err)
	}
	a, b := withSpecial.getBox(0), plain.getBox(0)
	if a == nil || b == nil {
		t.Fatal("box 0 is void")
	}
	if a.width != b.width || a.height != b.height || a.depth != b.depth {
		t.Errorf("special changed the box: %d/%d/%d, want %d/%d/%d",
			a.width, a.height, a.depth, b.width, b.height, b.depth)
	}
}

// specialLiteral resolves the position placeholders and rejects other drivers.
func TestSpecialLiteral(t *testing.T) {
	got, ok := specialLiteral(`gotex:<g transform="translate({?x},{?y})">{?nl}`, 12.5, 30)
	if !ok {
		t.Fatal("gotex: literal not recognised")
	}
	if want := "<g transform=\"translate(12.5,30)\">\n"; got != want {
		t.Errorf("literal = %q, want %q", got, want)
	}
	if _, ok := specialLiteral("dvips: ps: 0 0 moveto", 0, 0); ok {
		t.Error("a foreign driver's special must not be drawn")
	}
}

// The SVG driver writes the literal verbatim at the special's reference point, so
// a scope opened by one special brackets the material painted after it.
func TestSpecialInSVGOutput(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	src := `\special{gotex:<g stroke="red">}` +
		`\special{gotex:<path d="M 0 0 L {?x} {?y}"/>}` +
		`\special{gotex:</g>}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPage(10)
	if !strings.Contains(svg, `<g stroke="red">`) || !strings.Contains(svg, `</g>`) {
		t.Errorf("scope not emitted: %s", svg)
	}
	if !strings.Contains(svg, `<path d="M 0 0 L 10 10"/>`) {
		t.Errorf("placeholders not resolved at the reference point: %s", svg)
	}
	if strings.Contains(svg, "{?x}") {
		t.Errorf("unresolved placeholder in output: %s", svg)
	}
	// The scope must open before the path and close after it.
	iOpen := strings.Index(svg, `<g stroke="red">`)
	iPath := strings.Index(svg, `<path d="M 0 0`)
	iClose := strings.Index(svg, `</g>`)
	if !(iOpen < iPath && iPath < iClose) {
		t.Errorf("literals out of paint order: open=%d path=%d close=%d", iOpen, iPath, iClose)
	}
}

// A special in a driver this engine does not implement is carried but drawn by
// nobody — the SVG output is exactly what it would be without it.
func TestForeignSpecialNotDrawn(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`A\special{ps: 0 0 moveto}B`); err != nil {
		t.Fatal(err)
	}
	if svg := e.RenderPage(10); strings.Contains(svg, "moveto") {
		t.Errorf("foreign special leaked into the SVG: %s", svg)
	}
}

// The PDF driver interprets the same literals as real vector operators: a stroked
// path reaches the page's content stream with its colour, width and geometry.
func TestSpecialInPDFOutput(t *testing.T) {
	e := New()
	e.SetFont(testFont(t))
	// The picture rides in a paragraph, as a drawing package's box would.
	src := `\hsize=200pt Hi\special{gotex:<path d="M 0 0 L 20 20" stroke="red" ` +
		`stroke-width="2" fill="none"/>}\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 10); err != nil {
		t.Fatal(err)
	}
	ops := ""
	for _, s := range pdfStreams(t, buf.Bytes()) {
		if strings.Contains(s, "RG") {
			ops = s
		}
	}
	if ops == "" {
		t.Fatal("no page content stream carries the special's stroke")
	}
	for _, want := range []string{"1 0 0 RG", "2 w", " m\n", " l\n", "S"} {
		if !strings.Contains(ops, want) {
			t.Errorf("content stream lacks %q:\n%s", want, ops)
		}
	}
}

// TeX doubles a macro-parameter character when it writes a token list out, so a
// literal '#' (SVG's colour and fragment-reference sigil) must reach \special as
// an "other" character — via \string# or a catcode-12 definition, which is what a
// driver written for this engine does. The doubling is TeX's, not the driver's.
func TestSpecialHashNeedsOtherCatcode(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\special{gotex:a#b}\special{gotex:c\string#d}`); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, n := range e.mvl {
		if s, ok := n.(specialNode); ok {
			texts = append(texts, s.text)
		}
	}
	if len(texts) != 2 {
		t.Fatalf("got %d specials, want 2: %q", len(texts), texts)
	}
	if texts[0] != "gotex:a##b" {
		t.Errorf("a catcode-6 # = %q, want it doubled as TeX writes it", texts[0])
	}
	if texts[1] != "gotex:c#d" {
		t.Errorf(`\string# = %q, want a single # in the literal`, texts[1])
	}
}

// \special without a braced argument consumes nothing and places no node, so a
// malformed source keeps typesetting instead of swallowing what follows.
func TestSpecialWithoutGroup(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\special AB`); err != nil {
		t.Fatal(err)
	}
	if _, ok := firstSpecial(e.mvl); ok {
		t.Error("a special was placed without an argument")
	}
	if got := glyphString(e.mvl); got != "AB" {
		t.Errorf("typeset %q, want AB", got)
	}
	if _, err := New().Run(`\special`); err != nil { // at end of input
		t.Fatal(err)
	}
}

// A special in a vertical list is drawn at the vertical cursor, so a picture
// between two paragraphs lands where the page builder put it.
func TestSpecialInVerticalList(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=100pt \baselineskip=10pt A\par\special{gotex:<rect x="{?x}" y="{?y}"/>}B\par`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPage(0)
	if !strings.Contains(svg, `<rect x="0" y=`) {
		t.Errorf("special not placed at the vertical cursor: %s", svg)
	}
}
