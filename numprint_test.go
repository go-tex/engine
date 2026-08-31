// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \numprint and \np recover the number the undefined command used to DROP: the
// digits are grouped in threes (thin spaces, which mvlText reads over) and the
// optional [unit] follows as plain text.
func TestNumprintRecoversNumbersAndUnits(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `A\numprint{1234567.89}B ` +
		`C\numprint{-42}D ` +
		`E\np{1000}F ` +
		`G\numprint[km]{5}H ` +
		`I\numprint[kg]{12345}J`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	// The numbers are back (thin-space grouping is glue, so mvlText reads the bare
	// digits); the units set as plain text.
	for _, want := range []string{"1234567.89", "42", "1000", "5", "km", "12345", "kg"} {
		if !strings.Contains(txt, want) {
			t.Errorf("numprint lost %q: %q", want, txt)
		}
	}
	// The surrounding letters survive and the number did not vanish between A and B.
	if !strings.Contains(txt, "A1234567.89B") {
		t.Errorf("number dropped or displaced: %q", txt)
	}
}

// An empty or non-numeric argument typesets nothing rather than aborting — still no
// stray braces or leaked argument.
func TestNumprintEmptyArgIsSafe(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`X\numprint{}Y`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); txt != "XY" {
		t.Errorf("empty numprint = %q, want XY", txt)
	}
}
