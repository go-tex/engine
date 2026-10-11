// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \restorecr puts the end-of-line character back to catcode 5. latex.ltx:6648 is
// the whole definition:
//
//	\gdef\restorecr{\catcode`\^^M5 }
//
// It is the undo for \obeycr, and it is called DEFENSIVELY: fancyhdr's
// \fancy@reset opens with \restorecr\endlinechar=13 (fancyhdr.sty:322), once per
// head or foot built, which is why the census of undefined commands counted 111
// of them over 5 papers — the same five that ship fancyhdr.
//
// ⚠ It moves no ink: a witness page is byte-identical before and after, because
// the catcode it restores is the one already in force unless a document made the
// end of line active. What it buys is that a command the kernel defines is
// defined, and that a census of what is MISSING stops being led to it. The test
// therefore asserts the catcode, which is the thing that changed.
func TestRestorecrPutsTheEndOfLineBackToCatcodeFive(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// Move it away first, so the assertion cannot pass on the default.
	if _, err := e.Run(`\catcode13=12 \message{[\the\catcode13]}\restorecr\message{[\the\catcode13]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[12] [5]" {
		t.Errorf("catcode of the end of line, before and after \\restorecr = %s, want [12] [5]", got)
	}
}
