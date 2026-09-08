// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// A text-font package is not a skipped command — its macros are all defined — so
// nothing else in Diagnostics reveals that the document is set in a face it did not
// ask for. Width is what decides how many words fit on a line, hence how long the
// document is: measured on a controlled two-column acmart sigconf document (same
// text, same 9pt, same block, same leading, 57 lines per column in BOTH engines),
// our characters came out 4.619bp against tectonic's Linux Libertine at 3.990 —
// 15.8% wider, 8.77 words to a line instead of 10.04, and two extra pages out of
// eight (go-tex/engine#310). 39 of the 157 corpus papers ask for one.
func TestDiagnosticsReportsASubstitutedTextFont(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want map[string]string
	}{
		{`\documentclass{article}\usepackage{libertine}\begin{document}x\end{document}`,
			map[string]string{"libertine": "Linux Libertine"}},
		{`\documentclass{article}\usepackage{times}\usepackage{helvet}\begin{document}x\end{document}`,
			map[string]string{"times": "Times", "helvet": "Helvetica"}},
		// A class asking through \RequirePackage counts the same — that is how acmart
		// asks for Libertine — and one list naming several packages counts each.
		{`\documentclass{article}\usepackage{mathptmx,amssymb}\begin{document}x\end{document}`,
			map[string]string{"mathptmx": "Times"}},
		// Maths-only and encoding packages are not text faces.
		{`\documentclass{article}\usepackage{amsmath}\usepackage[T1]{fontenc}\begin{document}x\end{document}`,
			map[string]string{}},
	} {
		e, err := compile([]byte(tc.src), Options{Lenient: true})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		got := e.Diagnostics().FontsSubstituted
		if len(got) != len(tc.want) {
			t.Errorf("%q: got %v, want %v", tc.src, got, tc.want)
			continue
		}
		for pkg, face := range tc.want {
			if got[pkg] != face {
				t.Errorf("%q: %s -> %q, want %q", tc.src, pkg, got[pkg], face)
			}
		}
	}
}

// A font package the engine can honour swaps the text face for the real one, and is
// then NOT reported as a substitution. WIDTH is the point: measured on this same
// document, the built-in face sets 4.746bp per character where Linux Libertine sets
// 4.584 — and tectonic, asked the same question, answers 4.582 (go-tex/engine#310).
//
// Only packages the REFERENCE itself honours are mapped. Asked what it embeds,
// tectonic gives Latin Modern for times, mathptmx, txfonts and helvet — the old
// PSNFSS packages set a Type1 family its XeTeX path does not resolve — and the real
// face only for newtxtext, libertine and libertinus. A test pins that too.
func TestATextFontPackageIsLoadedWhenAvailable(t *testing.T) {
	tree := os.Getenv("GOTEX_TEXMF")
	if tree == "" {
		tree = "/Users/Shared/gotex/measure/texmf"
	}
	if _, err := os.Stat(filepath.Join(tree, "LinLibertine_R.otf")); err != nil {
		t.Skip("no LinLibertine_R.otf under GOTEX_TEXMF: nothing to load")
	}
	t.Setenv("GOTEX_TEXMF", tree)

	const body = `\begin{document}Hello world.\end{document}`
	loaded, err := compile([]byte(`\documentclass{article}\usepackage{libertine}`+body), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := loaded.Diagnostics().FontsSubstituted; len(got) != 0 {
		t.Errorf("libertine was loaded, so it must not be reported as substituted: %v", got)
	}

	// A package the reference does not honour either is left alone — and still
	// reported, because the document did ask for it.
	psnfss, err := compile([]byte(`\documentclass{article}\usepackage{times}`+body), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := psnfss.Diagnostics().FontsSubstituted; got["times"] != "Times" {
		t.Errorf("times must still be reported as substituted, got %v", got)
	}
}
