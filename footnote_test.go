package engine

import "testing"

// \footnote numbers its notes, drops a raised marker inline, and attaches each
// note to the vertical list so the page builder can place it at the foot.
func TestFootnote(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent A\footnote{First}B\footnote{Second}.`); err != nil {
		t.Fatal(err)
	}
	if e.footnoteCounter != 2 {
		t.Errorf("footnote counter = %d, want 2", e.footnoteCounter)
	}
	// Two footnote nodes reached the main vertical list.
	var notes []*boxNode
	for _, n := range e.mvl {
		if fn, ok := n.(footnoteNode); ok {
			notes = append(notes, fn.body)
		}
	}
	if len(notes) != 2 {
		t.Fatalf("footnote nodes on mvl = %d, want 2", len(notes))
	}
	// Bodies are numbered "N. text" (the space is glue, dropped by mvlText).
	if got := mvlText([]node{notes[0]}); got != "1.First" {
		t.Errorf("note 1 body = %q, want %q", got, "1.First")
	}
	if got := mvlText([]node{notes[1]}); got != "2.Second" {
		t.Errorf("note 2 body = %q, want %q", got, "2.Second")
	}
	// The inline marker is a raised (negative-shift) box on the first line.
	line, _ := e.mvl[0].(*boxNode)
	if line == nil {
		t.Fatal("no first line box")
	}
	raised := false
	for _, n := range line.list {
		if b, ok := n.(*boxNode); ok && b.shift < 0 {
			raised = true
		}
	}
	if !raised {
		t.Error("no raised footnote marker on the first line")
	}
}

// A footnoteNode reserves its body height plus the foot-area allowance, so the
// page breaks early enough to fit the note.
func TestFootnoteReservesHeight(t *testing.T) {
	body := &boxNode{kind: vbox, height: 20 * unity, depth: 3 * unity}
	got := vContribution(footnoteNode{body: body})
	want := 20*unity + 3*unity + footnoteReserve
	if got != want {
		t.Errorf("footnote vContribution = %d, want %d", got, want)
	}
}

// The assembled page lifts footnote bodies out of the content flow and stacks
// them below a separator rule.
func TestFootnotePageAssembly(t *testing.T) {
	e := New()
	e.hsize = 300 * unity
	content := &boxNode{kind: hbox, width: 100 * unity, height: 8 * unity}
	note := &boxNode{kind: vbox, width: 120 * unity, height: 10 * unity}
	page := e.assemblePage([]node{content, footnoteNode{body: note}})
	// The content box and the note box both appear, with a separator rule between.
	var boxes, rules int
	for _, n := range page.list {
		switch n.(type) {
		case *boxNode:
			boxes++
		case ruleNode:
			rules++
		}
	}
	if boxes != 2 {
		t.Errorf("assembled page has %d boxes, want 2 (content + note)", boxes)
	}
	if rules != 1 {
		t.Errorf("assembled page has %d rules, want 1 (footnote separator)", rules)
	}
}
