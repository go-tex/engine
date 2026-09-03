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

// With GOTEX_FLOATS unset (the default), the FloatPlacementSubstrate is never
// loaded: \@float keeps its classic inline definition, no float is captured, and
// the main vertical list carries no floatNode — so the default output is the
// untouched inline rendering. This is the byte-identical-off guarantee at the
// macro level (the integration cmp against the base binary covers the bytes).
func TestFloatFlagOffKeepsInline(t *testing.T) {
	t.Setenv("GOTEX_FLOATS", "") // force OFF regardless of the ambient environment
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
