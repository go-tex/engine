// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \thispagestyle{name} styles THIS page; the document's own \pagestyle resumes on
// the next one.
//
// It used to take \pagestyle's path and set the style permanently. \maketitle
// issues \thispagestyle{plain} (or {empty}) on nearly every paper, so one title
// page silently turned the running head off for the whole document. Beside
// tectonic on a three-page document with \thispagestyle{empty} on page 1, the
// reference heads pages 2 and 3 and this engine headed nothing.
//
// The page builder runs after the document has been read, so at the moment
// \thispagestyle is called nothing knows which page it falls on. The override is
// contributed to the vertical list and lands on that page by construction — which
// is how TeX gets the same answer, its output routine reading \@specialstyle for
// the page it is shipping.
func TestThisPagestyleAppliesToOnePage(t *testing.T) {
	// The override rides the vertical list, so the unit under test is that it is
	// CONTRIBUTED where the command was met and LIFTED by the page carrying it —
	// exactly once, leaving the list otherwise untouched.
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\thispagestyle{empty}`); err != nil {
		t.Fatal(err)
	}
	var found int
	for _, n := range e.mvl {
		if _, ok := n.(pageStyleNode); ok {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("%d pageStyleNode on the list, want 1", found)
	}
	style, rest, ok := takePageStyle(e.mvl)
	if !ok || style != "empty" {
		t.Errorf("takePageStyle = %q/%v, want \"empty\"/true", style, ok)
	}
	if len(rest) != len(e.mvl)-1 {
		t.Errorf("list is %d nodes after lifting, want %d", len(rest), len(e.mvl)-1)
	}
	// A second page finds nothing: the override went with the page that carried it.
	if _, _, ok := takePageStyle(rest); ok {
		t.Error("the override survived its own page")
	}
	// ⛔ An unknown name is KEPT, and it is routed by whether a head was declared
	// rather than by what it is called.
	//
	// It used to be rewritten to "plain", mirroring \pagestyle. That mirror was the
	// defect: assemblePage reads `style != "plain"` to let \thispagestyle{plain}
	// beat a class-wide running head, so a style flattened to "plain" claimed that
	// exception — and \pagestyle{headings} lost its head while headBand still
	// reserved 37pt for it. LaTeX does not flatten either: an undefined \ps@name
	// goes to \undefinedpagestyle (latex.ltx:13326-13329), not to plain.
	//
	// The outcome for a name nobody defined is unchanged, and the second assertion
	// is what keeps it so: with no \@oddhead, hasLatexHead() is false, so the head
	// path is not taken and the page is assembled plainly.
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	e2.Run(`\thispagestyle{nosuchstyle}`)
	if s, _, ok := takePageStyle(e2.mvl); !ok || s != "nosuchstyle" {
		t.Errorf("unknown style = %q/%v, want it kept as \"nosuchstyle\"/true", s, ok)
	}
	if e2.hasLatexHead() {
		t.Error("no \\@oddhead was declared, yet hasLatexHead() is true: " +
			"an unknown style would be routed to the head path by accident")
	}
}

// The running marks are recorded and read back, which is what a head showing
// \rightmark needs. \markright replaces only the right one.
func TestRunningMarks(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	out, err := e.Run(`\markboth{GAUCHE}{DROITE}\message{[\leftmark|\rightmark]}` +
		`\markright{AUTRE}\message{[\leftmark|\rightmark]}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[GAUCHE|DROITE]", "[GAUCHE|AUTRE]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %q", want, out)
		}
	}
}
