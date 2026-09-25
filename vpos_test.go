package engine

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// minipage, parbox, tabular and subfigure take a VERTICAL [pos] — t/c/b — and all
// four called scanOptBracketPos, which accepts the HORIZONTAL set l/c/r and
// returns 'c' for anything else. So [t] and [b] were silently [c] everywhere, and
// alignParbox's 't' and 'b' branches were unreachable from any caller: carefully
// written, never run.
//
// tabular, \ht/\dp of \hbox{\begin{tabular}[…]{l}AAA\\BBB\\CCC\end{tabular}},
// tectonic against this engine — exact in all six values after:
//
//	       tectonic          before            after
//	[c]   20.5 /15.5      32.4/3.6        20.5 /15.5     the DEFAULT
//	[t]   8.39996/27.6    32.4/3.6        8.4  /27.6
//	[b]   32.39996/3.6    32.4/3.6        32.4 /3.6      the only case we matched
//
// The TOTAL was right throughout — 36.0pt, 12pt a row — so this was never a
// sizing defect, only where the baseline sits inside the box. And the one anchor
// implemented was the one almost nobody writes: in document bodies across the
// 200-paper corpus, [b] is 0 occurrences for tabular against 628 defaulted and
// 347 explicit [c].
func TestTabularHonoursItsVerticalPosition(t *testing.T) {
	const src = `\documentclass{article}\newbox\bx\begin{document}` +
		`\setbox\bx=\hbox{\begin{tabular}{l}AAA\\BBB\\CCC\end{tabular}}CH=\the\ht\bx\par CD=\the\dp\bx\par` +
		`\setbox\bx=\hbox{\begin{tabular}[t]{l}AAA\\BBB\\CCC\end{tabular}}TH=\the\ht\bx\par TD=\the\dp\bx\par` +
		`\setbox\bx=\hbox{\begin{tabular}[b]{l}AAA\\BBB\\CCC\end{tabular}}BH=\the\ht\bx\par BD=\the\dp\bx\par` +
		`\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	for _, c := range []struct {
		key  string
		want float64
		what string
	}{
		{"CH", 20.5, `[c] hauteur`}, {"CD", 15.5, `[c] profondeur`},
		{"TH", 8.4, `[t] hauteur`}, {"TD", 27.6, `[t] profondeur`},
		{"BH", 32.4, `[b] hauteur`}, {"BD", 3.6, `[b] profondeur`},
	} {
		v, ok := got[c.key]
		if !ok {
			t.Errorf("%s= n'a pas été imprimé (%s)", c.key, c.what)
			continue
		}
		if d := v - c.want; d > 0.01 || d < -0.01 {
			t.Errorf("tabular %s = %.2fpt, la référence donne %.2fpt", c.what, v, c.want)
		}
	}
	// The three totals must agree: only the anchor moves, never the size. This is
	// what says a wrong anchor was never a sizing defect.
	for _, p := range [][2]string{{"CH", "CD"}, {"TH", "TD"}, {"BH", "BD"}} {
		if d := (got[p[0]] + got[p[1]]) - 36.0; d > 0.02 || d < -0.02 {
			t.Errorf("total %s+%s = %.2fpt, attendu 36.0pt: l'ancrage ne doit pas changer la TAILLE",
				p[0], p[1], got[p[0]]+got[p[1]])
		}
	}
}

// minipage, same defect. Here the assertion is on the RELATIONS, not the
// reference's numbers: our total is 30.73pt against the reference's 31.38pt (font
// metrics), so pinning 7.16pt would be pinning someone else's face.
//
//	       tectonic          before            after
//	[c]   18.19/13.19   17.865/12.865   17.865/12.865
//	[t]    7.16/24.22   17.865/12.865     6.62/24.11
//	[b]   31.16/00.22   17.865/12.865    30.62/00.11
//
// [t] anchors on the FIRST line's baseline, so its height is one line's height —
// 6.62pt here, the same value a one-row \halign and a one-row tabular give.
// [b] leaves the reference at the last line's baseline, so its height is the total
// less that line's depth. [c] centres on the math axis, height = total/2 + axis.
func TestMinipageHonoursItsVerticalPosition(t *testing.T) {
	const src = `\documentclass{article}\newbox\bx\begin{document}` +
		`\setbox\bx=\hbox{\begin{minipage}{60pt}AAA\\BBB\\CCC\end{minipage}}CH=\the\ht\bx\par CD=\the\dp\bx\par` +
		`\setbox\bx=\hbox{\begin{minipage}[t]{60pt}AAA\\BBB\\CCC\end{minipage}}TH=\the\ht\bx\par TD=\the\dp\bx\par` +
		`\setbox\bx=\hbox{\begin{minipage}[b]{60pt}AAA\\BBB\\CCC\end{minipage}}BH=\the\ht\bx\par BD=\the\dp\bx\par` +
		`\end{document}`
	got := printedDimens(t, onePage(t, src))
	t.Logf("%v", got)
	total := got["CH"] + got["CD"]
	if total < 20 {
		t.Fatalf("total invraisemblable (%.2fpt): les repères manquent — %v", total, got)
	}
	// The three anchors must be DISTINCT: identical values are the defect itself.
	if d := got["TH"] - got["CH"]; d > -1 {
		t.Errorf("[t] hauteur %.2f et [c] hauteur %.2f: [t] doit être BIEN plus haut placé",
			got["TH"], got["CH"])
	}
	if d := got["BH"] - got["CH"]; d < 1 {
		t.Errorf("[b] hauteur %.2f et [c] hauteur %.2f: [b] doit être BIEN plus bas placé",
			got["BH"], got["CH"])
	}
	// [b] keeps the reference at the last line's baseline: height = total - its depth.
	if d := (got["BH"] + got["BD"]) - total; d > 0.02 || d < -0.02 {
		t.Errorf("[b] total %.2f contre %.2f: l'ancrage ne doit pas changer la taille",
			got["BH"]+got["BD"], total)
	}
	// [c] centres on the math axis, above the baseline: height > half the total.
	if got["CH"] <= total/2 {
		t.Errorf("[c] hauteur %.2f n'est pas au-dessus de la moitié du total %.2f: "+
			"latex.ltx centre sur l'AXE, pas sur la ligne de base", got["CH"], total)
	}
}

// The horizontal callers must keep the horizontal set. \makebox[width][l|c|r] is
// l/c/r, and a union of letters would have let [t] through to code that switches on
// l/c/r and [l] through to alignParbox — each defaulting somewhere downstream
// instead of being ignored where it is wrong.
//
// The assertion is that [l] and [r] put the content at DIFFERENT x inside the same
// 60pt box, which is the only thing that says the letter reached the alignment. A
// version of this test that merely checked the words were on the page passed
// whatever the reader accepted, and said nothing.
func TestMakeboxKeepsTheHorizontalPositionSet(t *testing.T) {
	x := func(pos string) float64 {
		svg := onePage(t, `\documentclass{article}\begin{document}`+
			`\noindent\makebox[60pt][`+pos+`]{ZZ}\par\end{document}`)
		for _, m := range svgSpanX.FindAllStringSubmatch(svg, -1) {
			if strings.Contains(m[2], "ZZ") {
				v, err := strconv.ParseFloat(m[1], 64)
				if err != nil {
					t.Fatalf("x illisible pour [%s]: %v", pos, err)
				}
				return v
			}
		}
		t.Fatalf("le repère ZZ n'est pas sur la page pour [%s]", pos)
		return 0
	}
	xl, xc, xr := x("l"), x("c"), x("r")
	t.Logf("x de ZZ: [l] %.2f  [c] %.2f  [r] %.2f", xl, xc, xr)
	if !(xl < xc && xc < xr) {
		t.Errorf("[l] %.2f, [c] %.2f, [r] %.2f doivent être strictement croissants "+
			"dans une boîte de 60pt; des valeurs égales veulent dire que la lettre "+
			"n'atteint pas l'alignement", xl, xc, xr)
	}
}

var svgSpanX = regexp.MustCompile(`<tspan[^>]*\bx="([0-9.-]+)"[^>]*>(.*?)</tspan>`)
