package engine

import (
	"strings"
	"testing"
)

func TestBuildPages(t *testing.T) {
	// three paragraphs of height 3 each; vsize 8 → 2 per page (3+parskip1+3=7<=8), so 2 pages
	mk := func() VBox { return VBox{W: 10, H: 2, D: 1} }
	pages := BuildPages([]VBox{mk(), mk(), mk()}, 8, 1)
	if len(pages) != 2 {
		t.Errorf("pages=%d want 2", len(pages))
	}
}
func TestRenderStructure(t *testing.T) {
	p, _ := New().Typeset(`hello world`, mockFont{}, 100, 10, 10, 1.2)
	svg := RenderPageSVG(p.Box, nil, 200, 100, 5, 5)
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "</svg>") {
		t.Error("bad svg")
	}
}
