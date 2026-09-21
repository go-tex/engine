// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \algrenewcommand\algorithmicwhile{\textbf{While}} (algorithmicx.sty:622) names a
// pseudocode keyword. Undefined, the command is skipped and its braced argument is
// ORDINARY MATERIAL — so the keyword lands on the page. 2406.02421 opened on a page
// carrying nothing but "While For Do If Then Else End Return", 38 characters, and ran
// a page long against its reference (#293).
func TestAlgRenewCommandDoesNotTypesetItsArgument(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{algpseudocode}`+
		`\algrenewcommand\algorithmicwhile{\textbf{While}}`+
		`\algrenewcommand\algorithmicfor{\textbf{For}}`+
		`\begin{document}CORPS\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	for _, leaked := range []string{"While", "For"} {
		if strings.Contains(got, leaked) {
			t.Errorf("the keyword %q reached the page: %q", leaked, got)
		}
	}
	if !strings.Contains(got, "CORPS") {
		t.Errorf("the body is missing: %q", got)
	}
}

// The definition is KEPT, not gobbled: a document that writes the keyword itself gets
// what it asked for.
func TestAlgRenewCommandKeepsTheDefinition(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{algpseudocode}`+
		`\algrenewcommand\algorithmicwhile{TANTQUE}`+
		`\begin{document}A\algorithmicwhile B\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "TANTQUE") {
		t.Errorf("page = %q, want the keyword the document defined", got)
	}
}

// algorithmicx's own \algnewcommand\algorithmiccomment[1]{…} uses the argument form.
func TestAlgNewCommandTakesAnArgumentCount(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{algpseudocode}`+
		`\algnewcommand\algmark[1]{[#1]}`+
		`\begin{document}A\algmark{Z}B\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "[Z]") {
		t.Errorf("page = %q, want the one-argument macro to work", got)
	}
}
