// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ⛔ A CLOCK RUNNING OUT USED TO READ AS "THE ENGINE LOST EVERYTHING".
//
// Both compile errors were discarded (`_ = pipe.compileRef(…)`), so a timeout
// and a compile that produced nothing were the same event: neither leaves a
// PDF. The two sides were then treated DIFFERENTLY — an empty reference is
// `ref-unavailable` and the paper leaves the measurement, an empty gotex is
// `gotex-failed` and the paper is SCORED AT ZERO. So the host's slowness, or a
// paper that is merely slow, did not add symmetric noise: it moved the mean in
// one direction only, against the engine under test. See #751.

// timedOutPipe builds a pipeline in which the named side's compile reports a
// timeout the way the real runCmd does — a wrapped errTimedOut — and the other
// side succeeds with real text.
func timedOutPipe(side string) pipeline {
	ok := func(text string) func(_, outPDF string) error {
		return func(_, outPDF string) error {
			return os.WriteFile(outPDF, []byte(text), 0o644)
		}
	}
	timeout := func(_, _ string) error {
		return fmt.Errorf("tectonic: %w after 1m30s", errTimedOut)
	}
	pipe := pipeline{
		compileRef: ok("alpha beta gamma delta"),
		compileGot: ok("alpha beta gamma delta"),
		extract: func(pdf string) extraction {
			b, err := os.ReadFile(pdf)
			if err != nil {
				return extraction{}
			}
			return extraction{text: string(b), nonTrivial: len(b) >= nonTrivialPDFBytes}
		},
	}
	switch side {
	case "reference":
		pipe.compileRef = timeout
	case "gotex":
		pipe.compileGot = timeout
	default:
		panic("unknown side " + side)
	}
	return pipe
}

// The judge and the subject must count alike: whichever engine ran out of time,
// the paper leaves the measurement with the same status.
func TestATimedOutCompileIsTheSameVerdictOnEitherSide(t *testing.T) {
	for _, side := range []string{"reference", "gotex"} {
		t.Run(side, func(t *testing.T) {
			dir := makePaper(t, "article", "body")
			res := analyzePaper(dir, "p", filepath.Join(t.TempDir(), "w"), timedOutPipe(side))
			if res.status != statusTimedOut {
				t.Fatalf("%s timed out: status=%s want %s", side, res.status, statusTimedOut)
			}
			if !strings.Contains(res.diag, side) {
				t.Errorf("diagnostic does not name the side that ran out of time: %q", res.diag)
			}
		})
	}
}

// The consequence that matters, stated as a number: a paper the clock cut short
// must not drag the mean down. One paper at full recall beside one timed-out
// paper is a 100 % mean over one scored paper — not 50 % over two.
// It goes through analyzePaper rather than hand-built rows, so the whole chain
// is under the assertion: a hand-built statusTimedOut row would still pass if
// analyzePaper had never produced one.
func TestATimedOutPaperDoesNotDragTheMeanDown(t *testing.T) {
	good := makePaper(t, "article", "body")
	slow := makePaper(t, "article", "body")
	results := []paperResult{
		analyzePaper(good, "good", filepath.Join(t.TempDir(), "w1"),
			fakePipe(str("alpha beta gamma delta"), str("alpha beta gamma delta"))),
		analyzePaper(slow, "slow", filepath.Join(t.TempDir(), "w2"), timedOutPipe("gotex")),
	}
	if results[0].status != statusOK {
		t.Fatalf("the control paper is not scorable (status=%s); this test cannot say anything", results[0].status)
	}
	var out bytes.Buffer
	report(results, &out)
	got := out.String()
	if !strings.Contains(got, "1 paper(s) scored") {
		t.Errorf("the timed-out paper was counted as scored:\n%s", got)
	}
	if !strings.Contains(got, "mean recall over scored papers: 100.0%") {
		t.Errorf("mean moved by a clock, not by the engine:\n%s", got)
	}
	if !strings.Contains(got, statusTimedOut) {
		t.Errorf("the report does not list the timed-out paper at all:\n%s", got)
	}
}

// Excluding it silently would be its own defect: the mean then rests on fewer
// papers than were sampled and says nothing about it. #72 established the shape
// for the sibling case (a broken extractor); this is the same banner.
func TestARunThatLostPapersToTheClockSaysSoLoudly(t *testing.T) {
	results := []paperResult{
		{id: "good", status: statusOK, ratio: 1},
		{id: "slow", status: statusTimedOut, diag: "gotex: compile timed out after 1m30s"},
	}
	var errBuf bytes.Buffer
	warnTimedOut(&errBuf, timeoutDiagnostics(results))
	got := errBuf.String()
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "slow") {
		t.Errorf("no unmissable warning naming the paper:\n%s", got)
	}
	if !strings.Contains(got, "EXCLUDED") {
		t.Errorf("the warning does not say the paper left the score:\n%s", got)
	}

	var quiet bytes.Buffer
	warnTimedOut(&quiet, timeoutDiagnostics([]paperResult{{id: "good", status: statusOK}}))
	if quiet.Len() != 0 {
		t.Errorf("a run that lost nothing to the clock must say nothing: %q", quiet.String())
	}
}

// ⛔ And the wiring underneath: CommandContext KILLS the child, so cmd.Run
// reports a signal and the error itself never says "the budget ran out". Only
// the context knows. This runs a genuinely slow child — not a pre-expired
// context, which would never reach the kill path.
func TestASlowChildIsReportedAsATimeoutNotAsAFailure(t *testing.T) {
	// ⛔ PROBE FIRST, AND THE PROBE IS THE CONTROL. The qemu architecture lanes
	// run this suite through `go test -exec=qemu-<arch>`, so the test binary is
	// FOREIGN to the host and re-executing it depends on binfmt_misc being
	// registered in the container. Where it is not, the child fails instantly
	// with an exec error and the assertion below would be red for a reason that
	// has nothing to do with timeouts. So ask whether a child can start at all,
	// with a helper that exits at once — and say out loud when it cannot, rather
	// than skipping on a condition nobody can see.
	t.Setenv("GOTEX_REFDIFF_HELPER", "exit")
	if err := runCmd(30*time.Second, os.Args[0], "-test.run", "^TestHelperProcessExitsAtOnce$"); err != nil {
		t.Skipf("this environment cannot re-execute the test binary (%v), so the kill path cannot be reached here; the lanes that can still assert it", err)
	}

	t.Setenv("GOTEX_REFDIFF_HELPER", "sleep")
	start := time.Now()
	err := runCmd(200*time.Millisecond, os.Args[0], "-test.run", "^TestHelperProcessSleeps$")
	if !errors.Is(err, errTimedOut) {
		t.Fatalf("a child killed by the budget reported %v, which errors.Is(errTimedOut) cannot see", err)
	}
	// The time is the witness: an error returned instantly would mean the child
	// never ran, and the assertion above would be passing on the wrong event.
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Errorf("returned after %s, so the child cannot have been running when the budget expired", elapsed)
	}
}

// A child that fails for its own reasons must NOT be dressed up as a timeout,
// or the new status would absorb the failures it was meant to separate out.
func TestAChildThatFailsOnItsOwnIsNotCalledATimeout(t *testing.T) {
	err := runCmd(30*time.Second, filepath.Join(t.TempDir(), "definitely-not-a-binary"))
	if err == nil {
		t.Fatal("running a nonexistent binary should fail")
	}
	if errors.Is(err, errTimedOut) {
		t.Errorf("a missing binary was reported as a timeout: %v", err)
	}
}

// ⛔ THE TWIN. -layout has its own copy of the control flow (analyzeLayout) and
// had the same two discarded compile errors. It is exercised — unlike some
// twins — so it is fixed and asserted rather than left with a note.
func TestTheLayoutModeAlsoLeavesATimedOutPaperOutOfTheScore(t *testing.T) {
	for _, side := range []string{"reference", "gotex"} {
		t.Run(side, func(t *testing.T) {
			// A usable reference on both sides, so the paper would be SCORED
			// if the clock were not asked: an empty one would stop at
			// ref-unavailable and the test would pass for the wrong reason.
			usable := layoutExtraction{layout: parseBBox(bboxDoc(twoWordPage))}
			pipe := layoutGuardPipe(usable, usable)
			timeout := func(_, _ string) error {
				return fmt.Errorf("tectonic: %w after 1m30s", errTimedOut)
			}
			if side == "reference" {
				pipe.compileRef = timeout
			} else {
				pipe.compileGot = timeout
			}
			res := analyzeLayout(layoutPaper(t, "article", "body"), "p",
				filepath.Join(t.TempDir(), "w"), pipe)
			if res.status != statusTimedOut {
				t.Fatalf("%s timed out: status=%s want %s", side, res.status, statusTimedOut)
			}
			if !strings.Contains(res.diag, side) {
				t.Errorf("diagnostic does not name the side that ran out of time: %q", res.diag)
			}
		})
	}
}

// TestHelperProcessSleeps is not a test. It is the child process
// TestASlowChildIsReportedAsATimeoutNotAsAFailure runs through runCmd, selected
// by an environment variable the child inherits, so the slow process is
// portable to every lane instead of depending on a `sleep` binary.
func TestHelperProcessSleeps(t *testing.T) {
	if os.Getenv("GOTEX_REFDIFF_HELPER") != "sleep" {
		t.Skip("not the helper child")
	}
	time.Sleep(30 * time.Second)
}

// TestHelperProcessExitsAtOnce is not a test either. It is the probe child:
// running it answers "can a child be started here at all", which separates an
// environment that cannot re-execute this binary from a timeout that did not
// fire.
func TestHelperProcessExitsAtOnce(t *testing.T) {
	if os.Getenv("GOTEX_REFDIFF_HELPER") != "exit" {
		t.Skip("not the probe child")
	}
}
