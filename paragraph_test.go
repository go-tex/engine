package engine

import "testing"

// A top-level paragraph of measured words breaks into multiple lines of \hsize,
// each packed to the line width, stacked with interline glue.
func TestParagraphBreaksIntoLines(t *testing.T) {
	e := New()
	e.SetFont(spMock{}) // every letter 5pt, space 3pt
	e.hsize = 40 * unity
	e.baselineskip = 10 * unity
	// six 3-letter words: "www " ×6. Each word 15pt, space 3pt.
	src := `www www www www www www\par`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run: %v", err)
	}
	p := e.Page()
	if p == nil {
		t.Fatal("empty page")
	}
	// Count the line boxes in the main vertical list.
	nLines := 0
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			nLines++
			if b.width != 40*unity {
				t.Errorf("line width %d sp want %d (packed to hsize)", b.width, 40*unity)
			}
		}
	}
	if nLines < 2 {
		t.Fatalf("expected the paragraph to wrap into ≥2 lines, got %d", nLines)
	}
	// Page height must exceed a single line (interline glue + multiple lines).
	if p.height <= 7*unity {
		t.Errorf("page height %d sp too small for a multi-line paragraph", p.height)
	}
}

// \hsize is settable and reads back via \the.
func TestHsizeParam(t *testing.T) {
	e := New()
	got, _ := e.Run(`\hsize=100pt \message{\the\hsize}`)
	if trimNL(got) != "100.0pt" {
		t.Errorf("hsize got %q want 100.0pt", trimNL(got))
	}
}
