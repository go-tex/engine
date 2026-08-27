// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"os"
	"testing"
)

// pdfFontOrSkip returns a system font path suitable for PDF embedding, skipping the
// test when none is available (the SVG driver, not PDF, carries the live links).
func pdfFontOrSkip(t *testing.T) string {
	t.Helper()
	fp := "/System/Library/Fonts/Supplemental/Georgia.ttf"
	if _, err := os.Stat(fp); err != nil {
		t.Skip("no system font")
	}
	return fp
}

// firstInternalLink returns the first internalLinkNode reachable in a node tree,
// descending through boxes and into an internal link's own inner box (an internal
// link may be wrapped in a line hbox by the paragraph builder).
func firstInternalLink(nodes []node) (internalLinkNode, bool) {
	for _, n := range nodes {
		switch c := n.(type) {
		case internalLinkNode:
			return c, true
		case *boxNode:
			if in, ok := firstInternalLink(c.list); ok {
				return in, true
			}
		}
	}
	return internalLinkNode{}, false
}

// glyphs concatenates every charNode rune in a node tree, descending through boxes
// and into an internal link's inner box (a local variant of glyphString that also
// enters internalLinkNode content).
func glyphs(nodes []node) string {
	var s []rune
	for _, n := range nodes {
		switch c := n.(type) {
		case charNode:
			s = append(s, c.ch)
		case *boxNode:
			s = append(s, []rune(glyphs(c.list))...)
		case internalLinkNode:
			s = append(s, []rune(glyphs(c.inner.list))...)
		}
	}
	return string(s)
}

// internalLinkNode satisfies the node interface (a marker method).
func TestInternalLinkIsNode(t *testing.T) {
	var n node = internalLinkNode{}
	n.isNode()
}

// \hypertarget{name}{text} composes text normally and marks it as a named
// destination: the node carries the name, is flagged as a target and holds the
// text glyphs, and the SVG driver wraps it in <g id="name">.
func TestHypertargetNode(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hypertarget{t}{X}`); err != nil {
		t.Fatal(err)
	}
	in, ok := firstInternalLink(e.mvl)
	if !ok {
		t.Fatal("no internalLinkNode placed")
	}
	if !in.target {
		t.Error("hypertarget node must be a target")
	}
	if in.name != "t" {
		t.Errorf("name = %q, want %q", in.name, "t")
	}
	if got := glyphs([]node{in}); got != "X" {
		t.Errorf("target text = %q, want %q", got, "X")
	}
	svg := e.RenderPage(2)
	if !contains(svg, `<g id="t">`) {
		t.Errorf("SVG missing target anchor <g id=\"t\">: %s", svg)
	}
}

// \hyperlink{name}{text} composes text normally and makes it a same-document link:
// the node carries the name, is NOT a target and holds the text glyphs, and the SVG
// driver wraps it in <a href="#name"> (no target=_blank).
func TestHyperlinkNode(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hyperlink{t}{go}`); err != nil {
		t.Fatal(err)
	}
	in, ok := firstInternalLink(e.mvl)
	if !ok {
		t.Fatal("no internalLinkNode placed")
	}
	if in.target {
		t.Error("hyperlink node must not be a target")
	}
	if in.name != "t" {
		t.Errorf("name = %q, want %q", in.name, "t")
	}
	if got := glyphs([]node{in}); got != "go" {
		t.Errorf("link text = %q, want %q", got, "go")
	}
	svg := e.RenderPage(2)
	if !contains(svg, `<a href="#t">`) {
		t.Errorf("SVG missing internal link <a href=\"#t\">: %s", svg)
	}
	if contains(svg, `href="#t" target`) {
		t.Errorf("internal link must not carry target=_blank: %s", svg)
	}
	if !contains(svg, `</a>`) {
		t.Errorf("SVG missing </a> close: %s", svg)
	}
}

// An internalLinkNode is dimensionally transparent: its reference-point width,
// height and depth are exactly its inner box's, so it packs and breaks like the
// content it wraps. With spMock (5pt/7pt/2pt per glyph) "go" is 10pt wide.
func TestInternalLinkDims(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hyperlink{t}{go}`); err != nil {
		t.Fatal(err)
	}
	in, ok := firstInternalLink(e.mvl)
	if !ok {
		t.Fatal("no internalLinkNode placed")
	}
	if in.width() != in.inner.width || in.height() != in.inner.height || in.depth() != in.inner.depth {
		t.Errorf("node dims (%d,%d,%d) != inner (%d,%d,%d)",
			in.width(), in.height(), in.depth(), in.inner.width, in.inner.height, in.inner.depth)
	}
	if in.width() != 10*unity || in.height() != 7*unity || in.depth() != 2*unity {
		t.Errorf("dims = (%d,%d,%d), want (%d,%d,%d)",
			in.width(), in.height(), in.depth(), 10*unity, 7*unity, 2*unity)
	}
}

// An internal link nested inside running paragraph text is still placed as an
// internalLinkNode (the paragraph builder wraps it in a line hbox) and keeps its
// name and glyphs, and the surrounding words typeset around it.
func TestInternalLinkInParagraph(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`ab \hyperlink{t}{go} cd`); err != nil {
		t.Fatal(err)
	}
	in, ok := firstInternalLink(e.mvl)
	if !ok {
		t.Fatal("no internalLinkNode placed in paragraph")
	}
	if in.name != "t" || in.target {
		t.Errorf("nested link = (name %q, target %v), want (\"t\", false)", in.name, in.target)
	}
	if got := glyphs([]node{in}); got != "go" {
		t.Errorf("nested link text = %q, want %q", got, "go")
	}
	if all := glyphs(e.mvl); all != "abgocd" {
		t.Errorf("paragraph glyphs = %q, want %q", all, "abgocd")
	}
}

// A full document: a link near the top and its target lower down. The SVG carries
// BOTH the clickable <a href="#sec2"> and the reachable id="sec2", so a browser can
// navigate from one to the other.
func TestInternalLinkNavigationSVG(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hyperlink{sec2}{jump}

\hypertarget{sec2}{HERE}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPage(2)
	if !contains(svg, `<a href="#sec2">`) {
		t.Errorf("SVG missing link to target: %s", svg)
	}
	if !contains(svg, `<g id="sec2">`) {
		t.Errorf("SVG missing named destination: %s", svg)
	}
}

// The name goes into an SVG id/href attribute, so an ampersand (or other markup
// metacharacter) in it is escaped, keeping both the link and the target well-formed
// and the fragment identifier consistent between them.
func TestInternalLinkNameEscaping(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hyperlink{a&b}{go}` + "\n\n" + `\hypertarget{a&b}{X}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPage(2)
	if !contains(svg, `<a href="#a&amp;b">`) {
		t.Errorf("SVG link name not escaped: %s", svg)
	}
	if !contains(svg, `<g id="a&amp;b">`) {
		t.Errorf("SVG target name not escaped: %s", svg)
	}
}

// Malformed input must not panic: a missing {name} brace, a missing {text} brace
// and an empty name are all tolerated (the raw-arg reader reports failure rather
// than crashing, and an empty id is degenerate but harmless).
func TestInternalLinkErrorBranches(t *testing.T) {
	cases := []string{
		`\hyperlink`,        // no arguments at all
		`\hypertarget`,      // no arguments at all
		`\hyperlink{t}`,     // name but no {text}
		`\hypertarget{t}`,   // name but no {text}
		`\hyperlink{}{go}`,  // empty name
		`\hypertarget{}{X}`, // empty name
	}
	for _, src := range cases {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Errorf("%q: unexpected error %v", src, err)
		}
		// Rendering the malformed input must also not panic.
		_ = e.RenderPage(2)
	}
}

// An empty name still produces a well-formed (if degenerate) anchor rather than a
// panic: id="" for a target and href="#" for a link.
func TestInternalLinkEmptyName(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hypertarget{}{X}`); err != nil {
		t.Fatal(err)
	}
	in, ok := firstInternalLink(e.mvl)
	if !ok {
		t.Fatal("no internalLinkNode placed")
	}
	if in.name != "" || !in.target {
		t.Errorf("empty target = (name %q, target %v), want (\"\", true)", in.name, in.target)
	}
	if svg := e.RenderPage(2); !contains(svg, `<g id="">`) {
		t.Errorf("SVG missing empty-id anchor: %s", svg)
	}
}

// The PDF driver renders internal links and targets without a clickable jump
// (go-pdfkit exposes no destination/link-annotation API), but must still emit a
// valid PDF with the content typeset — exercising (*pdfDraw).internalLink on both
// the horizontal (inline) and vertical (block) paths.
func TestInternalLinkPDF(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	e.Run(`\hsize=300pt \baselineskip=15pt `)
	if _, err := e.Run(`text \hyperlink{t}{go} more

\hypertarget{t}{HERE is the target}\par`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	b := buf.Bytes()
	if len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("not a valid PDF (%d bytes, header %q)", len(b), b[:min(len(b), 8)])
	}
}

// An internalLinkNode contributed directly to the main vertical list (rather than
// inside a paragraph line) exercises the vertical packing/painting paths: vpackSP,
// paintVListSP, the PDF vlist case and the page builder's vContribution. Both the
// link and the target still render their anchor in the SVG.
func TestInternalLinkVerticalPaths(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hyperlink{v}{go}`); err != nil {
		t.Fatal(err)
	}
	in, ok := firstInternalLink(e.mvl)
	if !ok {
		t.Fatal("no internalLinkNode placed")
	}
	tgt := internalLinkNode{name: "v", target: true, inner: in.inner}

	// vpackSP over a vertical list of the two nodes: the width is the widest item
	// (both share the same inner box), the height stacks the first with the second's
	// depth carried, and the depth is the last item's.
	vb := vpackSP([]node{in, tgt}, packNatural, 0)
	if vb.width != in.inner.width {
		t.Errorf("vpack width = %d, want %d", vb.width, in.inner.width)
	}
	if vb.depth != tgt.depth() {
		t.Errorf("vpack depth = %d, want %d", vb.depth, tgt.depth())
	}
	if want := in.height() + in.depth() + tgt.height(); vb.height != want {
		t.Errorf("vpack height = %d, want %d", vb.height, want)
	}

	// vContribution reports the node's full vertical extent.
	if got, want := vContribution(in), in.height()+in.depth(); got != want {
		t.Errorf("vContribution = %d, want %d", got, want)
	}

	// paintVListSP: rendering the vbox emits both anchors.
	svg := renderBoxSVG(vb, 2, 2, 0, 0, spMock{}, "white")
	if !contains(svg, `<a href="#v">`) || !contains(svg, `<g id="v">`) {
		t.Errorf("vlist SVG missing anchors: %s", svg)
	}

	// PDF vlist path: contribute the nodes directly to the main vertical list and
	// render a PDF, exercising (*pdfDraw).internalLink on the vertical path. This
	// needs an embeddable font, so it extracts the nodes from a real-font engine.
	fp := pdfFontOrSkip(t)
	e2 := New()
	e2.LoadLaTeX()
	e2.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	e2.Run(`\hsize=300pt \baselineskip=15pt `)
	e2.Run(`\hyperlink{v}{go}\par`)
	pin, ok := firstInternalLink(e2.mvl)
	if !ok {
		t.Fatal("no internalLinkNode in real-font engine")
	}
	ptgt := internalLinkNode{name: "v", target: true, inner: pin.inner}
	e2.mvl = append(e2.mvl, pin, ptgt) // contribute directly to the vertical list
	var buf bytes.Buffer
	if err := e2.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF (vertical): %v", err)
	}
	if b := buf.Bytes(); len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("not a valid PDF (%d bytes)", len(b))
	}
}
