package engine

import "testing"

// A tabular with | rules and \hline produces vertical rule nodes in rows and
// full-width horizontal rule rows in the vbox.
func TestTabularRules(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{}) // 5pt letters
	e.Run(`\begin{tabular}{|l|r|}\hline A & B \\ \hline CC & D \\ \hline\end{tabular}`)
	tb := lastVbox(e)
	if tb == nil {
		t.Fatal("no tabular vbox")
	}
	hlines, rows := 0, 0
	for _, n := range tb.list {
		switch b := n.(type) {
		case ruleNode:
			hlines++ // a full-width \hline
		case *boxNode:
			if b.kind == hbox {
				rows++
				// each data row must contain vertical rule nodes (| borders)
				vr := 0
				for _, c := range b.list {
					if r, ok := c.(ruleNode); ok && r.heightRun {
						vr++
					}
				}
				if vr < 3 { // |l|r| ⇒ 3 vertical rules per row
					t.Errorf("row has %d vertical rules, want 3", vr)
				}
			}
		}
	}
	if hlines != 3 {
		t.Errorf("got %d \\hline rules, want 3", hlines)
	}
	if rows != 2 {
		t.Errorf("got %d data rows, want 2", rows)
	}
}

// No | and no \hline ⇒ no rule nodes (plain tabular still works, unchanged).
func TestTabularNoRules(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.Run(`\begin{tabular}{ll}A & B \\ C & D\end{tabular}`)
	tb := lastVbox(e)
	for _, n := range tb.list {
		if _, ok := n.(ruleNode); ok {
			t.Error("plain tabular should have no rule rows")
		}
	}
}
