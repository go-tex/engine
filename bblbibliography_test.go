// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LaTeX's own \bibliography does not read the .bib: BibTeX has already turned it
// into \jobname.bbl, a ready \begin{thebibliography} block, and \bibliography
// inputs THAT. An arXiv submission always carries its .bbl — arXiv runs no BibTeX —
// and 165 of the 200 papers in the reference corpus ship one against 131 with a .bib.
func TestBibliographyReadsTheJobBBL(t *testing.T) {
	dir := t.TempDir()
	bbl := `\begin{thebibliography}{9}
\bibitem{knuth} D.~Knuth, The TeXbook, 1984.
\end{thebibliography}`
	if err := os.WriteFile(filepath.Join(dir, "papier.bbl"), []byte(bbl), 0o644); err != nil {
		t.Fatal(err)
	}
	// A .bib beside it must NOT win: the .bbl is the formatted truth.
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(
		"@book{autre, author={Personne}, title={Jamais Cité}, year={2000}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\cite{knuth}\bibliography{refs}\end{document}`),
		Options{Lenient: true, JobName: "papier"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "TeXbook") {
		t.Errorf("la page porte %q — la bibliographie du .bbl manque", got)
	}
	if strings.Contains(got, "Jamais") {
		t.Errorf("la page porte le .bib alors qu'un .bbl existe: %q", got)
	}
}

// With no .bbl the .bib path still answers, and a job with no name keeps TeX's own
// default so nothing reads a file called ".bbl".
func TestBibliographyFallsBackToTheBib(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(
		"@book{knuth, author={D. Knuth}, title={The TeXbook}, year={1984}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\cite{knuth}\bibliography{refs}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := pageChars(e); !strings.Contains(got, "TeXbook") {
		t.Errorf("la page porte %q — le repli sur le .bib ne marche plus", got)
	}
}
