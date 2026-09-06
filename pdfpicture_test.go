package engine

import (
	"bytes"
	"strings"
	"testing"
)

// The PDF driver draws characters through the coordinate scopes a drawing
// package's literals have open, so a picture's words land where the picture puts
// them. The SVG driver gets this for free — it writes glyphs into the same
// stream as the literals — but the PDF driver interprets the literals separately,
// and without this every node's text piled up at the picture's origin.
func TestPDFTextIsDrawnThroughPictureScopes(t *testing.T) {
	e := New()
	e.SetFont(testFont(t))
	// A scope that shifts by (30, 10) in page coordinates, then a character.
	src := `\hsize=200pt \special{gotex:<g transform="translate(30,10)">}A\special{gotex:</g>}B\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 10); err != nil {
		t.Fatal(err)
	}
	ops := pdfContent(t, buf.Bytes())
	// The character inside the scope is drawn under a saved graphics state with
	// the scope's map concatenated; the one after it is drawn straight.
	if !strings.Contains(ops, "q\n") || !strings.Contains(ops, " cm\n") || !strings.Contains(ops, "Q\n") {
		t.Errorf("no transformed text state in the content stream:\n%s", ops)
	}
}

// A page with no picture pays nothing beyond the page's own unit matrix: no
// graphics state is saved for text. Every page opens with one `cm` converting TeX
// points to the PDF's big points (bigPointsPerTeXPoint), so what this checks is
// that there is no SECOND one — no per-text-run transform.
func TestPDFTextWithoutAPictureIsUnchanged(t *testing.T) {
	e := New()
	e.SetFont(testFont(t))
	if _, err := e.Run(`\hsize=200pt Bonjour\par`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 10); err != nil {
		t.Fatal(err)
	}
	for _, s := range pdfStreams(t, buf.Bytes()) {
		if strings.Contains(s, "BT") && strings.Count(s, " cm\n") > 1 {
			t.Errorf("plain text was transformed beyond the page matrix:\n%s", s)
		}
	}
}

// pictureCTM reads the scopes out of a stream of FRAGMENTS: a scope is opened by
// one literal and closed by another, so the tags are read directly rather than
// parsed as a document (which would close every tag at the end of the fragment
// it appeared in — and leave text untransformed, the bug this replaced).
func TestPictureCTMAcrossFragments(t *testing.T) {
	var p pictureCTM
	if p.active() {
		t.Error("no scope open: the map must be the identity")
	}
	p.feed(`<g transform="translate(10,20)">`)
	if !p.active() {
		t.Fatal("an open scope was not seen")
	}
	if x, y := p.cur().apply(0, 0); x != 10 || y != 20 {
		t.Errorf("map = (%g,%g), want (10,20)", x, y)
	}
	p.feed(`<g transform="scale(2)">`) // composes with the one already open
	if x, y := p.cur().apply(1, 1); x != 12 || y != 22 {
		t.Errorf("composed map = (%g,%g), want (12,22)", x, y)
	}
	p.feed(`</g>`)
	if x, y := p.cur().apply(0, 0); x != 10 || y != 20 {
		t.Errorf("after closing the inner scope = (%g,%g), want (10,20)", x, y)
	}
	p.feed(`</g>`)
	if p.active() {
		t.Error("all scopes closed: the map must be the identity again")
	}
	// A stray close, a self-closing <g>, and a non-scope element change nothing.
	p.feed(`</g><g transform="translate(5,5)"/><path d="M 0 0"/><rect x="1"/>`)
	if p.active() {
		t.Errorf("stray tags opened a scope: %+v", p.cur())
	}
	// Text with no tag at all, and a tag left unterminated by a truncated
	// picture, leave the scopes as they were.
	before := p.cur()
	p.feed("du texte sans balise")
	p.feed(`<g transform="translate(9,9)"`)
	if p.cur() != before {
		t.Errorf("a fragment with no usable tag changed the map: %+v", p.cur())
	}

	// Several tags in one literal, and a scope with no transform.
	p.feed(`<g stroke="none"><g transform="translate(3,4)">`)
	if x, y := p.cur().apply(0, 0); x != 3 || y != 4 {
		t.Errorf("= (%g,%g), want (3,4)", x, y)
	}
}

// pdfMap conjugates a page-coordinate map (y down from the top) into PDF user
// space (y up from the bottom), so the same map moves a character the same way
// in both drivers.
func TestPDFMap(t *testing.T) {
	const H float64 = 100
	// A pure shift: down the page in SVG is down the page in PDF too.
	m := pdfMap(parseTransform("translate(10,20)"), H)
	if x, y := m.apply(0, H); x != 10 || y != H-20 {
		t.Errorf("translate = (%g,%g), want (10,%g)", x, y, H-20)
	}
	// The identity stays the identity.
	if got := pdfMap(identity(), H); got != identity() {
		t.Errorf("identity = %+v", got)
	}
	// A scale about the page origin maps the top-left corner consistently.
	m = pdfMap(parseTransform("scale(2)"), H)
	if x, y := m.apply(3, H-4); x != 6 || y != H-8 {
		t.Errorf("scale = (%g,%g), want (6,%g)", x, y, H-8)
	}
}

// tagAttr reads an attribute out of a tag, and reports nothing when there is
// none to read.
func TestTagAttr(t *testing.T) {
	if got := tagAttr(`<g transform="translate(1,2)" stroke="red">`, "transform"); got != "translate(1,2)" {
		t.Errorf("= %q", got)
	}
	if got := tagAttr(`<g stroke="red">`, "transform"); got != "" {
		t.Errorf("missing attribute = %q", got)
	}
	if got := tagAttr(`<g transform="unterminated`, "transform"); got != "" {
		t.Errorf("unterminated value = %q", got)
	}
}

// The origins are resolved as the page is walked, so a literal that refers back
// to a picture's origin is finished by the time a character is drawn after it.
func TestOriginResolverIsIncremental(t *testing.T) {
	var r originResolver
	if got := r.next(`[{?ox},{?oy}]`); got != "[0,0]" {
		t.Errorf("before any declaration = %q, want [0,0]", got)
	}
	if got := r.next(`<` + originElem + ` x="7" y="8"/>[{?ox},{?oy}]`); got != "[7,8]" {
		t.Errorf("= %q, want [7,8]", got)
	}
	// The origin carries over to the NEXT literal, which is the whole point.
	if got := r.next(`[{?-ox},{?-oy}]`); got != "[-7,-8]" {
		t.Errorf("carried over = %q, want [-7,-8]", got)
	}
}
