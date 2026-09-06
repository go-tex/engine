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

// A footnoteNode reserves its OWN height, and nothing more. What the foot area
// costs the page beyond that — \skip\footins — is charged once for the page, not
// once per note (tex.web:19638).
func TestFootnoteReservesItsOwnHeightOnly(t *testing.T) {
	body := &boxNode{kind: vbox, height: 20 * unity, depth: 3 * unity}
	if got, want := vContribution(footnoteNode{body: body}), 20*unity+3*unity; got != want {
		t.Errorf("footnote vContribution = %d, want %d (its body, with no per-note allowance)", got, want)
	}
}

// \skip\footins is charged ONCE for a page however many notes land on it. Charging
// it per note reserved room nobody used: measured against tectonic on a page of
// four footnotes, our foot area ended 45pt above the bottom of the text block where
// the reference's ends flush with it.
func TestFootinsSkipIsChargedOncePerPage(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	note := func() footnoteNode {
		return footnoteNode{body: &boxNode{kind: vbox, height: 10 * unity}}
	}
	line := func() node { return &boxNode{kind: hbox, height: 10 * unity} }
	skip := e.footinsSkip()
	if skip <= 0 {
		t.Fatal("\\skip\\footins reads zero; the test would prove nothing")
	}
	// vsize large enough that neither list breaks: the break index is the length.
	e.vsize = 1000 * unity
	one := []node{line(), note()}
	two := []node{line(), note(), note()}
	if e.findPageBreak(one, 0) != len(one) || e.findPageBreak(two, 0) != len(two) {
		t.Fatal("the probe lists broke; vsize is too small for the test")
	}
	// Now squeeze: a vsize that fits one note's worth of foot area plus the lines.
	// With the skip charged per note the second list would break early; with it
	// charged once it does not.
	e.vsize = 10*unity + 10*unity + 10*unity + skip
	if got := e.findPageBreak(two, 0); got != len(two) {
		t.Errorf("page broke at %d of %d: \\skip\\footins looks charged per note, not per page", got, len(two))
	}
}

// The assembled page lifts footnote bodies out of the content flow and stacks
// them below a separator rule.
func TestFootnotePageAssembly(t *testing.T) {
	e := New()
	e.hsize = 300 * unity
	content := &boxNode{kind: hbox, width: 100 * unity, height: 8 * unity}
	note := &boxNode{kind: vbox, width: 120 * unity, height: 10 * unity}
	page := e.assemblePage([]node{content, footnoteNode{body: note}}, 1)
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
