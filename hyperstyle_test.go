// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// renderHyperDoc runs a small document and returns its SVG. The colour of a piece
// of link text shows up as a fill="#rrggbb" on the glyph rects (see boxrender.go),
// so the tests assert on the presence (or absence) of a colour in the SVG.
func renderHyperDoc(t *testing.T, body string) string {
	t.Helper()
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\begin{document}` + body + `\end{document}`); err != nil {
		t.Fatal(err)
	}
	return e.RenderPage(24)
}

// With colorlinks on and urlcolor=blue, \href text is painted blue.
func TestHrefColoredWhenColorlinks(t *testing.T) {
	svg := renderHyperDoc(t, `\hypersetup{colorlinks=true,urlcolor=blue}\href{http://example.com}{click here}`)
	if !strings.Contains(svg, `fill="#0000ff"`) {
		t.Errorf("\\href text was not painted urlcolor blue with colorlinks on")
	}
}

// Without colorlinks (hyperref's default) the same \href stays the default text
// colour — no blue appears.
func TestHrefNotColoredByDefault(t *testing.T) {
	svg := renderHyperDoc(t, `\hypersetup{urlcolor=blue}\href{http://example.com}{click here}`)
	if strings.Contains(svg, `fill="#0000ff"`) {
		t.Errorf("\\href text was coloured even though colorlinks is off")
	}
}

// \url text is coloured with urlcolor too.
func TestURLColoredWhenColorlinks(t *testing.T) {
	svg := renderHyperDoc(t, `\hypersetup{colorlinks,urlcolor=blue}\url{http://example.com}`)
	if !strings.Contains(svg, `fill="#0000ff"`) {
		t.Errorf("\\url text was not painted urlcolor blue with colorlinks on")
	}
}

// \hyperlink (an internal link) is painted linkcolor, and its \hypertarget
// counterpart — a destination, not a visible link — is not.
func TestHyperlinkUsesLinkColorButTargetDoesNot(t *testing.T) {
	link := renderHyperDoc(t, `\hypersetup{colorlinks,linkcolor=green}\hyperlink{a}{go}`)
	if !strings.Contains(link, `fill="#00ff00"`) {
		t.Errorf("\\hyperlink text was not painted linkcolor green")
	}
	target := renderHyperDoc(t, `\hypersetup{colorlinks,linkcolor=green}\hypertarget{a}{anchor}`)
	if strings.Contains(target, `fill="#00ff00"`) {
		t.Errorf("\\hypertarget destination text was coloured; only \\hyperlink should be")
	}
}

// The package-option form styles links exactly as \hypersetup does.
func TestColorlinksFromPackageOptions(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\usepackage[colorlinks=true,urlcolor=blue]{hyperref}` +
		`\begin{document}\href{http://example.com}{click}\end{document}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPage(24)
	if !strings.Contains(svg, `fill="#0000ff"`) {
		t.Errorf("\\usepackage[colorlinks,urlcolor=blue]{hyperref} did not colour the link")
	}
}

// allcolors sets link and url colours at once.
func TestAllColorsSetsBoth(t *testing.T) {
	e := New()
	e.applyHypersetup("colorlinks,allcolors=red")
	if e.hyperURLColor != 0xFF0000 || e.hyperLinkColor != 0xFF0000 {
		t.Errorf("allcolors=red set url=%06X link=%06X, want both FF0000", e.hyperURLColor, e.hyperLinkColor)
	}
}

// colorlinks defaults to hyperref's own colours (magenta url, red link) when
// enabled without explicit colours, and can be turned back off.
func TestColorlinksDefaultsAndToggle(t *testing.T) {
	e := New()
	if e.hyperURLColor != 0xFF00FF || e.hyperLinkColor != 0xFF0000 {
		t.Fatalf("default hyperref colours wrong: url=%06X link=%06X", e.hyperURLColor, e.hyperLinkColor)
	}
	e.applyHypersetup("colorlinks")
	if !e.hyperColorlinks {
		t.Errorf("bare colorlinks did not enable colouring")
	}
	e.applyHypersetup("colorlinks=false")
	if e.hyperColorlinks {
		t.Errorf("colorlinks=false did not disable colouring")
	}
}

// A braced value with an xcolor mix is parsed whole (the comma inside the braces
// does not split the segment) and resolved through the colour model.
func TestHypersetupBracedMixValue(t *testing.T) {
	e := New()
	e.applyHypersetup(`colorlinks, urlcolor={red!0!blue}`)
	if e.hyperURLColor != 0x0000FF { // red!0!blue = pure blue
		t.Errorf("braced mix value urlcolor={red!0!blue} = %06X, want 0000FF", e.hyperURLColor)
	}
}

// cutKeyVal reports has=false for a bare key and strips one brace wrapper.
func TestCutKeyValForms(t *testing.T) {
	if k, _, has := cutKeyVal("colorlinks"); k != "colorlinks" || has {
		t.Errorf("bare key: got (%q,%v), want (colorlinks,false)", k, has)
	}
	if k, v, has := cutKeyVal("urlcolor={blue}"); k != "urlcolor" || v != "blue" || !has {
		t.Errorf("braced value: got (%q,%q,%v), want (urlcolor,blue,true)", k, v, has)
	}
	if _, v, _ := cutKeyVal("k = {a=b}"); v != "a=b" {
		t.Errorf("brace-level '=' should not split: got %q, want a=b", v)
	}
}

// boolOpt reads the off-states as false and everything else as true.
func TestBoolOptStates(t *testing.T) {
	for _, off := range []string{"false", "OFF", "no", "0"} {
		if boolOpt(off, true) {
			t.Errorf("boolOpt(%q) = true, want false", off)
		}
	}
	if !boolOpt("true", true) || !boolOpt("", false) {
		t.Errorf("boolOpt should read true/bare as true")
	}
}
