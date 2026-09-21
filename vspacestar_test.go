package engine

import (
	"regexp"
	"strconv"
	"testing"
)

// firstBaselineY is the y of the topmost text drawn on an SVG page.
func firstBaselineY(t *testing.T, svg string) float64 {
	t.Helper()
	// Only a <tspan> carries a baseline: the page's own background rect sits at
	// y=0, and so does the zero-height rule the star inserts.
	re := regexp.MustCompile(`<tspan[^>]*\sy="([0-9.]+)"`)
	best := -1.0
	for _, m := range re.FindAllStringSubmatch(svg, -1) {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		if best < 0 || v < best {
			best = v
		}
	}
	if best < 0 {
		t.Fatalf("no text on the page:\n%.300s", svg)
	}
	return best
}

// \vspace* must survive a page break — that is the whole difference the star makes.
// LaTeX gets it by putting a ZERO-HEIGHT RULE in front of the glue (\@vspacer,
// latex.ltx:6584): a rule is not discardable, so the glue is no longer at the top of
// the page and the page builder keeps it.
//
// We read the star and dropped it, so \vspace* was \vspace and the space vanished
// exactly where it is asked for. book.cls opens every chapter with
// \vspace*{50\p@} (\@makechapterhead), so every chapter head sat 50pt too high.
func TestVspaceStarSurvivesAPageBreak(t *testing.T) {
	const amount = 50.0
	starred := `\vspace*{50pt}Top of page one.\par\penalty-10000 \vspace*{50pt}Top of page two.`
	plain := `\vspace{50pt}Top of page one.\par\penalty-10000 \vspace{50pt}Top of page two.`

	pagesOf := func(src string) []string {
		pages, err := CompileToSVGPages([]byte(src), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 2 {
			t.Fatalf("%d pages, want 2", len(pages))
		}
		return pages
	}

	sp := pagesOf(starred)
	y1, y2 := firstBaselineY(t, sp[0]), firstBaselineY(t, sp[1])
	if diff := y2 - y1; diff < -1 || diff > 1 {
		t.Errorf("starred: page two starts at %.1f, page one at %.1f — the 50pt did not survive", y2, y1)
	}

	// The unstarred form is discarded at a page top, as TeX discards any glue there.
	pp := pagesOf(plain)
	py1, py2 := firstBaselineY(t, pp[0]), firstBaselineY(t, pp[1])
	if py2 >= py1-amount/2 {
		t.Errorf("plain: page two starts at %.1f and page one at %.1f — the glue should have been discarded", py2, py1)
	}
}

// The rule the star inserts has no height: it holds the glue on the page without
// adding space of its own.
func TestVspaceStarAddsNoHeightOfItsOwn(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\vspace*{50pt}`); err != nil {
		t.Fatal(err)
	}
	var rules, total int
	for _, n := range e.mvl {
		if r, ok := n.(ruleNode); ok {
			rules++
			total += r.height + r.depth
		}
	}
	if rules != 1 {
		t.Errorf("%d rules contributed, want 1", rules)
	}
	if total != 0 {
		t.Errorf("the rule measures %dsp, want 0", total)
	}
}
