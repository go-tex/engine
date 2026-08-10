package engine

import "testing"

// A box (e.g. a tabular) contributed inside \begin{center} is wrapped to \hsize
// with fil on each side, so it is centred rather than flush-left.
func TestCenteredBox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.hsize = 100 * unity
	e.Run(`\begin{center}\begin{tabular}{l}a\end{tabular}\end{center}`)
	// find the wrapping hbox-to-hsize whose middle child is the tabular vbox
	var wrap *boxNode
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox && b.width == 100*unity {
			for _, c := range b.list {
				if vb, ok := c.(*boxNode); ok && vb.kind == vbox {
					wrap = b
				}
			}
		}
	}
	if wrap == nil {
		t.Fatal("tabular was not wrapped/centred in a hbox-to-hsize")
	}
	if _, ok := wrap.list[0].(glueNode); !ok {
		t.Errorf("centred box should have leading fil glue")
	}
}
