package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two \column's in a frame must not cost a page break. beamer's \beamer@colclose
// carries \end{minipage}\hfill\end{actionenv}\ignorespaces
// (beamerbaseframecomponents.sty:283); honouring only the \end{minipage} and
// emptying the whole macro dropped the \end{actionenv} pairing the
// \begin{actionenv} the column opened, so two columns left an environment group
// open and the \end{frame} that followed closed THAT — the next frame never began
// its own page, and material written after \end{columns} came out on the page
// BEFORE the frame's own content.
func TestTwoBeamerColumnsKeepTheirPageBreaks(t *testing.T) {
	// \beamer@colclose exists only in the REAL class; the built-in emulation makes
	// \column a \par and never opens an actionenv, so it cannot show this defect.
	// Skip rather than pass for the wrong reason where the tree is not installed.
	tree := os.Getenv("GOTEX_TEXMF")
	if tree == "" {
		tree = "/Users/Shared/gotex/measure/texmf"
	}
	if _, err := os.Stat(filepath.Join(tree, "beamer.cls")); err != nil {
		t.Skip("no real beamer.cls under GOTEX_TEXMF: the emulation does not exercise \\beamer@colclose")
	}
	t.Setenv("GOTEX_TEXMF", tree)
	src := `\documentclass{beamer}\begin{document}` +
		`\begin{frame}{Un}\begin{columns}` +
		`\column{0.5\textwidth}gauche` +
		`\column{0.5\textwidth}droite` +
		`\end{columns}\end{frame}` +
		`\begin{frame}{Deux}beta\end{frame}` +
		`\begin{frame}{Trois}gamma\end{frame}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) != 3 {
		var got []string
		for _, p := range pages {
			got = append(got, mvlText(p.list))
		}
		t.Fatalf("pages=%d want 3: %q", len(pages), got)
	}
	if txt := mvlText(pages[1].list); !strings.Contains(txt, "Deux") || strings.Contains(txt, "Trois") {
		t.Errorf("page 2 should carry the second frame alone: %q", txt)
	}
}

// An environment whose closer begins with \end{minipage} (\endcolumn is
// {\end{minipage}\hfill\end{actionenv}\@ignoretrue}, beamerbaseframecomponents.sty:278)
// closes its OWN group too. The minipage's body scanner consumes the \end{zzcol} and
// runs \endzzcol in its place; the \endgroup that \end adds after it used to be lost.
func TestAnEndWrapperAroundMinipageClosesItsGroup(t *testing.T) {
	got := ckRun(t, `\newenvironment{zzcol}{\begin{minipage}{50pt}}{\end{minipage}\relax}`+
		`\message{[\the\currentgrouplevel]}\begin{zzcol}x\end{zzcol}\message{[\the\currentgrouplevel]}`)
	parts := strings.SplitAfter(got, "]")
	if len(parts) < 2 || parts[0] != strings.TrimSpace(parts[1]) {
		t.Errorf("group levels before/after = %q, want the same level twice", got)
	}
}

// The bare \column shape: the minipage's \end is stored in \beamer@colclose and run by
// \end{columns}, whose \endcolumns begins with it (beamerbaseframecomponents.sty:237).
// The scanner runs \endcolumns in place of the \end{columns} it consumed, and the
// group \begin{columns} opened must close with it.
func TestAnEndThatLeadsToColcloseClosesItsGroup(t *testing.T) {
	got := ckRun(t, `\catcode`+"`"+`\@=11 \def\beamer@colclose{\end{minipage}}`+
		`\newenvironment{zzcols}{}{\beamer@colclose}`+
		`\message{[\the\currentgrouplevel]}\begin{zzcols}\begin{minipage}{50pt}x\end{zzcols}\message{[\the\currentgrouplevel]}`)
	parts := strings.SplitAfter(got, "]")
	if len(parts) < 2 || parts[0] != strings.TrimSpace(parts[1]) {
		t.Errorf("group levels before/after = %q, want the same level twice", got)
	}
}

// The column ENVIRONMENT, with text after \end{columns}: one page, in source order
// (go-tex/engine#205). The leaked group was taken by \end{columns}' \egroup, and
// AFTER came out on a page of its own BEFORE the frame.
func TestTextAfterColumnEnvironmentsStaysInTheFrame(t *testing.T) {
	tree := os.Getenv("GOTEX_TEXMF")
	if tree == "" {
		tree = "/Users/Shared/gotex/measure/texmf"
	}
	if _, err := os.Stat(filepath.Join(tree, "beamer.cls")); err != nil {
		t.Skip("no real beamer.cls under GOTEX_TEXMF: the emulation has no column environment of beamer's shape")
	}
	t.Setenv("GOTEX_TEXMF", tree)
	src := `\documentclass{beamer}\begin{document}\begin{frame}{FRAMETITLE}\begin{columns}[t]` +
		`\begin{column}{0.5\textwidth}LEFT\end{column}` +
		`\begin{column}{0.5\textwidth}RIGHT\end{column}` +
		`\end{columns}AFTER\end{frame}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) != 1 {
		var got []string
		for _, p := range pages {
			got = append(got, mvlText(p.list))
		}
		t.Fatalf("pages=%d want 1: %q", len(pages), got)
	}
	txt := mvlText(pages[0].list)
	if i, j := strings.Index(txt, "FRAMETITLE"), strings.Index(txt, "AFTER"); i < 0 || j < i {
		t.Errorf("want FRAMETITLE before AFTER on the page: %q", txt)
	}
}

// colCloseAfterEndMinipage keeps what follows the \end{minipage} it honours, and
// answers nil for a closer that does not begin with one (nothing honoured, nothing
// to drop).
func TestColCloseKeepsWhatFollowsTheEndMinipage(t *testing.T) {
	body := tokenizeTeX(`\end{minipage}\hfill\end{actionenv}\ignorespaces`)
	rest := colCloseAfterEndMinipage(body)
	e := New()
	if got := e.toksToString(rest); got != `\hfill \end {actionenv}\ignorespaces ` && !strings.Contains(got, "actionenv") {
		t.Errorf("rest = %q, want the \\end{actionenv} tail", got)
	}
	if colCloseAfterEndMinipage(tokenizeTeX(`\hfill`)) != nil {
		t.Error("a closer with no leading \\end{minipage} must be left alone")
	}
	if colCloseAfterEndMinipage(nil) != nil {
		t.Error("an empty closer must stay empty")
	}
}
