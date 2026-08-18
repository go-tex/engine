// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \subfile{file} (subfiles package) typesets a subfile as part of the document.
// The subfile carries its own \documentclass{subfiles}…\begin{document}…
// \end{document} wrapper; \subfile must neutralise it — in particular the
// subfile's \end{document} must NOT end the whole document — so both the subfile
// body and the material after the \subfile render. Undefined, \subfile was skipped
// and the body of a paper split into subfiles was silently dropped.
func TestSubfileTypesetsBodyWithoutEndingDocument(t *testing.T) {
	withTempDir(t, map[string]string{
		"chap.tex": `\documentclass[main]{subfiles}\begin{document}SUBFILEBODY content here.\end{document}`,
	}, func() {
		src := `\documentclass{article}\usepackage{subfiles}\begin{document}` +
			`\subfile{chap}AFTERSUBFILE tail text.\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		got := pageChars(e)
		if !strings.Contains(got, "SUBFILEBODY") {
			t.Errorf("subfile body was not typeset; got %q", got)
		}
		if !strings.Contains(got, "AFTERSUBFILE") {
			t.Errorf("the subfile's \\end{document} ended the whole document; material after \\subfile is missing; got %q", got)
		}
	})
}

// A bare-content subfile (no \documentclass/\begin{document} wrapper — the common
// arXiv shape, one section per file) is simply typeset in place.
func TestSubfileBareContent(t *testing.T) {
	withTempDir(t, map[string]string{
		"sec.tex": `\section{S}BARESUBFILE body words.`,
	}, func() {
		src := `\documentclass{article}\usepackage{subfiles}\begin{document}` +
			`\subfile{sec}AFTERBARE tail.\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		got := pageChars(e)
		if !strings.Contains(got, "BARESUBFILE") || !strings.Contains(got, "AFTERBARE") {
			t.Errorf("bare subfile not typeset correctly; got %q", got)
		}
	})
}
