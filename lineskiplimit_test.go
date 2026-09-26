package engine

import "testing"

// tex.web §679 names TWO parameters and interlineGlue used one of them for both
// roles:
//
//	if d<line_skip_limit then p:=new_param_glue(line_skip_code)
//	else  begin p:=new_skip_param(baseline_skip_code); width(temp_ptr):=d;
//
// \lineskiplimit is the THRESHOLD (0pt in plain TeX and LaTeX) and \lineskip is
// the REPLACEMENT (1pt). Testing d < \lineskip put the crossover a point too high.
//
// The instrument is the glue ISOLATED — \ht of the vbox less the two boxes'
// heights. The total is not the instrument, and this is the trap the test exists
// to record: measured whole, the three vboxes read 19.16/19.12, 21.16/20.62 and
// 19.16/18.62, so the DIVERGENT case showed the smallest difference of the three
// (0.04pt) because \hbox{A} is 6.62pt here against the reference's 7.16 and the
// two errors nearly cancel. Only the derived glue says 0.50 against 1.00.
//
//	second box   d       tectonic   before    after
//	11.5pt       +0.5      0.50pt   1.00pt   0.50pt   <- the divergent band
//	13pt         −1.0      1.00pt   1.00pt   1.00pt   control: d < limit
//	5pt          +7.0      7.00pt   7.00pt   7.00pt   control: d ≥ limit
func TestInterlineGlueUsesLineskiplimitAsTheThreshold(t *testing.T) {
	const src = `\documentclass{article}\newbox\bh \newbox\bx\begin{document}` +
		`\setbox\bh=\hbox{A}H1=\the\ht\bh\par` +
		`\setbox\bx=\vbox{\hbox{A}\hbox{\vrule height 11.5pt depth 0pt width 0pt}}T05=\the\ht\bx\par` +
		`\setbox\bx=\vbox{\hbox{A}\hbox{\vrule height 13pt depth 0pt width 0pt}}TNEG=\the\ht\bx\par` +
		`\setbox\bx=\vbox{\hbox{A}\hbox{\vrule height 5pt depth 0pt width 0pt}}TBIG=\the\ht\bx\par` +
		`\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	h1 := got["H1"]
	if h1 <= 0 {
		t.Fatalf("H1 = %v: sans la hauteur de la première boîte la glue n'est pas isolable", h1)
	}
	for _, c := range []struct {
		key  string
		boxH float64
		want float64
		what string
	}{
		{"T05", 11.5, 0.5, `d=+0.5, la bande divergente: \baselineskip de largeur d`},
		{"TNEG", 13, 1.0, `d=-1.0, contrôle: d < limite, donc \lineskip`},
		{"TBIG", 5, 7.0, `d=+7.0, contrôle: d >= limite, donc \baselineskip`},
	} {
		v, ok := got[c.key]
		if !ok {
			t.Errorf("%s= n'a pas été imprimé", c.key)
			continue
		}
		glue := v - h1 - c.boxH
		if d := glue - c.want; d > 0.01 || d < -0.01 {
			t.Errorf("%s: glue = %.2fpt, la référence donne %.2fpt", c.what, glue, c.want)
		}
	}
}

// \offinterlineskip is what the two parameters are FOR, and it is plain TeX's idiom
// for butting boxes together: \baselineskip-1000pt \lineskip\z@ \lineskiplimit\maxdimen
// (classkernel.go). With either parameter dead the limit never fires or the
// replacement is the wrong one, and the boxes still stood 1pt apart.
//
// The assertion is the internal relation — three boxes with no glue between them
// stack to exactly three times one box's height — because our "A" is 6.62pt against
// the reference's 7.16pt. The reference satisfies the same relation: its
// \offinterlineskip vbox is 21.48001pt and 3 x 7.16 = 21.48.
//
//	                 tectonic          before    after
//	3 x \hbox{A}      21.48 = 3x7.16   21.86    19.86 = 3x6.62
//	normal leading    31.16            30.62    30.62   control, unchanged
func TestOffinterlineskipLeavesNoGlue(t *testing.T) {
	const src = `\documentclass{article}\newbox\bh \newbox\bx\begin{document}` +
		`\setbox\bh=\hbox{A}H1=\the\ht\bh\par` +
		`\setbox\bx=\vbox{\hbox{A}\hbox{A}\hbox{A}}NORM=\the\ht\bx\par` +
		`\setbox\bx=\vbox{\offinterlineskip\hbox{A}\hbox{A}\hbox{A}}OFF=\the\ht\bx\par` +
		`\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	h1, norm, off := got["H1"], got["NORM"], got["OFF"]
	if h1 <= 0 || norm <= 0 || off <= 0 {
		t.Fatalf("une des trois valeurs manque: %v", got)
	}
	if d := off - 3*h1; d > 0.02 || d < -0.02 {
		t.Errorf(`\offinterlineskip a laissé %.2fpt de glue: %.2f contre 3 x %.2f = %.2f`,
			off-3*h1, off, h1, 3*h1)
	}
	// The control: normal leading is untouched, so this is not "all glue removed".
	if d := norm - (h1 + 2*12); d > 0.02 || d < -0.02 {
		t.Errorf("l'interligne ORDINAIRE a bougé: %.2f, attendu %.2f (hauteur + 2 x 12pt)",
			norm, h1+2*12)
	}
}

// Both parameters have to be assignable at all, which is what the shadowing broke:
// each was declared \newskip / \newdimen in the substrate, so an assignment wrote
// to a register and the engine's field never moved. Asserted on the FIELDS, since
// a \the would have read the register back quite happily and said nothing.
func TestLineskipParametersReachTheEngine(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\lineskip=5pt \lineskiplimit=7pt\relax`); err != nil {
		t.Fatal(err)
	}
	if e.lineskip != 5*unity {
		t.Errorf(`\lineskip=5pt a laissé e.lineskip à %d sp, attendu %d`, e.lineskip, 5*unity)
	}
	if e.lineskiplimit != 7*unity {
		t.Errorf(`\lineskiplimit=7pt a laissé e.lineskiplimit à %d sp, attendu %d`,
			e.lineskiplimit, 7*unity)
	}
	// Scoped like the other engine dimens: a group restores them.
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\lineskip=3pt{\lineskip=9pt}\relax`); err != nil {
		t.Fatal(err)
	}
	if e2.lineskip != 3*unity {
		t.Errorf(`\lineskip n'est pas restauré à la fermeture du groupe: %d sp, attendu %d`,
			e2.lineskip, 3*unity)
	}
}
