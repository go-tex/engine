package engine

import (
	"strings"
	"testing"
)

// \@setfontsize#1#2#3 states a size (#2) and a leading (#3). The leading reached
// \gotex@notefontsize only inside the \ifx#1\normalsize guard, so nine of the ten
// size commands stated a skip that went nowhere and line spacing never followed
// the font. latex.ltx:8533 \set@fontsize sets both from the one call.
//
// \the\baselineskip, tectonic against this engine:
//
//	                        tectonic   before    after
//	\normalsize                12.0     12.0     12.0
//	\small                     11.0     12.0     11.0
//	\footnotesize               9.5     12.0      9.5
//	\Large                     18.0     12.0     18.0
//
// Each reference value is exactly the third argument of that size's \@setfontsize
// call in the class size file, which is what makes the locus certain.
//
// The sequence matters as much as the values. B and C read the skip AFTER leaving
// a size group, and X/Y read it inside a second one: a fix that sets the leading
// without scoping \f@baselineskip passes an inside-one-group test and fails here.
func TestSetfontsizeLeadingFollowsTheSizeAndComesBack(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`N=\the\baselineskip\par` +
		`{\small S=\the\baselineskip\par}` +
		`{\footnotesize F=\the\baselineskip\par}` +
		`{\Large L=\the\baselineskip\par}` +
		`A=\the\baselineskip\par` +
		`{\footnotesize X=\the\baselineskip\par}B=\the\baselineskip\par` +
		`{\Large Y=\the\baselineskip\par}C=\the\baselineskip\par` +
		`\end{document}`
	pages, _, err := CompileToSVGPagesDiag([]byte(src), Options{Lenient: true})
	if err != nil || len(pages) == 0 {
		t.Fatalf("compilation: %v (%d pages)", err, len(pages))
	}
	got := printedDimens(t, string(pages[0]))
	t.Logf("skips: %v", got)
	// The reference's nine values, in order. A/B/C are the ones that say the skip
	// was RESTORED; X/Y that a second switch after a restore still works.
	for _, w := range []struct {
		key  string
		want float64
		what string
	}{
		{"N", 12, `\normalsize`},
		{"S", 11, `\small`},
		{"F", 9.5, `\footnotesize`},
		{"L", 18, `\Large`},
		{"A", 12, `retour au corps de texte`},
		{"X", 9.5, `\footnotesize après un retour`},
		{"B", 12, `retour depuis \footnotesize`},
		{"Y", 18, `\Large après un retour`},
		{"C", 12, `retour depuis \Large`},
	} {
		v, ok := got[w.key]
		if !ok {
			t.Errorf("%s= (%s) n'a pas été imprimé", w.key, w.what)
			continue
		}
		if d := v - w.want; d > 0.01 || d < -0.01 {
			t.Errorf("%s (%s) = %.2fpt, la référence donne %.1fpt", w.key, w.what, v, w.want)
		}
	}
}

// The register is not the point on its own — the engine acts on \baselineskip, so
// a frozen register was a frozen leading. Distance between two consecutive lines,
// tectonic against this engine:
//
//	                  tectonic   before    after
//	\normalsize          11.96    12.00    12.00
//	\footnotesize         9.46    12.00     9.50
//	\Large               17.93    12.00    18.00
//	back to \normalsize  11.96    12.00    12.00
//
// \normalsize is the CONTROL and was already right, which is why this never looked
// like a defect: body text was correct and only the size-switched blocks — captions,
// abstracts, table bodies, and in several classes the whole bibliography — were not.
// The residual +0.04pt is this engine's sp rounding and is present on the control too.
func TestSetfontsizeLeadingMovesTheLines(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`\noindent NORMALUN\par\noindent NORMALDEUX\par\vskip 20pt ` +
		`{\footnotesize\noindent PETITUN\par\noindent PETITDEUX\par}\vskip 20pt ` +
		`{\Large\noindent GRANDUN\par\noindent GRANDDEUX\par}\vskip 20pt ` +
		`\noindent RETOURUN\par\noindent RETOURDEUX\par\end{document}`
	svg := onePage(t, src)
	for _, c := range []struct {
		a, b string
		want float64
		what string
	}{
		{"NORMALUN", "NORMALDEUX", 11.96, `\normalsize (témoin, déjà juste)`},
		{"PETITUN", "PETITDEUX", 9.46, `\footnotesize`},
		{"GRANDUN", "GRANDDEUX", 17.93, `\Large`},
		{"RETOURUN", "RETOURDEUX", 11.96, `retour au corps de texte`},
	} {
		ya, oka := markerY(svg, c.a)
		yb, okb := markerY(svg, c.b)
		if !oka || !okb {
			t.Fatalf("repères %s/%s absents de la page", c.a, c.b)
		}
		got := yb - ya
		t.Logf("%-34s %.2f (référence %.2f)", c.what, got, c.want)
		// 0.15pt: the sp-rounding residual the control carries, not a tolerance on
		// the defect, which was 2.54pt and 5.93pt.
		if d := got - c.want; d > 0.15 || d < -0.15 {
			t.Errorf("interligne %s = %.2fpt, la référence donne %.2fpt (écart %+.2f)",
				c.what, got, c.want, d)
		}
	}
}

// \linespread must survive a size switch: latex.ltx carries it as \f@linespread
// and \fontsize passes the CURRENT \baselinestretch through (latex.ltx:7583), so
// the factor multiplies each size's own skip rather than being replaced by it.
//
// This is the direction that catches an unscoped \f@baselineskip from the other
// side: baselineStretchFactor recovers the factor as baselineskip/baseBaselineskip,
// so a baseBaselineskip left at 9.5pt after a \footnotesize group turns a plain
// document into a 1.26-spaced one.
//
//	                     tectonic   after
//	\linespread{1.5}        17.93    18.00   = 1.5 x 12
//	 + \footnotesize        14.20    14.25   = 1.5 x 9.5
//	 back out               17.93    18.00   the factor is still 1.5, not 1.26
func TestSetfontsizeLeadingKeepsTheLinespread(t *testing.T) {
	const src = `\documentclass{article}\linespread{1.5}\selectfont\begin{document}` +
		`\noindent ETIREUN\par\noindent ETIREDEUX\par\vskip 20pt ` +
		`{\footnotesize\noindent SPETITUN\par\noindent SPETITDEUX\par}\vskip 20pt ` +
		`\noindent SRETOURUN\par\noindent SRETOURDEUX\par\end{document}`
	svg := onePage(t, src)
	for _, c := range []struct {
		a, b string
		want float64
		what string
	}{
		{"ETIREUN", "ETIREDEUX", 17.93, `1.5 x \normalsize`},
		{"SPETITUN", "SPETITDEUX", 14.20, `1.5 x \footnotesize`},
		{"SRETOURUN", "SRETOURDEUX", 17.93, `1.5 rétabli après le groupe`},
	} {
		ya, oka := markerY(svg, c.a)
		yb, okb := markerY(svg, c.b)
		if !oka || !okb {
			t.Fatalf("repères %s/%s absents de la page", c.a, c.b)
		}
		got := yb - ya
		t.Logf("%-32s %.2f (référence %.2f)", c.what, got, c.want)
		if d := got - c.want; d > 0.15 || d < -0.15 {
			t.Errorf("interligne %s = %.2fpt, la référence donne %.2fpt (écart %+.2f)",
				c.what, got, c.want, d)
		}
	}
}

// onePage compiles through the path cmd/gotex takes and returns page 1.
func onePage(t *testing.T, src string) string {
	t.Helper()
	pages, _, err := CompileToSVGPagesDiag([]byte(src), Options{Lenient: true})
	if err != nil || len(pages) == 0 {
		t.Fatalf("compilation: %v (%d pages)", err, len(pages))
	}
	return string(pages[0])
}

var _ = strings.Contains
