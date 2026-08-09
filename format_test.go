package engine

import "testing"

// TestRealLoopMacro runs Knuth's plain.tex \loop…\repeat — a real, non-trivial
// TeX macro — on the engine (defined in TeX, executed by the gullet), driving a
// counter loop that accumulates a string. Proof of the "run real macros, don't
// reimplement" path to parity.
const plainLoop = `
\def\loop#1\repeat{\def\body{#1}\iterate}
\def\iterate{\body \let\next\iterate \else \let\next\relax \fi \next}
\let\repeat\relax
`

func TestRealLoopMacro(t *testing.T) {
	src := plainLoop + `
\count0=0 \edef\out{}
\loop \advance\count0 by 1 \edef\out{\out\the\count0}\ifnum\count0<5 \repeat
\message{[\out]}`
	out, err := New().Run(src)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "[12345]" {
		t.Errorf("loop output=%q want [12345]", out)
	}
}
