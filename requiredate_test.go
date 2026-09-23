package engine

import (
	"strings"
	"testing"
)

// A file spec takes TWO optional arguments — options and a DATE:
//
//	\@onefilewithoptions#1[#2][#3]#4      (latex.ltx:13735)
//
// The date is how a package states the version it needs. jabbrv.sty writes
// \RequirePackage{kvoptions}[2006/08/17], and unread it was TYPESET: every
// document loading jabbrv opened on a page carrying "[2006/08/17]", with its title
// pushed to page 2.
func TestPackageDateIsConsumedNotTypeset(t *testing.T) {
	for _, src := range []string{
		`\usepackage{amsmath}[2000/01/01]`,
		`\usepackage[fleqn]{amsmath}[2000/01/01]`,
		`\RequirePackage{kvoptions}[2006/08/17]`,
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(src + `\message{[after]}`); err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got := trimNL(e.out.String()); got != "[after]" {
			t.Errorf("%s left %q behind", src, got)
		}
	}
}

// The lookahead for that date must NOT expand: LaTeX's own is \@ifnextchar, which
// is \futurelet. Expanding runs whatever follows the load before the package is
// registered — \usepackage{p}\@ifpackageloaded{p}{…} then answers "not loaded",
// which is how this fix first broke two existing tests.
func TestPackageDateLookaheadExpandsNothing(t *testing.T) {
	withTempDir(t, map[string]string{
		"prov.sty": `\def\provcmd{}`,
	}, func() {
		out, _ := runLaTeX(t, `\usepackage{prov}`+
			`\@ifpackageloaded{prov}{\message{IS-LOADED}}{\message{NOT-LOADED}}`)
		if !strings.Contains(out, "IS-LOADED") {
			t.Errorf("the lookahead ran \\@ifpackageloaded too early; %q", out)
		}
	})
}

// A bracket that is NOT a date — a real one opening a paragraph — must survive.
// Both engines stop at the \par, so the reference keeps it too.
func TestABracketAfterAParagraphIsNotADate(t *testing.T) {
	src := `\documentclass{article}\begin{document}` + "\n" +
		`\usepackage{amsmath}` + "\n\n" + `[1] Une reference.` + "\n" + `\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 || !strings.Contains(pages[0], "1]") {
		t.Errorf("the bracket was eaten")
	}
}
