package engine

import "testing"

// \title[short]{full} (the amsart running-head form) must store the FULL title as
// the mandatory argument and keep the optional short form unexpanded. Taking the
// short form as the mandatory argument left the closing bracket and the real title
// in the input stream, and executed any command the short form carried (a real
// paper writes \title[\tiny …]{…}) in global scope — which set the whole document
// body half-size.
func TestTitleOptionalArg(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\title[\tiny Short Form]{Full Title Here}`); err != nil {
		t.Fatalf("title: %v", err)
	}
	if got := trimSpaces(e.toksToString(e.eq["@title"].body)); got != "Full Title Here" {
		t.Errorf(`\@title = %q, want %q`, got, "Full Title Here")
	}
	if e.eq["@shorttitle"] == nil {
		t.Fatal(`\@shorttitle not stored`)
	}
	if got := e.toksToString(e.eq["@shorttitle"].body); got == "" {
		t.Error(`\@shorttitle stored empty, want the short form`)
	}
	// Nothing from the optional argument (its \tiny, its text, or the bracket) may
	// have leaked into horizontal mode: \title only records, it does not typeset.
	if len(e.parList) != 0 {
		t.Errorf("title leaked %d node(s) into the paragraph", len(e.parList))
	}
}

// The plain one-argument form (no bracket) still stores the mandatory title.
func TestTitleNoOptionalArg(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\title{Plain Title}`); err != nil {
		t.Fatalf("title: %v", err)
	}
	if got := trimSpaces(e.toksToString(e.eq["@title"].body)); got != "Plain Title" {
		t.Errorf(`\@title = %q, want %q`, got, "Plain Title")
	}
}

// \author mirrors \title: an optional short form is stored, the full name is the
// mandatory argument, and nothing leaks.
func TestAuthorOptionalArg(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\author[A. Short]{Ada Longname}`); err != nil {
		t.Fatalf("author: %v", err)
	}
	if got := trimSpaces(e.toksToString(e.eq["@author"].body)); got != "Ada Longname" {
		t.Errorf(`\@author = %q, want %q`, got, "Ada Longname")
	}
	if e.eq["@shortauthor"] == nil {
		t.Fatal(`\@shortauthor not stored`)
	}
	if _, err := e.Run(`\author{Solo Author}`); err != nil {
		t.Fatalf("author plain: %v", err)
	}
	if got := trimSpaces(e.toksToString(e.eq["@author"].body)); got != "Solo Author" {
		t.Errorf(`\@author plain = %q, want %q`, got, "Solo Author")
	}
}
