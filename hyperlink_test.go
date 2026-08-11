package engine

import "testing"

// firstLink returns the first linkNode reachable in a node tree, descending through
// boxes and into a link's own inner box (a link may be wrapped in a line hbox).
func firstLink(nodes []node) (linkNode, bool) {
	for _, n := range nodes {
		switch c := n.(type) {
		case linkNode:
			return c, true
		case *boxNode:
			if ln, ok := firstLink(c.list); ok {
				return ln, true
			}
		}
	}
	return linkNode{}, false
}

// glyphString walks a node tree and concatenates every charNode rune in order,
// descending through boxes, frames and links.
func glyphString(nodes []node) string {
	var s []rune
	for _, n := range nodes {
		switch c := n.(type) {
		case charNode:
			s = append(s, c.ch)
		case *boxNode:
			s = append(s, []rune(glyphString(c.list))...)
		case frameNode:
			s = append(s, []rune(glyphString(c.inner.list))...)
		case linkNode:
			s = append(s, []rune(glyphString(c.inner.list))...)
		}
	}
	return string(s)
}

// \href{url}{text} composes text normally and wraps it in a link to url: the
// resulting linkNode carries the url and its inner box holds the text glyphs.
func TestHref(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\href{http://x}{Ab}`); err != nil {
		t.Fatal(err)
	}
	ln, ok := firstLink(e.mvl)
	if !ok {
		t.Fatal("no linkNode placed")
	}
	if ln.url != "http://x" {
		t.Errorf("url = %q, want %q", ln.url, "http://x")
	}
	if got := glyphString([]node{ln}); got != "Ab" {
		t.Errorf("link text = %q, want %q", got, "Ab")
	}
}

// \url{url} typesets the url literally and links it to itself.
func TestURL(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\url{http://y}`); err != nil {
		t.Fatal(err)
	}
	ln, ok := firstLink(e.mvl)
	if !ok {
		t.Fatal("no linkNode placed")
	}
	if ln.url != "http://y" {
		t.Errorf("url = %q, want %q", ln.url, "http://y")
	}
	if got := glyphString([]node{ln}); got != "http://y" {
		t.Errorf("url text = %q, want %q", got, "http://y")
	}
}

// \nolinkurl{url} typesets the url literally but produces NO link.
func TestNolinkurl(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\nolinkurl{zurl}`); err != nil {
		t.Fatal(err)
	}
	if _, ok := firstLink(e.mvl); ok {
		t.Error("\\nolinkurl must not place a linkNode")
	}
	if got := glyphString(e.mvl); got != "zurl" {
		t.Errorf("nolinkurl text = %q, want %q", got, "zurl")
	}
}

// \url reads its argument literally, so catcode-active URL characters (~ % # _ &)
// pass through unchanged and reach the linkNode verbatim without a parse error.
func TestURLSpecialChars(t *testing.T) {
	const url = "http://a.b/c?d=1&e_f~g#h"
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\url{` + url + `}`); err != nil {
		t.Fatalf("special-char url must not fail: %v", err)
	}
	ln, ok := firstLink(e.mvl)
	if !ok {
		t.Fatal("no linkNode placed")
	}
	if ln.url != url {
		t.Errorf("url = %q, want %q", ln.url, url)
	}
	if got := glyphString([]node{ln}); got != url {
		t.Errorf("url text = %q, want %q", got, url)
	}
}

// readRawBracedArg counts nested braces and reports failure when no '{' follows.
func TestReadRawBracedArg(t *testing.T) {
	e := New()
	e.base = []rune(`  {a{b}c}tail`)
	e.bpos = 0
	got, ok := e.readRawBracedArg()
	if !ok || got != "a{b}c" {
		t.Errorf("nested = (%q,%v), want (%q,true)", got, ok, "a{b}c")
	}
	if rest := string(e.base[e.bpos:]); rest != "tail" {
		t.Errorf("cursor left at %q, want %q", rest, "tail")
	}

	e2 := New()
	e2.base = []rune(`no brace`)
	e2.bpos = 0
	if got, ok := e2.readRawBracedArg(); ok || got != "" {
		t.Errorf("missing brace = (%q,%v), want (\"\",false)", got, ok)
	}
	if e2.bpos != 0 {
		t.Errorf("cursor moved to %d on missing brace, want 0", e2.bpos)
	}
}

// escapeXMLAttr neutralises the characters that could break out of an href="…".
func TestEscapeXMLAttr(t *testing.T) {
	got := escapeXMLAttr(`a&b<c>d"e'f`)
	want := `a&amp;b&lt;c&gt;d&quot;e&#39;f`
	if got != want {
		t.Errorf("escapeXMLAttr = %q, want %q", got, want)
	}
}

// The SVG driver wraps a link's content in an <a href="…"> element, and escapes
// an ampersand in the URL so the attribute stays well-formed.
func TestLinkSVG(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\url{http://x/a&b}`); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPage(2)
	if !contains(svg, `<a href="http://x/a&amp;b"`) {
		t.Errorf("SVG missing escaped <a href>: %s", svg)
	}
	if !contains(svg, `</a>`) {
		t.Errorf("SVG missing </a> close: %s", svg)
	}
}
