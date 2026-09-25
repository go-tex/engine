package engine

import (
	"strings"
	"testing"
)

// \strutbox was allocated and never set, and \strut was \def'd EMPTY, so a strut —
// the idiom for forcing a line to its full height — set nothing. latex.ltx:613-614
// and the \setbox inside \set@fontsize's size@update (l.8544) are now followed
// verbatim.
//
// Tectonic against this engine:
//
//	                                      tectonic   before     after
//	SH  \ht\strutbox                      8.39996     0.0pt   8.39996
//	SD  \dp\strutbox                      3.60004     0.0pt   3.60004
//	BH  \ht of \hbox{\strut}              8.39996     0.0pt   8.39996
//	BD  \dp of \hbox{\strut}              3.60004     0.0pt   3.60004
//	VH  \ht of \vbox{\hbox{\strut x}}     8.39996    4.73pt   8.39996
//	WH  \ht of \vbox{\hbox{x}}               4.31    4.73pt    4.73pt
//
// WH is the CONTROL: no strut, so it must not move — and it does not. Its standing
// 4.73-vs-4.31 gap is the bare height of an "x" in this engine's face against the
// reference's, a font-metrics difference this change does not touch and does not
// claim. VH beside it is what the strut is for: it takes the line from the letter's
// own height to the full strut height, which the reference nearly doubles.
func TestStrutSetsTheLineHeight(t *testing.T) {
	const src = `\documentclass{article}` +
		`\newbox\bs \newbox\bv \newbox\bw\begin{document}` +
		`SH=\the\ht\strutbox\par SD=\the\dp\strutbox\par` +
		`\setbox\bs=\hbox{\strut}BH=\the\ht\bs\par BD=\the\dp\bs\par` +
		`\setbox\bv=\vbox{\hbox{\strut x}}VH=\the\ht\bv\par` +
		`\setbox\bw=\vbox{\hbox{x}}WH=\the\ht\bw\par\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	for _, c := range []struct {
		keys []string
		want float64
		what string
	}{
		{[]string{"SH", "BH", "VH"}, 8.4, `hauteur de strut (.7 x 12pt)`},
		{[]string{"SD", "BD"}, 3.6, `profondeur de strut (.3 x 12pt)`},
	} {
		for _, k := range c.keys {
			v, ok := got[k]
			if !ok {
				t.Errorf("%s= n'a pas été imprimé (%s)", k, c.what)
				continue
			}
			if d := v - c.want; d > 0.01 || d < -0.01 {
				t.Errorf("%s (%s) = %.2fpt, la référence donne %.1fpt", k, c.what, v, c.want)
			}
		}
	}
	// The control. It carries a font-metrics gap of its own, so it is pinned to what
	// THIS engine produces without a strut: the assertion is that a strut changed
	// nothing here, not that the number matches the reference.
	if v, ok := got["WH"]; !ok {
		t.Error(`WH= n'a pas été imprimé`)
	} else if d := v - 4.73; d > 0.01 || d < -0.01 {
		t.Errorf("le témoin SANS strut a bougé: %.2fpt au lieu de 4.73pt", v)
	}
}

// \strutbox must be rebuilt wherever the leading moves, which is two paths in this
// engine and one in the reference. \@setfontsize covers \tiny…\Huge; \linespread
// and setspace's commands are bound to setLineStretch in Go and do NOT route
// through \@setfontsize, so they need their own call (refreshStrutBox) — the
// reference gets both for free because \linespread goes through \set@fontsize.
//
//	\ht\strutbox                     tectonic   after   = .7 x
//	\normalsize                       8.39996  8.39996    12
//	\footnotesize                     6.64996  6.64996     9.5
//	\Large                           12.59995 12.59995    18
//	\linespread{1.5}\selectfont      12.59995 12.59995    1.5 x 12
//
// The last row is the one the \@setfontsize call alone does not reach: it read
// 8.39996pt with the substrate change in and the setLineStretch hook out.
func TestStrutBoxFollowsEveryLeadingChange(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`NH=\the\ht\strutbox\par` +
		`{\footnotesize FH=\the\ht\strutbox\par}` +
		`{\Large GH=\the\ht\strutbox\par}` +
		`\linespread{1.5}\selectfont SH=\the\ht\strutbox\par\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	for _, c := range []struct {
		key  string
		want float64
		what string
	}{
		{"NH", 8.4, `\normalsize`},
		{"FH", 6.65, `\footnotesize`},
		{"GH", 12.6, `\Large`},
		{"SH", 12.6, `\linespread{1.5} (passe par setLineStretch, pas \@setfontsize)`},
	} {
		v, ok := got[c.key]
		if !ok {
			t.Errorf("%s= n'a pas été imprimé (%s)", c.key, c.what)
			continue
		}
		if d := v - c.want; d > 0.01 || d < -0.01 {
			t.Errorf("\\ht\\strutbox sous %s = %.2fpt, la référence donne %.2fpt", c.what, v, c.want)
		}
	}
}

// A strut is a rule wanted for its METRICS alone — width\z@ — and the renderer was
// emitting a <rect width="0"> for it. Filled, not stroked, so it paints nothing;
// but it is one element per strut, and one corpus paper went from 1 such rect to 20
// the moment \strut started working.
//
// Both directions, because a renderer that draws nothing is also wrong:
// zero-area rules must vanish and real ones must survive.
func TestZeroAreaRulesAreNotEmitted(t *testing.T) {
	svg := onePage(t, `\documentclass{article}\begin{document}`+
		`\noindent A\strut\vrule height 8pt depth 3pt width 0pt B\par`+
		`\noindent C\rule{40pt}{3pt}D\par\end{document}`)
	if n := strings.Count(svg, `width="0"`); n != 0 {
		t.Errorf(`%d rectangle(s) d'aire nulle émis, attendu 0`, n)
	}
	if !strings.Contains(svg, `width="40" height="3"`) {
		t.Error(`la \rule{40pt}{3pt} VISIBLE n'est plus dessinée: le garde est trop large`)
	}
}
