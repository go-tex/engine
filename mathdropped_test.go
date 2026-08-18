// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// An unknown math command drops the WHOLE equation inside the go-tex/math layer —
// invisible content loss inside a formula. Diagnostics.MathDropped surfaces it as
// math, keyed by the triggering command, and lifts it OUT of the text-mode Skipped
// tally so a math feature gap is not mistaken for a missing text macro. The raw drop
// stays in SkippedCommands (the general "what was dropped" surface is unchanged).
func TestDiagnosticsMathDropped(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`$\nosuchmathprimitive$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	d := e.Diagnostics()
	if d.MathDropped["\\nosuchmathprimitive"] == 0 {
		t.Fatalf("expected the dropped equation surfaced under MathDropped, got %v", d.MathDropped)
	}
	// It must NOT also appear in the text-mode Skipped tally: math drops are lifted out.
	if _, ok := d.Skipped["\\nosuchmathprimitive"]; ok {
		t.Errorf("math drop leaked into Skipped (should be MathDropped only): %v", d.Skipped)
	}
	// Backward compatibility: the raw SkippedCommands surface still carries it.
	if e.SkippedCommands()["\\nosuchmathprimitive"] == 0 {
		t.Errorf("SkippedCommands lost the raw math drop: %v", e.SkippedCommands())
	}
}

// A non-command math failure (not "unknown command \X") is tallied under the generic
// "$math$" sentinel so the loss is still counted, just without a command to blame.
func TestDiagnosticsMathDroppedGenericSentinel(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	// \left without a matching \right is a structural math error, not an unknown command.
	if _, err := e.Run(`$\left( x$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	d := e.Diagnostics()
	total := 0
	for _, v := range d.MathDropped {
		total += v
	}
	if total == 0 {
		t.Fatalf("expected a math drop recorded for the malformed equation, got %v", d.MathDropped)
	}
}

// Valid math renders fully: nothing is dropped, so MathDropped is empty. This guards
// against the instrumentation over-reporting (crying wolf on math that renders fine).
func TestDiagnosticsMathDroppedEmptyForValidMath(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`Text $\frac{a}{b} + \sum_{i=1}^{n} x_i \cdot \alpha$ more.`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if d := e.Diagnostics(); len(d.MathDropped) != 0 {
		t.Fatalf("valid math must drop nothing, got MathDropped=%v", d.MathDropped)
	}
}

// The MathDropped view is an independent copy: mutating it must not corrupt the
// engine's internal tally (Diagnostics is a read-only snapshot).
func TestDiagnosticsMathDroppedIsACopy(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`$\nosuchmathprimitive$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	d := e.Diagnostics()
	d.MathDropped["\\nosuchmathprimitive"] = 999
	if e.mathDropped["\\nosuchmathprimitive"] == 999 {
		t.Error("Diagnostics().MathDropped aliased the engine's internal map")
	}
}
