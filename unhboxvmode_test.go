package engine

import "testing"

// \unhbox and \unhcopy met in vertical mode start an indented paragraph before
// anything is unpacked (tex.web:21104-21109: vmode+un_hbox → back_input;
// new_graf(true)), so the text after them is on the SAME line. LaTeX's \strut is
// \unhcopy\strutbox, and it opens beamer's frame title
// (beamerouterthemedefault.sty:172); the engine put the strut's pieces on the
// vertical list and started the text a line lower (go-tex/engine#924).
//
// Against tectonic on article, every variant below sets its line where the plain
// paragraph sets it (y 126.73 for all), and only \copy, a VERTICAL box, adds a
// line. Asserted as equalities with the plain case, so the font does not matter.
func TestUnhboxInVerticalModeStartsTheParagraph(t *testing.T) {
	page := func(body string) string {
		t.Helper()
		src := `\documentclass{article}\pagestyle{empty}\begin{document}` + body + `\end{document}`
		pages, err := CompileToSVGPages([]byte(src), Options{})
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if len(pages) != 1 {
			t.Fatalf("%s: %d pages, want 1", body, len(pages))
		}
		return pages[0]
	}
	plain := baselineYs(t, page(`Word`))
	if len(plain) != 1 {
		t.Fatalf("plain paragraph: baselines %v, want one", plain)
	}
	for _, body := range []string{
		`\strut Word`,
		`\unhcopy\strutbox Word`,
		`\setbox0\hbox{\vrule height 5pt depth 2pt width 0pt}\unhbox0 Word`,
		`\unhbox200 Word`, // a void register: still leaves vertical mode
	} {
		got := baselineYs(t, page(body))
		if len(got) != 1 || got[0] != plain[0] {
			t.Errorf("%s: baselines %v, want [%v] like the plain paragraph", body, got, plain[0])
		}
	}
	// The control: \copy contributes a box to the vertical list, so the text does
	// start a line lower there, in the reference as here.
	if got := baselineYs(t, page(`\copy\strutbox Word`)); len(got) != 1 || got[0] <= plain[0] {
		t.Errorf(`\copy\strutbox Word: baselines %v, want one below %v`, got, plain[0])
	}
}

// The paragraph \unhbox starts is indented, as new_graf(TRUE) makes it: the word
// starts where the plain paragraph's does.
func TestUnhboxInVerticalModeIndents(t *testing.T) {
	x := func(body string) float64 {
		t.Helper()
		e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`), Options{})
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		v, ok := firstGlyphX(e.Pages()[0])
		if !ok {
			t.Fatalf("%s: no glyph on the page", body)
		}
		return v
	}
	if p, s := x(`Word`), x(`\strut Word`); p != s {
		t.Errorf(`\strut Word starts at x=%v, the plain paragraph at %v`, s, p)
	}
}
