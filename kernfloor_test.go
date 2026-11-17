package engine

import "testing"

// ⛔⛔ A KERN WE CANNOT READ MUST NOT ARRIVE AS A ZERO.
//
// kernSP asks go-opentype for the kern between two glyphs, and that is a PAIR
// query. GPOS lookup types 7 (Contextual) and 8 (Chained Contextual) position
// pairs too, and a pair query cannot evaluate them — such a rule is matched
// against the glyphs AROUND a position, which (prev, cur) does not carry. The
// library skips those subtables and answers 0, which is also what it answers for
// a pair the font deliberately leaves alone. Nothing here distinguished the two,
// and that is #716: 1.12pt per "AV" invisible to every number this engine's
// fidelity measurement steers by.
//
// go-opentype/opentype v0.22.0 reports the condition per face, and this surfaces
// it next to the other silent-swallow flags, where a reader of a corpus run will
// see it.
//
// ⚠ THESE WITNESSES TEST THE WIRING, NOT A FONT, and that is on purpose. A test
// asserting "notosans.ttf kerns contextually" would be an assertion about a
// third-party font's internals, which a version bump may flip for reasons that
// have nothing to do with this engine. The evidence that real fonts do this is a
// census over the 81 fonts go-opentype/fonts bundles — 8 of them, seven
// complex-script Noto faces and notosans.ttf, a general Latin sans — and it is
// recorded on go-opentype/opentype#71, with the library's own tests covering the
// detection itself.
type kernFloorMock struct {
	spMock
	floor bool
}

func (m kernFloorMock) KerningIsAFloor() bool { return m.floor }

func TestDiagnosticsSayWhenTheKerningIsOnlyAFloor(t *testing.T) {
	for _, c := range []struct {
		name string
		face fontFace
		want bool
		why  string
	}{
		{
			name: "a face whose kerning we cannot fully read",
			face: kernFloorMock{floor: true}, want: true,
			why: "the flag is the whole point: without it the compile's kern figures " +
				"look measured when they are a floor",
		},
		{
			name: "a face whose kerning is pair adjustment",
			face: kernFloorMock{floor: false}, want: false,
			why: "the ordinary font must NOT raise the flag, or it says nothing at all",
		},
		{
			// ⛔ A face that cannot be ASKED is not a face whose kerning is a floor.
			// fontFace is implemented by mocks with no font behind them, and
			// noteKerningFloor is a type assertion rather than an interface method
			// precisely so they are not forced to answer a question about OpenType
			// lookups that has no meaning for them. The assertion failing is the
			// right answer, not a missing case — and this row is what says so.
			name: "a face that cannot be asked at all",
			face: spMock{}, want: false,
			why: "a mock has no lookup list; answering true would flag every " +
				"mock-driven test in this package",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			if got := e.Diagnostics().KerningIsAFloor; got {
				t.Fatalf("a fresh engine already reports a kerning floor: %v", got)
			}
			e.SetFont(c.face)
			if got := e.Diagnostics().KerningIsAFloor; got != c.want {
				t.Errorf("KerningIsAFloor = %v, want %v — %s", got, c.want, c.why)
			}
		})
	}
}

// ⛔ IT LATCHES, and it has to. A document may switch faces many times — \rmfamily,
// \ttfamily, a math font, a size change that re-faces — and the figures of the
// whole compile are a floor if ANY of them carried kerning we could not read. A
// flag that tracked only the current face would answer about whichever font
// happened to be selected when Diagnostics was called, which for most documents
// is whatever set the last word.
func TestTheKerningFloorLatchesAcrossAFontSwitch(t *testing.T) {
	e := New()
	e.SetFont(kernFloorMock{floor: true})
	e.SetFont(kernFloorMock{floor: false})
	if !e.Diagnostics().KerningIsAFloor {
		t.Error("a later pair-adjustment face cleared the flag: the text set in the " +
			"first face still has kerns this engine could not read")
	}
	// The control, so the latch is not simply "always true once a font is set":
	// two readable faces in a row must leave it false.
	e2 := New()
	e2.SetFont(kernFloorMock{floor: false})
	e2.SetFont(kernFloorMock{floor: false})
	if e2.Diagnostics().KerningIsAFloor {
		t.Error("two readable faces raised the flag")
	}
}

// ⛔⛔ AND THROUGH selectFont, WHICH IS THE PATH A DOCUMENT ACTUALLY TAKES.
//
// SetFont is how a host installs the base face once. \rmfamily, \ttfamily, a math
// font and every size change go through selectFont instead — so the two tests
// above, which switch faces the way a HOST does, would have passed in full while
// the flag stayed blind to every switch a DOCUMENT makes. That hole was found by
// reading selectFont after writing them, not by running them.
//
// ⚠ And it must survive the group. selectFont saves the previous face for
// restoration at the end of `{ \bf … }`; the flag is not restored with it, because
// the text set inside that group still carries kerns this engine could not read.
func TestTheKerningFloorIsNoticedThroughSelectFontAndSurvivesTheGroup(t *testing.T) {
	e := New()
	e.SetFont(kernFloorMock{floor: false}) // a readable base face
	if e.Diagnostics().KerningIsAFloor {
		t.Fatal("the readable base face raised the flag")
	}
	// A group selects a face whose kerning we cannot read, then ends.
	e.beginGroup()
	e.selectFont(kernFloorMock{floor: true})
	if !e.Diagnostics().KerningIsAFloor {
		t.Fatal("selectFont did not notice the floor: a document's own font switches " +
			"are invisible to this flag, which is every switch that matters")
	}
	e.endGroup()
	if !e.Diagnostics().KerningIsAFloor {
		t.Error("the group's end cleared the flag: the face was restored, the text " +
			"set in it was not")
	}
}
