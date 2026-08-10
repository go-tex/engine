package engine

import (
	"strings"
	"testing"
)

// A top-level "document": two hboxes separated by \vskip contribute to the main
// vertical list, and Page() vpacks them into a single page box.
func TestMainVerticalListPage(t *testing.T) {
	e := New()
	src := `\hbox{\vrule width6pt height10pt}` +
		`\vskip4pt ` +
		`\hbox{\vrule width6pt height8pt}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run: %v", err)
	}
	p := e.Page()
	if p == nil {
		t.Fatal("empty page")
	}
	// height = 10 (box1) + 4 (explicit \vskip) + 4 (interline glue: baselineskip
	// 12 − prevdepth 0 − box2 height 8) + 8 (box2) = 26pt; width = max = 6pt.
	if p.width != 6*unity {
		t.Errorf("page width %d sp want %d", p.width, 6*unity)
	}
	if p.height != 26*unity {
		t.Errorf("page height %d sp want %d", p.height, 26*unity)
	}
	svg := e.RenderPage(0)
	mustContain(t, svg, `width="6pt" height="26pt"`)
	// box1's rule at top: y=0, height 10
	mustContain(t, svg, `<rect x="0" y="0" width="6" height="10"/>`)
	// box2's rule after 10+4+4=18pt: y=18, height 8
	mustContain(t, svg, `<rect x="0" y="18" width="6" height="8"/>`)
}

func TestEmptyPageRendersNothing(t *testing.T) {
	e := New()
	e.Run(`\count0=1 `)
	if got := e.RenderPage(0); got != "" {
		t.Errorf("expected empty render, got %q", got)
	}
	if strings.Contains(e.RenderPage(0), "rect") {
		t.Error("unexpected content")
	}
}
