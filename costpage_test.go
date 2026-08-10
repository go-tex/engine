package engine

import "testing"

func countPages(e *Engine) int { return len(e.Pages()) }

// A forced break (\penalty-10000, i.e. \eject) starts a new page even when the
// material would otherwise fit on one page.
func TestForcedPageBreak(t *testing.T) {
	e := New()
	e.vsize = 1000 * unity // huge: everything fits
	e.Run(`\hbox{\vrule width5pt height10pt}\hbox{\vrule width5pt height10pt}`)
	if n := countPages(e); n != 1 {
		t.Fatalf("without forced break expected 1 page, got %d", n)
	}
	e2 := New()
	e2.vsize = 1000 * unity
	e2.Run(`\hbox{\vrule width5pt height10pt}\penalty-10000 \hbox{\vrule width5pt height10pt}`)
	if n := countPages(e2); n != 2 {
		t.Fatalf("a forced \\penalty-10000 should split into 2 pages, got %d", n)
	}
}

// \penalty >= 10000 forbids a page break there (glue that would otherwise be a
// legal break is preceded by the infinite penalty, so no break is taken early).
func TestInfinitePenaltyForbidsBreak(t *testing.T) {
	e := New()
	e.vsize = 25 * unity // room for ~2 boxes of 10pt
	// three 10pt boxes with an infinite penalty gluing the first two together
	e.Run(`\hbox{\vrule width5pt height10pt}\penalty10000 \vskip1pt ` +
		`\hbox{\vrule width5pt height10pt}\vskip1pt ` +
		`\hbox{\vrule width5pt height10pt}`)
	pages := e.Pages()
	// first page must contain the first two boxes (break after box1 is forbidden)
	if len(pages) < 2 {
		t.Fatalf("expected the tall material to paginate, got %d pages", len(pages))
	}
	nb := 0
	for _, n := range pages[0].list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			nb++
		}
	}
	if nb < 2 {
		t.Errorf("infinite penalty should keep boxes 1&2 together on page 1, got %d boxes", nb)
	}
}
