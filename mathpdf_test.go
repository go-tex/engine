package engine

import (
	"bytes"
	"testing"
)

// A document with inline math produces a PDF whose content stream contains path
// fills (the math drawn as vector paths) — i.e. math is in the PDF, not skipped.
func TestMathInPDF(t *testing.T) {
	var buf bytes.Buffer
	src := []byte(`\hsize=300pt A fraction $\frac{1}{2}$ and a root $\sqrt{2}$ inline.\par`)
	if _, err := CompileToPDF(src, Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	// PDF content streams are usually deflated; compare against a no-math baseline
	// by size — the math version must be substantially larger from the vector ops.
	var plain bytes.Buffer
	CompileToPDF([]byte(`\hsize=300pt A fraction and a root inline.\par`), Options{}, &plain)
	if buf.Len() <= plain.Len() {
		t.Errorf("math PDF (%d) not larger than plain PDF (%d) — math likely not drawn", buf.Len(), plain.Len())
	}
}

// The SVG-path scanner reads commands and numbers, including negatives without
// separators (as go-tex/math emits, e.g. "M6.3 -5.96L6.24-5.96").
func TestPathTokenizer(t *testing.T) {
	pt := newPathTokens("M6.3 -5.96L6.24-5.96Z")
	if pt.cmd() != 'M' || pt.num() != 6.3 || pt.num() != -5.96 {
		t.Fatal("M parse")
	}
	if pt.cmd() != 'L' || pt.num() != 6.24 || pt.num() != -5.96 {
		t.Fatal("L parse (glued negative)")
	}
	if pt.cmd()|0x20 != 'z' {
		t.Fatal("Z parse")
	}
}
