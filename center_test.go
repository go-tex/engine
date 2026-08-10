package engine

import "testing"

// \begin{center}…\end{center} sets fil \leftskip and \rightskip for its lines and
// reverts them afterwards (group-scoped), so following text is justified again.
func TestCenterEnvironment(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.hsize = 100 * unity
	e.Run(`\begin{center}aa aa\end{center}`)
	// inside center, rightskip had fil; after \end{center} it must be back to zero
	if e.rightskip != (glueSpec{}) {
		t.Errorf("rightskip leaked after \\end{center}: %+v", e.rightskip)
	}
	if e.leftskip != (glueSpec{}) {
		t.Errorf("leftskip leaked after \\end{center}: %+v", e.leftskip)
	}
	// the centered line has leftskip fil at its start (centred)
	var line *boxNode
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			line = b
			break
		}
	}
	if line == nil {
		t.Fatal("no centered line")
	}
	if g, ok := line.list[0].(glueNode); !ok || g.spec.stretchOrder != 1 {
		t.Errorf("centered line should start with a fil (leftskip), got %T %+v", line.list[0], line.list[0])
	}
}
