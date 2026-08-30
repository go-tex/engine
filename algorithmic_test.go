// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The algorithm float and the algorithmic / algpseudocode pseudocode body render
// rather than being dropped as an undefined environment. Both the UPPERCASE
// (algorithmic) and the MixedCase (algorithmicx / algpseudocode) command spellings
// are recognised, and the block keywords expand to their words.
func TestAlgorithmicRenders(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"uppercase", `\hsize=400pt\begin{algorithm}\caption{C}\begin{algorithmic}[1]
\REQUIRE $n\ge0$
\STATE $x\gets0$
\FOR{$i=1$ to $n$}
\IF{cond}\STATE step\ELSE\STATE other\ENDIF
\ENDFOR
\RETURN $x$
\end{algorithmic}\end{algorithm}`,
			[]string{"Algorithm", "Require:", "if", "then", "else", "endif", "for", "do", "endfor", "return"}},
		{"mixedcase", `\hsize=400pt\begin{algorithmic}
\State init
\While{running}\State tick\EndWhile
\end{algorithmic}`,
			[]string{"while", "do", "endwhile"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := New()
			if err := e.LoadLaTeX(); err != nil {
				t.Fatal(err)
			}
			e.SetFont(spMock{})
			if _, err := e.Run(tc.src); err != nil {
				t.Fatal(err)
			}
			if n := e.undefinedEnvs["algorithmic"]; n != 0 {
				t.Fatalf("algorithmic was treated as undefined (%d) — dropped, not rendered", n)
			}
			var b strings.Builder
			collectChars(e.mvl, &b)
			got := b.String()
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("rendered pseudocode missing %q; got: %s", w, got)
				}
			}
		})
	}
}

// The optional [line-numbering] argument of \begin{algorithmic}[N] is consumed, not
// typeset as a stray "[N]".
func TestAlgorithmicEatsOptional(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=400pt\begin{algorithmic}[1]
\STATE done
\end{algorithmic}`); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if strings.Contains(b.String(), "[1]") {
		t.Errorf("the [1] optional leaked into the output: %s", b.String())
	}
}
