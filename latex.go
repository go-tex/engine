// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

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
\def\begin#1{\gotex@checkenv{#1}\csname #1\endcsname}
\def\end#1{\csname end#1\endcsname}
\def\document{\catcode64=12 }
\def\enddocument{\par\vfill\penalty-10000 }
\def\rm{}
\def\bf{}
\def\it{}
\def\sl{}
\def\tt{}
\def\sf{}
% \text… go through the NFSS declarations (\bfseries/\itshape/…), which the
% engine aliases to the real font switches (see aliasFontSwitches). Using the
% NFSS names — not the deprecated \bf/\it/… — keeps \textbf working even when a
% document does \renewcommand{\bf}{\textbf}, which would otherwise make \textbf
% recurse into itself and swallow the rest of the input.
\def\textbf#1{{\bfseries #1}}
\def\textit#1{{\itshape #1}}
\def\texttt#1{{\ttfamily #1}}
\def\textsf#1{{\sffamily #1}}
\def\textrm#1{{\rmfamily #1}}
\def\emph#1{{\itshape #1}}
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
% \title and \author accept an OPTIONAL short form — \title[short]{full} — in the
% amsart family (used for the running head). The short argument is only STORED
% (in \@shorttitle / \@shortauthor); executing it would run any font or spacing
% command it carries in global scope. A real paper writes \title[\tiny …]{…}, and
% taking that short form as the mandatory argument let the \tiny escape into the
% document body and set every following paragraph half-size. Detect the bracket and
% keep the short form unexpanded; with no bracket the plain one-argument form holds.
\def\title{\@ifnextchar[{\@titleopt}{\@titlemand}}
\def\@titleopt[#1]#2{\def\@shorttitle{#1}\def\@title{#2}}
\def\@titlemand#1{\def\@title{#1}}
\def\author{\@ifnextchar[{\@authoropt}{\@authormand}}
\def\@authoropt[#1]#2{\def\@shortauthor{#1}\def\@author{#2}}
\def\@authormand#1{\def\@author{#1}}
\def\date#1{\def\@date{#1}}
\def\@title{}
\def\@author{}
\def\@date{}
\def\@shorttitle{}
\def\@shortauthor{}
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
\def\itemize{\par\smallskip\begingroup\advance\leftskip by24pt\advance\c@itemdepth by1\relax\ifcase\c@itemdepth\or\def\item{\@ifnextbracket{\@itemopt}{\@bulletitem\labelitemi}}\or\def\item{\@ifnextbracket{\@itemopt}{\@bulletitem\labelitemii}}\or\def\item{\@ifnextbracket{\@itemopt}{\@bulletitem\labelitemiii}}\else\def\item{\@ifnextbracket{\@itemopt}{\@bulletitem\labelitemiv}}\fi\@enumitemopt{itemize}}
\def\enditemize{\par\endgroup\smallskip}
\def\enumerate{\par\smallskip\begingroup\advance\leftskip by24pt\advance\c@enumdepth by1\relax\ifcase\c@enumdepth\or\c@enumi=0\relax\def\item{\@ifnextbracket{\@itemopt}{\@listitem\c@enumi\theenumi}}\or\c@enumii=0\relax\def\item{\@ifnextbracket{\@itemopt}{\@listitem\c@enumii\theenumii}}\or\c@enumiii=0\relax\def\item{\@ifnextbracket{\@itemopt}{\@listitem\c@enumiii\theenumiii}}\else\c@enumiv=0\relax\def\item{\@ifnextbracket{\@itemopt}{\@listitem\c@enumiv\theenumiv}}\fi\@enumitemopt{enumerate}}
\def\endenumerate{\par\@enumitemrec\endgroup\smallskip}
% description: each \item[term] sets the bold term in the left margin, with the
% following text indented like the other list environments. \item here always
% takes a [label]; the label may overflow the 24pt margin (not reflowed onto a
% separate line as full LaTeX would) — acceptable for this kernel.
\def\description{\par\smallskip\begingroup\advance\leftskip by24pt\def\item{\@descitem}\@enumitemopt{description}}
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
% \include{FILE}/\includeonly. LaTeX's \include starts a fresh page, reads
% FILE.tex, then starts another fresh page (it also keeps a per-file .aux, which
% the engine has no equivalent for), so it reduces to \clearpage\input\clearpage.
% Without it, \include is an undefined control sequence: in lenient mode the
% command is dropped and its {FILE} argument is typeset as stray text, so a paper
% whose whole body lives in \include'd files (the LaTeX \include/\includeonly
% convention for splitting a manuscript) renders zero pages. \includeonly only
% narrows which files \include loads; honouring it would need comma-list
% membership the mini-kernel has no \@for for, and over-including merely adds
% content it would have skipped, so it is a safe no-op for best-effort rendering.
\def\include#1{\clearpage\input{#1}\clearpage}
\def\includeonly#1{}
% The float mechanism's list registers. LaTeX holds pending floats in these
% token-list macros and empties them as the floats are placed; the engine has no
% float mechanism, so they simply stay empty — but they must be DEFINED (as
% \@empty) all the same. placeins's \FloatBarrier forms
% \edef\@tempa{\@fb@botlist\@deferlist\@dbldeferlist} and only no-ops when that
% expands to nothing; with the lists undefined the test never matches empty and its
% "dump a float, \newpage, and recurse" branch runs without end — one forced page
% break per turn, a 5000-page runaway on an ordinary article that loads placeins
% and gives \section a \FloatBarrier (its [section] option).
% Defined directly as empty (not \let\@empty): this kernel loads before \@empty
% itself is defined, so a \let would capture an undefined meaning.
\def\@botlist{}
\def\@deferlist{}
\def\@dbldeferlist{}
\def\@toplist{}
\def\@midlist{}
\def\@currlist{}
% \subfile{file} / \subfileinclude{file} (subfiles package): typeset file.tex as
% part of this document. A subfile is often bare section content, but it may carry
% its own \documentclass[main]{subfiles} … \begin{document} … \end{document}
% wrapper. Neutralise that wrapper for the duration of the input — gobble the
% \documentclass line's [option]{class}, and make \begin{document}/\end{document}
% (\document/\enddocument) no-ops so the subfile's \end{document} does not end the
% WHOLE document — then \input the file. Grouping restores the three afterwards, so
% the outer document's real \end{document} still fires. Without \subfile the whole
% body of a paper split into subfiles is silently dropped (118 corpus papers use it).
\long\def\subfile#1{{%
  \let\documentclass\subfile@gobbleclass
  \let\document\@empty \let\enddocument\@empty
  \input{#1}}}
\let\subfileinclude\subfile
\def\subfile@gobbleclass{\@ifnextchar[{\subfile@gc}{\subfile@gc[]}}
\def\subfile@gc[#1]#2{}
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
% ─── amsmath subequations ────────────────────────────────────────────────────
% \begin{subequations} numbers the equations inside as Na, Nb, … : it steps the
% parent equation number, freezes it in \@parentequation, then temporarily reuses
% \c@equation as the sub-counter (from 0) with \theequation set to parent+letter,
% so every inner \begin{equation} advances the letter. \end{subequations} restores
% \c@equation to the parent number (so the next equation is N+1) and \theequation.
% Because \c@equation stays the active counter, \label/\eqref and \tag work inside.
\newcount\@saveeq
\def\subequations{\global\advance\c@equation by1\relax\edef\@parentequation{\the\c@equation}\@saveeq=\c@equation\relax\c@equation=0\relax\def\theequation{\@parentequation\@alph\c@equation}}
\def\endsubequations{\c@equation=\@saveeq\relax\def\theequation{\the\c@equation}}
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
% ─── booktabs rules (feat/booktabs) ──────────────────────────────────────────
% \toprule/\midrule/\bottomrule/\cmidrule are consumed raw inside the tabular
% body (collectTabularBody matches them by name and draws the rules directly), so
% these definitions are only a harmless fallback that keeps the commands from
% being "undefined" if they appear outside a tabular — mirroring \hline/\cline.
\def\toprule{}
\def\midrule{}
\def\bottomrule{}
\def\cmidrule#1{}
% ─── typed cross-references (feat/typedrefs) ─────────────────────────────────
% \autoref/\nameref (hyperref) and \cref/\Cref (cleveref) print a reference with
% the NAME of what it points to ("Section 1", "Equation (1)"). To type a ref, the
% engine must record — beside the number (\@currentlabel) — the KIND of thing the
% last counter-stepping command produced (\@currentreftype) and its title text
% (\@currentlabelname, for \nameref). Rather than touch the originals, the
% number-freezing macros are redefined here as NEW \def lines (later definition
% wins) that additionally set those two macros; \label freezes all three (see
% typedrefs.go). The sectioning/caption redefinitions preserve the \@tocentry call
% that feat/toc bound, so the table of contents is unaffected. \@currentreftype
% and \@currentlabelname default to empty so a \label with no typed context still
% resolves (\autoref/\cref then fall back to a bare number).
\def\@currentreftype{}
\def\@currentlabelname{}
\def\equation{\global\advance\c@equation by1\relax\edef\@currentlabel{\theequation}\def\@currentreftype{equation}\def\@currentlabelname{}\@equationbody}
\def\@nsection#1{\par\medskip\advance\c@section by1 \c@subsection=0 \edef\@currentlabel{\thesection}\def\@currentreftype{section}\def\@currentlabelname{#1}\@tocentry{toc}{1}{\thesection}{#1}\noindent{\Large\bf\thesection\quad#1}\par\nobreak\smallskip}
\def\@nsubsection#1{\par\smallskip\advance\c@subsection by1 \edef\@currentlabel{\thesubsection}\def\@currentreftype{subsection}\def\@currentlabelname{#1}\@tocentry{toc}{2}{\thesubsection}{#1}\noindent{\large\bf\thesubsection\quad#1}\par\nobreak}
\def\caption#1{\par\smallskip\global\expandafter\advance\csname c@\@captype\endcsname by1\relax\edef\@currentlabel{\csname the\@captype\endcsname}\edef\@currentreftype{\@captype}\def\@currentlabelname{#1}\@tocentry{\@captype}{1}{\csname the\@captype\endcsname}{#1}{\small{\bf\csname fnum@\@captype\endcsname:} #1}\par}
\def\@listitem#1#2{\par\noindent\advance#1 by1\relax\edef\@currentlabel{#2}\def\@currentreftype{item}\def\@currentlabelname{}\llap{#2\enspace}}
\def\@npart#1{\par\bigskip\advance\c@part by1 \edef\@currentlabel{\thepart}\def\@currentreftype{part}\def\@currentlabelname{#1}\centerline{\Large\bf Part \thepart}\smallskip\centerline{\Large\bf#1}\par\bigskip}
\def\@begintheorem#1#2{\def\@currentreftype{theorem}\def\@currentlabelname{}\noindent{\bf #1\ #2}\@ifnextbracket{\@opargbegintheorem}{\@stdbegintheorem}}
% ── siunitx unit-name macros ────────────────────────────────────────────────
% Standalone expansions for the unit/prefix/power macros. \si, \unit, \SI, \qty
% and \ang read their arguments raw (no expansion) and resolve names in Go, so
% these definitions matter only when a unit macro is used outside those commands.
\def\meter{m}
\def\metre{m}
\def\second{s}
\def\kilogram{kg}
\def\gram{g}
\def\kelvin{K}
\def\ampere{A}
\def\mole{mol}
\def\candela{cd}
\def\newton{N}
\def\pascal{Pa}
\def\joule{J}
\def\watt{W}
\def\volt{V}
\def\ohm{Ω}
\def\hertz{Hz}
\def\percent{\char37\relax}
\def\hour{h}
\def\minute{min}
\def\liter{L}
\def\litre{L}
\def\coulomb{C}
\def\farad{F}
\def\tesla{T}
\def\weber{Wb}
\def\henry{H}
\def\siemens{S}
\def\radian{rad}
\def\steradian{sr}
\def\becquerel{Bq}
\def\gray{Gy}
\def\sievert{Sv}
\def\lumen{lm}
\def\lux{lx}
\def\kilo{k}
\def\milli{m}
\def\micro{µ}
\def\centi{c}
\def\nano{n}
\def\mega{M}
\def\giga{G}
\def\deci{d}
\def\deca{da}
\def\hecto{h}
\def\pico{p}
\def\femto{f}
\def\tera{T}
\def\peta{P}
\def\per{/}
\def\squared{²}
\def\cubed{³}
% ── end siunitx unit-name macros ────────────────────────────────────────────
% Width lengths used as the target of \begin{tabularx}{...} (and tabular*): they
% all resolve to \hsize, the current line width, so \linewidth/\textwidth/
% \columnwidth are accepted wherever a rigid <dimen> is scanned.
\let\linewidth\hsize
\let\textwidth\hsize
\let\columnwidth\hsize
% ─── sub-captions / \captionof / \captionsetup (feat/subcaption) ─────────────
% A pragmatic subset of the caption / subcaption packages. Everything here is
% pure macro glue that REUSES the existing counter, \caption and \parbox
% machinery — none of the \caption / \figure / \table / \c@figure lines above are
% edited; the two environment openers are only re-\def'd additively (later
% definition wins) to zero the sub-panel counter.
%
%   \captionof{TYPE}{TEXT}
%       A caption used OUTSIDE a float. It sets \@captype to TYPE (figure/table)
%       and defers to \caption, so it steps \c@TYPE, prints "Figure N: TEXT" and
%       freezes \@currentlabel — \label/\ref then resolve to the plain number "N",
%       exactly as an in-float \caption does. This is the caption package's key
%       feature (a caption for non-float material, e.g. inside a minipage).
%
%   \subcaptionbox{SUBCAP}{CONTENT}
%       One lettered sub-panel of a figure. It steps \c@subfigure and typesets
%       CONTENT (typically an \includegraphics, \fbox or \rule) above a centred
%       "(a) SUBCAP" inside a \parbox whose width is the NATURAL width of CONTENT
%       (measured with \settowidth into \subcaptionwidth), so several panels
%       placed in a row sit side by side and the sub-caption wraps to the panel.
%       A \label inside the panel resolves to parent+letter ("1a"): the
%       frozen \@currentlabel is \the\@subparent\thesubfigure, where \@subparent
%       is the number the pending main \caption will assign to the figure
%       (\c@figure + 1) and \thesubfigure is \alph{subfigure}. The reference type
%       is recorded as "subfigure" (for \autoref/\cref). This is the sub-panel
%       command that is FULLY implemented; \subfloat (below) is an alias.
%
%   \subfloat[SUBCAP]{CONTENT}
%       subfig-package spelling, mapped onto \subcaptionbox. The sub-caption is
%       the OPTIONAL argument (\subfloat{CONTENT} gives an unlettered-caption
%       panel that still steps the counter), matching subfig's signature.
%
%   \captionsetup[FLOAT]{OPTIONS}
%       Accepted and ignored: it gobbles an optional [float type] and the required
%       {options} group so caption-configuring documents do not break. Caption
%       STYLING options are not modelled (LIMITATION).
%
% CONVENTIONS & LIMITATIONS. Sub-panels must appear BEFORE the figure's main
% \caption (the usual layout), so that \c@figure + 1 is the figure's eventual
% number; the panel is sized to the natural width of its content (a caption
% longer than the content wraps to that width, with no manual [width] option);
% and caption styling requested through \captionsetup is not modelled.
\newcount\c@subfigure
\newcount\@subparent
\newdimen\subcaptionwidth
\def\thesubfigure{\@alph\c@subfigure}
\def\p@subfigure{\the\@subparent}
\def\captionof#1#2{\def\@captype{#1}\caption{#2}}
\def\captionsetup{\@ifnextbracket{\@captionsetupopt}{\@captionsetupnoopt}}
\def\@captionsetupopt[#1]#2{}
\def\@captionsetupnoopt#1{}
\def\subcaptionbox#1#2{\global\advance\c@subfigure by1\relax\@subparent=\c@figure \advance\@subparent by1\relax\edef\@currentlabel{\p@subfigure\thesubfigure}\def\@currentreftype{subfigure}\def\@currentlabelname{}\settowidth\subcaptionwidth{#2}\noindent\parbox[b]{\subcaptionwidth}{\centering #2\\{\small(\thesubfigure) #1}}\quad}
\def\subfloat{\@ifnextbracket{\@subfloatopt}{\@subfloatnoopt}}
\def\@subfloatopt[#1]#2{\subcaptionbox{#1}{#2}}
\def\@subfloatnoopt#1{\subcaptionbox{}{#1}}
\def\figure{\par\bigskip\begingroup\centering\def\@captype{figure}\global\advance\c@subfigure by-\c@subfigure\relax\@discardopt}
\def\table{\par\bigskip\begingroup\centering\def\@captype{table}\global\advance\c@subfigure by-\c@subfigure\relax\@discardopt}
% ─── end sub-captions / \captionof / \captionsetup ───────────────────────────
% ─── real-world preamble robustness ──────────────────────────────────────────
% Real papers configure many packages in the preamble with commands that do not
% affect this engine's output. Rather than abort on an "undefined control
% sequence", accept the common ones: define them as no-ops that gobble their
% arguments, and the usual page-layout dimensions / pdfTeX counters as registers,
% so a document gets past its preamble to the body. This is best-effort robustness,
% NOT package emulation — a command that genuinely draws (a TikZ picture) still
% cannot render, it just no longer aborts the preamble. (Derived from an arXiv
% compatibility study: these were the commands most often blocking real papers.)
\def\makeatletter{\catcode64=11\relax}
\def\makeatother{\catcode64=12\relax}
\newcount\pdfoutput
\newcount\pdfminorversion
\newdimen\voffset\newdimen\hoffset
\newdimen\topmargin\newdimen\oddsidemargin\newdimen\evensidemargin
\newdimen\headheight\newdimen\headsep\newdimen\footskip
% \textheight is the page's text height, i.e. the page builder's \vsize — the
% same identity \textwidth has with \hsize above. The standard classes size the
% text block by assigning \textheight (\setlength\textheight{\@tempcnta
% \baselineskip}\addtolength\textheight{\topskip}); aliasing it to \vsize makes
% that assignment set the page-break budget, instead of leaving \vsize at the
% plain-TeX 8.9in default and overfilling every page.
\let\textheight\vsize
\newdimen\marginparwidth\newdimen\marginparsep\newdimen\marginparpush
\def\em{\it}
\def\normalem{}
\def\ULforem{}
\def\useunder{}
\def\sloppy{}
\def\fussy{}
\def\raggedbottom{}
\def\flushbottom{}
\def\MFUhyphentrue{}
\def\allowdisplaybreaks{\@ifnextbracket\@gobbleoptonly\relax}
\def\@gobbleoptonly[#1]{}
\def\hypersetup#1{}
% xcolor's colour-model machinery, as a package reads it. This engine keeps
% colours in RGB rather than xcolor's model tables, but a package that draws asks
% which model is in force so it can pick the right device (pgf does this to choose
% between RGB, CMYK and gray shadings). Answering "rgb" is both true here and what
% lets those packages load at all — the TikZ fadings library stops on the very
% first of these names otherwise.
% \color hands the colour on to the drawing package when one is loaded and a
% picture is open, so a mark it puts on the page is coloured like the text
% around it (this is how TikZ's \draw[red] shorthand reaches the driver — it
% asks the colour package, not pgf, to set the colour).
\def\gotex@pgfcolor#1{\ifdefined\pgfsetcolor\ifpgfpicture\pgfsetcolor{#1}\fi\fi}
\def\XC@sdef#1#2{\edef#1{#2}}
\def\XC@tgt@mod#1{rgb}
\def\XC@mod@rgb{rgb}
\def\XC@mod@cmyk{cmyk}
\def\XC@mod@gray{gray}
% \extractcolorspec{name}{\cmd} hands back a colour as {model}{values}, and
% \convertcolorspec converts one to another model. Both read the stored form this
% engine publishes for every named colour (see colorbridge.go); since that form is
% RGB, a conversion to RGB is the value itself, and any other target is answered
% in RGB — the only model this engine has.
\def\extractcolorspec#1#2{%
  \expandafter\ifx\csname\string\color@#1\endcsname\relax
    \def#2{{rgb}{0,0,0}}%
  \else
    \expandafter\expandafter\expandafter\gotex@extractspec
    \csname\string\color@#1\endcsname{#2}%
  \fi}
\def\gotex@extractspec#1#2#3#4#5#6{\def#6{{#4}{#5}}}
\def\convertcolorspec#1#2#3#4{\def#4{#2}}
% The pgf system layer picks its driver from \pgfsysdriver when one is already
% defined, so naming this engine's own (texmf/pgfsys-gotex.def) here is what makes
% the real pgf/TikZ draw rather than load and define nothing. Harmless when pgf is
% never loaded.
\def\pgfsysdriver{pgfsys-gotex.def}
\def\usetikzlibrary#1{}
\def\pgfplotsset#1{}
\def\tikzset#1{}
\def\lstset#1{}
\def\microtypesetup#1{}
\def\microtypecontext#1{}
\def\zcsetup#1{}
\def\setdisplayskipstretch#1{}
\def\bibpunct#1{}
\def\thanks#1{}
\def\address#1{}
\def\email#1{}
\def\keywords#1{}
\def\AtBeginDocument#1{}
\def\AtEndDocument#1{}
\def\newboolean#1{}
\def\setboolean#1#2{}
\def\numberwithin#1#2{}
\def\newaliascnt#1#2{}
% \DeclareMathOperator{\Aut}{Aut} now DEFINES \Aut as \operatorname{Aut} (the
% starred form uses \operatorname* for display limits). In math the raw scanner
% emits \Aut verbatim; go-tex/math does not know it, but the math-source macro
% resolver expands the zero-parameter \Aut to \operatorname{Aut}, which it renders
% — so a paper's custom operators typeset instead of being dropped.
\def\DeclareMathOperator{\@ifstar\@gtxdeclmathopstar\@gtxdeclmathop}
\def\@gtxdeclmathop#1#2{\def#1{\operatorname{#2}}}
\def\@gtxdeclmathopstar#1#2{\def#1{\operatorname*{#2}}}
% \DeclarePairedDelimiter\cmd{L}{R} (mathtools) defines \cmd{x} as the auto-sized
% \left L x \right R — real papers use it for \abs \norm \ceil \floor \set. It is a
% plain one-argument macro so the math resolver can expand it textually (a runtime
% \@ifstar for the * variant cannot survive that string substitution). The auto-size
% \left…\right already matches what the starred form asks for.
\def\DeclarePairedDelimiter#1#2#3{\newcommand#1[1]{\left#2 ##1\right#3}}
\def\SetKwInput#1#2{}
\def\algnewcommand#1#2{}
\def\setlist{\@ifnextbracket\@setlistopt\@setlistarg}
\def\@setlistopt[#1]#2{}
\def\@setlistarg#1{}
\def\RequirePackage{\usepackage}
\def\and{\quad}
\def\affil#1{}
\def\crefname#1#2#3{}
\def\Crefname#1#2#3{}
\def\urlstyle#1{}
\def\urladdr#1{}
\def\aliascntresetthe#1{}
\def\subjclass{\@ifnextbracket\@subjclassopt\@subjclassarg}
\def\@subjclassopt[#1]#2{}
\def\@subjclassarg#1{}
% Break hints and math-boldness / spacing switches: this engine does its own
% page and line breaking and has no bold-math or spacing modes, so these are
% accepted as no-ops. \pagebreak & co. gobble their optional [priority].
\def\pagebreak{\@ifnextbracket\@gobbleoptonly\relax}
\def\nopagebreak{\@ifnextbracket\@gobbleoptonly\relax}
\def\linebreak{\@ifnextbracket\@gobbleoptonly\relax}
\def\nolinebreak{\@ifnextbracket\@gobbleoptonly\relax}
\def\boldmath{}
\def\unboldmath{}
\def\frenchspacing{}
\def\nonfrenchspacing{}
% \qedhere (amsthm): the end-of-proof square requested mid-line/mid-display is
% dropped here (the proof's trailing \qed still sets the mark).
\def\qedhere{}
% \footnotetext[n]{text} and \newcolumntype{x}[n]{spec}: accepted and gobbled
% whole (optional [.] plus the required group) instead of leaking their bodies.
\def\@gobbleoptarg[#1]#2{}
\def\footnotetext{\@ifnextbracket\@gobbleoptarg\@gobble}
\def\newcolumntype#1{\@ifnextbracket\@gobbleoptarg\@gobble}
% Body-level commands seen across the corpus that, left undefined, DROP real
% content rather than mere configuration (from the skip census):
% \ensuremath{x} typesets x in math — undefined, its argument fell into text mode
% and its sub/superscripts were lost; \texorpdfstring{tex}{pdf} keeps the TeX form
% (hyperref bookmarks); \xspace is a smart space that here is a harmless no-op.
\def\ensuremath#1{\ifmmode#1\else$#1$\fi}
\def\texorpdfstring#1#2{#1}
\def\xspace{}
% algorithmicx / algpseudocode (algorithm bodies in CS papers): best-effort so the
% pseudocode reads as lines with keywords instead of being dropped command-by-
% command. Not the real indented layout — each control word starts a line and
% prints its keyword; conditions/bodies that follow render as ordinary text.
\def\State{\par\noindent}
\def\Statex{\par\noindent}
\def\Require{\par\noindent\textbf{Require: }}
\def\Ensure{\par\noindent\textbf{Ensure: }}
\def\Input{\par\noindent\textbf{Input: }}
\def\Output{\par\noindent\textbf{Output: }}
\def\Return{\textbf{return} }
\def\Comment#1{}
\def\If#1{\par\noindent\textbf{if} #1 \textbf{then}}
\def\ElsIf#1{\par\noindent\textbf{else if} #1 \textbf{then}}
\def\Else{\par\noindent\textbf{else}}
\def\EndIf{}
\def\For#1{\par\noindent\textbf{for} #1 \textbf{do}}
\def\ForAll#1{\par\noindent\textbf{for all} #1 \textbf{do}}
\def\EndFor{}
\def\While#1{\par\noindent\textbf{while} #1 \textbf{do}}
\def\EndWhile{}
\def\Repeat{\par\noindent\textbf{repeat}}
\def\Until#1{\par\noindent\textbf{until} #1}
\def\Loop{\par\noindent\textbf{loop}}
\def\EndLoop{}
\def\Function#1#2{\par\noindent\textbf{function} #1(#2)}
\def\EndFunction{}
\def\Procedure#1#2{\par\noindent\textbf{procedure} #1(#2)}
\def\EndProcedure{}
% ─── end real-world preamble robustness ──────────────────────────────────────
`

// LoadLaTeX loads the Plain macros (if not already) and the minimal LaTeX kernel.
func (e *Engine) LoadLaTeX() error {
	if err := e.LoadPlain(); err != nil {
		return err
	}
	if err := e.LoadFormat(MiniLaTeXKernel); err != nil {
		return err
	}
	if err := e.LoadFormat(LaTeX2eKernelHelpers); err != nil {
		return err
	}
	if err := e.LoadFormat(LaTeXHooks); err != nil {
		return err
	}
	if err := e.LoadFormat(LaTeX2eClassKernel); err != nil {
		return err
	}
	if err := e.LoadFormat(LaTeX2eClassLead); err != nil {
		return err
	}
	return e.LoadFormat(AMSClassSubstrate)
}

// doNewcommand implements LaTeX's \newcommand / \renewcommand / \providecommand:
//
//	\newcommand{\name}[nargs][default]{body}   (braces optional around \name)
//
// It defines \name as a macro of nargs undelimited parameters (#1…#nargs in the
// body). When a second bracket [default] is present, the FIRST of the nargs
// parameters becomes optional: at call time #1 comes from a bracketed [..]
// argument if one is supplied, otherwise from default (standard LaTeX semantics).
func (e *Engine) doNewcommand() { e.doNewcommandMode(false) }

// doProvidecommand implements \providecommand: it reads and consumes the same
// grammar as \newcommand but only defines \name when it is not already defined,
// so a fallback definition never clobbers a real one.
func (e *Engine) doProvidecommand() { e.doNewcommandMode(true) }

func (e *Engine) doNewcommandMode(provide bool) {
	e.peekStar() // \newcommand* / \renewcommand* / \providecommand* (short form): consume the *
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
	body := e.scanBody() // always consume the body, even when we won't (re)define
	if provide && name != "" && e.eq[name] != nil {
		return // \providecommand: keep the existing definition
	}
	var params []tok
	for i := 1; i <= nargs && i <= 9; i++ {
		params = append(params, tok{ch: rune('0' + i), cat: catParam})
	}
	if name != "" {
		m := &meaning{
			kind: mMacro, params: params, body: body,
			optArg: optArg && nargs >= 1, optDefault: optDefault,
		}
		e.define(name, m, false)
		// A command WITH an optional argument is a "robust" LaTeX command: latex.ltx
		// splits it into a front end (\name) and an internal (the control sequence
		// literally named "\name", reached as \csname\string\name\endcsname) that holds
		// the [#1]#2… body. Class code rebinds the front end to a wrapper that calls
		// that internal — amsart does exactly this for \title / \author
		// (\edef\title{\@dblarg\@xp\@nx\csname\string\title\endcsname}). Bind the
		// internal to the same body so the wrapper resolves to real code instead of an
		// undefined \relax (which would leak the arguments and derail \maketitle).
		if m.optArg {
			e.define("\\"+name, m, false)
		}
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
		// The count may arrive BRACED — \newcommand\foo[{1}]… — because the caller
		// built the call by expansion rather than typing it. TeX strips the braces
		// of an argument that is entirely one group, so read the bracket's content
		// and take the number from what is left.
		// The '[' goes back so the bracket reader (which applies TeX's
		// brace-stripping rule) can take the whole thing; it is there, so the
		// reader always finds it.
		e.back(t)
		toks, _ := e.scanOptBracketToks()
		n := 0
		neg := false
		for _, u := range toks {
			switch {
			case u.cs_:
			case u.ch == '-':
				neg = !neg
			case u.ch >= '0' && u.ch <= '9':
				n = n*10 + int(u.ch-'0')
			}
		}
		if neg {
			n = -n
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
				return stripOuterGroup(toks), true
			}
			toks = append(toks, u)
		}
	}
	e.back(t)
	return nil, false
}

// stripOuterGroup removes the braces of an argument that is ENTIRELY one group,
// which is what TeX does when it grabs a delimited parameter (§399): \def\d[#1]{}
// called as \d[{g}] sees "g", while \d[{a}{b}] — two groups, not one enclosing
// them — keeps its braces. Checked against real TeX.
//
// The engine reads [optional arguments] in Go rather than through a delimited
// macro, so the rule has to be applied here. It matters at once for
// \newcommand: the kernel's own \@testopt hands the default over BRACED, and
// beamer builds a definition as \newcommand\foo[{1}][{}]{…} through a chain of
// \expandafter — with the braces left on, the argument count was not a number and
// \foo came out as a macro with no arguments at all, which put the body of every
// beamer template on the page instead of storing it.
func stripOuterGroup(toks []tok) []tok {
	if len(toks) < 2 || toks[0].cs_ || toks[0].cat != catBegin {
		return toks
	}
	depth := 0
	for i, t := range toks {
		switch {
		case !t.cs_ && t.cat == catBegin:
			depth++
		case !t.cs_ && t.cat == catEnd:
			depth--
			if depth == 0 {
				if i != len(toks)-1 {
					return toks // the group ends before the argument does
				}
				return toks[1 : len(toks)-1]
			}
		}
	}
	return toks
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

// doGobbleEnv discards the body of a non-renderable environment (tikzpicture,
// pgfpicture, tikzcd, …) whole and leaves a framed placeholder box in its place.
// It is reached via \begin{name} — already expanded to the \name control sequence
// — so the input cursor sits just past it. See gobbleEnvBody.
func (e *Engine) doGobbleEnv(name string) {
	e.gobbleEnvBody(name, true)
}

// doGobbleEnvSilent discards an environment body whole and emits NOTHING — for
// content that must vanish entirely rather than reserve space: the comment
// package's \begin{comment}…\end{comment} (and any env made with
// \excludecomment). Typesetting such a body instead of gobbling it lets stray
// \item, \\, or an unbalanced brace inside the comment leak out and can leave a
// group open at end of document, dropping the page it sits on (a whole-body
// swallow on real papers).
func (e *Engine) doGobbleEnvSilent(name string) {
	e.gobbleEnvBody(name, false)
}

// gobbleEnvBody consumes tokens up to and including the matching \end{name},
// honouring nested \begin{name}…\end{name} of the same name. The scan is at the
// token level (via getNext), so it works whether the environment sits in the raw
// source or is replayed from a macro/box body; nothing from the body is emitted,
// so no command inside it can leak into the typeset output. When placeholder is
// true a framed box is left where the environment sat; when false nothing is.
func (e *Engine) gobbleEnvBody(name string, placeholder bool) {
	depth := 1
	for {
		t, ok := e.getNext()
		if !ok {
			return // unterminated: input exhausted
		}
		if !t.cs_ {
			continue
		}
		switch t.cs {
		case "begin":
			if e.gobbleEnvName() == name {
				depth++
			}
		case "end":
			if e.gobbleEnvName() == name {
				depth--
				if depth == 0 {
					if placeholder {
						e.emitPicturePlaceholder(name)
					}
					return
				}
			}
		}
	}
}

// registerExcludedComment makes `name` a silently-gobbled environment: its
// \begin{name} triggers doGobbleEnvSilent(name) and its \end is consumed
// literally by that scan (the endname prim is a no-op, defined for safety).
func (e *Engine) registerExcludedComment(name string) {
	e.prim(name, func(e *Engine) { e.doGobbleEnvSilent(name) })
	e.prim("end"+name, func(e *Engine) {})
}

// grabEnvNameArg reads a required {name} argument as a plain string (the
// environment name for \excludecomment / \includecomment), or "" if the next
// token is not an opening brace (input left untouched in that case).
func (e *Engine) grabEnvNameArg() string {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return ""
	}
	return trimSpaces(e.toksToString(e.grabGroup()))
}

// emitPicturePlaceholder stands a gobbled picture environment (TikZ/PGF/tikz-cd)
// in for the diagram it would have drawn: a framed empty box of a modest, fixed
// figure size. Reserving this space — rather than emitting nothing — keeps the
// surrounding text flowing where the real diagram sat, which is both more
// faithful than a blank and keeps pagination stable.
func (e *Engine) emitPicturePlaceholder(name string) {
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	e.skippedCS[name]++
	e.startImage()
	inner := &boxNode{kind: hbox, width: 96 * unity, height: 60 * unity}
	e.parList = append(e.parList, frameNode{inner: inner, sep: fboxSep, rule: fboxRule})
}

// gobbleEnvName consumes a following {name} group (skipping leading spaces) and
// returns the environment name it spells. It always consumes the group so the
// caller can discard it; a missing group yields "" with the token pushed back.
func (e *Engine) gobbleEnvName() string {
	t, ok := e.getNext()
	for ok && t.cat == catSpace && !t.cs_ {
		t, ok = e.getNext()
	}
	if !ok {
		return ""
	}
	if !(t.cat == catBegin && !t.cs_) {
		e.back(t)
		return ""
	}
	var sb strings.Builder
	depth := 1
	for {
		u, ok := e.getNext()
		if !ok {
			break
		}
		if u.cs_ {
			continue
		}
		if u.cat == catBegin {
			depth++
		} else if u.cat == catEnd {
			if depth--; depth == 0 {
				break
			}
		} else {
			sb.WriteRune(u.ch)
		}
	}
	return sb.String()
}
