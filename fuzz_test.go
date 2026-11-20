// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// fuzzOptions are the options every fuzzed input is compiled under.
//
// ⛔ THE ENGINE IS TURING-COMPLETE, so a fuzz target with the production limits
// would spend most of its budget inside `\def\a{\a}\a` and its cousins. The two
// expansion guards are lowered far enough that a runaway trips in milliseconds —
// and NOT so far that they trip during the format load, which is the trap this
// file's positive control exists to catch (see fuzzReachesTheDocument).
//
// Lenient, because a strict compile stops at the first undefined control
// sequence, and a fuzzer that stops at the first unknown name explores almost
// nothing past it.
func fuzzOptions() Options {
	return Options{
		Lenient:         true,
		StepLimit:       fuzzStepLimit,
		NoProgressLimit: fuzzNoProgressLimit,
	}
}

const (
	// fuzzMaxInput bounds one input. Go's fuzzer grows inputs without limit; a
	// TeX source past this size is not reaching a new code path, it is only
	// making each execution slower.
	fuzzMaxInput = 2048

	// fuzzStepLimit and fuzzNoProgressLimit are the expansion guards under fuzz.
	//
	// Set from a measurement, after the first version was set from a guess
	// (2_000_000 / 400_000, "above what the format load costs", that cost never
	// measured). Measured with GOTEX_TRACE on `cmd/gotex`, per pass:
	//
	//	article, one page                              3 597 steps, no-progress peak 942
	//	two-pass, 2 \section + \ref + 2 lists + math   4 499 steps, no-progress peak 942
	//
	// The 942 is the same in both and it is INSIDE the format: a mutation setting
	// the no-progress limit to 500 fails the format load itself ("loading LaTeX:
	// … ceiling reached: no-progress"). So 50_000 is about 11× the heaviest
	// measured pass and 20_000 about 21× the no-progress peak.
	//
	// What it buys, measured on three runaways: \def\a{\a}\a goes from 12.1 ms
	// (400 002 steps) to 2.4 ms (20 002), \def\a{\a x}\a from 11.3 ms to 2.7 ms —
	// about five times faster. Modest, and honestly so.
	//
	// ⛔⛔ AND NOT WHAT IT WAS FIRST CLAIMED TO BUY. A 60-second run at the old
	// limits showed `execs` frozen for 18–24 s at a time, and this comment first
	// said each generated loop "ran all the way to two million steps". That was
	// wrong by three orders of magnitude: a runaway under the OLD limits costs
	// 12 ms, so it could never freeze a worker for 18 s. Every freeze ended with
	// `new interesting` going up by exactly one — the fuzzer MINIMISING a new
	// input, up to -fuzzminimizetime (60 s by default), with those executions not
	// counted. Confirmed against a control: with -fuzzminimizetime 2s the freezes
	// vanish and one minute finds 84 new inputs against 14–18. See FuzzCompile's
	// invocation, which is where the real throughput lives.
	//
	// TestFuzzOptionsReachTheDocument is what says these are not too low.
	fuzzStepLimit       = 50_000
	fuzzNoProgressLimit = 20_000
)

// FuzzCompile feeds arbitrary bytes through the public pipeline: source →
// engine → page assembly. The property is that it never PANICS. An error is a
// legitimate answer to malformed input, and a tripped expansion guard is a
// refusal — both are the engine behaving; a panic is the engine falling over.
//
// go-tex/engine#794 had this repository at zero fuzz targets while its tracker
// held, found the expensive way, exactly the defects a fuzzer finds first. The
// seeds below are those defects, so the corpus replays them on every `go test`:
//
//   - \the\count300 and \the\count99999 panicked with "index out of range
//     [300] with length 256" until #844 — a one-line legal eTeX document taking
//     the process down;
//   - \advance\dimen300 read the register before the write guard could refuse it;
//   - a malformed `true` unit drove a 266-page runaway (#81);
//   - an unbalanced brace in a name scanner truncated the document (#77);
//   - \def\a{\a}\a is the runaway every Turing-complete fuzz target must survive.
//
// ⚠ Active fuzzing is NOT run in CI. The seeds run there on every `go test`
// (Go runs a Fuzz function's corpus as ordinary sub-tests), which is the
// regression half and costs nothing. `go test -fuzz` itself is opt-in, because
// the ubuntu lane is already past its original 30-minute bound with the cause
// unattributed (#823) and an unbounded fuzz step would make that worse. Run it as:
//
//	GOMEMLIMIT=2GiB go test -run '^$' -fuzz '^FuzzCompile$' -fuzztime 5m \
//	    -fuzzminimizetime 2s -parallel 2 .
//
// ⭐ -fuzzminimizetime 2s is the flag that matters, measured: at the default
// 60 s, each new input froze a worker for 18–24 s while it was minimised and a
// minute found 14–18 inputs; at 2 s the freezes vanish and a minute finds 84.
// Five times the coverage per minute, from one flag. -parallel 2 because this
// machine is shared; GOMEMLIMIT is a soft GC target and not a hard cap, and
// macOS does not enforce `ulimit -v`, so on darwin the bound is the expansion
// guards and fuzzMaxInput rather than the kernel.
func FuzzCompile(f *testing.F) {
	for _, seed := range []string{
		// Register defects fixed by #844.
		`\the\count300`,
		`\the\count99999`,
		`\count300=5 \the\count300`,
		`\advance\dimen300 by 1pt \the\dimen300`,
		`\the\skip300 \multiply\skip300 by 3`,
		`\count-3=5 \the\count-3`,
		`\setbox300=\hbox{A}\copy300`,
		`\newcount\x \count\x=5 \the\x`,
		// The historical defect classes #794 lists.
		`\dimen0=3 true pt \the\dimen0`,
		`\dimen0=3truept`,
		`\def\tag#1{#1}\tag{\ref{}}`,
		`{{{{`,
		`}}}}`,
		`\def\a{\a}\a`,
		`\def\a{\a x}\a`,
		// Ordinary documents, so the corpus also exercises the paths that work.
		`Hello \TeX\ world.\par`,
		"\\documentclass{article}\\begin{document}Hello.\\end{document}",
		"\\documentclass{article}\\begin{document}\\section{A}B\\par\\end{document}",
		"\\documentclass{article}\\begin{document}$x^2+y_1$\\end{document}",
		"\\documentclass{article}\\begin{document}\\begin{itemize}\\item a\\end{itemize}\\end{document}",
		"\\documentclass{article}\\begin{document}\\label{x}\\ref{x}\\end{document}",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		if len(src) > fuzzMaxInput {
			t.Skip("past the input bound")
		}
		e, err := compile(src, fuzzOptions())
		if err != nil {
			return // an error is an answer, not a crash
		}
		_ = e.Pages() // page assembly is where a recovered list meets the page builder
		_ = e.Diagnostics()
	})
}

// ⛔⛔ THE POSITIVE CONTROL: under the fuzz options, a real document must reach
// its own body and produce a page.
//
// A fuzz target that never reaches the document fuzzes nothing, and looks
// exactly like one that does: every input "passes". If fuzzStepLimit sat below
// what the LaTeX format load costs, every input carrying \documentclass would
// trip the guard inside the format, return an error, and the fuzzer would spend
// its whole budget exercising the format loader on identical work. No panic,
// no finding, no signal — the zero an instrument that measures nothing returns.
//
// So this asserts the body text is on the page, not merely that no error came
// back: a guard tripped late in the document can still return some pages.
func TestFuzzOptionsReachTheDocument(t *testing.T) {
	for _, c := range []struct {
		name, src, want string
	}{
		{"plain", `Reached the body.\par`, "Reached"},
		{"latex", "\\documentclass{article}\\begin{document}Reached the body.\\end{document}", "Reached"},
		{"latex two-pass", "\\documentclass{article}\\begin{document}\\section{S}\\label{x}See \\ref{x}. Reached.\\end{document}", "Reached"},
		// The document fuzzStepLimit was MEASURED against (4 499 steps per pass):
		// the bound is asserted against the heaviest case it was chosen from, not
		// against a lighter one that would pass a tighter, wrong limit.
		{"latex, the measured heaviest", "\\documentclass{article}\\begin{document}" +
			"\\section{Alpha}\\label{a}See Section~\\ref{a} and Section~\\ref{b}." +
			"\\begin{itemize}\\item one\\item two\\end{itemize}" +
			"\\begin{enumerate}\\item three\\end{enumerate}" +
			"\\section{Beta}\\label{b}$x^2 + y_1 = \\frac{a}{b}$ Reached.\\end{document}", "Reached"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(c.src), fuzzOptions())
			// ⛔ The two guards fail through DIFFERENT branches here, and each branch
			// is load-bearing for one of them. Measured with `mutate`: the
			// no-progress limit below its 942 peak makes the FORMAT LOAD return an
			// error ("loading LaTeX: … ceiling reached: no-progress"), while the
			// step limit below the document's cost returns NO error and sets
			// Diagnostics.Runaway instead. Drop either check and one guard's
			// mutation survives.
			//
			// And this message does not presume which guard tripped: an earlier
			// version said "the step limit is below what this document costs" on
			// a failure the engine itself labelled "ceiling reached: no-progress".
			// The wrapped error names the ceiling; this line must not contradict it.
			if err != nil {
				t.Fatalf("under the fuzz options, %s did not compile: %v — an expansion guard tripped before the document body, so the fuzzer would never reach one", c.name, err)
			}
			if d := e.Diagnostics(); d.Runaway {
				t.Fatalf("under the fuzz options, %s tripped the runaway guard — fuzzStepLimit %d / fuzzNoProgressLimit %d are too low for an ordinary document", c.name, fuzzStepLimit, fuzzNoProgressLimit)
			}
			if len(e.Pages()) == 0 {
				t.Fatalf("under the fuzz options, %s produced no page", c.name)
			}
			if got := pageText(e); !strings.Contains(got, c.want) {
				t.Errorf("under the fuzz options, %s's body never reached the page: %q", c.name, got)
			}
		})
	}
}

// And the guard must still BITE under the fuzz options, or the fuzzer hangs on
// the first runaway it generates. The control above shows the limits are not too
// low; this shows they are not too high.
func TestFuzzOptionsStopARunaway(t *testing.T) {
	for _, src := range []string{`\def\a{\a}\a`, `\def\a{\a x}\a`} {
		e, err := compile([]byte(src), fuzzOptions())
		if err == nil && !e.Diagnostics().Runaway {
			t.Errorf("%s: under the fuzz options the runaway guard did not trip, so a generated loop would run until the fuzzer's own timeout", src)
		}
	}
}
