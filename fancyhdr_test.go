package engine

import (
	"strings"
	"testing"
)

// countRules counts rule nodes directly in a vertical list.
func countRules(nodes []node) int {
	n := 0
	for _, x := range nodes {
		if _, ok := x.(ruleNode); ok {
			n++
		}
	}
	return n
}

// \pagestyle{fancy} with header/footer fields places them (and a header rule) on the
// page, and \thepage in a field reflects the real page number.
func TestFancyHdr(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.vsize = 300 * unity
	src := `\pagestyle{fancy}
\lhead{Left}\rhead{Right}\cfoot{\thepage}
First page.\newpage Second page.`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	var b strings.Builder
	collectChars(pages[0].list, &b)
	p1 := b.String()
	if !strings.Contains(p1, "Left") || !strings.Contains(p1, "Right") {
		t.Errorf("page 1 header missing Left/Right; got %q", p1)
	}
	if !strings.Contains(p1, "1") { // \thepage in the centre footer
		t.Errorf("page 1 footer should show 1; got %q", p1)
	}
	// The header rule is present.
	if countRules(pages[0].list) == 0 {
		t.Error("page 1 should have a header rule")
	}
	// Page 2's \thepage footer shows 2.
	var b2 strings.Builder
	collectChars(pages[1].list, &b2)
	if !strings.Contains(b2.String(), "2") {
		t.Errorf("page 2 footer should show 2; got %q", b2.String())
	}
}

// \fancyhf{} clears all six fields; \fancyhead[R]{x} sets only the right header.
func TestFancyHfAndPos(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\lhead{old}\fancyhf{}\fancyhead[R]{NEW}`); err != nil {
		t.Fatal(err)
	}
	if len(e.fancyHF[fldHL]) != 0 {
		t.Error("\\fancyhf{} should have cleared the left header")
	}
	if len(e.fancyHF[fldHR]) == 0 {
		t.Error("\\fancyhead[R] should have set the right header")
	}
	if len(e.fancyHF[fldHC]) != 0 {
		t.Error("\\fancyhead[R] must not set the centre header")
	}
}

// scanFancyPos decodes the [LCR] mask.
func TestScanFancyPos(t *testing.T) {
	check := func(src string, want int) {
		e := New()
		e.base = []rune(src)
		e.bpos = 0
		if got := e.scanFancyPos(); got != want {
			t.Errorf("scanFancyPos(%q) = %d, want %d", src, got, want)
		}
	}
	check("[L]", 1)
	check("[C]", 2)
	check("[R]", 4)
	check("[LR]", 5)
	check("[]", 7) // empty bracket ⇒ all
	check("nobracket", 0)
}

// book.cls and report.cls say \pagestyle{headings} in their preamble, so
// \@oddhead is defined for the WHOLE document. A page that asks for "plain" —
// every chapter opening, and the first page of the contents list — must still be
// plain: its folio belongs centred at the FOOT, not in the running head at the
// top. The assembler used to take the running-head path whenever \@oddhead
// existed, whatever style was in force. Measured against tectonic on an 11pt
// book: the folio centred at x=277.49 near the bottom of the page there, and it
// came out at x=100.90 near the top here.
func TestThisPageStylePlainBeatsAClassWideRunningHead(t *testing.T) {
	e, err := compile([]byte(`\documentclass{book}
\begin{document}
\chapter{One}
Body.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The control: without a class-wide \@oddhead this test would pass for the
	// wrong reason, because there would be no running head to lose to.
	if !e.hasLatexHead() {
		t.Fatal("book.cls defined no \\@oddhead here; the interaction under test cannot arise")
	}
	pages := e.Pages()
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
	top := pages[0].list
	if len(top) < 2 {
		t.Fatalf("the page has %d items", len(top))
	}
	// The plain style ends the page with vertical fil and the folio box, so the
	// folio is the LAST thing on the page. The running-head style would put it in
	// a box at the very top instead.
	last, ok := top[len(top)-1].(*boxNode)
	if !ok || !boxDraws(last, '1') {
		t.Errorf("the last item on the page is %T and does not hold the folio; a plain page ends with it", top[len(top)-1])
	}
	if first, ok := top[0].(*boxNode); ok && boxDraws(first, '1') {
		t.Error("the folio is in a box at the TOP of the page: the running head won over \\thispagestyle{plain}")
	}
}

// boxDraws reports whether the box draws that character anywhere inside it.
func boxDraws(b *boxNode, ch rune) bool {
	for _, n := range b.list {
		switch v := n.(type) {
		case charNode:
			if v.ch == ch {
				return true
			}
		case *boxNode:
			if boxDraws(v, ch) {
				return true
			}
		}
	}
	return false
}
