// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// MiniLaTeX is a small LaTeX-flavoured kernel written *in TeX* — the engine runs
// these macro definitions through its gullet exactly as a real format does; they
// are not reimplemented in Go. It is deliberately tiny (the road to parity is to
// grow this by loading the real latex.ltx, not to hand-code commands).
const MiniLaTeX = `
\def\LaTeX{LaTeX}
\def\TeX{TeX}
\def\newcommand#1#2{\def#1{#2}}
\def\renewcommand#1#2{\def#1{#2}}
\def\emph#1{#1}
\def\textbf#1{#1}
\def\textit#1{#1}
\def\ldots{...}
`

// LoadFormat executes a string of TeX definitions (a format/preamble) through
// the gullet, defining its macros in the engine's eqtb without typesetting.
func (e *Engine) LoadFormat(src string) error {
	// Save/restore the base input so a subsequent Typeset starts clean.
	oldBase, oldPos := e.base, e.bpos
	e.base, e.bpos = []rune(src), 0
	e.mainLoop()
	e.base, e.bpos = oldBase, oldPos
	return e.err
}
