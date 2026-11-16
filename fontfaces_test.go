package engine

import (
	"bytes"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	texmath "github.com/go-tex/math"
)

// A glyph must carry the face it was measured with, which is tex.web §134's own
// char_node ("@d font == type {the font code in a char_node}"). Without it the
// drivers each held ONE face for the whole document and redrew every glyph in it:
// \textbf measured in the bold and drew in the roman (#590).
//
// This runs everywhere: the two faces are two instances of the bundled font, so
// the test needs no system font. It asserts on the box tree, which is where the
// face is either recorded or lost — both drivers read it from there.
func TestAGlyphCarriesTheFaceItWasMeasuredWith(t *testing.T) {
	font := texmath.DefaultFont()
	e, err := buildEngine(Options{Font: font, BoldFont: font, Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	bold := e.eq["bf"]
	if bold == nil || bold.kind != mFont || bold.font == nil {
		t.Fatalf("\\bf is not bound to a font: %+v", bold)
	}
	if _, err := e.Run(`\setbox0=\hbox{r\textbf{b}}`); err != nil {
		t.Fatal(err)
	}
	faces := map[rune]fontFace{}
	for _, n := range e.box[0].list {
		if c, ok := n.(charNode); ok {
			faces[c.ch] = c.face
		}
	}
	if faces['r'] == nil || faces['b'] == nil {
		t.Fatalf("a glyph names no face at all: roman=%v bold=%v", faces['r'], faces['b'])
	}
	if faces['b'] != bold.font {
		t.Errorf("the glyph inside \\textbf names %p, not the bold face %p — the driver will draw it in the roman",
			faces['b'], bold.font)
	}
	if faces['r'] == faces['b'] {
		t.Error("both glyphs name the same face: the switch left no trace in the box tree")
	}
}

// And end to end, on the family this package EMBEDS, so it runs everywhere —
// including on CI, where the system-font version below skips and has therefore
// never once asserted anything (#590).
//
// A document that switches face must embed that face and select it in the
// content stream. Both are asserted, because either alone passes for the wrong
// reason: embedding every bound face eagerly satisfies the first, and selecting
// one resource that is the same font twice satisfies the second.
//
// ⛔ THE SUBSET TAG IS THE ONLY FIELD THAT TELLS TWO FACES OF ONE FAMILY APART,
// and this test keeps it. A reading that drops it — the ordinary way to name a
// font from a PDF, because the tag is noise when families differ — collapses a
// correct two-face document to one entry. That reading is what reported \textbf
// as never reaching the page on #590, against output that was already right:
// the Lora in use there ships a bold whose own name table says "Lora-Regular"
// (go-opentype/opentype#72, go-opentype/fonts#38), so both subsets came back
// under one name. Latin Modern names its four faces apart, which is why this
// test can assert on names at all — and it asserts on the full entry, tag
// included, so a future family that does not cannot make it pass by accident.
func TestEveryFaceOfTheDefaultFamilyReachesThePDF(t *testing.T) {
	var buf bytes.Buffer
	src := `\documentclass{article}
\begin{document}
\noindent romanroman \textbf{boldbold} \emph{italicital} \texttt{monomono}
\end{document}
`
	if _, err := CompileToPDF([]byte(src), Options{Lenient: true}, &buf); err != nil {
		t.Fatal(err)
	}
	pdf := buf.Bytes()

	embedded := distinct(regexp.MustCompile(`/BaseFont\s*/([A-Za-z0-9+,.-]+)`), pdf)
	if len(embedded) != 4 {
		t.Errorf("%d font(s) embedded, 4 expected (roman, bold, italic, mono): %v", len(embedded), embedded)
	}
	for _, want := range []string{
		"LMRoman10-Regular", "LMRoman10-Bold", "LMRoman10-Italic", "LMMono10-Regular",
	} {
		if !containsSuffix(embedded, want) {
			t.Errorf("%s is not embedded: %v", want, embedded)
		}
	}
	// Each entry carries its own six-letter subset tag, so four entries are four
	// distinct font programs and not one name counted four times.
	tags := map[string]bool{}
	for _, name := range embedded {
		if i := strings.IndexByte(name, '+'); i == 6 {
			tags[name[:i]] = true
		} else {
			t.Errorf("%q carries no six-letter subset tag", name)
		}
	}
	if len(tags) != 4 {
		t.Errorf("%d distinct subset tag(s) over 4 embedded faces: %v — two faces sharing a "+
			"subset are one font program embedded twice", len(tags), embedded)
	}
	selected := distinct(regexp.MustCompile(`(/F[0-9]+)\s+[0-9.]+\s+Tf`), pdf)
	if len(selected) != 4 {
		t.Errorf("%d font resource(s) selected with Tf, 4 expected: %v — a face that is embedded "+
			"and never selected puts nothing on the page", len(selected), selected)
	}
}

// The same assertion over faces the CALLER supplies, in three different
// FAMILIES, so a substitution is unmistakable. They are system fonts, so this
// one skips where they are absent — which is every CI runner this repository
// has; the test above is the one that runs everywhere.
func TestEveryFaceAFileNamesReachesThePDF(t *testing.T) {
	paths := map[string]string{
		"roman": "/System/Library/Fonts/Supplemental/Georgia.ttf",
		"bold":  "/System/Library/Fonts/Supplemental/Georgia Bold.ttf",
		"mono":  "/System/Library/Fonts/Supplemental/Courier New.ttf",
	}
	files := map[string][]byte{}
	for kind, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Skipf("no %s system font: %v", kind, err)
		}
		files[kind] = b
	}
	var buf bytes.Buffer
	src := `\documentclass{article}
\begin{document}
\noindent romanroman \textbf{boldbold} \texttt{monomono}
\end{document}
`
	if _, err := CompileToPDF([]byte(src), Options{
		Font: files["roman"], BoldFont: files["bold"], MonoFont: files["mono"], Lenient: true,
	}, &buf); err != nil {
		t.Fatal(err)
	}
	pdf := buf.Bytes()

	embedded := distinct(regexp.MustCompile(`/BaseFont\s*/([A-Za-z0-9+,.-]+)`), pdf)
	if len(embedded) != 3 {
		t.Errorf("%d font(s) embedded, 3 expected (roman, bold, mono): %v", len(embedded), embedded)
	}
	// The names carry a six-letter subset tag, so match on the family.
	for _, want := range []string{"Georgia", "Georgia-Bold", "CourierNewPSMT"} {
		if !containsSuffix(embedded, want) {
			t.Errorf("%s is not embedded: %v", want, embedded)
		}
	}
	selected := distinct(regexp.MustCompile(`(/F[0-9]+)\s+[0-9.]+\s+Tf`), pdf)
	if len(selected) != 3 {
		t.Errorf("%d font resource(s) selected with Tf, 3 expected: %v — a face that is embedded "+
			"and never selected puts nothing on the page", len(selected), selected)
	}
}

// markFace is a face whose every glyph is one recognisable outline, so an SVG
// shows which face the driver ASKED for rather than which one it was handed.
type markFace struct{ spMock }

func (markFace) glyphPathAt(rune) string { return markPath }

const markPath = "M9 9 l1 1"

// The SVG driver reads the glyph's face too. It is a separate path from the PDF
// one — same defect, same fix — and a mutation that made it ignore the face
// survived the whole suite until this test existed.
//
// spMock draws nothing (glyphPathAt returns ""), so the mark can only come from
// the face bound to \bf, which is the one the glyph names.
func TestTheSVGDriverDrawsAGlyphWithItsOwnFace(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.bindFont("bf", markFace{})
	if _, err := e.Run(`\setbox0=\hbox{a{\bf b}}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderBox(0, 1)
	if !strings.Contains(svg, markPath) {
		t.Errorf("the glyph set in \\bf was not drawn with the \\bf face: no %q in\n%s", markPath, svg)
	}
	// And the control, so the assertion above cannot pass by drawing EVERYTHING
	// with the mark: the roman glyph still comes from the face it was measured
	// with, which paints nothing.
	if n := strings.Count(svg, markPath); n != 1 {
		t.Errorf("%d marks drawn, 1 expected: the face is being applied to glyphs that do not name it", n)
	}
}

// distinct returns the sorted distinct first submatches of re in b.
func distinct(re *regexp.Regexp, b []byte) []string {
	seen := map[string]bool{}
	for _, m := range re.FindAllSubmatch(b, -1) {
		seen[string(m[1])] = true
	}
	var out []string
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func containsSuffix(names []string, suffix string) bool {
	for _, n := range names {
		if len(n) >= len(suffix) && n[len(n)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}
