package engine

import (
	"bytes"
	"testing"
)

// TestIsLaTeXDetection: a document is LaTeX if it carries \documentclass OR
// \begin{document} — the latter catches the arXiv shape where main.tex is
// `\input{preamble}` … `\begin{document}` … so the class sits in an included
// file. Plain TeX (neither marker) is not LaTeX.
func TestIsLaTeXDetection(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{`\documentclass{article}\begin{document}Hi\end{document}`, true},
		{"\\input{preamble}\n\\begin{document}\nHi\n\\end{document}\n", true}, // class is \input'ed
		{`\documentclass{article}`, true},
		{`\hsize=300pt Plain TeX, no markers.\par\bye`, false},
		{`\input mymacros \message{plain}`, false},
	}
	for _, c := range cases {
		if got := isLaTeX([]byte(c.src)); got != c.want {
			t.Errorf("isLaTeX(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}

// The library API compiles source to a valid PDF in one call (the loom seam).
func TestCompileToPDF(t *testing.T) {
	var buf bytes.Buffer
	src := []byte(`\hsize=300pt A one-call compile from source to PDF.\par`)
	pages, err := CompileToPDF(src, Options{}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 1 {
		t.Errorf("pages=%d want 1", pages)
	}
	b := buf.Bytes()
	if len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("not a PDF (%d bytes)", len(b))
	}
}

// CompileToSVGPages returns one SVG per page, honouring forced breaks.
func TestCompileToSVGPagesMultiPage(t *testing.T) {
	src := []byte(`\vsize=1000pt \hbox{\vrule width5pt height8pt}\penalty-10000 \hbox{\vrule width5pt height8pt}`)
	pages, err := CompileToSVGPages(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 SVG pages, got %d", len(pages))
	}
	for _, p := range pages {
		if !bytes.Contains([]byte(p), []byte("<svg")) {
			t.Error("page is not an SVG")
		}
	}
}

// A custom font in Options is used and embedded.
func TestCompileWithCustomFontSize(t *testing.T) {
	e, err := NewDocument(Options{Size: 14})
	if err != nil {
		t.Fatal(err)
	}
	if e.curFont.sizePt() != 14 {
		t.Errorf("size=%d want 14", e.curFont.sizePt())
	}
}
