package engine

import (
	"strings"
	"testing"
	"time"
)

// A page break discards the glue/penalty/kern at the break. When a document
// leaves a very long trailing run of such discardable nodes — some class macros
// emit hundreds of thousands of forced-break \penalty nodes — Pages() must skip
// that run in one linear pass. A regression once advanced one node at a time
// while rescanning the whole remaining run each step, making pagination O(n²):
// half a million nodes took ~90s. This builds one real page followed by a long
// forced-break-penalty tail and asserts it paginates in linear time.
func TestPageBuilderDiscardableTailLinear(t *testing.T) {
	e := New()
	e.vsize = 100 * unity
	// One real box (becomes page 1), then a long run of forced-break penalties
	// with nothing after them: every break lands on an empty page (emitted as
	// none), so the loop must walk the whole tail without quadratic rescanning.
	list := []node{&boxNode{width: 10 * unity, height: 10 * unity}}
	const tail = 300000
	for i := 0; i < tail; i++ {
		list = append(list, penaltyNode{penalty: -int(InfPenalty)})
	}
	e.mvl = list

	done := make(chan int, 1)
	start := time.Now()
	go func() { done <- len(e.Pages()) }()
	select {
	case n := <-done:
		if n != 1 {
			t.Fatalf("expected 1 page (one box, then only discardable nodes), got %d", n)
		}
		if el := time.Since(start); el > 10*time.Second {
			t.Fatalf("pagination of a %d-node discardable tail took %s; expected linear time", tail, el)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("Pages() did not finish in 20s on a %d-node discardable tail (quadratic page builder)", tail)
	}
}

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

// A class that computes its page height through a custom \output routine the
// engine never runs (aastex/ltxgrid, revtex) leaves \vsize at 0. Breaking against
// 0 would make every box its own overfull page — hundreds of near-empty pages.
// The page builder falls back to a sane default height, so the material stays on
// a handful of pages and every box survives.
func TestPageBuilderZeroVsize(t *testing.T) {
	e := New()
	e.vsize = 0 // as a ltxgrid/aastex class leaves it
	e.baselineskip = 0
	e.lineskip = 0
	for i := 0; i < 60; i++ {
		e.Run(`\hbox{\vrule width5pt height8pt}\vskip2pt `)
	}
	pages := e.Pages()
	if len(pages) == 0 {
		t.Fatal("zero \\vsize produced no pages (content dropped)")
	}
	if len(pages) > 5 {
		t.Fatalf("zero \\vsize exploded into %d pages for 60 small boxes; fallback height not applied", len(pages))
	}
	rules := 0
	for _, p := range pages {
		for _, n := range p.list {
			if b, ok := n.(*boxNode); ok && len(b.list) == 1 {
				rules++
			}
		}
	}
	if rules != 60 {
		t.Errorf("expected all 60 boxes preserved, got %d", rules)
	}
}

// Pages() is capped at maxPages so a class whose page-fitting machinery runs
// away (WileyNJD's \BX \vsplit loop can emit 142841 near-empty pages) cannot
// exhaust memory/disk; the cap is recorded so it is not silent.
func TestPageBuilderCap(t *testing.T) {
	e := New()
	e.baselineskip = 0
	e.lineskip = 0
	// far more forced breaks than the cap allows.
	e.Run(strings.Repeat(`\hbox{\vrule width1pt height1pt}\penalty-10000 `, maxPages+1000))
	if n := len(e.Pages()); n != maxPages {
		t.Errorf("Pages() returned %d, want the cap %d", n, maxPages)
	}
	if e.skippedCS["gotex@pagelimit"] == 0 {
		t.Error("page-limit cap was hit but not recorded")
	}
}

func TestVsizeParam(t *testing.T) {
	e := New()
	got, _ := e.Run(`\vsize=200pt \message{\the\vsize}`)
	if trimNL(got) != "200.0pt" {
		t.Errorf("vsize got %q want 200.0pt", trimNL(got))
	}
}
