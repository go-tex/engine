package engine

import "testing"

// The end-of-isolated-run marker is put back into the input by any scanner that
// looks one token past what it consumed — tex.web §442, and scanInt in practice:
// 44 times over 8 of the 200 corpus papers, all of them through \number inside an
// \edef. It cannot be prevented from landing in the stream, so it must be harmless
// when it does: \relax, which terminates a number, typesets nothing and closes
// nothing. Undefined — as it was — it is a skipped command in lenient mode and an
// error in strict mode.
func TestSentinelMeansRelax(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}A\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	m := e.eq[sentinel.cs]
	if m == nil {
		t.Fatalf("%q has no meaning: an escaped marker is an undefined command", sentinel.cs)
	}
	if m.kind != mPrim || m.name != "relax" {
		t.Errorf("%q means %v/%q, want prim/relax", sentinel.cs, m.kind, m.name)
	}
}

// And it must stay harmless where it actually lands: on the page, mid-paragraph.
// Running the marker must add no glyph and report no skipped command.
func TestSentinelOnThePageIsInvisible(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}AB\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	want := pageChars(e)

	e2, err := compile([]byte(`\documentclass{article}\begin{document}A\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	// Step the marker itself, exactly as the main loop would on an escaped one,
	// then a B so the paragraph is the same as above.
	e2.push([]tok{sentinel, chTok('B', catLetter)})
	for {
		tk, ok := e2.getXToken()
		if !ok {
			break
		}
		if !e2.stepToken(tk) {
			break
		}
	}
	if n := e2.skippedCS[sentinel.cs]; n != 0 {
		t.Errorf("the marker was reported as a skipped command %d time(s)", n)
	}
	if got := pageChars(e2); got != want {
		t.Errorf("page = %q, want %q — the marker is not invisible", got, want)
	}
}
