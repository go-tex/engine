// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A \par inside the argument of a macro that is not \long abandons the call
// (tex.web §392/§396). That is an ALARM — the call is dropped, so whatever it would
// have set is gone — and it now has its own count. It used to be tallied in Skipped
// under the error message itself, so the report printed "\Paragraph ended before
// argument was complete" as if it were a missing command, and a corpus census read
// an error as the fifth most frequent feature gap.
func TestRunawayArgumentIsCountedNotNamedAsACommand(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run("\\def\\one#1{[#1]}\\one{A\n\n B}\\par"); err != nil {
		t.Fatal(err)
	}
	d := e.Diagnostics()
	if d.RunawayArgs != 1 {
		t.Errorf("RunawayArgs = %d, want 1", d.RunawayArgs)
	}
	for name := range d.Skipped {
		if name == "Paragraph ended before argument was complete" {
			t.Error("the error message is still filed as an undefined command")
		}
	}
}

// A well-formed call is not counted.
func TestWellFormedArgumentIsNotARunaway(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\def\one#1{[#1]}\one{AB}\par`); err != nil {
		t.Fatal(err)
	}
	if n := e.Diagnostics().RunawayArgs; n != 0 {
		t.Errorf("RunawayArgs = %d on a well-formed call, want 0", n)
	}
}

// The count alone says a document lost something; it does not say what. Over the
// corpus, 199 abandoned calls were a sum with no handle on it; naming them put 179
// of them — 90% — on \@hangfrom, a defect in our own substrate rather than any
// document's malformed macro. RunawayMacros carries that name.
func TestRunawayArgumentNamesTheMacro(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run("\\def\\one#1{[#1]}\\def\\two#1{(#1)}\\one{A\n\n B}\\two{C\n\n D}\\one{E\n\n F}\\par"); err != nil {
		t.Fatal(err)
	}
	got := e.Diagnostics().RunawayMacros
	if got["one"] != 2 {
		t.Errorf(`RunawayMacros["one"] = %d, want 2`, got["one"])
	}
	if got["two"] != 1 {
		t.Errorf(`RunawayMacros["two"] = %d, want 1`, got["two"])
	}
}
