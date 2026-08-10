// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file is a minimal LaTeX layer written in TeX on top of the Plain macros:
// the \begin/\end environment mechanism (via \csname, exactly as latex.ltx does),
// sectioning, the common text-formatting commands, and list/quote environments.
// It is deliberately small — the road to full LaTeX is to grow this toward the
// real latex.ltx, not to hand-code commands in Go. \documentclass/\usepackage are
// argument-gobbling primitives (they accept an optional [options] and a {name}).

// MiniLaTeXKernel is the LaTeX-flavoured macro layer loaded by LoadLaTeX (after
// the Plain macros).
const MiniLaTeXKernel = `
\catcode64=11
\def\begin#1{\csname #1\endcsname}
\def\end#1{\csname end#1\endcsname}
\def\document{\catcode64=12 }
\def\enddocument{\par\vfill\penalty-10000 }
\def\rm{}
\def\bf{}
\def\it{}
\def\sl{}
\def\tt{}
\def\sf{}
\def\textbf#1{{\bf #1}}
\def\textit#1{{\it #1}}
\def\texttt#1{{\tt #1}}
\def\textsf#1{{\sf #1}}
\def\textrm#1{{\rm #1}}
\def\emph#1{{\it #1}}
\def\underline#1{#1}
\def\mbox#1{\hbox{#1}}
\newcount\c@section
\newcount\c@subsection
\def\thesection{\the\c@section}
\def\thesubsection{\the\c@section.\the\c@subsection}
\def\section#1{\par\medskip\advance\c@section by1 \c@subsection=0 \noindent\thesection\quad#1\par\nobreak\smallskip}
\def\subsection#1{\par\smallskip\advance\c@subsection by1 \noindent\thesubsection\quad#1\par\nobreak}
\def\subsubsection#1{\par\smallskip\noindent#1\par\nobreak}
\def\paragraph#1{\par\noindent#1\quad}
\def\title#1{\def\@title{#1}}
\def\author#1{\def\@author{#1}}
\def\date#1{\def\@date{#1}}
\def\@title{}
\def\@author{}
\def\@date{}
\def\maketitle{\par\bigskip\centerline{\@title}\smallskip\centerline{\@author}\smallskip\centerline{\@date}\bigskip}
\def\itemize{\par\smallskip}
\def\enditemize{\par\smallskip}
\def\enumerate{\par\smallskip}
\def\endenumerate{\par\smallskip}
\def\quotation{\par\smallskip}
\def\endquotation{\par\smallskip}
\def\center{\par}
\def\endcenter{\par}
\def\item{\par\noindent\quad-- }
\def\LaTeXe{LaTeX2e}
\def\ldots{...}
\def\dots{...}
\def\newpage{\par\penalty-10000 }
\def\clearpage{\par\penalty-10000 }
`

// LoadLaTeX loads the Plain macros (if not already) and the minimal LaTeX kernel.
func (e *Engine) LoadLaTeX() error {
	if err := e.LoadPlain(); err != nil {
		return err
	}
	return e.LoadFormat(MiniLaTeXKernel)
}

// doNewcommand implements LaTeX's \newcommand / \renewcommand / \providecommand:
//
//	\newcommand{\name}[nargs][default]{body}   (braces optional around \name)
//
// It defines \name as a macro of nargs undelimited parameters (#1…#nargs in the
// body). An optional-argument default ([default]) is consumed but not yet honoured
// (the arg is treated as mandatory), which covers the common no-optional-arg use.
func (e *Engine) doNewcommand() {
	name := e.scanCmdName()
	nargs := e.scanOptBracketInt()
	e.scanOptBracketSkip() // optional [default] — consumed; optional-arg not modelled
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return
	}
	body := e.scanBody()
	var params []tok
	for i := 1; i <= nargs && i <= 9; i++ {
		params = append(params, tok{ch: rune('0' + i), cat: catParam})
	}
	if name != "" {
		e.define(name, &meaning{kind: mMacro, params: params, body: body}, false)
	}
}

// scanCmdName reads the command name for \newcommand: either {\name} or \name.
// It reads raw (no expansion) so a \renewcommand target that is already defined
// is not expanded before its name is captured.
func (e *Engine) scanCmdName() string {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok {
		return ""
	}
	if t.cat == catBegin && !t.cs_ {
		n, ok := e.getNext()
		name := ""
		if ok && n.cs_ {
			name = n.cs
		}
		if c, ok := e.getNext(); ok && !(c.cat == catEnd && !c.cs_) {
			e.back(c)
		}
		return name
	}
	if t.cs_ {
		return t.cs
	}
	e.back(t)
	return ""
}

// scanOptBracketInt reads an optional [n] and returns n (0 if absent).
func (e *Engine) scanOptBracketInt() int {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 0
	}
	if !t.cs_ && t.ch == '[' {
		n := e.scanInt()
		if c, ok := e.getXToken(); ok && !(!c.cs_ && c.ch == ']') {
			e.back(c)
		}
		return n
	}
	e.back(t)
	return 0
}

// scanOptBracketSkip consumes an optional [...] group (its content is ignored).
func (e *Engine) scanOptBracketSkip() {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return
	}
	if !t.cs_ && t.ch == '[' {
		for {
			u, ok := e.getNext()
			if !ok || (!u.cs_ && u.ch == ']') {
				return
			}
		}
	}
	e.back(t)
}

// doDocumentClass gobbles \documentclass[options]{class} (both parts optional in
// practice); it selects no behaviour yet — the class is ignored.
func (e *Engine) doGobbleOptAndGroup() {
	e.skipOptSpace()
	// optional [options]
	if t, ok := e.getXToken(); ok {
		if !t.cs_ && t.ch == '[' {
			for {
				u, ok := e.getNext()
				if !ok || (!u.cs_ && u.ch == ']') {
					break
				}
			}
		} else {
			e.back(t)
		}
	}
	// required {name}
	e.skipOptSpace()
	if t, ok := e.getXToken(); ok {
		if t.cat == catBegin && !t.cs_ {
			e.grabGroup()
		} else {
			e.back(t)
		}
	}
}
