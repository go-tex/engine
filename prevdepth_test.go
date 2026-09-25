package engine

import "testing"

// \prevdepth is the depth of the last box on the current vertical list, and the
// value appendToPage measures the next interline glue against (paragraph.go:171).
// It was a \newdimen register in the substrate with no connection to e.prevDepth,
// so it read 0pt always and an assignment to it did nothing.
//
// READING. The instrument is the INTERNAL relation \the\prevdepth = \the\dp of the
// box, not the reference's number: our "p" is 2.2pt deep against the reference's
// 1.93999pt, a font-metrics difference. The reference's role is to establish that
// the relation holds there — it reads 1.93999 for both — and ours is asserted
// against our own box.
//
//	                              tectonic   before    after
//	\dp of \hbox{ppp}              1.93999    2.2pt    2.2pt
//	\the\prevdepth after it        1.93999    0.0pt    2.2pt
//
// A descender is not optional here. With \hbox{AAA} both engines answer 0.0pt and
// reading looks correct, because 0 IS the right answer for a box with no descender:
// the first version of this check passed for that reason and said nothing.
func TestPrevdepthReadsTheLastBoxDepth(t *testing.T) {
	const src = `\documentclass{article}\newbox\bp \newbox\bd\begin{document}` +
		`\setbox\bd=\hbox{ppp}D=\the\dp\bd\par` +
		`\setbox\bp=\vbox{\hbox{ppp}\xdef\pdone{\the\prevdepth}\hbox{qqq}\xdef\pdtwo{\the\prevdepth}}` +
		`\noindent UN=\pdone\par\noindent DEUX=\pdtwo\par\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	d, okd := got["D"]
	if !okd || d <= 0 {
		t.Fatalf(`\dp\hbox{ppp} = %v: sans jambage ce test ne mesure rien`, got["D"])
	}
	for _, k := range []string{"UN", "DEUX"} {
		v, ok := got[k]
		if !ok {
			t.Errorf(`%s= n'a pas été imprimé: \the\prevdepth n'a rien produit`, k)
			continue
		}
		if x := v - d; x > 0.01 || x < -0.01 {
			t.Errorf(`%s: \the\prevdepth = %.2fpt, la profondeur de la boîte est %.2fpt`, k, v, d)
		}
	}
}

// WRITING, which is what \nointerlineskip is: plain TeX defines it as
// \prevdepth-1000pt, and ignoreDepth (paragraph.go:16) is exactly that sentinel.
//
//	\ht of a \vbox                            tectonic   before    after
//	\hbox{AAA}\hbox{BBB}            (control)    19.16   18.62    18.62
//	\hbox{AAA}\prevdepth=-1000pt \hbox{BBB}      13.99   18.62    13.19
//	\hbox{AAA}\nointerlineskip\hbox{BBB}         13.99   18.62    13.19
//
// The control must not move and does not; the absolute 18.62-vs-19.16 gap is this
// engine's metrics. What is asserted is that the two suppressing forms drop below
// the control by the interline glue, and that they agree with each other.
func TestNointerlineskipSuppressesTheInterlineGlue(t *testing.T) {
	const src = `\documentclass{article}\newbox\bx\begin{document}` +
		`\setbox\bx=\vbox{\hbox{AAA}\hbox{BBB}}CTL=\the\ht\bx\par` +
		`\setbox\bx=\vbox{\hbox{AAA}\prevdepth=-1000pt \hbox{BBB}}RAW=\the\ht\bx\par` +
		`\setbox\bx=\vbox{\hbox{AAA}\nointerlineskip\hbox{BBB}}NIL=\the\ht\bx\par\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	ctl, raw, nil_ := got["CTL"], got["RAW"], got["NIL"]
	if ctl <= 0 || raw <= 0 || nil_ <= 0 {
		t.Fatalf("une des trois hauteurs manque: %v", got)
	}
	if raw >= ctl {
		t.Errorf(`\prevdepth=-1000pt n'a pas supprimé la glue: %.2fpt contre %.2fpt sans`, raw, ctl)
	}
	if d := nil_ - raw; d > 0.01 || d < -0.01 {
		t.Errorf(`\nointerlineskip (%.2fpt) et \prevdepth=-1000pt (%.2fpt) doivent coïncider: `+
			`la macro EST cette assignation`, nil_, raw)
	}
}

// \removelastskip is \ifdim\lastskip=\z@\else\vskip-\lastskip\fi (latex.ltx:604).
//
// ⛔ The obvious witness cannot fail. A space does not terminate a glue
// specification — TeX is still looking for plus/minus and expands the macro during
// that lookahead, so \ifdim\lastskip runs BEFORE the 20pt reaches the list and
// finds 0pt. Measured:
//
//	\ht of a \vbox                                  tectonic   before    after
//	\hbox{A}\vskip 20pt \hbox{B}           (control)    39.16   38.62    38.62
//	\hbox{A}\vskip 20pt \removelastskip\hbox{B}        39.16   38.62    38.62
//	\hbox{A}\vskip 20pt\relax\removelastskip\hbox{B}   19.16   38.62    18.62
//
// The middle row is identical in both engines with and without a fix, so a test
// built on it goes green having exercised nothing. The third row is the one that
// discriminates, and the middle row is kept as the control that says the fix did
// not start removing skips it should leave alone.
func TestRemovelastskipNeedsItsGlueTerminated(t *testing.T) {
	const src = `\documentclass{article}\newbox\bx\begin{document}` +
		`\setbox\bx=\vbox{\hbox{A}\vskip 20pt \hbox{B}}CTL=\the\ht\bx\par` +
		`\setbox\bx=\vbox{\hbox{A}\vskip 20pt \removelastskip\hbox{B}}BARE=\the\ht\bx\par` +
		`\setbox\bx=\vbox{\hbox{A}\vskip 20pt\relax\removelastskip\hbox{B}}REL=\the\ht\bx\par\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	ctl, bare, rel := got["CTL"], got["BARE"], got["REL"]
	if ctl <= 0 || bare <= 0 || rel <= 0 {
		t.Fatalf("une des trois hauteurs manque: %v", got)
	}
	if d := bare - ctl; d > 0.01 || d < -0.01 {
		t.Errorf(`\vskip 20pt \removelastskip a retiré quelque chose (%.2f contre %.2f): `+
			`la référence ne le fait pas non plus, la macro s'expanse pendant la relecture de la glue`, bare, ctl)
	}
	if d := ctl - rel; d < 19.9 || d > 20.1 {
		t.Errorf(`\vskip 20pt\relax\removelastskip a retiré %.2fpt, attendu les 20pt du \vskip`, d)
	}
}
