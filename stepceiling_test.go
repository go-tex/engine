package engine

import (
	"strings"
	"testing"
)

// ⛔ THE REPORT MUST CARRY THE EVIDENCE, AND MUST NOT PICK A SIDE.
//
// "possible infinite loop" alone sent go-tex/engine#360 after the pgf path layer
// for seven weeks on arXiv 2405.11065, which has no loop: raising the no-progress
// ceiling twentyfold stops the compile at the same byte with the same 7 pages of
// 18, while raising the ABSOLUTE ceiling renders all 18 and trips nothing.
//
// ⛔ And the opposite claim is false too, which is why this test asserts it:
// "the absolute ceiling fired, so the input was advancing, so it is not a loop"
// is wrong whenever the absolute ceiling is the LOWER one — every test that
// lowers stepLimit, and TestRunawayGuardStrict is a genuine loop doing exactly
// that. So both shapes below must produce the same even-handed report, with the
// counters in it; a message that named a cause would be caught here.
func TestTheRunawayReportCarriesItsEvidenceWithoutPickingASide(t *testing.T) {
	for _, c := range []struct {
		name  string
		src   string
		steps int // absolute ceiling
		tight int // no-progress ceiling
	}{{
		// A real loop, stopped by the no-progress ceiling.
		name:  "a loop caught by the no-progress ceiling",
		src:   `\def\x{\x}\x`,
		steps: 100_000_000,
		tight: 50_000,
	}, {
		// The SAME loop, but the absolute ceiling is the lower one, so that is
		// what fires. The report must not conclude "not a loop" from it.
		name:  "the same loop caught by the absolute ceiling",
		src:   `\def\x{\x}\x`,
		steps: 50_000,
		tight: 100_000_000,
	}, {
		// Not a loop: every step reads one more token of base input, so the
		// no-progress counter keeps resetting. This is the shape of a tikzDevice
		// figure — a great many steps with the input advancing throughout.
		name:  "a document that merely needs more budget",
		src:   `\def\spin#1{#1}` + strings.Repeat(`\spin{a}`, 4000),
		steps: 2_000,
		tight: 100_000_000,
	}} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			if err := e.LoadPlain(); err != nil {
				t.Fatalf("LoadPlain: %v", err)
			}
			e.stepLimit, e.tightLimit = c.steps, c.tight
			_, err := e.Run(c.src)
			if err == nil {
				t.Fatalf("no error: neither ceiling fired (steps=%d, peak=%d)",
					e.Diagnostics().ExpansionSteps, e.Diagnostics().NoProgressPeak)
			}
			msg := err.Error()
			// The word the rest of the suite and its callers match on.
			if !strings.Contains(msg, "runaway") {
				t.Errorf("report no longer says runaway, which other callers match on: %v", err)
			}
			// Both readings, so nobody repeats #360's wrong turn...
			for _, want := range []string{"expansion steps", "since the input last advanced",
				"non-terminating", "needs more than the budget", "StepLimitHeavy"} {
				if !strings.Contains(msg, want) {
					t.Errorf("report is missing %q, which is what makes it actionable:\n  %v", want, err)
				}
			}
			// ...and no verdict either way.
			for _, forbidden := range []string{"NOT a loop", "is a loop"} {
				if strings.Contains(msg, forbidden) {
					t.Errorf("report decides between the two readings (%q) and cannot: a legitimate "+
						"tikzDevice \\path and a loop both spend millions of steps reading no input:\n  %v", forbidden, err)
				}
			}
		})
	}
}

// ⛔ THE PROPERTY THAT MAKES RAISING THE ABSOLUTE CEILING SAFE, asserted at the
// values a renderer actually ships (StepLimitHeavy beside NoProgressLimitHeavy).
//
// A loop must still be caught by the NO-PROGRESS ceiling, not by the absolute
// one: that is what keeps the abort quick after the absolute ceiling was raised
// twenty-five-fold. Measured here: the loop stops at ~20M steps, the no-progress
// ceiling, nowhere near the absolute one. Without this, a later raise of
// NoProgressLimitHeavy above StepLimitHeavy would silently make every loop run
// to the bigger ceiling, and nothing but a stopwatch would notice.
func TestALoopIsStillCaughtByTheNoProgressCeilingAtShippedLimits(t *testing.T) {
	e := New()
	if err := e.LoadPlain(); err != nil {
		t.Fatalf("LoadPlain: %v", err)
	}
	e.tightLimit, e.stepLimit = NoProgressLimitHeavy, StepLimitHeavy
	_, err := e.Run(`\def\x{\x}\x`)
	if err == nil {
		t.Fatal("a self-referential macro was not stopped at all")
	}
	if e.tripCeiling != "no-progress" {
		t.Errorf("loop stopped by the %q ceiling; the no-progress one must catch it first, "+
			"or raising the absolute ceiling makes every loop take as long as the absolute ceiling allows",
			e.tripCeiling)
	}
	if d := e.Diagnostics(); d.ExpansionSteps > 2*NoProgressLimitHeavy {
		t.Errorf("loop ran %d steps before stopping; the no-progress ceiling is %d, so it should stop near it, "+
			"not near the absolute %d", d.ExpansionSteps, NoProgressLimitHeavy, StepLimitHeavy)
	}
}

// ⛔ A COUNTER NOBODY READS CANNOT WARN. The margin to each ceiling has to be
// reported on a compile that SUCCEEDS, because by the time one fires the
// document is already truncated. Asserted on a document that trips neither:
// steps must be counted, and the no-progress peak must stay far below them,
// which is what "the input kept advancing" means.
func TestTheStepCountersAreReportedOnASuccessfulCompile(t *testing.T) {
	e := New()
	if err := e.LoadPlain(); err != nil {
		t.Fatalf("LoadPlain: %v", err)
	}
	if _, err := e.Run(`\def\a#1{#1}` + strings.Repeat(`\a{x}`, 200)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	d := e.Diagnostics()
	if d.ExpansionSteps == 0 {
		t.Error("ExpansionSteps is 0 on a document that expanded 200 macros: the counter is not reported")
	}
	if d.NoProgressPeak >= d.ExpansionSteps {
		t.Errorf("no-progress peak %d is not below the total %d: a document whose input advances throughout "+
			"cannot have spent every step without advancing", d.NoProgressPeak, d.ExpansionSteps)
	}
}
