package engine

import (
	"strings"
	"testing"
)

// \c@page is a real count register: it reads, advances and compares, and a header
// field sees the page being assembled. Witness: tectonic 0.17 on the same source
// prints "[1]" for the first \message and pages whose footers read "p1 odd", "p2
// even", "p3 odd".
func TestCPageIsACountRegister(t *testing.T) {
	const src = `\documentclass{article}\usepackage{fancyhdr}\pagestyle{fancy}\makeatletter` +
		`\fancyhf{}\cfoot{p\the\c@page\ifodd\c@page\ odd\else\ even\fi}` +
		`\begin{document}` +
		`A\newpage B\newpage C` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pages := e.RenderPages(e.renderMargin(0))
	if len(pages) != 3 {
		t.Fatalf("%d pages, want 3", len(pages))
	}
	for i, want := range []string{"p1 odd", "p2 even", "p3 odd"} {
		got := stripSVGTags(pages[i])
		if !strings.Contains(strings.Join(strings.Fields(got), " "), want) {
			t.Errorf("page %d footer: want %q in %q", i+1, want, firstN(got, 120))
		}
	}
}

// In running text the register holds its initial value, and arithmetic on it works
// like on any \count; it is \count0, as in TeX.
func TestCPageArithmetic(t *testing.T) {
	const src = `\documentclass{article}\makeatletter\begin{document}` +
		`[\the\c@page]\c@page=5 \advance\c@page by 2 [\the\c@page][\the\count0]` +
		`\ifnum\c@page>6 big\else small\fi\ifodd\c@page odd\fi\stepcounter{page}[\the\c@page]` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	text = strings.Join(strings.Fields(text), "")
	if !strings.Contains(text, "[1][7][7]bigodd[8]") {
		t.Errorf(`\c@page arithmetic: got %q, want "[1][7][7]bigodd[8]"`, firstN(text, 100))
	}
}
