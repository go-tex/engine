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
// ⛔⛔ AND THE ANSWER IS NO: THE FORMAT IS NOT WHERE THE THIRTY MINUTES GO.
// That was the standing belief — a recorded one, "a third raise is not the fix;
// the per-test cost is", pointing at this very call — and measuring it refutes it.
//
// Measured on this file's own benchmarks, darwin/arm64, Apple M4 Max,
// GOMAXPROCS=1 and -parallel 1 so that the suite and the benchmark count ALIKE
// (a serial sum of per-test costs compared against a 16-way parallel wall clock
// is not a share, it is two different quantities):
//
//	suite, 2012 tests, serial        58.04 s
//	BenchmarkLoadLaTeX                3.658 ms/op
//	BenchmarkNewWithoutTheFormat      0.105 ms/op
//	format expansion                  3.553 ms/op
//
// 2012 × 3.658 ms = 7.36 s, which is 12.7% of the suite — and that is an UPPER
// BOUND, since not every test calls LoadLaTeX. So a format built once per package
// would buy about an eighth of the runtime. Worth having; nowhere near the 30
// minutes, and 87% of the cost is somewhere this benchmark does not look.
//
// ⚠ ONE CAVEAT ON THE 58 s, STATED BECAUSE IT CHANGES NOTHING AND MIGHT:
// the machine's 1-minute load average was 4.95 before that run and 28.02 after,
// so the figure may be inflated by whatever rose during it. The share argument
// survives it — the suite would have to be about 8 s serial for the format to
// dominate, and it is 20 s even with sixteen-way parallelism — but the 58 s
// itself should be retaken on a quiet machine before it is quoted as a baseline.
//
// Where the other 87% goes is open, and it needs a per-test instrument rather
// than a second guess (go-fleettools/fleettools#328 is the neighbouring gate, for
// jobs with no -timeout at all).
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
