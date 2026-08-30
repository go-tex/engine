// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A loaded file is spliced as TEXT, so the markers appended after it are read with
// whatever catcodes are in force AT THE END of that file — not the ones the loader
// set. A package that ends with \makeatother, which is how most author-written .sty
// files end, therefore read \@endofpackagehook as \@ followed by the letters
// "endofpackagehook": the engine printed the name of its own end-of-load marker on
// the page, and \@gotex@endload never ran, so the load frame was never popped and @
// stayed a letter for the rest of the document.
func TestPackageEndingWithMakeatotherLeavesNothingBehind(t *testing.T) {
	sty := []byte("\\ProvidesPackage{loc}\n\\makeatletter\n\\def\\@foo{x}\n\\makeatother\n")
	opt := Options{Lenient: true, Resolve: func(name string) ([]byte, bool) {
		if name == "loc.sty" {
			return sty, true
		}
		return nil, false
	}}
	e, err := compile([]byte(`\documentclass{article}\usepackage{loc}`+
		`\begin{document}Bonjour\end{document}`), opt)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := pageChars(e); got != "Bonjour" {
		t.Errorf("la page porte %q, want %q", got, "Bonjour")
	}
	if len(e.loadStack) != 0 {
		t.Errorf("%d trame(s) de chargement encore empilée(s): \\@gotex@endload n'a pas couru",
			len(e.loadStack))
	}
}
