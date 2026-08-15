package engine

import (
	"strings"
	"testing"
)

// \mathds (dsfont) and \Tilde (amsmath) are aliased in the class kernel to the
// blackboard-bold and tilde forms the math layer already renders, so real papers
// using them typeset instead of dropping the whole equation. These were the two
// largest math-command gaps found in an arXiv real-world sweep.
func TestMathAliases(t *testing.T) {
	for _, src := range []string{
		`\documentclass{article}\begin{document}$\mathds{R}\subset\mathds{C}$\end{document}`,
		`\documentclass{article}\begin{document}$\Tilde{x}+\Tilde{y}$\end{document}`,
	} {
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		// The alias must render, not drop: neither the text-mode key (mathds) nor the
		// math-layer key (\mathds) may appear. A benign class-load "input" tally is
		// always present and unrelated.
		sk := e.SkippedCommands()
		for _, k := range []string{"mathds", "\\mathds", "Tilde", "\\Tilde"} {
			if sk[k] != 0 {
				t.Errorf("%q still dropped %q (%d): %v", src, k, sk[k], sk)
			}
		}
		svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
		if !strings.Contains(svg, "<path") {
			t.Errorf("%q rendered no glyph paths", src)
		}
	}
}
