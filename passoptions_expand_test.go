package engine

import (
	"strings"
	"testing"
)

// \PassOptionsToClass expands its first argument (ltfiles.dtx \@pass@ptions \xdef's
// it), so the canonical forwarding idiom hands the user's option on. elsarticle.cls:110
// is exactly this line; with the argument read unexpanded the option vanished, article
// loaded size10.clo under [12pt], and #939's \normalsize rebase then set the body at
// 10pt (2205.03780). \@ptsize is the witness: 2 is size12.clo.
func TestPassOptionsToClassExpandsCurrentOption(t *testing.T) {
	withTempDir(t, map[string]string{
		"fwd.cls": `\ExecuteOptions{a4paper,10pt}` +
			`\DeclareOption*{\PassOptionsToClass{\CurrentOption}{article}}` +
			`\ProcessOptions\relax\LoadClass{article}`,
	}, func() {
		src := `\documentclass[12pt]{fwd}\makeatletter\begin{document}` +
			`PT=\@ptsize\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if got := pageChars(e); !strings.Contains(got, "PT=2") {
			t.Errorf("12pt was not forwarded to article: want PT=2, got %q", got)
		}
		if got := e.baseFontPt; got < 11.9 || got > 12.1 {
			t.Errorf("body size %.2fpt, want 12 (size12.clo \\@xiipt)", got)
		}
	})
}
