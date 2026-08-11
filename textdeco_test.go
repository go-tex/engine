package engine

import (
	"strings"
	"testing"
)

// findDeco returns the first decoNode reachable in a node tree.
func findDeco(nodes []node) (decoNode, bool) {
	for _, n := range nodes {
		switch v := n.(type) {
		case decoNode:
			return v, true
		case *boxNode:
			if d, ok := findDeco(v.list); ok {
				return d, true
			}
		}
	}
	return decoNode{}, false
}

// \underline places a decoNode of kind 'u' whose depth reserves room for the rule
// below the baseline, and whose content is preserved.
func TestUnderline(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\underline{abc}`); err != nil {
		t.Fatal(err)
	}
	d, ok := findDeco(e.mvl)
	if !ok {
		t.Fatal("no decoNode placed for \\underline")
	}
	if d.kind != 'u' {
		t.Errorf("kind = %q, want 'u'", d.kind)
	}
	// depth must be at least the gap+rule so the underline is not clipped.
	if d.depth() < decoGap+decoRule {
		t.Errorf("underline depth = %d, want >= %d", d.depth(), decoGap+decoRule)
	}
	if d.width() != d.inner.width || d.inner.width == 0 {
		t.Errorf("width = %d, inner = %d", d.width(), d.inner.width)
	}
	// The rule sits below the baseline (positive y-down offset).
	if d.decoRuleTop() != decoGap {
		t.Errorf("underline rule top = %d, want %d", d.decoRuleTop(), decoGap)
	}
}

// \sout strikes through the text: kind 's', rule above the baseline, dimensions
// unchanged from the content.
func TestStrikeout(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\sout{abc}`); err != nil {
		t.Fatal(err)
	}
	d, ok := findDeco(e.mvl)
	if !ok {
		t.Fatal("no decoNode for \\sout")
	}
	if d.kind != 's' {
		t.Errorf("kind = %q, want 's'", d.kind)
	}
	if d.height() != d.inner.height || d.depth() != d.inner.depth {
		t.Errorf("strike must not change dims: got h=%d d=%d, inner h=%d d=%d",
			d.height(), d.depth(), d.inner.height, d.inner.depth)
	}
	if d.decoRuleTop() >= 0 {
		t.Errorf("strike rule should be above baseline (negative), got %d", d.decoRuleTop())
	}
}

// \textoverline adds a rule above the content, extending the height by gap+rule.
func TestOverline(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\textoverline{abc}`); err != nil {
		t.Fatal(err)
	}
	d, ok := findDeco(e.mvl)
	if !ok {
		t.Fatal("no decoNode for \\textoverline")
	}
	if d.kind != 'o' {
		t.Errorf("kind = %q, want 'o'", d.kind)
	}
	if d.height() != d.inner.height+decoGap+decoRule {
		t.Errorf("overline height = %d, want %d", d.height(), d.inner.height+decoGap+decoRule)
	}
}

// The decoration inherits the current colour, so \textcolor around it colours the rule.
func TestDecoColour(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\textcolor{red}{\underline{x}}`); err != nil {
		t.Fatal(err)
	}
	d, ok := findDeco(e.mvl)
	if !ok {
		t.Fatal("no decoNode")
	}
	if d.color == 0 {
		t.Errorf("underline inside \\textcolor{red} should carry a colour, got 0")
	}
}

// The SVG driver emits a rule <rect> for the decoration.
func TestDecoRendersRule(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\underline{abc}`); err != nil {
		t.Fatal(err)
	}
	pages := e.RenderPages(72)
	if len(pages) == 0 || !strings.Contains(pages[0], "<rect") {
		t.Errorf("underline should emit a <rect> rule in SVG")
	}
}
