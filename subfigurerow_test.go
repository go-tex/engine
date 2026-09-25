package engine

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A \subfigure panel is horizontal material: met in vertical mode it starts a
// paragraph, so the next panel joins the SAME line. Without that each panel took
// a line of its own and a three-panel figure stood three rules tall — which is
// what puts a float alone on a page and leaves ten near-empty pages in a paper
// the reference sets in one (2406.13839).
//
// The y of the first word AFTER the figure, tectonic against this engine:
//
//	panels   tectonic   before   after
//	1          210.48   202.64  207.89
//	2          215.46   242.49  207.89
//	3          215.46   282.34  207.89
//
// The reference does not move between two panels and three; we grew 39.85pt each
// time, which is the rule's height. The assertion is that shape — the marker does
// not move when a panel is added — not the absolute number, which also carries
// the float's own spacing.
//
// Measured with the reference and the subject in SEPARATE directories: a
// subfigure.sty placed where tectonic can find it is read by THIS engine too,
// instead of its own definition, and executing the real package is itself lossy.
func TestSubfigurePanelsShareALine(t *testing.T) {
	var ys []float64
	for n := 1; n <= 3; n++ {
		src := `\documentclass{article}\begin{document}\begin{figure}`
		for i := 0; i < n; i++ {
			src += `\subfigure{\rule{0.2\textwidth}{40pt}}`
		}
		src += `\caption{c}\end{figure}SUITE\end{document}`
		// CompileToSVGPagesDiag is what cmd/gotex calls: the float machinery runs
		// and the pages come out paginated. compile()+RenderPage(1) does NOT show
		// this defect — the first version of this test passed with and without the
		// fix, on identical numbers, because it was reading a page the floats had
		// not been flushed into.
		pages, _, err := CompileToSVGPagesDiag([]byte(src), Options{Lenient: true})
		if err != nil || len(pages) == 0 {
			t.Fatalf("%d panneau(x): %v (%d pages)", n, err, len(pages))
		}
		y, ok := markerY(string(pages[0]), "SUITE")
		if !ok {
			t.Fatalf("%d panneau(x): le repère SUITE n'est pas sur la page", n)
		}
		ys = append(ys, y)
	}
	t.Logf("y de SUITE pour 1/2/3 panneaux: %v", ys)
	for i := 1; i < len(ys); i++ {
		if d := ys[i] - ys[0]; d > 1 || d < -1 {
			t.Errorf("ajouter un panneau a descendu SUITE de %.2f (y = %v); "+
				"les panneaux doivent partager une ligne", d, ys)
		}
	}
}

// markerY returns the y of the <tspan> that spells word. The page is ONE <text>
// element (see textlayer_test.go on why search depends on that), so the
// coordinates live on the tspans, not on the text.
var svgSpan = regexp.MustCompile(`<tspan[^>]*\by="([0-9.]+)"[^>]*>([^<]*)</tspan>`)

func markerY(svg, word string) (float64, bool) {
	for _, m := range svgSpan.FindAllStringSubmatch(svg, -1) {
		if strings.Contains(m[2], word) {
			y, err := strconv.ParseFloat(m[1], 64)
			return y, err == nil
		}
	}
	return 0, false
}
