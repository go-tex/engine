package engine

import (
	"strings"
	"testing"
)

// runPath compiles a snippet and returns the glyphs it typeset.
func runPath(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\usepackage{url}\begin{document}` + src + `\end{document}`); err != nil {
		t.Fatalf("run %q: %v", src, err)
	}
	return glyphString(e.mvl) + glyphString(e.parList)
}

// url.sty's \path sets its argument verbatim, braced or fenced by any character.
//
// url.sty line 204: \@ifundefined{path}{\DeclareUrlCommand\path{\urlstyle{tt}}}{}
// — \url's sibling in typewriter. Undefined, the text inside it was dropped, and
// five corpus papers lose their DOIs, a mail address and a file path that way.
func TestUrlPathSetsItsArgument(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"braced", `\path{doi:10.1016/j.jsc.2022.08.021}\par`, "doi:10.1016/j.jsc.2022.08.021"},
		// The fence is whatever non-brace character comes first; 2404.16039
		// writes \urldef{\mailAM}\path|alexmos@kma.zcu.cz |.
		{"fenced by a bar", `\path|alexmos@kma.zcu.cz|\par`, "alexmos@kma.zcu.cz"},
		{"fenced by a plus", `\path+a/b/c+\par`, "a/b/c"},
		{"a file path", `\path{petsc/src/snes/tutorials}\par`, "petsc/src/snes/tutorials"},
	} {
		if got := runPath(t, c.src); !strings.Contains(got, c.want) {
			t.Errorf("%s: %q does not contain %q", c.name, got, c.want)
		}
	}
}

// \path is NOT a link of its own — a bibliography writes
// \href{https://doi.org/…}{\path{doi:…}} and the href carries the link. hyperref's
// \url is the one that links to itself.
func TestUrlPathIsNotALinkOfItsOwn(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\usepackage{url}\begin{document}\path{doi:10/x}\par\end{document}`); err != nil {
		t.Fatal(err)
	}
	if hasLinkNode(e.mvl) {
		t.Error("\\path made a link; url.sty's \\path does not")
	}
	// …while \url does, so the test above is not passing for want of links.
	e2 := New()
	if err := e2.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\documentclass{article}\usepackage{url}\begin{document}\url{http://x/y}\par\end{document}`); err != nil {
		t.Fatal(err)
	}
	if !hasLinkNode(e2.mvl) {
		t.Error("\\url made no link; the control for the assertion above is broken")
	}
}

// hasLinkNode reports whether any link was placed.
func hasLinkNode(nodes []node) bool {
	for _, n := range nodes {
		switch v := n.(type) {
		case linkNode:
			return true
		case *boxNode:
			if hasLinkNode(v.list) {
				return true
			}
		}
	}
	return false
}

// \urldef{\name}\path|…| defines and typesets NOTHING at the definition, like
// \urldef with \url. The delimited form is what sent it down the wrong path
// before: the argument reader only knew braces.
func TestUrldefTakesPathWithAFence(t *testing.T) {
	got := runPath(t, `X\urldef{\mail}\path|user@example.org|Y[\mail]\par`)
	i, j := strings.Index(got, "X"), strings.Index(got, "Y")
	if i < 0 || j < 0 {
		t.Fatalf("markers missing: %q", got)
	}
	if strings.Contains(got[i:j], "example.org") {
		t.Errorf("the address was typeset at the definition: %q", got)
	}
	if !strings.Contains(got[j:], "user@example.org") {
		t.Errorf("the address was not typeset where \\mail was used: %q", got)
	}
}

// An unterminated fence must not swallow the rest of the document.
func TestUrlPathWithNoClosingFenceEatsNothing(t *testing.T) {
	got := runPath(t, `\path|unterminated and then MARKERZZ\par`)
	if !strings.Contains(got, "MARKERZZ") {
		t.Errorf("text after an unclosed \\path was eaten: %q", got)
	}
}

// Without url or hyperref, \path is not url's: the name belongs to whoever the
// document loaded, and binding it anyway set forest's
// \path [draw, \forestoption{edge}] as a URL — measured, 2402.04711 lost a page.
func TestUrlPathIsNotBoundWithoutThepackage(t *testing.T) {
	for _, c := range []struct{ name, pre string }{
		{"no package at all", ``},
		{"tikz first, then url", `\usepackage{tikz}\usepackage{url}`},
		{"forest first, then hyperref", `\usepackage{forest}\usepackage{hyperref}`},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		e.lenient = true
		src := `\documentclass{article}` + c.pre + `\begin{document}\path (0,0) -- (1,1);\par\end{document}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if e.SkippedCommands()["path"] == 0 {
			t.Errorf("%s: \\path was bound to url.sty's, and it is not url's here", c.name)
		}
	}
}
