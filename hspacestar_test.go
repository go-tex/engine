package engine

import (
	"regexp"
	"sort"
	"strconv"
	"testing"
)

// \hspace* must survive a break, exactly as \vspace* must survive a page break
// (#371). LaTeX gets it with a ZERO-WIDTH RULE in front of the glue (\@hspacer,
// latex.ltx:6630): a rule is not discardable, so the glue is no longer at a break
// point. Reading the star and dropping it made \hspace* identical to \hspace.
//
// The expected values are tectonic's: a line opening with \hspace*{50pt} starts
// 50pt in, and the unstarred form is discarded — in both engines.
func TestHspaceStarSurvivesALineBreak(t *testing.T) {
	// \\ is LaTeX's; the kernel gives it to us, so the document is a real one.
	src := `\documentclass{article}\begin{document}` + "\n" +
		`\noindent A\\` + "\n" + `\hspace*{50pt}B\\` + "\n" + `\hspace{50pt}C` + "\n" +
		`\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("%d pages, want 1", len(pages))
	}
	re := regexp.MustCompile(`<tspan[^>]*\sx="([0-9.]+)"[^>]*\sy="([0-9.]+)"`)
	firstX := map[string]float64{}
	order := []string{}
	for _, m := range re.FindAllStringSubmatch(pages[0], -1) {
		x, _ := strconv.ParseFloat(m[1], 64)
		y := m[2]
		if prev, seen := firstX[y]; !seen || x < prev {
			if !seen {
				order = append(order, y)
			}
			firstX[y] = x
		}
	}
	if len(order) < 3 {
		t.Fatalf("%d lines, want at least 3", len(order))
	}
	// Top to bottom; the page number is a line too, and it is not one of ours.
	sort.Slice(order, func(i, j int) bool {
		a, _ := strconv.ParseFloat(order[i], 64)
		b, _ := strconv.ParseFloat(order[j], 64)
		return a < b
	})
	plain, starred, unstarred := firstX[order[0]], firstX[order[1]], firstX[order[2]]
	if starred <= plain+40 {
		t.Errorf("the starred line starts at %.1f, the plain one at %.1f — the 50pt did not survive",
			starred, plain)
	}
	if unstarred > plain+1 {
		t.Errorf("the unstarred line starts at %.1f, the plain one at %.1f — that glue should be discarded",
			unstarred, plain)
	}
}
