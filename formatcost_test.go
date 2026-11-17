package engine

import "testing"

// ⛔⛔ THIS IS WHERE THE WINDOWS LANE'S THIRTY MINUTES GO, and until this file
// existed the repository had NO benchmark at all, so the cost was argued about
// instead of being read.
//
// The lane runs `go test -timeout 30m -race ./...` and the root package came back
// `FAIL github.com/go-tex/engine 1800.560s` on go-tex/engine#805, having printed
// its coverage first — the suite nearly finished and then the alarm fired. The
// same commit passed on the `push` event, so the lane sits exactly on the bound.
//
// ⚠ THE TEST THE PANIC NAMES IS INNOCENT, and a reader who starts there audits
// the wrong thing. The trace said:
//
//	running tests:
//		TestTransformErrorBranches (4s)
//
// four seconds into its own run. Go names whatever happened to be executing when
// the alarm went off; what it was DOING is the part that matters, and the
// goroutine dump says so:
//
//	engine.(*Engine).define            engine.go:1202
//	engine.(*Engine).assignLetMeaning  primitives.go:333
//	engine.(*Engine).doLet             primitives.go:319
//	engine.(*Engine).loadPrimitives.func5
//	engine.(*Engine).execCS            engine.go:2353
//	engine.(*Engine).mainLoop          engine.go:2113
//	engine.(*Engine).LoadFormat        format.go:141
//	engine.(*Engine).LoadLaTeX         latex.go:1942
//	engine.TestTransformErrorBranches  transform_test.go:283
//
// It was re-expanding the whole LaTeX format. Nearly every test in this package
// opens with `New()` then `LoadLaTeX()`, so the format is built from source once
// per test — which is the exact work TeX's `.fmt` files exist to avoid, paid
// several hundred times a run.
//
// ⛔ A THIRD TIMEOUT RAISE IS NOT THE FIX. It has been raised twice already
// (#156 gave the lane a bound windows-latest could meet, #653 raised the emulated
// lanes), and the file's own comment admits what that is: "the lane is simply that
// slow, so the bound is raised rather than the suite trimmed". Every test added
// since has made the next raise nearer. The per-test cost is the defect.
//
// This benchmark is the number that argument needs. Run it on each lane and the
// comparison is a measurement rather than a guess:
//
//	go test -run '^$' -bench 'BenchmarkLoadLaTeX' -benchtime 10x .
//
// ⚠ -benchtime in ITERATIONS, not seconds: at this cost per operation a
// duration-based default would spend minutes deciding how many to run.
//
// ⛔⛔ THE FORMAT IS ABOUT 30% OF THE SUITE — NOT THE WHOLE THIRTY MINUTES, AND
// NOT THE EIGHTH THIS COMMENT CLAIMED WHEN IT WAS MERGED.
//
// Measured on this file's own benchmarks, darwin/arm64, Apple M4 Max,
// GOMAXPROCS=1 and -parallel 1 so that the suite and the benchmark count ALIKE
// (a serial sum of per-test costs compared against a 16-way parallel wall clock
// is not a share, it is two different quantities):
//
//	suite, 2012 tests, serial        17.76 s
//	BenchmarkLoadLaTeX                2.621 ms/op
//	BenchmarkNewWithoutTheFormat      0.0758 ms/op
//	format expansion                  2.545 ms/op
//
// 2012 × 2.621 ms = 5.27 s, which is 29.7% of the suite, and an UPPER BOUND since
// not every test calls LoadLaTeX.
//
// ⛔⛔ THE FIRST VERSION OF THIS NOTE SAID 12.7%, AND BOTH ITS NUMBERS WERE TAKEN
// ON A LOADED MACHINE. It even said so — "the 58 s wants retaking on a quiet
// machine, the 1-minute load average was 4.95 before that run and 28.02 after" —
// and then leaned on the figure anyway. Retaken with the machine quiet (1-minute
// load 4.24 at the start, 4.82 at the end) the suite came back 17.76 s against
// 58.04 s and the benchmark 2.621 ms against 3.658 ms.
//
// ⚠ THE TWO ERRORS DID NOT CANCEL: the wall clock stretched 3.3× and the tight
// in-process loop only 1.4×, so the ratio moved by more than either. A caveat
// written and not acted on is a figure published wrong — the load rule exists
// for the whole measurement, not for the half of it that looks fragile.
//
// AND IT CHANGED THE CONCLUSION, which is why the correction mattered. At 12.7% a
// format built once per package was "worth having, not the fix". At 30% it is a
// candidate fix for the windows lane, because that lane is not far over the line:
// it fails at 1800.5 s against a 1800 s bound (#799). It does not need to be twice
// as fast, it needs to fit.
//
// ⚠ Still not claimed: that 30% of the local suite is 30% of that lane. The
// runner may stretch the format expansion and the rest of a test by different
// factors — the same mistake one layer up. The remaining ~70% is unlocated and
// needs per-test timing ON the lane (go-fleettools/fleettools#328 is the
// neighbouring gate, for jobs with no -timeout at all).
func BenchmarkLoadLaTeX(b *testing.B) {
	for b.Loop() {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			b.Fatal(err)
		}
	}
}

// And the half of the cost that is NOT the format, so the benchmark above can be
// read as a share rather than an absolute. `New()` alone is what a test would pay
// if the format were loaded once for the package instead of once per test.
//
// ⛔ The pair is the measurement; neither number alone says anything. If New() is
// a microsecond and LoadLaTeX is tens of milliseconds, a format built once per
// package removes essentially the whole suite's runtime — and if they are close,
// the format is not where the time goes and this whole reading is wrong.
func BenchmarkNewWithoutTheFormat(b *testing.B) {
	for b.Loop() {
		_ = New()
	}
}
