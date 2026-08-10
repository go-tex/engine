package engine

import (
	"bytes"
	"testing"
)

// A small LaTeX document (documentclass + document environment + section)
// compiles to a PDF via the auto-detected LaTeX layer.
func TestLaTeXDocumentCompiles(t *testing.T) {
	src := []byte(`\documentclass[12pt]{article}
\title{A Title}
\author{An Author}
\begin{document}
\maketitle
\section{Introduction}
This is a LaTeX document compiled by the pure-Go engine. It uses the
environment mechanism and sectioning defined in the minimal LaTeX kernel.
\begin{itemize}
\item First point.
\item Second point.
\end{itemize}
\end{document}`)
	var buf bytes.Buffer
	pages, err := CompileToPDF(src, Options{}, &buf)
	if err != nil {
		t.Fatalf("LaTeX compile: %v", err)
	}
	if pages < 1 {
		t.Fatalf("expected >=1 page, got %d", pages)
	}
	if b := buf.Bytes(); string(b[:5]) != "%PDF-" {
		t.Fatalf("not a PDF")
	}
}

// \begin{env}...\end{env} runs \env and \endenv via \csname (the LaTeX mechanism).
func TestBeginEndEnvironment(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	// define a custom environment and check begin/end dispatch to it
	got, err := e.Run(`\def\myenv{[A]}\def\endmyenv{[B]}\message{\begin{myenv}\end{myenv}}`)
	if err != nil {
		t.Fatal(err)
	}
	if trimNL(got) != "[A][B]" {
		t.Errorf("begin/end env got %q want [A][B]", trimNL(got))
	}
}
