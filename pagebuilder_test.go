package engine

import "testing"

// With a small \vsize, a stack of boxes taller than one page splits into several.
func TestPageBuilderSplits(t *testing.T) {
	e := New()
	e.vsize = 30 * unity
	e.baselineskip = 0 // isolate box heights from interline glue for a clean count
	e.lineskip = 0
	// ten 8pt-tall boxes with 2pt vskip between → ~10pt pitch; 30pt/page ⇒ 3/page.
	for i := 0; i < 10; i++ {
		e.Run(`\hbox{\vrule width5pt height8pt}\vskip2pt `)
	}
	pages := e.Pages()
	if len(pages) < 3 {
		t.Fatalf("expected the material to span ≥3 pages, got %d", len(pages))
	}
	for i, p := range pages {
		if p.height > e.vsize {
			t.Errorf("page %d height %d sp exceeds vsize %d", i, p.height, e.vsize)
		}
	}
	// every box must survive somewhere (10 rules total across all pages)
	rules := 0
	for _, p := range pages {
		for _, n := range p.list {
			if b, ok := n.(*boxNode); ok && len(b.list) == 1 {
				rules++
			}
		}
	}
	if rules != 10 {
		t.Errorf("expected all 10 boxes preserved across pages, got %d", rules)
	}
}

func TestVsizeParam(t *testing.T) {
	e := New()
	got, _ := e.Run(`\vsize=200pt \message{\the\vsize}`)
	if trimNL(got) != "200.0pt" {
		t.Errorf("vsize got %q want 200.0pt", trimNL(got))
	}
}
