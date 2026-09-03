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
