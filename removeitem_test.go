package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// \unskip, \unkern and \unpenalty were no-ops (tex.web §1104 remove_item, §1105
// delete_last). The instrument is \the\wd of an \hbox, which reports the width in
// scaled points with no layout in the way.
//
// Widths under tectonic, and here before the fix:
//
//	                                   tectonic    before     after
//	A  \hbox{abc \unskip d}              20.84pt   21.87pt   19.87pt
//	B  \hbox{abcd}                       20.84pt   19.87pt   19.87pt
//	C  \hbox{x\kern10pt\unkern y}        10.56pt   19.62pt    9.62pt
//	D  \hbox{xy}                         10.56pt    9.62pt    9.62pt
//	E  \hbox{x\kern10pt\unskip y}        20.56pt   19.62pt   19.62pt
//	F  \hbox{x\kern10pt y}               20.56pt   19.62pt   19.62pt
//
// The absolute widths differ from tectonic's throughout (different metrics, a
// separate matter); what is asserted is the three RELATIONS the reference holds
// and we did not: A=B, C=D, and E-D=+10pt.
//
// E-D is the one that matters most and the one E=F cannot give. \unskip must
// leave a KERN alone ("if type(tail)=cur_chr", §1105), and a no-op satisfies
// E=F trivially — it removes nothing, so of course the two agree. Only E-D=+10pt
// says the kern survived a \unskip that did fire.
func TestRemoveItemHonoursTheTypeGuard(t *testing.T) {
	const src = `\documentclass{article}` +
		`\newbox\ba \newbox\bb \newbox\bc \newbox\bd \newbox\be \newbox\bg` +
		`\begin{document}` +
		`\setbox\ba=\hbox{abc \unskip d}\setbox\bb=\hbox{abcd}` +
		`\setbox\bc=\hbox{x\kern10pt\unkern y}\setbox\bd=\hbox{xy}` +
		`\setbox\be=\hbox{x\kern10pt\unskip y}\setbox\bg=\hbox{x\kern10pt y}` +
		`A=\the\wd\ba\par B=\the\wd\bb\par C=\the\wd\bc\par ` +
		`D=\the\wd\bd\par E=\the\wd\be\par F=\the\wd\bg\par\end{document}`
	pages, _, err := CompileToSVGPagesDiag([]byte(src), Options{Lenient: true})
	if err != nil || len(pages) == 0 {
		t.Fatalf("compilation: %v (%d pages)", err, len(pages))
	}
	w := printedDimens(t, string(pages[0]))
	for _, k := range []string{"A", "B", "C", "D", "E", "F"} {
		if _, ok := w[k]; !ok {
			t.Fatalf("%s= n'a pas été imprimé (largeurs lues: %v)", k, w)
		}
	}
	t.Logf("largeurs: %v", w)
	if d := w["A"] - w["B"]; d > 0.01 || d < -0.01 {
		t.Errorf(`\unskip n'a pas retiré l'espace intermot: \hbox{abc \unskip d} `+
			`fait %.2fpt de plus que \hbox{abcd} (attendu 0)`, d)
	}
	if d := w["C"] - w["D"]; d > 0.01 || d < -0.01 {
		t.Errorf(`\unkern n'a pas retiré le kern: %.2fpt d'écart avec \hbox{xy} (attendu 0)`, d)
	}
	if d := w["E"] - w["D"]; d < 9.99 || d > 10.01 {
		t.Errorf(`\unskip a touché un KERN: \hbox{x\kern10pt\unskip y} fait %.2fpt `+
			`de plus que \hbox{xy}, attendu les 10pt du kern`, d)
	}
}

// \unpenalty has no width, so the instrument is the line BREAK a penalty causes:
// \penalty-10000 forces one, and removing it puts the two words back on one line.
// (\the\lastpenalty would have been the direct reading, but this engine has no
// \lastpenalty — \the printed nothing at all — and an introspection primitive
// that does not exist is not an instrument. The effect is the better witness
// anyway: it is what a document would notice.)
//
// The reference sets AVANT\penalty-10000 \unpenalty APRES as a SINGLE word on one
// line — pdftotext reads "AVANTAPRES" at y=126.73 — while the control
// DEUXA\penalty-10000 DEUXB stays broken across two (168.58 and 180.53). Both are
// in the one document, so the control says the fix removes the tail penalty and
// not every penalty it meets.
func TestUnpenaltyRemovesThePenaltyAndOnlyTheTail(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`\noindent AVANT\penalty-10000 \unpenalty APRES\par\vskip 30pt ` +
		`\noindent DEUXA\penalty-10000 DEUXB\par\end{document}`
	pages, _, err := CompileToSVGPagesDiag([]byte(src), Options{Lenient: true})
	if err != nil || len(pages) == 0 {
		t.Fatalf("compilation: %v (%d pages)", err, len(pages))
	}
	svg := string(pages[0])
	y := map[string]float64{}
	for _, w := range []string{"AVANT", "APRES", "DEUXA", "DEUXB"} {
		v, ok := markerY(svg, w)
		if !ok {
			t.Fatalf("le repère %s n'est pas sur la page", w)
		}
		y[w] = v
	}
	t.Logf("y: AVANT %.2f APRES %.2f / DEUXA %.2f DEUXB %.2f",
		y["AVANT"], y["APRES"], y["DEUXA"], y["DEUXB"])
	if d := y["APRES"] - y["AVANT"]; d > 1 || d < -1 {
		t.Errorf(`\unpenalty n'a pas retiré la pénalité: APRES est %.2fpt sous AVANT, `+
			`donc la coupure forcée a tenu`, d)
	}
	if d := y["DEUXB"] - y["DEUXA"]; d < 1 {
		t.Errorf(`la pénalité TÉMOIN a disparu: DEUXB est à %.2fpt de DEUXA, `+
			`attendu une ligne d'écart`, d)
	}
}

// The mode question, which the SOURCE and the PROSE answer differently and which
// only a measurement settles. §1104 says remove_item "is not allowed in vertical
// mode (except internal vertical mode)"; §1105 refuses only when the contribution
// list is empty TOO, "(mode=vmode)and(tail=head)". Implementing the prose would
// have been wrong in the common direction.
//
// Gap to the next line, tectonic against this engine:
//
//	                              tectonic   before    after
//	\par\vskip40pt\unskip  APRES    11.96    52.00    12.00   <- glue removed
//	\par\vskip40pt         TEMOIN   51.81    52.00    52.00   <- control, kept
//	\par\unskip            APRES    11.96    12.00    12.00   <- nothing to remove
//
// The second row is the control that says the fix removes THE tail and not any
// \vskip it meets. The third needs no mode test to come out right: after \par the
// tail of the vertical list is the paragraph's last LINE, so the type guard
// declines it — and TeX prints no error either (§1106 suppresses the message when
// the page does not end with glue, which is why the reference log is clean).
func TestUnskipInVerticalModeTakesTheTailAndOnlyTheTail(t *testing.T) {
	const doc = `\documentclass{article}\begin{document}` +
		`\noindent Premier paragraphe.\par %s` +
		`\noindent APRES\par\vskip 40pt \noindent TEMOIN\par\end{document}`
	for _, c := range []struct {
		name        string
		mid         string
		wantAfter   float64
		wantControl float64
	}{
		{`\vskip 40pt \unskip`, `\vskip 40pt \unskip `, 12, 52},
		{`\unskip seul`, `\unskip `, 12, 52},
	} {
		t.Run(c.name, func(t *testing.T) {
			pages, _, err := CompileToSVGPagesDiag([]byte(fmt.Sprintf(doc, c.mid)), Options{Lenient: true})
			if err != nil || len(pages) == 0 {
				t.Fatalf("compilation: %v", err)
			}
			svg := string(pages[0])
			y := map[string]float64{}
			for _, w := range []string{"Premier", "APRES", "TEMOIN"} {
				v, ok := markerY(svg, w)
				if !ok {
					t.Fatalf("le repère %s n'est pas sur la page", w)
				}
				y[w] = v
			}
			got, ctl := y["APRES"]-y["Premier"], y["TEMOIN"]-y["APRES"]
			t.Logf("Premier->APRES %.2f, APRES->TEMOIN %.2f", got, ctl)
			if d := got - c.wantAfter; d > 1 || d < -1 {
				t.Errorf("écart Premier->APRES %.2f, attendu %.0f", got, c.wantAfter)
			}
			if d := ctl - c.wantControl; d > 1 || d < -1 {
				t.Errorf("écart APRES->TEMOIN %.2f, attendu %.0f: le \\vskip témoin "+
					"a été touché alors qu'il n'est pas en queue", ctl, c.wantControl)
			}
		})
	}
}

// A helper whose alphabet is narrower than its callers' reports a missing value as
// a missing PRINT, which reads as an engine failure. This has now been widened
// twice for that reason: from [A-F] (written for the labels below) when the leading
// tests used N/L/X/Y, and from one letter to several when the strut tests used SH,
// SD, BH, BD. Callers look up the keys they asked for, so a wider pattern costs
// them nothing — which is the argument for making it wide once rather than each
// time a caller trips over it.
var printedDimenRE = regexp.MustCompile(`\b([A-Z][A-Z0-9]{0,3})\s*=\s*([0-9.]+)\s*pt`)

// printedDimens reads the "A=19.87001pt" that \the\wd wrote into the page.
func printedDimens(t *testing.T, svg string) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	txt := strings.Join(strings.Fields(stripTags(svg)), " ")
	for _, m := range printedDimenRE.FindAllStringSubmatch(txt, -1) {
		v, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			t.Fatalf("%q illisible: %v", m[0], err)
		}
		out[m[1]] = v
	}
	return out
}
