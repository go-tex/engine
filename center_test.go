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

// \centering zeroes \parfillskip (latex.ltx:11018) so the last line of a centred
// paragraph carries TWO infinite glues, \leftskip and \rightskip, and sits halfway
// across. With \parfillskip left at its 0pt plus 1fil default it carries three and
// settles a THIRD of the way across: measured against tectonic on one centred word
// in an article, reference x=305.6 (the page centre), ours x=264.8 before, 306.8
// after. \raggedright is the exception and keeps it, exactly as the reference does.
func TestCenteringZeroesParfillskip(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	read := func() glueSpec { return e.namedSkip("parfillskip") }
	deflt := glueSpec{stretch: unity, stretchOrder: 1}
	if got := read(); got != deflt {
		t.Fatalf("\\parfillskip default = %+v, want 0pt plus 1fil (latex.ltx:546)", got)
	}
	for _, c := range []struct {
		cmd  string
		want glueSpec
	}{
		{`\centering`, glueSpec{}},
		{`\raggedleft`, glueSpec{}},
		{`\raggedright`, deflt}, // keeps it: the line is flush left either way
	} {
		e2 := New()
		if err := e2.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e2.SetFont(spMock{})
		if _, err := e2.Run(c.cmd); err != nil {
			t.Fatal(err)
		}
		if got := e2.namedSkip("parfillskip"); got != c.want {
			t.Errorf("%s: \\parfillskip = %+v, want %+v", c.cmd, got, c.want)
		}
	}
}
