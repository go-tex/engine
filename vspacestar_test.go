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
	_ = y1

	// ⛔⛔ "PAGE TWO LINES UP WITH PAGE ONE" IS NOT WHAT THE REFERENCE DOES, and this
	// assertion used to demand it to within 1pt. On this very document tectonic puts
	// starred page one at 196.53 and starred page two at 194.48 — **2.05pt apart** —
	// and the reason is exactly the quantity \@vspacer saves and restores:
	//
	//	page one   \prevdepth 0 at the body's start  → \glue(\baselineskip) 4.95
	//	page two   \prevdepth 2.05998, the depth of page one's line
	//	                                             → \glue(\baselineskip) 2.89
	//	                                               4.95 − 2.89 = 2.06
	//
	// Two pages whose preceding depth differs are not meant to line up. Demanding it
	// pinned a coincidence of the old code, where neither page got interline glue at
	// all because \prevdepth was never restored after the rule.
	//
	// ⭐ What "the 50pt survived the break" really means is measured against the
	// UNSTARRED form, whose glue the page builder discards at a page top: starred
	// page two must sit a full `amount` below plain page two. In the reference that
	// is 194.48 against 134.76, a gap of 59.72.
	pp0 := pagesOf(plain)
	if plainY2 := firstBaselineY(t, pp0[1]); y2-plainY2 < amount {
		t.Errorf("starred page two at %.1f, plain page two at %.1f — écart %.1f, "+
			"au moins %.0f attendu: c'est le filet non rejetable qui retient la "+
			"colle au sommet de la page", y2, plainY2, y2-plainY2, amount)
	}

	// ⛔⛔ THE UNSTARRED FORM IS DISCARDED ON PAGE ONE TOO, and this half of the test
	// used to assert the opposite — it required page one to KEEP the glue and only
	// page two to lose it. The oracle says otherwise, on this very document:
	//
	//	tectonic                       page 1 baseline y = 134.76
	//	this engine, before            page 1 baseline y = 184.56   ← 49.8pt too low
	//	this engine, discarding        page 1 baseline y = 134.74
	//
	// build_page discards a glue, kern or penalty node outright while
	// page_contents < box_there (tex.web:19495-19502, `goto done1`), and the main
	// vertical list is empty when the document opens. Page one is a page top like any
	// other. The old expectation was this engine's own 50pt error written down as a
	// requirement, which is how a test stops being a check.
	pp := pagesOf(plain)
	py1, py2 := firstBaselineY(t, pp[0]), firstBaselineY(t, pp[1])
	if diff := py2 - py1; diff < -1 || diff > 1 {
		t.Errorf("plain: page one starts at %.1f and page two at %.1f — both are page "+
			"tops and the glue is discarded at both", py1, py2)
	}

	// ⛔ AND THE DISCRIMINATOR, which is what this test exists for: the STAR is the
	// whole difference. \@vspacer puts a zero-height rule in front of the glue
	// (latex.ltx:6584); a rule is not discardable, so the glue is no longer at the top
	// of the page and survives. Without this line the two assertions above would both
	// pass for an engine that discarded the starred form as well.
	//
	// The bound is "at least the glue", not "exactly the glue", and the slack is a
	// measured quantity rather than room to be vague. With a zero-height rule as the
	// page's first box, \topskip is not reduced at all (tex.web:19513 subtracts
	// height(p), and the rule's is 0), so the starred page carries the full \topskip
	// plus the 50pt plus whatever the rule contributes before the first text line.
	// On this document:
	//
	//	tectonic   starred 196.53, plain 134.76  → 61.77
	//	this engine starred 191.66, plain 134.74 → 56.92
	//
	// ⚠ That remaining 4.87pt is a DIFFERENT defect and not this one's business: the
	// same starred document gives 191.66 both before and after this change, so it
	// predates it. Filed separately.
	if diff := y1 - py1; diff < amount {
		t.Errorf("starred page one at %.1f, plain page one at %.1f — écart %.1f, "+
			"au moins %.0f attendu: le filet de hauteur nulle doit retenir la colle "+
			"que la forme non étoilée perd", y1, py1, diff, amount)
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
