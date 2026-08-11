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
