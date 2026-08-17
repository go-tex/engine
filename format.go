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

// Plain is a small set of plain-TeX structural macros, written *in TeX* on top
// of the box/glue primitives (\hbox to, \hfil, \vskip). Loaded with LoadPlain,
// they let a document use the familiar commands without any Go-side support — the
// same growth path as the kernel: add macros, do not hand-code commands.
const Plain = `
% Plain TeX's category-code constants. \active is the one a package reaches for
% by name: it makes a character active, and asks whether one already is
% (\ifnum\catcode of a character = \active). Without it such a block does not
% run, and everything defined after it in that file is lost with it.
\chardef\active=13
\def\TeX{TeX}
\def\LaTeX{LaTeX}
\def\empty{}
\def\centerline#1{\hbox to\hsize{\hfil#1\hfil}}
\def\leftline#1{\hbox to\hsize{#1\hfil}}
\def\rightline#1{\hbox to\hsize{\hfil#1}}
\def\llap#1{\hbox to0pt{\hss#1}}
\def\rlap#1{\hbox to0pt{#1\hss}}
\def\bigskip{\vskip12pt}
\def\medskip{\vskip6pt}
\def\smallskip{\vskip3pt}
\def\quad{\hskip1em}
\def\qquad{\hskip2em}
\def\enspace{\hskip.5em}
\def\thinspace{\hskip.16667em}
\def\negthinspace{\hskip-.16667em}
\def\raggedright{\rightskip=0pt plus 1fil\relax}
\def\justified{\rightskip=0pt\relax}
\def\vfill{\vskip0pt plus 1fil\relax}
\def\eject{\par\penalty-10000 }
\def\supereject{\par\penalty-10000 }
\def\break{\penalty-10000 }
\def\nobreak{\penalty10000 }
\def\goodbreak{\par\penalty-500 }
\def\smallbreak{\par\penalty-100 \smallskip}
\def\medbreak{\par\penalty-100 \medskip}
\def\bigbreak{\par\penalty-100 \bigskip}
\def\bye{\par\vfill\penalty-10000 }
\def\%{\char37\relax}
\def\${\char36\relax}
\def\&{\char38\relax}
\def\#{\char35\relax}
\def\_{\char95\relax}
\def\{{\char123\relax}
\def\}{\char125\relax}
\def\S{\char167\relax}
\def\P{\char182\relax}
\def\copyright{\char169\relax}
\def\dag{\char134\relax}
\def\ddag{\char135\relax}
\def\pounds{\char163\relax}
\def\oe{\char339\relax}
\def\OE{\char338\relax}
\def\ae{\char230\relax}
\def\AE{\char198\relax}
\def\o{\char248\relax}
\def\O{\char216\relax}
\def\aa{\char229\relax}
\def\AA{\char197\relax}
\def\ss{\char223\relax}
\def\l{\char322\relax}
\def\L{\char321\relax}
\def\i{\char305\relax}
\def\j{\char567\relax}
`

// LoadPlain defines the Plain structural macros in the engine.
func (e *Engine) LoadPlain() error { return e.LoadFormat(Plain) }

// LoadFormat executes a string of TeX definitions (a format/preamble) through
// the gullet, defining its macros in the engine's eqtb without typesetting.
func (e *Engine) LoadFormat(src string) error {
	// Save/restore the base input (and its source-position state) so a subsequent
	// Run/Typeset starts clean and reports its own line numbers.
	oldBase, oldPos := e.base, e.bpos
	oldStarts, oldSP, oldLine, oldCol := e.lineStarts, e.srcPos, e.curSrcLine, e.curSrcCol
	e.base, e.bpos = []rune(src), 0
	e.lineStarts = nil
	e.buildLineStarts()
	e.mainLoop()
	e.base, e.bpos = oldBase, oldPos
	e.lineStarts, e.srcPos, e.curSrcLine, e.curSrcCol = oldStarts, oldSP, oldLine, oldCol
	return e.err
}
