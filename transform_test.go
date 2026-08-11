// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"os"
	"testing"
)

// firstTransform returns the first transformNode reachable in a node tree,
// descending through boxes (but not into a transform's own inner box).
func firstTransform(nodes []node) (transformNode, bool) {
	for _, n := range nodes {
		switch c := n.(type) {
		case transformNode:
			return c, true
		case *boxNode:
			if tn, ok := firstTransform(c.list); ok {
				return tn, true
			}
		}
	}
	return transformNode{}, false
}

// runTransform runs src and returns the first transformNode it places.
func runTransform(t *testing.T, src string) transformNode {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	tn, ok := firstTransform(e.mvl)
	if !ok {
		t.Fatalf("%q: no transformNode placed", src)
	}
	return tn
}

// \scalebox{2}{x} doubles every dimension of the inner box (spMock 'x' is 5pt
// wide, 7pt tall, 2pt deep).
func TestScaleboxUniform(t *testing.T) {
	tn := runTransform(t, `\scalebox{2}{x}`)
	if tn.inner.width != 5*unity || tn.inner.height != 7*unity || tn.inner.depth != 2*unity {
		t.Fatalf("inner dims = %d/%d/%d sp", tn.inner.width, tn.inner.height, tn.inner.depth)
	}
	if got, want := tn.width(), 2*tn.inner.width; got != want {
		t.Errorf("width = %d sp, want %d (doubled)", got, want)
	}
	if got, want := tn.height(), 2*tn.inner.height; got != want {
		t.Errorf("height = %d sp, want %d (doubled)", got, want)
	}
	if got, want := tn.depth(), 2*tn.inner.depth; got != want {
		t.Errorf("depth = %d sp, want %d (doubled)", got, want)
	}
	if tn.refDX != 0 {
		t.Errorf("refDX = %d sp, want 0 for a positive scale", tn.refDX)
	}
}

// \scalebox{2}[1]{x} scales the width by 2 but leaves height and depth untouched.
func TestScaleboxAnisotropic(t *testing.T) {
	tn := runTransform(t, `\scalebox{2}[1]{x}`)
	if got, want := tn.width(), 2*tn.inner.width; got != want {
		t.Errorf("width = %d sp, want %d (×2)", got, want)
	}
	if got, want := tn.height(), tn.inner.height; got != want {
		t.Errorf("height = %d sp, want %d (×1)", got, want)
	}
	if got, want := tn.depth(), tn.inner.depth; got != want {
		t.Errorf("depth = %d sp, want %d (×1)", got, want)
	}
}

// \scalebox{1}[-1] mirrors vertically: height and depth swap.
func TestScaleboxVerticalMirror(t *testing.T) {
	tn := runTransform(t, `\scalebox{1}[-1]{x}`)
	if got, want := tn.height(), tn.inner.depth; got != want {
		t.Errorf("height = %d sp, want old depth %d", got, want)
	}
	if got, want := tn.depth(), tn.inner.height; got != want {
		t.Errorf("depth = %d sp, want old height %d", got, want)
	}
}

// \reflectbox{x} mirrors horizontally: the width magnitude is preserved, the map
// is a horizontal flip (a<0), and the content origin moves to the right edge.
func TestReflectbox(t *testing.T) {
	tn := runTransform(t, `\reflectbox{x}`)
	if got, want := tn.width(), tn.inner.width; got != want {
		t.Errorf("width = %d sp, want %d (magnitude preserved)", got, want)
	}
	if tn.a >= 0 {
		t.Errorf("a = %v, want < 0 (mirrored)", tn.a)
	}
	if got, want := tn.refDX, tn.inner.width; got != want {
		t.Errorf("refDX = %d sp, want %d (origin at right edge)", got, want)
	}
	if got, want := tn.height(), tn.inner.height; got != want {
		t.Errorf("height = %d sp, want %d (unchanged)", got, want)
	}
}

// \resizebox{40pt}{!}{x} scales so the width is exactly 40pt, the height following
// the same factor to keep the aspect ratio (spMock 'x': 5pt → ×8 → 56pt tall).
func TestResizeboxAspect(t *testing.T) {
	tn := runTransform(t, `\resizebox{40pt}{!}{x}`)
	if got, want := tn.width(), 40*unity; got != want {
		t.Errorf("width = %d sp, want %d (40pt)", got, want)
	}
	// factor = 40pt / 5pt = 8, so height = 7pt × 8 = 56pt.
	if got, want := tn.height(), 56*unity; got != want {
		t.Errorf("height = %d sp, want %d (aspect-preserved 56pt)", got, want)
	}
}

// \resizebox{40pt}{20pt}{x} sets both dimensions independently (no aspect lock).
func TestResizeboxBoth(t *testing.T) {
	tn := runTransform(t, `\resizebox{40pt}{20pt}{x}`)
	if got, want := tn.width(), 40*unity; got != want {
		t.Errorf("width = %d sp, want %d (40pt)", got, want)
	}
	if got, want := tn.height(), 20*unity; got != want {
		t.Errorf("height = %d sp, want %d (20pt)", got, want)
	}
}

// \resizebox{!}{20pt}{x} takes the aspect from the height axis instead.
func TestResizeboxWidthBang(t *testing.T) {
	tn := runTransform(t, `\resizebox{!}{14pt}{x}`)
	// factor = 14pt / 7pt = 2, so width = 5pt × 2 = 10pt.
	if got, want := tn.width(), 10*unity; got != want {
		t.Errorf("width = %d sp, want %d (aspect-preserved 10pt)", got, want)
	}
	if got, want := tn.height(), 14*unity; got != want {
		t.Errorf("height = %d sp, want %d (14pt)", got, want)
	}
}

// \rotatebox{90}{xx} turns the box a quarter-turn: the axis-aligned bounding box
// width becomes the old vertical extent (height+depth) and the bbox height becomes
// the old width, with no depth left below the baseline.
func TestRotatebox90(t *testing.T) {
	tn := runTransform(t, `\rotatebox{90}{xx}`)
	oldW := tn.inner.width                      // 10pt
	oldVert := tn.inner.height + tn.inner.depth // 9pt
	if got := tn.width(); got != oldVert {
		t.Errorf("bbox width = %d sp, want %d (old height+depth)", got, oldVert)
	}
	if got := tn.height(); got != oldW {
		t.Errorf("bbox height = %d sp, want %d (old width)", got, oldW)
	}
	if got := tn.depth(); got != 0 {
		t.Errorf("bbox depth = %d sp, want 0", got)
	}
	// The task's approximate identities, asserted within half a point.
	tol := unity / 2
	if d := tn.width() - tn.inner.height; d < -tol-tn.inner.depth || d > tol+tn.inner.depth {
		t.Errorf("bbox width %d not ≈ old height %d", tn.width(), tn.inner.height)
	}
	if d := tn.height() - tn.inner.width; d < -tol || d > tol {
		t.Errorf("bbox height %d not ≈ old width %d", tn.height(), tn.inner.width)
	}
}

// \rotatebox{0} is the identity: dimensions are unchanged.
func TestRotatebox0(t *testing.T) {
	tn := runTransform(t, `\rotatebox{0}{xx}`)
	if tn.width() != tn.inner.width || tn.height() != tn.inner.height || tn.depth() != tn.inner.depth {
		t.Errorf("identity rotation changed dims: %d/%d/%d vs inner %d/%d/%d",
			tn.width(), tn.height(), tn.depth(), tn.inner.width, tn.inner.height, tn.inner.depth)
	}
}

// \rotatebox[origin=c]{90}{xx} accepts and ignores an optional key-value list.
func TestRotateboxOptionalArg(t *testing.T) {
	tn := runTransform(t, `\rotatebox[origin=c]{90}{xx}`)
	if got := tn.height(); got != tn.inner.width {
		t.Errorf("bbox height = %d sp, want old width %d", got, tn.inner.width)
	}
}

// A transform works inside an \hbox (the boxNodeFor path): the containing box's
// width is the transform's reserved width.
func TestScaleboxInsideHbox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{\scalebox{2}{x}}`); err != nil {
		t.Fatal(err)
	}
	b := e.getBox(0)
	if b == nil {
		t.Fatal("box0 void")
	}
	if got, want := b.width, 2*5*unity; got != want {
		t.Errorf("hbox width = %d sp, want %d (scaled 'x')", got, want)
	}
	if _, ok := firstTransform(b.list); !ok {
		t.Error("no transformNode inside the hbox")
	}
}

// resizeFactors handles degenerate inputs without dividing by zero.
func TestResizeFactorsDegenerate(t *testing.T) {
	if sx, sy := resizeFactors(0, 0, 40*unity, false, 20*unity, false); sx != 1 || sy != 1 {
		t.Errorf("zero natural size: sx/sy = %v/%v, want 1/1", sx, sy)
	}
	if sx, sy := resizeFactors(5*unity, 7*unity, 0, true, 0, true); sx != 1 || sy != 1 {
		t.Errorf("both bang: sx/sy = %v/%v, want 1/1", sx, sy)
	}
}

// The SVG driver emits a <g transform="matrix(...)"> wrapper realising the scale;
// spMock draws no glyph path, so the vrule content gives a deterministic geometry.
func TestTransformSVG(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{\scalebox{2}{\vrule width2pt height5pt depth0pt}}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderBox(0, 0)
	// box is 4pt wide, 10pt tall; the matrix scales the inner rule by 2 about the
	// left baseline (origin at x=0, baseline at y=10).
	mustContain(t, svg, `width="4pt" height="10pt"`)
	mustContain(t, svg, `<g transform="matrix(2,0,0,2,0,10)">`)
	mustContain(t, svg, `<rect x="0" y="-5" width="2" height="5"/>`) // the inner rule, pre-transform
}

// A reflect emits a negative horizontal scale and positions the origin at the
// right edge (matrix e = refDX = inner width).
func TestReflectSVG(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{\reflectbox{\vrule width2pt height5pt depth0pt}}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderBox(0, 0)
	mustContain(t, svg, `<g transform="matrix(-1,0,0,1,2,5)">`)
}

// The SVG driver also paints transforms stacked in a vbox (the vlist path).
func TestTransformSVGInVBox(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\vbox{\scalebox{2}{\vrule width2pt height5pt depth0pt}}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderBox(0, 0)
	mustContain(t, svg, `matrix(2,0,0,2,`)
}

// Error branches must not panic: missing braces, empty groups and bad numbers all
// degrade gracefully rather than crashing.
func TestTransformErrorBranches(t *testing.T) {
	srcs := []string{
		`\scalebox`,               // no arguments at all
		`\scalebox{2}`,            // factor but no content
		`\scalebox{}{x}`,          // empty factor → zero scale
		`\scalebox{2}{x`,          // unterminated content group
		`\reflectbox`,             // no content
		`\resizebox{}{}{x}`,       // empty dimensions
		`\resizebox{40pt}{!}`,     // no content
		`\rotatebox{abc}{x}`,      // non-numeric angle
		`\rotatebox{30}`,          // angle but no content
		`\rotatebox[opts]{45}`,    // optional arg, no content
		`\scalebox x{y}`,          // factor not a braced group
		`\scalebox{2x}{y}`,        // trailing junk before the factor's closing brace
		`\scalebox{2}[3]x`,        // optional v-scale, then non-braced content
		`\resizebox 5pt{20pt}{x}`, // first dimension not a braced group
		`\resizebox{5pt y}{!}{z}`, // trailing junk before a dimension's closing brace
		`\rotatebox[opts x`,       // unterminated optional bracket
		`\rotatebox`,              // nothing at all after the control sequence
		`\scalebox{2}[3x]{y}`,     // junk before the optional v-scale's closing bracket
		`\resizebox{!x}{!}{y}`,    // junk after a '!' dimension
	}
	for _, src := range srcs {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Errorf("%q: unexpected error %v", src, err)
		}
	}
}

// The PDF driver renders transforms through go-pdfkit's CTM (Save/Transform/
// Restore), producing a valid PDF with the graphics-state operators present.
func TestTransformPDF(t *testing.T) {
	fp := "/System/Library/Fonts/Supplemental/Georgia.ttf"
	if _, err := os.Stat(fp); err != nil {
		t.Skip("no system font")
	}
	e := New()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	e.Run(`\hsize=400pt \baselineskip=15pt `)
	e.Run(`\scalebox{2}{Big} \reflectbox{Mirror} \rotatebox{30}{Tilted} \resizebox{120pt}{!}{Stretched}\par`)
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	b := buf.Bytes()
	if len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("not a valid PDF (%d bytes)", len(b))
	}
	if !bytes.Contains(b, []byte("%%EOF")) {
		t.Error("PDF missing EOF marker")
	}
	// A page's content stream (with the transforms drawn) must be present; the
	// exact q/Q/cm operators may be inside a compressed stream, so we only assert
	// the driver ran end to end and produced a page with a font subset.
	if !bytes.Contains(b, []byte("/FontFile2")) && !bytes.Contains(b, []byte("/FontFile3")) {
		t.Error("no embedded font subset in transform PDF")
	}
}
