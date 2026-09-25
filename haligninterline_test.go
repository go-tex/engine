package engine

import "testing"

// \halign stacked its rows with no interline glue at all, so a multi-row
// alignment was about half the height it should be. tex.web §679 append_to_vlist
// is the rule, and it was implemented in three places — appendToPage, tabular's
// cell stacker, listings' line stacker — and absent from the fourth.
//
//	\ht of \vbox{\halign{#\cr …\cr}}   tectonic   before    after
//	3 rows                              31.16pt   19.88pt   30.62pt
//	1 row                                7.16pt    6.62pt    6.62pt
//	INCREMENT PER ROW                   12.00pt    6.63pt   12.00pt
//
// The increment is the instrument, not the total: it cancels the one-row metrics
// difference (6.62 against 7.16 is this engine's face) and it identifies the
// missing quantity as \baselineskip exactly rather than "some vertical space".
// Before, the increment was the height of a line of text — the rows were butted
// together.
//
// Asserted against e.baselineskip rather than the literal 12pt, so the test says
// what the rule IS and keeps holding if a class states another leading.
func TestHalignRowsTakeInterlineGlue(t *testing.T) {
	const src = `\documentclass{article}\newbox\bh\begin{document}` +
		`\setbox\bh=\vbox{\halign{#\cr AAA\cr BBB\cr CCC\cr}}H=\the\ht\bh\par` +
		// The trailing space matters: these fragments are concatenated, and
		// `…\par` + `B=…` makes the control word \parB, which lenient mode skips —
		// so "B=" never reached the page and the value read as missing.
		`\setbox\bh=\vbox{\halign{#\cr AAA\cr}}U=\the\ht\bh\par ` +
		`B=\the\baselineskip\par\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	h, u, bl := got["H"], got["U"], got["B"]
	if h <= 0 || u <= 0 || bl <= 0 {
		t.Fatalf("une des trois valeurs manque: %v", got)
	}
	inc := (h - u) / 2
	if d := inc - bl; d > 0.01 || d < -0.01 {
		t.Errorf("incrément par rangée de \\halign = %.2fpt, attendu \\baselineskip = %.2fpt "+
			"(hauteurs %.2f pour 3 rangées, %.2f pour 1)", inc, bl, h, u)
	}
}

// The extraction put four call sites on one function, so the three that already
// worked must not move. Their own tests cover them (TestTabularRowsCarryTheArrayStrut,
// TestArrayStretchScalesTheRowStrut, the listings and paragraph suites); this pins
// the rule's two branches directly, which none of them does.
//
// §679: d = \baselineskip - prev_depth - height, and the \lineskip branch when
// that is too small. ⚠ TeX tests d < \lineskiplimit and this tests d < \lineskip —
// a divergence kept deliberately and recorded, see interlineGlue's comment.
func TestInterlineGlueHasBothBranches(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})

	if _, ok := e.interlineGlue(ignoreDepth, 0); ok {
		t.Error("prev_depth = ignore_depth doit donner AUCUNE glue (§679 le teste en premier)")
	}
	// Ordinary case: the gap makes the baselines \baselineskip apart.
	g, ok := e.interlineGlue(0, 5*unity)
	if !ok {
		t.Fatal("une profondeur ordinaire doit donner de la glue")
	}
	if want := e.baselineskip - 5*unity; g.spec.width != want {
		t.Errorf("glue = %d, attendu \\baselineskip - hauteur = %d", g.spec.width, want)
	}
	// Crowded case: a box taller than \baselineskip falls back to \lineskip.
	g, ok = e.interlineGlue(0, e.baselineskip+10*unity)
	if !ok {
		t.Fatal("le cas serré doit tout de même donner de la glue")
	}
	if g.spec.width != e.lineskip {
		t.Errorf("cas serré: glue = %d, attendu \\lineskip = %d", g.spec.width, e.lineskip)
	}
}
