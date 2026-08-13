// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// amsartDoc is a representative amsart paper: title/author/address/abstract, two
// numbered sections, a numbered equation and an align, and a \cite — the shape of a
// typical arXiv amsart submission.
const amsartDoc = `\documentclass{amsart}
\begin{document}
\title{On the Structure of Things}
\author{Ada Lovelace}
\address{Analytical Engine Dept}
\email{ada@example.org}
\begin{abstract}
We study the structure of things and prove a small theorem.
\end{abstract}
\maketitle
\section{Introduction}
This is the introduction. Consider the equation
\begin{equation}
E = mc^2.
\end{equation}
\section{Main result}
We now prove the main result~\cite{key}.
\begin{align}
a &= b + c \\
d &= e + f.
\end{align}
That completes the proof.
\end{document}
`

// The engine loads the REAL amsart.cls (embedded in texmf/) — not the built-in
// emulation — and runs it to completion: the class kernel, its plain-TeX substrate
// (token registers, \newinsert, the mode conditionals), and its title machinery all
// resolve, so the document typesets its title, author, abstract, section headings and
// equations without running away.
func TestAmsartLoadsRealClass(t *testing.T) {
	e, err := compile([]byte(amsartDoc), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if e.runaway {
		t.Fatalf("amsart compile ran away (steps=%d)", e.steps)
	}
	got := pageChars(e)
	for _, w := range []string{
		"OntheStructureofThings", // title
		"AdaLovelace",            // author
		"Abstract",               // abstract heading
		"structureofthings",      // abstract body
		"Introduction",           // section 1 heading
		"Mainresult",             // section 2 heading
		"introduction",           // section 1 body
		"completestheproof",      // section 2 body
		"ada@example.org",        // address block
	} {
		if !strings.Contains(got, w) {
			t.Errorf("amsart output missing %q\n full: %q", w, got)
		}
	}
}

// amsart defines its OWN sectioning (\@startsection→\@sect→\@hangfrom/\@xsect,
// distinct from the engine's built-in \@startsection used by article). Its section
// headings render with both the number and the title — the substrate supplies the
// \@hangfrom / \@xsect / \textup pieces \@sect needs, so the heading is neither
// dropped nor its number gobbled.
func TestAmsartSectionHeadingRenders(t *testing.T) {
	src := `\documentclass{amsart}\begin{document}` +
		`\section{Groundwork}First paragraph.\section{Consequences}Second paragraph.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	for _, w := range []string{"1", "Groundwork", "2", "Consequences", "First", "Second"} {
		if !strings.Contains(got, w) {
			t.Errorf("section heading output missing %q: %q", w, got)
		}
	}
}

// amsart is routed to the real class, i.e. it is NOT on the emulated-class list.
func TestAmsartNotEmulated(t *testing.T) {
	if emulatedClasses["amsart"] {
		t.Fatal("amsart should load the real embedded class, not the emulation")
	}
	if _, ok := embeddedTeXFile("amsart.cls"); !ok {
		t.Fatal("amsart.cls is not embedded in texmf/")
	}
}

// The \maketitle author machinery (amsart's \andify / \nxandlist token-register
// recursion) terminates and typesets multiple authors joined by "and".
func TestAmsartMultipleAuthors(t *testing.T) {
	src := `\documentclass{amsart}\begin{document}` +
		`\title{T}\author{Alice}\author{Bob}\maketitle Body.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.runaway {
		t.Fatal("multi-author \\maketitle ran away")
	}
	got := pageChars(e)
	for _, w := range []string{"Alice", "Bob"} {
		if !strings.Contains(got, w) {
			t.Errorf("missing author %q in %q", w, got)
		}
	}
}
