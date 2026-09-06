package engine

import (
	"bytes"
	"strings"
	"testing"
)

// A formula reaches the PDF's text layer. The glyphs are drawn as vector paths
// (mathpdf.go), which carry no characters, so without this every equation — and
// every \mbox/\text inside one — is missing from a search, a copy or a screen
// reader. It is written in render mode 3, which paints nothing, so the page looks
// exactly the same.
func TestMathReachesThePDFTextLayer(t *testing.T) {
	var buf bytes.Buffer
	if _, err := CompileToPDF([]byte(`\hsize=300pt ONE $x+y$ two.\par`), Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(pdfStreams(t, buf.Bytes()), "\n"), "3 Tr") {
		t.Error("no invisible-text run in the content stream: the formula is not in the text layer")
	}
}

// ⚠ The invisible run is scoped by q/Q, NOT by setting the mode back.
//
// Tr is graphics state and persists across BT/ET, and pdfkit writes it only when
// it is non-zero, so putting the mode back to RenderFill emits nothing at all and
// the page stays in mode 3. Measured before this was q/Q-wrapped: on
// "A $\mbox{RRRRR}$ B" the B simply vanished from the rendered page — 110 pixels
// of ink gone at 150dpi, with none added.
func TestInvisibleMathRunIsScopedByQQ(t *testing.T) {
	var buf bytes.Buffer
	if _, err := CompileToPDF([]byte(`\hsize=300pt A $x+y$ B\par`), Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	s := strings.Join(pdfStreams(t, buf.Bytes()), "\n")
	i := strings.LastIndex(s, "3 Tr")
	if i < 0 {
		t.Fatal("no invisible-text run to check")
	}
	if !strings.Contains(s[i:], "Q") {
		t.Error("render mode 3 is never closed by a Q: everything after the last formula stays invisible")
	}
	if !strings.Contains(s[:i], "q") {
		t.Error("render mode 3 is not opened inside a q: the graphics state it changes is not saved")
	}
}
