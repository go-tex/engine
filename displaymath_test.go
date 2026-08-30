package engine

import "testing"

// The displaymath environment (the kernel's unnumbered \[ … \]) and amsmath's
// full-width flalign(*) must render as display math, not be skipped as undefined
// environments (which drops every \frac/\sum/\left inside as an unknown command).
// Corpus census: displaymath dropped in 10 papers, flalign* in 14.
func TestDisplayMathEnvironments(t *testing.T) {
	for _, body := range []string{
		`\begin{displaymath} x=\frac{1}{2} \end{displaymath}`,
		`\begin{flalign*} a &= b & c &= d \end{flalign*}`,
		`\begin{flalign} a &= b \end{flalign}`,
	} {
		t.Run(body, func(t *testing.T) {
			e := New()
			e.LoadLaTeX()
			e.SetFont(spMock{})
			if _, err := e.Run(`\hsize=300pt` + "\n" + body); err != nil {
				t.Fatal(err)
			}
			if !hasMathNode(e.mvl) {
				t.Errorf("no math box placed — environment was dropped, not rendered")
			}
		})
	}
}

// Display math $$…$$ ends the paragraph and is centred on its own line (an hbox
// to \hsize with a math node between two fils).
func TestDisplayMathCentered(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 200 * unity
	if _, err := e.Run(`before $$x=\frac{1}{2}$$ after\par`); err != nil {
		t.Fatal(err)
	}
	// the display line: an hbox to hsize containing [fil, math, fil]
	var disp *boxNode
	for _, n := range e.mvl {
		b, ok := n.(*boxNode)
		if !ok || b.kind != hbox {
			continue
		}
		for _, c := range b.list {
			if _, ok := c.(mathNode); ok {
				disp = b
			}
		}
	}
	if disp == nil {
		t.Fatal("no display-math line found")
	}
	if disp.width != 200*unity {
		t.Errorf("display line width %d want hsize %d", disp.width, 200*unity)
	}
	// fil on both sides ⇒ math is centred
	if _, ok := disp.list[0].(glueNode); !ok {
		t.Errorf("display line should start with fil glue")
	}
	if _, ok := disp.list[len(disp.list)-1].(glueNode); !ok {
		t.Errorf("display line should end with fil glue")
	}
	// there should be at least 3 lines total: 'before', the display, 'after'
	nLines := 0
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			nLines++
		}
	}
	if nLines < 3 {
		t.Errorf("expected before/display/after ⇒ ≥3 lines, got %d", nLines)
	}
}
