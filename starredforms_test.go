// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A command with a starred variant reads the * itself. One that does not sees it
// as ordinary input: the star is typeset, the arguments after it are no longer
// where the reader expects them, and the whole lot ends up as text
// (go-tex/engine#636, a census of 27 such commands).
//
// ⛔ The assertion is SYMMETRY — the starred form must put no MORE on the page
// than the plain one, whatever each command does with its star. A rule like "no *
// appears" would miss a command that leaks only its braces, and would fire on one
// that legitimately prints a star.
func TestAStarredFormAddsNothingToThePage(t *testing.T) {
	for _, c := range []struct{ name, plain, star string }{
		{`\ref`, `\ref{gotexsec}`, `\ref*{gotexsec}`},
		{`\pageref`, `\pageref{gotexsec}`, `\pageref*{gotexsec}`},
		{`\citet`, `\citet{gotexkey}`, `\citet*{gotexkey}`},
		{`\citep`, `\citep{gotexkey}`, `\citep*{gotexkey}`},
		{`\citeauthor`, `\citeauthor{gotexkey}`, `\citeauthor*{gotexkey}`},
	} {
		plain := pageOf(t, `\section{S}\label{gotexsec}`+c.plain)
		star := pageOf(t, `\section{S}\label{gotexsec}`+c.star)
		if star != plain {
			t.Errorf("%s: la forme étoilée donne %q, la forme simple %q", c.name, star, plain)
		}
	}
}

// \caption* is a caption WITHOUT a number — caption.sty: "the caption without
// label and without entry in the list of figures/tables". Reading a mandatory
// argument straight away bound it to the star, so the page read "Figure 1: *" and
// the caption's own text fell into the body as prose. 382 papers write \caption*.
func TestCaptionStarHasNoNumberAndKeepsItsText(t *testing.T) {
	// ⚠ Not inside \begin{figure}: a float's content is deferred and placed by the
	// page builder, so it is not in the main vertical list this probe walks — both
	// the starred AND the unstarred case came back empty, which says nothing about
	// either. \@captype is what \caption dispatches on, so setting it directly
	// exercises the same two branches in the body.
	const captype = `\makeatletter\def\@captype{figure}\makeatother`
	got := pageOf(t, captype+`\caption*{GOTEXCAPTION}`)
	if !strings.Contains(got, "GOTEXCAPTION") {
		t.Errorf("le texte de la légende est absent: %q", got)
	}
	if strings.Contains(got, "Figure1") || strings.Contains(got, "Figure 1") {
		t.Errorf("la légende étoilée porte un numéro: %q", got)
	}
	if strings.Contains(got, "*") {
		t.Errorf("l'étoile est composée: %q", got)
	}
	// And the unstarred one still numbers, or the fix has simply disabled \caption.
	num := pageOf(t, captype+`\caption{GOTEXCAPTION}`)
	if !strings.Contains(num, "Figure1") {
		t.Errorf("la légende NON étoilée a perdu son numéro: %q", num)
	}
}

// natbib's citation commands "may take one or two optional arguments to add some
// text before and after the citation" (natbib.pdf, Basic commands). \citep went
// through \cite and already read them; \citet and \citeauthor did not, so
// readBraceName met "[" instead of "{" and the note AND the key list were typeset.
// 1870 papers of the arXiv pool write \citet[ or \citep[.
func TestNatbibCitationsTakeTheirOptionalNotes(t *testing.T) {
	const bib = `\begin{thebibliography}{9}\bibitem{gotexkey}Someone.\end{thebibliography}`
	for _, c := range []struct{ name, src string }{
		{`\citet`, `\citet[chap. 2]{gotexkey}`},
		{`\citet` + " deux notes", `\citet[see][chap. 2]{gotexkey}`},
		{`\citeauthor`, `\citeauthor[chap. 2]{gotexkey}`},
	} {
		got := pageOf(t, c.src+bib)
		if strings.Contains(got, "gotexkey") {
			t.Errorf("%s: la clé a fui sur la page: %q", c.name, got)
		}
		if strings.Contains(got, "][") {
			t.Errorf("%s: les crochets ont fui sur la page: %q", c.name, got)
		}
	}
}

// pageOf returns the characters a document put on the page.
func pageOf(t *testing.T, body string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile %q: %v", body, err)
	}
	return mvlText(e.mvl)
}
