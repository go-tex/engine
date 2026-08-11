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
\def\textcolor#1#2{{\color{#1}#2}}
\def\tiny{\gotexsize500\relax}
\def\scriptsize{\gotexsize700\relax}
\def\footnotesize{\gotexsize800\relax}
\def\small{\gotexsize900\relax}
\def\normalsize{\gotexsize1000\relax}
\def\large{\gotexsize1200\relax}
\def\Large{\gotexsize1440\relax}
\def\LARGE{\gotexsize1728\relax}
\def\huge{\gotexsize2074\relax}
\def\Huge{\gotexsize2488\relax}
\def\mbox#1{\hbox{#1}}
\newcount\c@section
\newcount\c@subsection
\newcount\c@equation
\def\theequation{\the\c@equation}
\def\equation{\global\advance\c@equation by1\relax\edef\@currentlabel{\theequation}\@equationbody}
\def\endequation{}
\def\@currentlabel{}
\def\thesection{\the\c@section}
\def\thesubsection{\the\c@section.\the\c@subsection}
\def\section{\@ifstar\@ssection\@nsection}
\def\@nsection#1{\par\medskip\advance\c@section by1 \c@subsection=0 \edef\@currentlabel{\thesection}\noindent{\Large\bf\thesection\quad#1}\par\nobreak\smallskip}
\def\@ssection#1{\par\medskip\noindent{\Large\bf#1}\par\nobreak\smallskip}
\def\subsection{\@ifstar\@ssubsection\@nsubsection}
\def\@nsubsection#1{\par\smallskip\advance\c@subsection by1 \edef\@currentlabel{\thesubsection}\noindent{\large\bf\thesubsection\quad#1}\par\nobreak}
\def\@ssubsection#1{\par\smallskip\noindent{\large\bf#1}\par\nobreak}
\def\subsubsection{\@ifstar\@ssubsubsection\@nsubsubsection}
\def\@nsubsubsection#1{\par\smallskip\noindent#1\par\nobreak}
\def\@ssubsubsection#1{\par\smallskip\noindent#1\par\nobreak}
\def\paragraph#1{\par\noindent#1\quad}
\def\title#1{\def\@title{#1}}
\def\author#1{\def\@author{#1}}
\def\date#1{\def\@date{#1}}
\def\@title{}
\def\@author{}
\def\@date{}
\def\maketitle{\par\bigskip\centerline{\@title}\smallskip\centerline{\@author}\smallskip\centerline{\@date}\bigskip}
\def\bullet{\char8226\relax}
\def\cdot{\char183\relax}
% Per-level counters and depth trackers for nested itemize/enumerate. Because
% count/skip registers are group-local, \begingroup + \advance means each level
% of nesting accumulates its indentation and its depth, then unwinds cleanly at
% \endgroup — enclosing levels keep their own counters intact.
\newcount\c@enumi
\newcount\c@enumii
\newcount\c@enumiii
\newcount\c@enumiv
\newcount\c@enumdepth
\newcount\c@itemdepth
\def\@alph#1{\ifcase#1\or a\or b\or c\or d\or e\or f\or g\or h\or i\or j\or k\or l\or m\or n\or o\or p\or q\or r\or s\or t\or u\or v\or w\or x\or y\or z\fi}
\def\@Alph#1{\ifcase#1\or A\or B\or C\or D\or E\or F\or G\or H\or I\or J\or K\or L\or M\or N\or O\or P\or Q\or R\or S\or T\or U\or V\or W\or X\or Y\or Z\fi}
\def\theenumi{\the\c@enumi.}
\def\theenumii{(\@alph\c@enumii)}
\def\theenumiii{\romannumeral\c@enumiii.}
\def\theenumiv{\@Alph\c@enumiv.}
\def\labelitemi{\bullet}
\def\labelitemii{--}
\def\labelitemiii{*}
\def\labelitemiv{\cdot}
\def\@listitem#1#2{\par\noindent\advance#1 by1\relax\edef\@currentlabel{#2}\llap{#2\enspace}}
\def\@bulletitem#1{\par\noindent\llap{#1\enspace}}
% \@itemopt reads the optional [label] of an \item in itemize/enumerate and uses
% it verbatim in place of the default bullet/number. \@descitem does the same for
% description, but emboldens the term. Both are delimited macros, so they consume
% the bracket that \@ifnextbracket has already confirmed is present.
\def\@itemopt[#1]{\par\noindent\llap{#1\enspace}}
\def\@descitem[#1]{\par\noindent\llap{{\bf #1}\enspace}}
\def\itemize{\par\smallskip\begingroup\advance\leftskip by24pt\advance\c@itemdepth by1\relax\ifcase\c@itemdepth\or\def\item{\@ifnextbracket{\@itemopt}{\@bulletitem\labelitemi}}\or\def\item{\@ifnextbracket{\@itemopt}{\@bulletitem\labelitemii}}\or\def\item{\@ifnextbracket{\@itemopt}{\@bulletitem\labelitemiii}}\else\def\item{\@ifnextbracket{\@itemopt}{\@bulletitem\labelitemiv}}\fi}
\def\enditemize{\par\endgroup\smallskip}
\def\enumerate{\par\smallskip\begingroup\advance\leftskip by24pt\advance\c@enumdepth by1\relax\ifcase\c@enumdepth\or\c@enumi=0\relax\def\item{\@ifnextbracket{\@itemopt}{\@listitem\c@enumi\theenumi}}\or\c@enumii=0\relax\def\item{\@ifnextbracket{\@itemopt}{\@listitem\c@enumii\theenumii}}\or\c@enumiii=0\relax\def\item{\@ifnextbracket{\@itemopt}{\@listitem\c@enumiii\theenumiii}}\else\c@enumiv=0\relax\def\item{\@ifnextbracket{\@itemopt}{\@listitem\c@enumiv\theenumiv}}\fi}
\def\endenumerate{\par\endgroup\smallskip}
% description: each \item[term] sets the bold term in the left margin, with the
% following text indented like the other list environments. \item here always
% takes a [label]; the label may overflow the 24pt margin (not reflowed onto a
% separate line as full LaTeX would) — acceptable for this kernel.
\def\description{\par\smallskip\begingroup\advance\leftskip by24pt\def\item{\@descitem}}
\def\enddescription{\par\endgroup\smallskip}
\newcount\c@bibitem
\def\thebibitem{\the\c@bibitem}
\def\thebibliography#1{\par\bigskip\noindent\bf References\rm\par\smallskip\c@bibitem=0\begingroup\leftskip=24pt}
\def\endthebibliography{\par\endgroup\smallskip}
\def\bibitem#1{\par\noindent\advance\c@bibitem by1\relax\edef\@currentlabel{\thebibitem}\label{#1}\llap{[\thebibitem]\enspace}}
\newcount\c@figure
\newcount\c@table
\def\thefigure{\the\c@figure}
\def\thetable{\the\c@table}
\def\fnum@figure{Figure \thefigure}
\def\fnum@table{Table \thetable}
\def\figure{\par\bigskip\begingroup\centering\def\@captype{figure}\@discardopt}
\def\endfigure{\par\endgroup\bigskip}
\def\table{\par\bigskip\begingroup\centering\def\@captype{table}\@discardopt}
\def\endtable{\par\endgroup\bigskip}
\def\caption#1{\par\smallskip\global\expandafter\advance\csname c@\@captype\endcsname by1\relax\edef\@currentlabel{\csname the\@captype\endcsname}{\small{\bf\csname fnum@\@captype\endcsname:} #1}\par}
\def\quote{\par\begingroup\leftskip=20pt\rightskip=20pt\smallskip}
\def\endquote{\par\endgroup\smallskip}
\def\quotation{\par\begingroup\leftskip=20pt\rightskip=20pt\smallskip}
\def\endquotation{\par\endgroup\smallskip}
\def\verse{\par\begingroup\leftskip=20pt\smallskip}
\def\endverse{\par\endgroup\smallskip}
\def\centering{\leftskip=0pt plus 1fil\rightskip=0pt plus 1fil\relax}
\def\raggedleft{\leftskip=0pt plus 1fil\rightskip=0pt\relax}
\def\center{\par\begingroup\centering}
\def\endcenter{\par\endgroup}
\def\flushleft{\par\begingroup\raggedright}
\def\endflushleft{\par\endgroup}
\def\flushright{\par\begingroup\raggedleft}
\def\endflushright{\par\endgroup}
\def\item{\par\noindent\quad-- }
\def\\{\penalty-10000 }
\def\newline{\penalty-10000 }
\def\LaTeXe{LaTeX2e}
\def\ldots{...}
\def\dots{...}
\def\newpage{\par\penalty-10000 }
\def\clearpage{\par\penalty-10000 }
\def\hline{}
\def\cline#1{}
% amsthm-style theorem support. \newtheorem (a Go primitive) generates, per
% environment, \the<env> and the \<env>/\end<env> macros; those hand the shared
% formatting to the fixed macros below. \@begintheorem sets the bold heading
% "Heading N", then \@ifnextbracket picks the with-note or plain continuation —
% both switch to italic for the body (amsthm's "plain" style). \@endtheorem ends
% the paragraph, closes the environment group (reverting \it) and adds space.
\def\@begintheorem#1#2{\noindent{\bf #1\ #2}\@ifnextbracket{\@opargbegintheorem}{\@stdbegintheorem}}
\def\@stdbegintheorem{{\bf .}\ \it }
\def\@opargbegintheorem[#1]{{\bf\ (#1).}\ \it }
\def\@endtheorem{\par\endgroup\medskip}
% proof: an italic "Proof." head (overridable via \begin{proof}[Proof of …]), a
% roman body, and a QED box flushed to the right margin at \end{proof}.
\def\qedsymbol{\rule{6pt}{6pt}}
\def\qed{\hfill\qedsymbol\par}
\def\proof{\par\medskip\begingroup\@ifnextbracket{\@opargproof}{\@stdproof}}
\def\@stdproof{\noindent{\it Proof.}\ }
\def\@opargproof[#1]{\noindent{\it #1.}\ }
\def\endproof{\qed\endgroup\medskip}
% ─── table of contents (feat/toc) ───────────────────────────────────────────
% \@tocentry{kind}{level}{number}{title} (a Go primitive) records one contents
% line on the auxiliary pass. The numbered sectioning and caption macros are
% redefined here — as new \def lines, to avoid touching the originals above — so
% they emit an entry in addition to typesetting their heading. Starred forms
% (\@ssection/\@ssubsection) are untouched and never record, matching LaTeX.
\newcount\c@tocdepth
\def\@nsection#1{\par\medskip\advance\c@section by1 \c@subsection=0 \edef\@currentlabel{\thesection}\@tocentry{toc}{1}{\thesection}{#1}\noindent{\Large\bf\thesection\quad#1}\par\nobreak\smallskip}
\def\@nsubsection#1{\par\smallskip\advance\c@subsection by1 \edef\@currentlabel{\thesubsection}\@tocentry{toc}{2}{\thesubsection}{#1}\noindent{\large\bf\thesubsection\quad#1}\par\nobreak}
\def\caption#1{\par\smallskip\global\expandafter\advance\csname c@\@captype\endcsname by1\relax\edef\@currentlabel{\csname the\@captype\endcsname}\@tocentry{\@captype}{1}{\csname the\@captype\endcsname}{#1}{\small{\bf\csname fnum@\@captype\endcsname:} #1}\par}
% ─── LaTeX counter interface (feat/counters) ─────────────────────────────────
% Value-reading and counter-formatting commands. The mutating commands
% (\newcounter/\setcounter/\addtocounter/\stepcounter/\refstepcounter) and the
% \@Roman helper are Go primitives (see counters.go / primitives.go); these are
% the pure-macro one-liners, added as new \def lines to avoid touching the
% originals above. \value{c} expands to the register \c@c, so it is usable as a
% <number> (e.g. \setcounter{x}{\value{y}}, \ifnum\value{x}>0). \arabic/\roman/
% \Roman feed the register to a number-scanning operator (\number, \romannumeral,
% \@Roman). \alph/\Alph/\fnsymbol feed it to an \ifcase macro, so \expandafter
% first turns \csname c@c\endcsname into the single \c@c token that \ifcase reads.
\def\value#1{\csname c@#1\endcsname}
\def\arabic#1{\number\csname c@#1\endcsname}
\def\roman#1{\romannumeral\csname c@#1\endcsname}
\def\Roman#1{\@Roman\csname c@#1\endcsname}
\def\alph#1{\expandafter\@alph\csname c@#1\endcsname}
\def\Alph#1{\expandafter\@Alph\csname c@#1\endcsname}
\def\@fnsymbol#1{\ifcase#1\or *\or †\or ‡\or §\or ¶\or ‖\or **\or ††\or ‡‡\fi}
\def\fnsymbol#1{\expandafter\@fnsymbol\csname c@#1\endcsname}
% ─── length interface (feat/lengths) ─────────────────────────────────────────
% \stretch{n} is a rubber length "0pt plus n fil" (order-1 infinite stretch), so
% \setlength{\x}{\stretch{2}} keeps the stretch and \hskip\stretch{1} behaves
% like \hfil. \newlength/\setlength/\addtolength/\settoX are Go primitives.
\def\stretch#1{0pt plus #1fil}
% ─── sectioning extensions (feat/sectioning) ─────────────────────────────────
% \part, \appendix and the abstract/titlepage environments. All are new \def
% lines (later definition wins), so the \section/\@nsection/\thesection lines
% above are left untouched. In particular \appendix retargets numbering purely
% by REDEFINING the formatting macros \thesection/\thesubsection to letters — it
% never touches \@nsection (which a later block bound to \@tocentry), so both the
% heading and the contents line pick up "A"/"A.1" unchanged. \part numbers with
% \c@part (Roman) and freezes \@currentlabel so \label/\ref resolve to "I".
\newcount\c@part
\def\thepart{\@Roman\c@part}
\def\part{\@ifstar\@spart\@npart}
\def\@npart#1{\par\bigskip\advance\c@part by1 \edef\@currentlabel{\thepart}\centerline{\Large\bf Part \thepart}\smallskip\centerline{\Large\bf#1}\par\bigskip}
\def\@spart#1{\par\bigskip\centerline{\Large\bf#1}\par\bigskip}
\def\appendix{\par\c@section=0 \c@subsection=0 \def\thesection{\@Alph\c@section}\def\thesubsection{\thesection.\the\c@subsection}}
\def\abstract{\par\bigskip\begingroup\centerline{\small\bf Abstract}\smallskip\leftskip=20pt\rightskip=20pt\small}
\def\endabstract{\par\endgroup\bigskip}
\def\titlepage{\par\penalty-10000 \begingroup}
\def\endtitlepage{\par\endgroup\penalty-10000 }
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
// body). When a second bracket [default] is present, the FIRST of the nargs
// parameters becomes optional: at call time #1 comes from a bracketed [..]
// argument if one is supplied, otherwise from default (standard LaTeX semantics).
func (e *Engine) doNewcommand() {
	name := e.scanCmdName()
	nargs := e.scanOptBracketInt()
	optDefault, optArg := e.scanOptBracketToks() // optional [default]: 1st arg optional
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
		e.define(name, &meaning{
			kind: mMacro, params: params, body: body,
			optArg: optArg && nargs >= 1, optDefault: optDefault,
		}, false)
	}
}

// doNewenvironment implements \newenvironment{name}[nargs][default]{begin}{end}:
// it defines \name (a macro of nargs parameters whose body is the begin-code) and
// \endname (a 0-parameter macro whose body is the end-code), so \begin{name} and
// \end{name} run them via \csname. When a [default] bracket follows [nargs], the
// environment's first argument is optional (as for \newcommand).
func (e *Engine) doNewenvironment() {
	name := e.readBraceName()
	nargs := e.scanOptBracketInt()
	optDefault, optArg := e.scanOptBracketToks()
	begin := e.readBodyGroup()
	end := e.readBodyGroup()
	if name == "" {
		return
	}
	var params []tok
	for i := 1; i <= nargs && i <= 9; i++ {
		params = append(params, tok{ch: rune('0' + i), cat: catParam})
	}
	e.define(name, &meaning{
		kind: mMacro, params: params, body: begin,
		optArg: optArg && nargs >= 1, optDefault: optDefault,
	}, false)
	e.define("end"+name, &meaning{kind: mMacro, body: end}, false)
}

// doRuleNode builds a node for LaTeX's \rule[lift]{width}{height}: a filled
// rectangle of the given width and height, raised by the optional lift.
func (e *Engine) doRuleNode() ruleNode {
	lift := 0
	e.skipOptSpace()
	if t, ok := e.getXToken(); ok {
		if !t.cs_ && t.ch == '[' {
			lift = e.scanDimen()
			if c, ok := e.getXToken(); ok && !(!c.cs_ && c.ch == ']') {
				e.back(c)
			}
		} else {
			e.back(t)
		}
	}
	w := e.readBraceDimen()
	h := e.readBraceDimen()
	return ruleNode{width: w, height: h - lift, depth: lift}
}

// readBraceDimen reads a {dimen} group and returns the dimension in sp.
func (e *Engine) readBraceDimen() int {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return 0
	}
	d := e.scanDimen()
	if c, ok := e.getXToken(); ok && !(c.cat == catEnd && !c.cs_) {
		e.back(c)
	}
	return d
}

// readBodyGroup reads a {…} group as a macro body (converting #n to parameters).
func (e *Engine) readBodyGroup() []tok {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return nil
	}
	return e.scanBody()
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

// scanOptBracketToks reads an optional [...] group and returns its token content
// together with whether the bracket was present. Brace groups nested inside are
// tracked by depth so that a ] appearing within {…} does not close the group
// early. When no bracket follows, it pushes back the peeked token and reports
// (nil, false).
func (e *Engine) scanOptBracketToks() ([]tok, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return nil, false
	}
	if !t.cs_ && t.ch == '[' {
		var toks []tok
		depth := 0
		for {
			u, ok := e.getNext()
			if !ok {
				return toks, true
			}
			switch {
			case !u.cs_ && u.cat == catBegin:
				depth++
			case !u.cs_ && u.cat == catEnd:
				if depth > 0 {
					depth--
				}
			case depth == 0 && !u.cs_ && u.ch == ']':
				return toks, true
			}
			toks = append(toks, u)
		}
	}
	e.back(t)
	return nil, false
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
