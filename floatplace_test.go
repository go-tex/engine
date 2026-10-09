// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// floatMeaningBody returns the token text of \@float's current meaning, used to
// confirm which definition (classic inline vs the placer hook) is in force.
func floatMeaningBody(e *Engine) string {
	m := e.eq["@float"]
	if m == nil {
		return ""
	}
	var b strings.Builder
	for _, t := range m.body {
		if t.cs_ {
			b.WriteString("\\" + t.cs)
		} else {
			b.WriteRune(t.ch)
		}
	}
	return b.String()
}

// With GOTEX_FLOATS=0, the FloatPlacementSubstrate is never
// loaded: \@float keeps its classic inline definition, no float is captured, and
// the main vertical list carries no floatNode — so the default output is the
// untouched inline rendering. This is the byte-identical-off guarantee at the
// macro level (the integration cmp against the base binary covers the bytes).
func TestFloatFlagOffKeepsInline(t *testing.T) {
	t.Setenv("GOTEX_FLOATS", "0") // opt back out of the placer
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if body := floatMeaningBody(e); !strings.Contains(body, "\\begingroup") || strings.Contains(body, "floatbegin") {
		t.Fatalf("flag off: \\@float should keep the classic inline definition, got %q", body)
	}
	if _, err := e.Run(`\begin{figure}\caption{Plot}\end{figure}Body text.`); err != nil {
		t.Fatal(err)
	}
	if e.mvlHasFloats() {
		t.Error("flag off: a figure was captured as a floatNode (should stay inline)")
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "Figure1:Plot") {
		t.Errorf("flag off: caption missing from inline output: %q", txt)
	}
}

// With GOTEX_FLOATS set, \@float routes to the placer hook, a standard figure is
// captured into a floatNode, and Pages() paginates through the float placer while
// the caption is still typeset.
func TestFloatFlagOnCaptures(t *testing.T) {
	t.Setenv("GOTEX_FLOATS", "1")
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if body := floatMeaningBody(e); !strings.Contains(body, "floatbegin") {
		t.Fatalf("flag on: \\@float should route to the placer hook, got %q", body)
	}
	// A \documentclass rewires \figure to \@float{figure} (the bare kernel's \figure
	// bypasses \@float), so capture is exercised the way a real document reaches it.
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\begin{figure}\caption{Plot}\end{figure}` +
		strings.Repeat(`Body text paragraph. `, 40) + `\par`); err != nil {
		t.Fatal(err)
	}
	if !e.mvlHasFloats() {
		t.Fatal("flag on: the figure was not captured as a floatNode")
	}
	pages := e.Pages()
	if len(pages) == 0 {
		t.Fatal("flag on: no pages produced")
	}
}

// A float that asks for [h] (here) placement stays inline even with the placer on:
// LaTeX keeps an [h] float roughly where written, so floating it would misplace it.
func TestFloatHereStaysInline(t *testing.T) {
	t.Setenv("GOTEX_FLOATS", "1")
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\begin{figure}[h]\caption{Here}\end{figure}Body.`); err != nil {
		t.Fatal(err)
	}
	if e.mvlHasFloats() {
		t.Error("[h] float should stay inline, but it was captured for floating")
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "Figure1:Here") {
		t.Errorf("[h] float caption missing from inline output: %q", txt)
	}
}

// A captured [t] float is placed at the TOP of a page: its box is the first
// box-like node on the page it lands on, with the body text flowing below it.
func TestFloatTopPlacement(t *testing.T) {
	t.Setenv("GOTEX_FLOATS", "1")
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// A tall, distinctive rule stands in for the figure body so the float box is
	// unambiguously identifiable by height among the ~12pt text-line boxes.
	const tallPt = 200
	src := `\documentclass{article}\begin{document}` +
		`\begin{figure}[t]\rule{10pt}{` + itoa(tallPt) + `pt}\caption{Tall}\end{figure}` +
		strings.Repeat(`Line of body text here. `, 60) + `\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if !e.mvlHasFloats() {
		t.Fatal("[t] float was not captured")
	}
	pages := e.Pages()
	if len(pages) == 0 {
		t.Fatal("no pages produced")
	}
	// Find the tall float box and confirm that on the page it sits on, it precedes
	// any text-line box (i.e. it is at the top).
	tall := tallPt * unity
	found := false
	for _, p := range pages {
		firstBoxTall := -1
		for i, n := range p.list {
			if b, ok := n.(*boxNode); ok {
				if b.height >= tall {
					firstBoxTall = i
					break
				}
				// a text-line box before the float ⇒ float not at the top
				firstBoxTall = -2
				break
			}
		}
		if firstBoxTall >= 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("[t] float box was not found at the top of any page")
	}
}

// The captured body is a box being built, so an assignment inside it must not
// escape into the document. The real case is a figure holding
// \put(-0.33\textwidth,…): \put is undefined, so \textwidth reads as the start of
// an assignment, takes the missing number as zero, and — before the body was given
// a group of its own — left \hsize at 0pt for the rest of the paper, which then set
// one word per line (1439 pages against a reference of 333).
func TestCapturedFloatBodyCannotChangeTheTextWidth(t *testing.T) {
	t.Setenv("GOTEX_FLOATS", "1")
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\begin{figure}\textwidth=0pt\caption{Plot}\end{figure}` +
		strings.Repeat(`Body text paragraph. `, 20) + `\par`); err != nil {
		t.Fatal(err)
	}
	if e.hsize <= 0 {
		t.Fatalf("\\hsize = %d sp after the float: an assignment escaped the captured body", e.hsize)
	}
}

// A float written halfway down a page belongs at the top of THAT page: LaTeX
// contributes it at its anchor and \@addtocurcol (latex.ltx:15636) tests it against
// the room left in the column. Taking only floats anchored before the page began
// pushed every one of them at least a page later.
func TestFloatAnchoredInsideThePageRidesIt(t *testing.T) {
	t.Setenv("GOTEX_FLOATS", "1")
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// Text, then a figure, then more text: the figure is anchored inside page 1.
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		strings.Repeat(`Opening paragraph. `, 10) + `\par` +
		`\begin{figure}\caption{Plot}\end{figure}` +
		strings.Repeat(`Body text paragraph. `, 10) + `\par`); err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) != 1 {
		t.Fatalf("the whole document fits one page, got %d", len(pages))
	}
	if txt := mvlText(pages[0].list); !strings.Contains(txt, "Figure1:Plot") {
		t.Errorf("the float did not ride the page it was written on: %q", txt)
	}
}

// Every float-placement parameter is a thing the CLASS says, and all seven were
// restated here with article's values whatever class was loaded. article.cls:
// 121-126 and amsart.cls:1317-1324:
//
//	                 article   amsart
//	topnumber              2        4
//	bottomnumber           1        4
//	totalnumber            3        4
//	topfraction           .7      .97
//	bottomfraction        .3      .97
//	textfraction          .2      .03
//	floatpagefraction     .5       .9
//
// Measured against tectonic on forty identical figures in amsart prose: the
// reference puts FOUR floats on a page where we put three, 11 pages against 14.
// With the parameters read: 11 against 11, four per page on both sides.
func TestFloatParametersComeFromTheClass(t *testing.T) {
	for _, c := range []struct {
		class                            string
		top, bot, total                  int
		topFrac, textFrac, floatPageFrac float64
	}{
		{"article", 2, 1, 3, 0.7, 0.2, 0.5},
		{"amsart", 4, 4, 4, 0.97, 0.03, 0.9},
	} {
		t.Run(c.class, func(t *testing.T) {
			e, err := compile([]byte("\\documentclass{"+c.class+"}\n\\begin{document}x\\end{document}"), Options{})
			if err != nil {
				t.Fatal(err)
			}
			// Defaults passed here are DELIBERATELY wrong, so a value that is not
			// actually read shows up as the nonsense rather than as the right answer.
			if got := e.floatCount("topnumber", -1); got != c.top {
				t.Errorf("topnumber = %d, want %d", got, c.top)
			}
			if got := e.floatCount("bottomnumber", -1); got != c.bot {
				t.Errorf("bottomnumber = %d, want %d", got, c.bot)
			}
			if got := e.floatCount("totalnumber", -1); got != c.total {
				t.Errorf("totalnumber = %d, want %d", got, c.total)
			}
			if got := e.floatFraction("topfraction", -1); got != c.topFrac {
				t.Errorf("topfraction = %v, want %v", got, c.topFrac)
			}
			if got := e.floatFraction("textfraction", -1); got != c.textFrac {
				t.Errorf("textfraction = %v, want %v", got, c.textFrac)
			}
			if got := e.floatFraction("floatpagefraction", -1); got != c.floatPageFrac {
				t.Errorf("floatpagefraction = %v, want %v", got, c.floatPageFrac)
			}
		})
	}
	// The two classes must not agree on everything, which is the whole point: a
	// hardcoded value passes any single-class test.
	a := compiledFor(t, "article")
	b := compiledFor(t, "amsart")
	if a.floatCount("topnumber", 0) == b.floatCount("topnumber", 0) {
		t.Error("article and amsart report the same topnumber: the value is not coming from the class")
	}
}

func compiledFor(t *testing.T, class string) *Engine {
	t.Helper()
	e, err := compile([]byte("\\documentclass{"+class+"}\n\\begin{document}x\\end{document}"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// A fraction outside (0,1] is not one, and a counter that cannot bound anything
// is not a bound: both fall back rather than paginate on nonsense.
func TestFloatParameterFallbacks(t *testing.T) {
	e := compiledFor(t, "article")
	if got := e.floatFraction("nosuchfraction", 0.42); got != 0.42 {
		t.Errorf("an undefined fraction = %v, want the default 0.42", got)
	}
	if got := e.floatCount("nosuchcounter", 7); got != 7 {
		t.Errorf("an undefined counter = %d, want the default 7", got)
	}
	e.define("badfraction", &meaning{kind: mMacro, body: stringToToks("12")}, true)
	if got := e.floatFraction("badfraction", 0.42); got != 0.42 {
		t.Errorf("a fraction of 12 was accepted as %v", got)
	}
	e.define("emptyfraction", &meaning{kind: mMacro, body: stringToToks("")}, true)
	if got := e.floatFraction("emptyfraction", 0.42); got != 0.42 {
		t.Errorf("an empty fraction was accepted as %v", got)
	}
}

// …and that the placer actually USES them. Reading the parameters is not the
// same as obeying them: a hardcoded `topMax := 2` passes every test above,
// because those call floatCount directly. The end-to-end quantity is how many
// pages a run of floats takes.
//
// Forty identical figures in amsart prose: the reference takes 11 pages and put
// four floats on each of pages 2, 3 and 4. We took 14 pages and three per page
// with the parameters restated; 11 and four with them read.
func TestFloatParametersReachThePlacer(t *testing.T) {
	var b strings.Builder
	b.WriteString("\\documentclass[10pt,reqno]{amsart}\n\\begin{document}\n")
	for i := 0; i < 40; i++ {
		b.WriteString("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor " +
			"incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud " +
			"exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.\n\n" +
			"\\begin{figure}\\rule{4cm}{2cm}\\caption{Une legende}\\end{figure}\n\n")
	}
	b.WriteString("\\end{document}")
	e, err := compile([]byte(b.String()), Options{})
	if err != nil {
		t.Fatal(err)
	}
	// 11 in the reference. Three per page instead of four cost three pages.
	if n := len(e.Pages()); n > 12 {
		t.Errorf("forty amsart floats took %d pages; the reference takes 11, and 14 is what "+
			"article's totalnumber=3 gives when amsart says 4", n)
	}
}

// The fractions have to EXIST, or a document that writes
// \renewcommand\topfraction{0.85} — neurips_2021.sty:223 does — is renewing a
// macro that is not there, and the engine's fallback to article's values is
// accidental rather than stated. Under an emulated class they were undefined,
// so acmart reported an empty \topfraction where amsart reported .97.
func TestFloatFractionsExistUnderEveryClass(t *testing.T) {
	for _, c := range []struct {
		class string
		top   float64
	}{
		{"article", 0.7},
		{"amsart", 0.97}, // the class states its own
		{"acmart", 0.7},  // emulated: article's, but DECLARED
	} {
		e, err := compile([]byte("\\documentclass{"+c.class+"}\n\\begin{document}x\\end{document}"), Options{})
		if err != nil {
			t.Fatalf("%s: %v", c.class, err)
		}
		if e.eq["topfraction"] == nil {
			t.Errorf("%s: \\topfraction is undefined; \\renewcommand on it would not be a renewal", c.class)
		}
		// -1 as the default, so a value that is not read shows up as nonsense.
		if got := e.floatFraction("topfraction", -1); got != c.top {
			t.Errorf("%s: \\topfraction = %v, want %v", c.class, got, c.top)
		}
	}
}
