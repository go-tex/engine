// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// figure* and table* are the two-column forms. A class this engine emulates itself
// (revtex, IEEEtran, acmart, elsarticle …) had no starred form at all, so
// \begin{figure*}[t] resolved to \relax and typeset its own placement key: every
// such float carried a stray "[t]" on the page, in 23 of the 200 arXiv papers for
// figure* and 11 for table*. The class here is one of those emulated ones: the real
// article.cls, which the engine embeds, has always defined them through \@dblfloat.
func TestStarredFloatsAreFloats(t *testing.T) {
	for _, c := range []struct{ env, veut string }{
		{"figure*", "Figure1:Légende"},
		{"table*", "Table1:Légende"},
	} {
		e, err := compile([]byte(`\documentclass{revtex4-2}\begin{document}Avant.`+
			`\begin{`+c.env+`}[t]Contenu.\caption{Légende}\end{`+c.env+`}Après.\end{document}`),
			Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.env, err)
		}
		got := pageChars(e)
		if strings.Contains(got, "[t]") {
			t.Errorf("%s: la clé de placement est composée: %q", c.env, got)
		}
		if !strings.Contains(got, c.veut) {
			t.Errorf("%s: la page porte %q, elle doit porter %q", c.env, got, c.veut)
		}
	}
}
