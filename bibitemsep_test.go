// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"sort"
	"strconv"
	"testing"
)

// interItemSpace is the inter-item glue a list shows on the page: the largest gap
// between consecutive body baselines, minus the smallest.
//
// ⛔ It is computed from the GAPS and not from fixed indices, because the two
// environments this test compares do not have the same shape — thebibliography opens
// with a \section*{\refname} heading and itemize does not, so ys[1]-ys[0] means a
// different thing in each. Indexing by hand read the heading as a body line and
// reported -20.88pt. Gaps wider than 50pt are the drop to the page number and are
// not between two body lines at all.
func interItemSpace(t *testing.T, svg string) float64 {
	t.Helper()
	ys := bibBaselines(t, svg)
	var gaps []float64
	for i := 1; i < len(ys); i++ {
		if g := ys[i] - ys[i-1]; g <= 50 { // >50 is the drop to the page number
			gaps = append(gaps, g)
		}
	}
	if len(gaps) < 2 {
		t.Fatalf("%d écart(s) entre lignes de corps, want au moins 2: %v", len(gaps), ys)
	}
	sort.Float64s(gaps)
	// ⛔ THE SMALLEST GAP THAT IS NOT THE BASELINESKIP, not the largest. The largest
	// is the \section*{\refname} heading's afterskip plus \topsep — 20.88pt above the
	// baselineskip here — and reading it as the inter-item space is what the first
	// version of this test did.
	lo := gaps[0]
	for _, g := range gaps[1:] {
		if g > lo+0.5 {
			return g - lo
		}
	}
	t.Fatalf("tous les écarts valent %.2fpt: aucune espace inter-item sur la page: %v", lo, ys)
	return 0
}

func bibBaselines(t *testing.T, svg string) []float64 {
	t.Helper()
	re := regexp.MustCompile(`<tspan[^>]*\sy="([0-9.]+)"`)
	seen := map[float64]bool{}
	var ys []float64
	for _, m := range re.FindAllStringSubmatch(svg, -1) {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil || seen[v] {
			continue
		}
		seen[v] = true
		ys = append(ys, v)
	}
	sort.Float64s(ys)
	return ys
}

// ⛔⛔ A BIBLIOGRAPHY ENTRY IS AN \item, SO IT CARRIES \itemsep + \parsep.
//
// article.cls:565-583 builds thebibliography on \list, and the kernel's \@lbibitem
// and \@bibitem open each entry with \item (latex.ltx:13275, 13279). So every entry
// after the first gets what \@item adds (latex.ltx:11567) — \itemsep, plus \parsep,
// because \parskip is \parsep inside a list. Here the entries were separated by a
// bare \par and nothing else.
//
// The registers were already right, which is what made this one call the whole
// defect: \itemsep, \parsep, \topsep, \labelsep and \labelwidth read IDENTICAL to
// the reference's from inside both engines — 4pt plus 2 minus 1, 4pt plus 2 minus 1,
// 8pt plus 2 minus 4, 5pt, 20pt — and \@iteminterspace had been computing the sum
// for \itemize and \enumerate all along.
//
// Measured against tectonic on two entries, last baseline of one to first of the
// next: the reference puts 19.93pt, this engine put 11.96 — exactly one
// \baselineskip. 7.97pt per entry: a 40-entry bibliography was more than half a page
// short, a 100-entry one a page and a half.
//
// ⭐ THE ASSERTION IS FONT-INDEPENDENT. It is the gap ACROSS the entry boundary minus
// the gap WITHIN an entry, which is \baselineskip either way, so what is left is the
// inter-item space alone: \itemsep + \parsep = 4pt + 4pt.
func TestABibliographyEntryCarriesItemsepAndParsep(t *testing.T) {
	// ⛔ A FULL DOCUMENT, and that is not ceremony: \itemsep and \parsep get their
	// values from \@listI in size10.clo, which only runs when a class is loaded. On a
	// bare source they are 0 and every line sits one \baselineskip from the next —
	// the test would pass for an engine that never adds the space at all.
	const src = `\documentclass{article}\begin{document}` +
		`\begin{thebibliography}{9}` +
		`\bibitem{a} Une entree assez longue pour depasser la largeur de la colonne et forcer une coupure de ligne quelque part dans ce texte.` +
		`\bibitem{b} Une autre entree tout aussi longue qui doit elle aussi se couper au moins une fois pour montrer la mesure employee.` +
		`\end{thebibliography}\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := interItemSpace(t, pages[0]); got < 7.9 || got > 8.1 {
		t.Errorf("entre deux entrées: %.2fpt de plus qu'entre deux lignes d'une "+
			"même entrée, want 8.00 (\\itemsep 4pt + \\parsep 4pt, size10.clo:217-219) — "+
			"un \\par seul n'en met aucun", got)
	}
}

// ⛔ THE CONTROL. \itemize already went through \@iteminterspace, and it must keep
// giving the same 8pt: if this number came from somewhere else — a \topsep, a
// \parskip — it would not match across two environments that share only the one
// macro.
func TestAnItemizeItemCarriesTheSameInterItemSpace(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`\begin{itemize}` +
		`\item Un element assez long pour depasser la largeur de la colonne et forcer une coupure de ligne quelque part dans ce texte.` +
		`\item Un autre element tout aussi long qui doit lui aussi se couper au moins une fois pour montrer la mesure employee.` +
		`\end{itemize}\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := interItemSpace(t, pages[0]); got < 7.9 || got > 8.1 {
		t.Errorf("itemize: %.2fpt entre deux items, want 8.00 — le témoin de "+
			"\\@iteminterspace", got)
	}
}
