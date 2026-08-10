package engine

import "testing"

// \centerline{ab} builds an \hbox to\hsize with \hfil on both sides; the two
// fils split the slack equally, so each set width is (hsize - content)/2.
func TestCenterlineMacro(t *testing.T) {
	e := New()
	if err := e.LoadPlain(); err != nil {
		t.Fatalf("LoadPlain: %v", err)
	}
	e.SetFont(spMock{}) // 'a','b' = 5pt each ⇒ content 10pt
	e.hsize = 100 * unity
	if _, err := e.Run(`\centerline{ab}`); err != nil {
		t.Fatal(err)
	}
	if len(e.mvl) == 0 {
		t.Fatal("no line contributed")
	}
	line, ok := e.mvl[0].(*boxNode)
	if !ok || line.width != 100*unity {
		t.Fatalf("expected an hbox to hsize, got %+v", e.mvl[0])
	}
	// first child is the leading \hfil; it must be set to (100-10)/2 = 45pt
	g, ok := line.list[0].(glueNode)
	if !ok {
		t.Fatalf("first child not glue: %+v", line.list[0])
	}
	if w := line.setWidth(g.spec); w != 45*unity {
		t.Errorf("leading \\hfil set width %d sp want %d (centered)", w, 45*unity)
	}
}

// \leftline puts all the slack on the right (leading content flush left).
func TestLeftlineMacro(t *testing.T) {
	e := New()
	e.LoadPlain()
	e.SetFont(spMock{})
	e.hsize = 100 * unity
	e.Run(`\leftline{ab}`)
	line := e.mvl[0].(*boxNode)
	// first child should be a char (no leading glue)
	if _, isGlue := line.list[0].(glueNode); isGlue {
		t.Errorf("\\leftline should not lead with glue: %+v", line.list)
	}
}
