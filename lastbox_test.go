// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

func lastboxEngine(t *testing.T, src string) *Engine {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run %q: %v", src, err)
	}
	return e
}

// ⭐ \lastbox HANDS BACK THE LAST BOX AND TAKES IT OUT OF THE LIST.
//
// It answered void and removed nothing, which is go-tex/engine#430. Asked from
// inside tectonic the same source gives a real box — an ordinary 10pt line is
// ht=6.94pt dp=0.10999pt wd=345.0pt there — and 0/0/0 here, silently.
//
// tex.web:20990 is `cur_box:=tail; tail:=p; link(p):=null`: the value AND the
// removal, in that order.
// ⚠ \global, and it is not a detail. \setbox is scoped, and a \vbox IS a group,
// so a plain \setbox0\lastbox inside one is undone the moment the box closes —
// in this engine and in tectonic alike, where the same probe reads 0.0pt. That
// is exactly why the kernel writes {\setbox\z@\lastbox} in braces: \@doendpe
// wants the REMOVAL and throws the box away. A first version of this test used
// the plain form, read a void register and called the primitive broken.
func TestLastboxHandsBackTheBoxAndRemovesIt(t *testing.T) {
	e := lastboxEngine(t, `\setbox1\vbox{\hbox to 20pt{}\global\setbox0\lastbox}`)
	got := e.getBox(0)
	if got == nil {
		t.Fatal("\\lastbox gave a void box where the list ended with an hbox")
	}
	if got.width != 20*unity {
		t.Errorf("the box is %.2fpt wide, want 20.00pt — \\lastbox handed back something else",
			spToPt(got.width))
	}
	outer := e.getBox(1)
	if outer == nil {
		t.Fatal("box 1 is void")
	}
	// ⛔ THE REMOVAL IS HALF THE PRIMITIVE. A \lastbox that returns the box but
	// leaves it on the list would pass the width check above and still be the
	// defect: \@doendpe's whole purpose is that the \parindent box STOPS being
	// in the paragraph.
	for _, n := range outer.list {
		if _, ok := n.(*boxNode); ok {
			t.Fatalf("the vbox still holds a box: \\lastbox read it instead of removing it (list %T…)", outer.list[0])
		}
	}
}

// §20975-20977: only an hlist or a vlist ends the search. Anything else at the
// tail yields a VOID box and removes nothing — the same type guard deleteLast
// documents for \unskip, and tectonic agrees that \unskip meeting a kern
// removes nothing.
func TestLastboxDeclinesATailThatIsNotABox(t *testing.T) {
	e := lastboxEngine(t, `\setbox1\vbox{\hbox to 20pt{}\kern5pt\global\setbox0\lastbox}`)
	if b := e.getBox(0); b != nil && b.width != 0 {
		t.Errorf("\\lastbox took a box past a kern (width %.2fpt); the tail was not a box, so the value is void",
			spToPt(b.width))
	}
	outer := e.getBox(1)
	if outer == nil {
		t.Fatal("box 1 is void")
	}
	found := false
	for _, n := range outer.list {
		if _, ok := n.(*boxNode); ok {
			found = true
		}
	}
	if !found {
		t.Error("the hbox was removed although a kern stood between it and \\lastbox")
	}
}

// ⛔⛔ THE OUTER VERTICAL LIST IS REFUSED, AND THE LITERAL tex.web TEST WOULD NOT
// REFUSE IT. TeX asks `(mode=vmode)and(head=tail)` because its page builder has
// already carried every finished line onto the current page, so the list really
// is empty. This engine keeps the whole document in one vertical list and
// paginates at the end, so that test would pass and \lastbox would silently
// delete a real line. The reason is ported, not the three tokens.
func TestLastboxWillNotReachOntoThePage(t *testing.T) {
	e := lastboxEngine(t, `\documentclass{article}\begin{document}`+
		`Alfa bravo charlie.\par\setbox0\lastbox\end{document}`)
	if b := e.getBox(0); b != nil && b.width != 0 {
		t.Errorf("\\lastbox took a box (width %.2fpt) from the main vertical list; "+
			"that list holds the document's finished lines here, and TeX refuses "+
			"the same reach with \"I usually can't take things from the current page\"",
			spToPt(b.width))
	}
	// And the document must still hold its line.
	if len(e.Pages()) == 0 {
		t.Fatal("no pages")
	}
	if n := boxesUnder(e.Pages()[0]); n == 0 {
		t.Error("the page lost its text: \\lastbox removed a line of the document")
	}
}

// boxesUnder counts the boxes a page box holds, at any depth. (countBoxes is
// already taken in this package, by a helper that counts a []node.)
func boxesUnder(b *boxNode) int {
	if b == nil {
		return 0
	}
	n := 0
	for _, x := range b.list {
		if c, ok := x.(*boxNode); ok {
			n++
			n += boxesUnder(c)
		}
	}
	return n
}
