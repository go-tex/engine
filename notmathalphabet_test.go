// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The standard NFSS pattern redefines a font switch with a reference to itself as
// \not@math@alphabet's first argument:
//
//	\DeclareRobustCommand*{\bfseries}{\not@math@alphabet\bfseries\mathbf …}
//
// \not@math@alphabet must consume both arguments (in text mode it does nothing) so
// that self-reference is swallowed, not executed. Undefined, it was skipped and the
// redefined \bfseries re-executed on every \textbf, recursing forever and swallowing
// the document (a real article's incl_settings.tex does exactly this redefinition).
func TestBfseriesRedefinitionDoesNotRecurse(t *testing.T) {
	src := `\documentclass{article}` +
		`\makeatletter\DeclareRobustCommand*{\bfseries}{\not@math@alphabet\bfseries\mathbf\fontseries\bfdefault\selectfont\boldmath}\makeatother` +
		`\begin{document}\textbf{BOLD} BODYMARKER text after.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.runaway {
		t.Error("expansion ran away: \\not@math@alphabet did not absorb the self-referential \\bfseries")
	}
	if got := pageChars(e); !strings.Contains(got, "BODYMARKER") {
		t.Errorf("body swallowed by \\bfseries recursion; want BODYMARKER, got %q", got)
	}
}
