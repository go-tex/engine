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
