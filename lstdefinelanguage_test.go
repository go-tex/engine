// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \lstdefinelanguage was undefined, so everything it was given became ordinary
// material and the whole definition was TYPESET — the language name and the entire
// key=value body, where the reference sets nothing. 13 corpus papers define one.
//
// Every documented shape is covered, not just the one the corpus happens to use:
// a gobbler of the wrong ARITY fails silently, which is worse than being undefined,
// since an undefined command at least appears in -report-skipped. That is what a
// fixed \def\algnewcommand#1#2{} did to \algnewcommand\algorithmiccomment[1]{…}
// until #378.
func TestLstDefineLanguageSwallowsEveryForm(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"plain", `\lstdefinelanguage{L}{morekeywords={FUITE},sensitive=false}`},
		{"dialect", `\lstdefinelanguage[d]{L}{morekeywords={FUITE}}`},
		{"base language", `\lstdefinelanguage{L}[b]{Base}{morekeywords={FUITE}}`},
		{"trailing key list", `\lstdefinelanguage{L}{morekeywords={FUITE}}[keywords]`},
		{"style", `\lstdefinestyle{S}{basicstyle=\ttfamily,FUITE}`},
	} {
		e, err := compile([]byte(`\documentclass{article}\usepackage{listings}`+c.src+
			`\begin{document}DEBUT\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		got := pageChars(e)
		if strings.Contains(got, "FUITE") {
			t.Errorf("%s: the definition reached the page: %q", c.name, got)
		}
		if !strings.Contains(got, "DEBUT") {
			t.Errorf("%s: the body is missing: %q", c.name, got)
		}
	}
}
