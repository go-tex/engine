package engine

import (
	"strings"
	"testing"
)

// \begin{document} starts the body at \normalsize (ltfiles.dtx:302), whatever size
// the preamble left selected. beamer's sidebar outer theme selects the \Large
// frame-title font in the preamble to measure its head band
// (beamerouterthemesidebar.sty:18-21); without the reset, every frame of a
// Rochester talk was set at 14.4pt on 18pt lines, and a listing ran off the page.
func TestDocumentStartsAtNormalsize(t *testing.T) {
	for _, tc := range []struct {
		name, preamble string
	}{
		{"Large", `\Large`},
		{"tiny", `\tiny`},
		{"fontsize", `\fontsize{30}{36}\selectfont`},
	} {
		src := `\documentclass{article}` + tc.preamble +
			`\begin{document}\noindent x [\the\baselineskip]\end{document}`
		e, err := compile([]byte(src), Options{})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := glyphSizes(e)['x']; got != 10 {
			t.Errorf("%s in the preamble: body glyph at %vpt, want 10 (\\normalsize)", tc.name, got)
		}
		if txt := pageText(e); !strings.Contains(txt, "[12.0pt]") {
			t.Errorf("%s in the preamble: body \\baselineskip %q, want [12.0pt]", tc.name, txt)
		}
	}
}

// \normalsize also installs the first-level list parameters: size10.clo's
// \normalsize ends with \let\@listi\@listI (size10.clo:47-53). So a style that
// redefines \@listi in the preamble, as tmlr.sty:179 does
// (\def\@listi{\leftmargin\leftmargini}), is overridden at \begin{document}, and
// the body's first-level lists and quotes keep \topsep, \parsep and \itemsep.
// Without the reset, a TMLR abstract (a quote) sat 12pt closer to its heading than
// LaTeX puts it.
func TestDocumentInstallsFirstLevelListParameters(t *testing.T) {
	src := `\documentclass{article}\makeatletter\def\@listi{\leftmargin\leftmargini}\makeatother` +
		`\begin{document}\makeatletter` +
		`\ifx\@listi\@listI LISTI-INSTALLED\else LISTI-PLACEHOLDER\fi\end{document}`
	e, err := compile([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if txt := pageText(e); !strings.Contains(txt, "LISTI-INSTALLED") {
		t.Errorf("\\@listi at \\begin{document}: %q, want it to be \\@listI", txt)
	}
}

// A size chosen in the body is not undone: the reset happens once, at
// \begin{document}.
func TestBodySizeChoiceStands(t *testing.T) {
	src := `\documentclass{article}\begin{document}\Large\noindent x [\the\baselineskip]\end{document}`
	e, err := compile([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := glyphSizes(e)['x']; got != 14.4 {
		t.Errorf("body glyph at %vpt, want 14.4 (\\Large)", got)
	}
	if txt := pageText(e); !strings.Contains(txt, "[18.0pt]") {
		t.Errorf("body \\baselineskip %q, want [18.0pt]", txt)
	}
}
