// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"testing"
)

// famMock is a deterministic fontFace whose glyph width encodes its identity:
// every character is w points wide. Binding distinct famMocks to \rm/\tt/\sf
// lets a test observe which face a node was measured with by its width alone,
// with no dependency on system fonts.
type famMock struct{ w int }

func (f famMock) charDimsSP(rune) (int, int, int) { return f.w * unity, 7 * unity, 2 * unity }
func (famMock) spaceSP() glueSpec                 { return glueSpec{width: 3 * unity} }
func (famMock) glyphPathAt(rune) string           { return "" }
func (famMock) kernSP(_, _ rune) int              { return 0 }
func (famMock) sizePt() int                       { return 10 }

// \texttt selects the mono face and \textsf the sans face, and both revert to the
// roman face at the group end — mirroring \textbf/\bf. Deterministic mock faces
// (roman 5pt, mono 8pt, sans 6pt per glyph) make the selected face observable by
// the measured width, so this needs no system fonts.
func TestFontFamilyMonoSansMock(t *testing.T) {
	roman := famMock{w: 5}
	mono := famMock{w: 8}
	sans := famMock{w: 6}

	e := New()
	if err := e.LoadPlain(); err != nil {
		t.Fatal(err)
	}
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(roman)
	e.bindFont("rm", roman)
	e.bindFont("tt", mono)
	e.bindFont("sf", sans)

	// \tt / \sf must be bound to a real font (not the kernel's no-op macro).
	if m := e.eq["tt"]; m == nil || m.kind != mFont || m.font == nil {
		t.Fatalf("\\tt not bound to a font: %+v", e.eq["tt"])
	}
	if m := e.eq["sf"]; m == nil || m.kind != mFont || m.font == nil {
		t.Fatalf("\\sf not bound to a font: %+v", e.eq["sf"])
	}

	if _, err := e.Run(`\setbox0=\hbox{\texttt{x}}\setbox1=\hbox{\textsf{x}}\setbox2=\hbox{x}`); err != nil {
		t.Fatal(err)
	}
	if got := e.box[0].width; got != 8*unity {
		t.Errorf("\\texttt{x} width = %d sp; want mono 8pt (%d)", got, 8*unity)
	}
	if got := e.box[1].width; got != 6*unity {
		t.Errorf("\\textsf{x} width = %d sp; want sans 6pt (%d)", got, 6*unity)
	}
	if got := e.box[2].width; got != 5*unity {
		t.Errorf("roman x width = %d sp; want roman 5pt (%d)", got, 5*unity)
	}
	// After the \texttt/\textsf groups closed, the current font reverted to roman.
	if e.curFont != fontFace(roman) {
		t.Errorf("curFont after groups = %v; want roman face reverted", e.curFont)
	}
}

// With a mono and a sans font in Options, buildEngine binds \tt and \sf to them,
// so \texttt / \textsf measure with a face whose metrics differ from the roman
// face for the same text — mirroring TestFontFamilyBold's Options plumbing.
func TestFontFamilyMonoSansReal(t *testing.T) {
	romanPath := "/System/Library/Fonts/Supplemental/Georgia.ttf"
	monoPath := "/System/Library/Fonts/Supplemental/Courier New.ttf"
	sansPath := "/System/Library/Fonts/Supplemental/Arial.ttf"
	for _, p := range []string{romanPath, monoPath, sansPath} {
		if _, err := os.Stat(p); err != nil {
			t.Skip("no system font: " + p)
		}
	}
	rb, _ := os.ReadFile(romanPath)
	mb, _ := os.ReadFile(monoPath)
	sb, _ := os.ReadFile(sansPath)
	e, err := buildEngine(Options{Font: rb, MonoFont: mb, SansFont: sb, Size: 20}, true)
	if err != nil {
		t.Fatal(err)
	}
	if m := e.eq["tt"]; m == nil || m.kind != mFont {
		t.Fatalf("\\tt not bound to a font: %+v", e.eq["tt"])
	}
	if m := e.eq["sf"]; m == nil || m.kind != mFont {
		t.Fatalf("\\sf not bound to a font: %+v", e.eq["sf"])
	}
	// "il" is a proportional pair in Georgia/Arial but fixed-width in Courier,
	// so all three faces yield distinct widths for the same text.
	if _, err := e.Run(`\setbox0=\hbox{il}\setbox1=\hbox{\texttt{il}}\setbox2=\hbox{\textsf{il}}`); err != nil {
		t.Fatal(err)
	}
	rw, tw, sw := e.box[0].width, e.box[1].width, e.box[2].width
	if tw == rw {
		t.Errorf("mono il width == roman il width (%d); expected the mono face to differ", rw)
	}
	if sw == rw {
		t.Errorf("sans il width == roman il width (%d); expected the sans face to differ", rw)
	}
}

// A malformed MonoFont or SansFont makes buildEngine fail, exercising the error
// return of each new bindOptionalFont call site.
func TestFontFamilyBadFonts(t *testing.T) {
	if _, err := buildEngine(Options{MonoFont: []byte("not a font")}, false); err == nil {
		t.Error("buildEngine with bad MonoFont: want error, got nil")
	}
	if _, err := buildEngine(Options{SansFont: []byte("not a font")}, false); err == nil {
		t.Error("buildEngine with bad SansFont: want error, got nil")
	}
}
