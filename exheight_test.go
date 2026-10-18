package engine

import (
	"strings"
	"testing"
)

// xhMock is a face whose 'x' is DELIBERATELY not half its size, and whose other
// glyphs are taller: if `ex` ever went back to reading the size, or started
// reading whatever glyph it happens to measure first, these numbers move.
type xhMock struct{ size, xh int }

func (m xhMock) charDimsSP(r rune) (int, int, int) {
	if r == 'x' {
		return 5 * unity, m.xh, 0
	}
	return 5 * unity, 9 * unity, 2 * unity
}
func (xhMock) spaceSP() glueSpec       { return glueSpec{width: 3 * unity} }
func (xhMock) glyphPathAt(rune) string { return "" }
func (xhMock) kernSP(rune, rune) int   { return 0 }
func (m xhMock) sizePt() int           { return m.size }

// ⛔⛔ 1ex is the font's X-HEIGHT (tex.web §455 "The x-height for cur_font",
// §10675 "param[5]=x_height is the size of one ex in the font"), not half its
// size. Half the size was 16% too big against the reference — 5.0pt where it
// gives 4.31pt at 10pt in Latin Modern — and classes state a great deal in ex.
func TestExIsTheFontsXHeight(t *testing.T) {
	e := New()
	e.SetFont(xhMock{size: 10, xh: ptToSP(4.31)})
	e.Run(`\dimen0=1ex \dimen1=3.5ex \dimen2=2.3ex \message{\the\dimen0|\the\dimen1|\the\dimen2}`)
	// The reference's own numbers for these three, at 10pt Latin Modern.
	if got := trimNL(e.out.String()); got != "4.31pt|15.08499pt|9.913pt" {
		t.Errorf("ex got %q want 4.31pt|15.08499pt|9.913pt", got)
	}
}

// A face whose x-height is NOT half the size must not read as half the size: this
// is the half of the assertion that distinguishes the two rules.
func TestExFollowsTheFaceAndNotTheSize(t *testing.T) {
	e := New()
	e.SetFont(xhMock{size: 10, xh: 8 * unity}) // an 8pt x-height on a 10pt face
	e.Run(`\dimen0=1ex \message{\the\dimen0}`)
	if got := trimNL(e.out.String()); got != "8.0pt" {
		t.Errorf("ex got %q want 8.0pt — it is following the size, not the face", got)
	}
}

// ⛔ An ASSIGNED \fontdimen5 wins over any measurement of the face. A document
// that writes the parameter means it, exactly as extraSpaceOf honours an assigned
// \fontdimen7.
func TestAnAssignedFontdimenFiveWins(t *testing.T) {
	e := New()
	f := xhMock{size: 10, xh: ptToSP(4.31)}
	e.SetFont(f)
	e.setFontDimen(f, 5, 6*unity)
	if got := e.exHeightSP(); got != 6*unity {
		t.Errorf("exHeightSP = %d, want %d (le \\fontdimen5 assigné)", got, 6*unity)
	}
}

// ⚠ A face whose 'x' has no height — a symbol or dingbat font made current —
// falls back to half the size rather than returning 0. A 0pt ex would silently
// collapse every ex-stated skip in the document to nothing, which is worse than
// the approximation it replaces.
func TestExFallsBackWhenTheFaceHasNoX(t *testing.T) {
	e := New()
	e.SetFont(noXMock{size: 12})
	if got, want := e.exHeightSP(), 6*unity; got != want {
		t.Errorf("exHeightSP = %d, want %d (la moitié de 12pt)", got, want)
	}
}

type noXMock struct{ size int }

func (noXMock) charDimsSP(rune) (int, int, int) { return 5 * unity, 0, 0 }
func (noXMock) spaceSP() glueSpec               { return glueSpec{width: 3 * unity} }
func (noXMock) glyphPathAt(rune) string         { return "" }
func (noXMock) kernSP(rune, rune) int           { return 0 }
func (m noXMock) sizePt() int                   { return m.size }

// With no current font there is nothing to measure, and the old approximation is
// right for exactly that case.
func TestExWithNoFontIsHalfTheDefaultSize(t *testing.T) {
	e := New()
	e.curFont = nil
	if got, want := e.exHeightSP(), 5*unity; got != want {
		t.Errorf("exHeightSP = %d, want %d", got, want)
	}
}

// ⭐ And em must NOT move. It is the quad, which this engine takes as the size,
// and the reference agrees at 10.0pt — so a change to ex that also moved em would
// be trading one error for another.
func TestEmStaysTheQuad(t *testing.T) {
	e := New()
	e.SetFont(xhMock{size: 10, xh: ptToSP(4.31)})
	e.Run(`\dimen0=1em \dimen1=18mu \message{\the\dimen0|\the\dimen1}`)
	if got := trimNL(e.out.String()); got != "10.0pt|10.0pt" {
		t.Errorf("em/mu got %q want 10.0pt|10.0pt (18mu = 1em)", got)
	}
}

// ⛔ A \par INSIDE a group ends the paragraph under that group's \baselineskip; a
// \par after the closing brace ends it under the enclosing one. This is the TeX
// property the heading fix relies on, tested on its own because it is the thing
// that must hold — \@gxhead's own effect needs the oracle to see, and the oracle
// has already said it: three section witnesses land within 0.002pt.
func TestAParInsideAGroupTakesThatGroupsBaselineskip(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\vbox{A\par{\baselineskip50pt B\par}}` +
		`\setbox1=\vbox{A\par{\baselineskip50pt B}\par}`); err != nil {
		t.Fatal(err)
	}
	in, out := e.box[0], e.box[1]
	if in == nil || out == nil {
		t.Fatal("une des deux boîtes est vide")
	}
	if in.height+in.depth <= out.height+out.depth {
		t.Errorf("\\par dans le groupe: %.2fpt, hors du groupe: %.2fpt — "+
			"le \\baselineskip du groupe doit atteindre sa dernière ligne",
			float64(in.height+in.depth)/float64(unity),
			float64(out.height+out.depth)/float64(unity))
	}
}

// ⛔⛔ And \@gxhead must place its \par accordingly. This asserts the SHAPE of the
// kernel definition, which is weaker than asserting behaviour and is here on
// purpose: the behaviour is a 5.978pt shift that only the oracle resolves, and
// mutate showed that moving the brace back survives every other test in this
// repository. A shape assertion that catches the regression beats no assertion.
//
// LaTeX's own \@sect is the authority: \begingroup #6\relax … #8\@@par \endgroup.
func TestGxheadEndsTheHeadingParagraphInsideTheGroup(t *testing.T) {
	const inside = `\noindent{#1 #2\par}\vskip\@tempskipa`
	const outside = `\noindent{#1 #2}\par\vskip\@tempskipa`
	if !strings.Contains(LaTeX2eClassLead, inside) {
		t.Errorf("\\@gxhead ne termine pas le paragraphe du titre DANS le groupe de style")
	}
	if strings.Contains(LaTeX2eClassLead, outside) {
		t.Errorf("\\@gxhead termine le paragraphe HORS du groupe: 5,978pt par titre")
	}
}
