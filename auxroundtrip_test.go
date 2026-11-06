// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ THE .aux IS THE CHANNEL A REAL PACKAGE USES TO TALK TO ITSELF.
//
// \@auxout was \def\@auxout{-1}, a number no \openout ever opens, so everything a
// package wrote to the .aux went nowhere and nothing was ever read back. This engine
// resolves its OWN \label/\ref in memory and reruns until the numbers stop moving
// (api.go), which is sound and stays — but a package has no way into that.
//
// Measured on a witness with the real natbib.sty beside the document:
//
//	\citep{alpha}   no natbib.sty   "[1]"                      (numeric emulation)
//	                real natbib.sty "(?)"                      (unresolved)
//	                this branch     "(Author et al., 2021)"
//	tectonic                        "(Author et al., 2021)"
//
// Loading the real package made the answer WORSE until the channel existed: natbib
// writes \bibcite{key}{⟨label⟩} from \bibitem's optional argument and reads it back
// next pass to resolve \citep. The package was never the missing half (#711).
func TestTheAuxFileRoundTrips(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	// \bibcite is \@newl@bel b (latex.ltx:13281), so it defines \b@<key>.
	if _, err := e.Run(`\makeatletter\bibcite{alpha}{Smith et al.(2020)}` +
		`\message{[\csname b@alpha\endcsname]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[Smith et al.(2020)]" {
		t.Errorf("got %q want %q — \\bibcite must land where \\citep looks for it", got,
			"[Smith et al.(2020)]")
	}
}

// ⛔ AND THE STREAM IS REAL. \@auxout at -1 is not merely unopened: it is a number
// \openout never takes, so the defect survived any amount of \openout elsewhere.
func TestAuxoutIsAnAllocatedStream(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\makeatletter\message{[\meaning\@auxout]}`); err != nil {
		t.Fatal(err)
	}
	got := trimNL(e.out.String())
	if strings.Contains(got, "-1") || !strings.Contains(got, `\count`) {
		t.Errorf("\\@auxout is %s, want an allocated \\count: a stream nothing can "+
			"open is a channel nothing can use", got)
	}
}

// ⛔⛔ AND EVERY NAME THE FILE CAN CARRY IS DEFINED BEFORE IT IS READ. An .aux holds
// more than answers — \citation{key}, \bibdata{f}, \bibstyle{s}, \@writefile{toc}{…}
// are notes for BibTeX and for the contents file, and LaTeX gobbles all of them on
// the way back in (latex.ltx:13282, :13296-13297).
//
// Undefined, each one met its argument and lenient mode set that argument AS TEXT:
// the witness's first line read "alpha alpha Texte (Author et al., 2021)…" — the
// citation key, twice, in the body. Opening an .aux round-trip without these is
// strictly worse than leaving it closed.
func TestTheAuxBookkeepingNamesAreGobbled(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`\makeatletter\citation{ZZKEYZZ}\bibdata{ZZDATAZZ}\bibstyle{ZZSTYLEZZ}` +
		`\@writefile{toc}{ZZTOCZZ}\makeatother Texte.\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(pages, "")
	for _, leak := range []string{"ZZKEYZZ", "ZZDATAZZ", "ZZSTYLEZZ", "ZZTOCZZ"} {
		if strings.Contains(all, leak) {
			t.Errorf("%q est sur la page: une écriture de comptabilité .aux a laissé "+
				"son argument en texte", leak)
		}
	}
}
