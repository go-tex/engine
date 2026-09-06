// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file adds the low-level TeX primitives a real LaTeX class file (article.cls
// and friends, loaded by packages.go) needs beyond what the engine already had:
// \ifdim, \divide, \newbox, \leavevmode, \everypar, \sfcode, \long, \endinput and
// the \hb@xt@ shorthand. They are registered by loadClassPrims (called from New).
// The higher-level class machinery a class builds from these (\@startsection,
// \list, …) is defined in TeX by LaTeX2eClassLead, loaded after the class kernel.

// loadClassPrims registers the primitives real class files rely on.
func (e *Engine) loadClassPrims() {
	// \ifdim <dimen><rel><dimen>: a dimension comparison, the \ifnum for lengths.
	e.prim("ifdim", func(e *Engine) { e.doIf(e.scanCond(e.evalIfdim)) })
	// \divide <reg> by <int>: the counterpart of \multiply.
	e.prim("divide", func(e *Engine) { e.doDivide(false) })
	// \newbox \cs: allocate a box register bound to \cs (plain TeX's bare-cs form of
	// \newsavebox{\cs}).
	e.prim("newbox", func(e *Engine) {
		name := e.scanCSName()
		if name == "" || e.allocBox >= 256 {
			return
		}
		if m := e.eq[name]; m != nil && m.kind == mBoxRef {
			return
		}
		e.define(name, &meaning{kind: mBoxRef, code: e.allocBox}, true)
		e.allocBox++
	})
	// \leavevmode: switch to horizontal mode (start a paragraph) if in vertical mode,
	// so a following \hbox/\rule/box attaches to a line.
	e.prim("leavevmode", func(e *Engine) {
		if !e.inPar {
			e.beginParagraph(false)
		}
	})
	// \everypar{toks} / \everypar=<toks|\toksreg>: the paragraph-start token list,
	// fired by beginParagraph. The value may be a braced group or another toks
	// register (amsart uses \everypar\dth@everypar), so read it like any toks
	// assignment; the setting is group-scoped so a list restores it at \endgroup.
	e.prim("everypar", func(e *Engine) {
		e.setEverypar(e.readToksValue())
	})
	// \sfcode<charcode>=<int>: space-factor code assignment. No space-factor model
	// yet; consume the assignment as a no-op.
	e.prim("sfcode", func(e *Engine) {
		e.scanInt()
		e.scanEquals()
		e.scanInt()
	})
	// \long: a \def prefix for macros whose arguments may contain \par. The engine
	// does not distinguish long/short macros, so \long is an accepted no-op prefix.
	// \long is a PREFIX: tex.web §1211 accumulates it and §1218 stores it in the
	// macro's command code. It was a no-op here, so \long\def produced an ordinary
	// macro and the \par check of §392 could never fire.
	e.prim("long", func(e *Engine) { e.pendingLong = true })
	// \outer: like \long, an accepted no-op prefix.
	e.prim("outer", func(e *Engine) {})
	// \endinput: stop reading the current file. The splicer appends an end
	// sentinel after each \input file (\gotexendinput, io.go) and each
	// \usepackage/\documentclass file (\@endofpackagehook / \@endofclasshook,
	// packages.go). Skip forward to the nearest one so text AFTER a file's
	// \endinput — trailing documentation, cite.sty's "Test file integrity: …"
	// line with its stray \] and ^_ characters — is not processed (which can hang
	// the engine). Jumping to the package/class hook (not past it) keeps any
	// \AtEndOfPackage/Class code and the frame-popping \@gotex@endload intact.
	e.prim("endinput", func(e *Engine) {
		best := -1
		for _, m := range fileEndMarkers {
			if i := runeIndexFrom(e.base, e.bpos, m); i >= 0 && (best < 0 || i < best) {
				best = i
			}
		}
		if best >= 0 {
			e.bpos = best
		}
	})
	// \hb@xt@: LaTeX's shorthand for "\hbox to".
	e.eq["hb@xt@"] = &meaning{kind: mMacro, body: []tok{csTok("hbox"), chTok(' ', catSpace), chTok('t', catLetter), chTok('o', catLetter)}}
}

// evalIfdim evaluates \ifdim's <dimen><rel><dimen> comparison.
func (e *Engine) evalIfdim() bool {
	a := e.scanDimen()
	e.skipOptSpace()
	rel, ok := e.getXToken()
	b := 0
	if ok {
		b = e.scanDimen()
	}
	switch {
	case rel.is('<', catOther):
		return a < b
	case rel.is('>', catOther):
		return a > b
	case rel.is('=', catOther):
		return a == b
	}
	return false
}

// doDivide handles \divide <reg> by <int> for count, dimen and skip registers.
func (e *Engine) doDivide(global bool) {
	if t, ok := e.getXToken(); ok {
		if e.scaleEngineParam(t, true, global) {
			return
		}
		e.back(t)
	}
	if i, ok := e.countIndex(); ok {
		e.skipByKeyword()
		if v := e.scanInt(); v != 0 {
			e.setCount(i, e.count[i]/v, global)
		}
		return
	}
	if i, ok := e.dimenIndex(); ok {
		e.skipByKeyword()
		if v := e.scanInt(); v != 0 {
			e.setDimen(i, e.dimen[i]/v, global)
		}
		return
	}
	if i, ok := e.skipIndex(); ok {
		e.skipByKeyword()
		if v := e.scanInt(); v != 0 {
			g := e.skip[i]
			g.width /= v
			g.stretch /= v
			g.shrink /= v
			e.setSkip(i, g, global)
		}
	}
}

// LaTeX2eClassLead is the best-effort high-level class machinery a real class builds
// on top of the kernel: sectioning (\@startsection and its helpers), the generic
// \list, and small no-ops. It is intentionally simplified — headings are typeset in
// their requested style without the exact spacing/numbering of real LaTeX — so that
// loading a real class produces readable structured output rather than aborting. It
// is loaded after LaTeX2eClassKernel.
const LaTeX2eClassLead = `
% Fallbacks so this layer is self-contained: the class-kernel substrate (loaded
% first) normally defines these font/spacing/symbol commands; \providecommand only
% fills in a safe default when the substrate is absent, and never overrides it.
\providecommand{\normalfont}{}
\providecommand{\bfseries}{}
\providecommand{\itshape}{}
\providecommand{\Large}{}
\providecommand{\large}{}
\providecommand{\medskip}{\par}
\providecommand{\textbullet}{*}
% ── sectioning ───────────────────────────────────────────────────────────────
% \@startsection{name}{level}{indent}{beforeskip}{afterskip}{style} then *|[toc]|{title}.
% The unstarred form NUMBERS the heading: when level<=secnumdepth it steps the
% name's counter and prefixes \the<name>; the starred form is unnumbered. The
% optional [toc-title] is accepted and ignored.
%
% The beforeskip (#4) and afterskip (#5) are the real vertical space a heading
% consumes and MUST be honoured, not replaced by a fixed \medskip: article.cls
% passes \section a beforeskip of 3.5ex (≈15pt) and an afterskip of 2.3ex (≈10pt),
% so a heading spaced with a rigid 6pt \medskip and nothing after saves ~19pt —
% and a section-heavy article (dozens of headings) then packs several extra pages
% of them, under-paginating against tectonic. beforeskip is applied here as
% vertical space above the heading (its sign is LaTeX's indent flag, so the space
% is its magnitude); afterskip is threaded through to \@gxhead, which puts it below
% a display heading (positive) or runs the text in beside the heading (negative).
\def\@startsection#1#2#3#4#5#6{\par
  \@tempskipa#4\relax
  \ifdim\@tempskipa<\z@ \@tempskipa-\@tempskipa\fi
  \vskip\@tempskipa
  \@ifstar{\@gxsec{#1}{#2}{#6}{#5}\@gxstar}{\@gxsec{#1}{#2}{#6}{#5}\@gxnum}}
\def\@gxsec#1#2#3#4#5{\@ifnextchar[{\@gxsecopt{#1}{#2}{#3}{#4}{#5}}{\@gxsecplain{#1}{#2}{#3}{#4}{#5}}}
\def\@gxsecopt#1#2#3#4#5[#6]#7{#5{#1}{#2}{#3}{#4}{#7}}
\def\@gxsecplain#1#2#3#4#5#6{#5{#1}{#2}{#3}{#4}{#6}}
\def\@gxstar#1#2#3#4#5{\@gxhead{#3}{#5}{#4}}
\def\@gxnum#1#2#3#4#5{\ifnum#2>\c@secnumdepth\@gxhead{#3}{#5}{#4}\else\refstepcounter{#1}\@tocentry{toc}{#2}{\csname the#1\endcsname}{#5}\@gxhead{#3}{\csname the#1\endcsname\quad#5}{#4}\fi}
% \@gxhead{style}{title}{afterskip}: set the heading, then its trailing space. A
% positive afterskip is a display heading — end the line and skip that far down; a
% negative one is a run-in heading — leave horizontal space and let the body text
% follow on the same line (\paragraph/\subparagraph).
\def\@gxhead#1#2#3{\@tempskipa#3\relax
  \ifdim\@tempskipa<\z@
    \noindent{#1 #2}\hskip-\@tempskipa
  \else
    \noindent{#1 #2}\par\vskip\@tempskipa
  \fi}
\def\@afterheading{}
% \secdef\CMDA\CMDB: the unstarred branch goes through \@dblarg so a command with an
% optional argument (\chapter/\part's \@chapter[#1]#2) receives its mandatory title
% as the optional one too — without this, \chapter{T} mis-parses (\@chapter would
% scan for a '[' that is not there and drop the title).
\def\secdef#1#2{\@ifstar{#2}{\@dblarg{#1}}}
\long\def\@dblarg#1{\@ifnextchar[{#1}{\@xdblarg{#1}}}
\long\def\@xdblarg#1#2{#1[{#2}]{#2}}
\def\@dottedtocline#1#2#3#4#5{}
\def\@textsuperscript#1{#1}
% \textsuperscript / \textsubscript: the user-level commands (only the internal
% \@textsuperscript was defined). Undefined, they were skipped and their content —
% the "2" in mc\textsuperscript{2}, footnote marks, "st"/"nd" ordinals — silently
% dropped. Render through the math layer as a raised/lowered script, which both
% preserves the text and sets it small and shifted as LaTeX does.
\def\textsuperscript#1{\ensuremath{^{#1}}}
\def\textsubscript#1{\ensuremath{_{#1}}}
% \text (amsmath) used in TEXT mode: the math layer handles \text inside $…$/\[…\]
% from the raw source, but a \text{…} written in ordinary text reached execCS
% undefined and was skipped, dropping its words. Typeset the argument in place. In
% math this definition is never consulted (the source goes to the math layer).
\def\text#1{#1}
% \@thanks accumulates \thanks footnotes for \maketitle; \thanks is best-effort
% (its note is dropped). Defining them keeps a real class's \maketitle from
% aborting on \@thanks. (\@starttoc is a Go primitive bridging to the engine's
% two-pass contents table, so it is not defined here.)
\def\@thanks{}
\def\thanks#1{}
% ── float environments (figure/table via the class's \@float) ────────────────
% \@float{type}[placement] starts a centred float block and records \@captype so
% \caption numbers it; \end@float closes it. \@dblfloat is the two-column (figure*/
% table*) form: it routes to \gotex@dblfloat (twocolumn.go), which in two-column mode
% typesets the float at the FULL text width and places it as a band spanning BOTH columns
% (\@dblfloat / \@topnewpage), and in one column falls back to the ordinary one-column
% float \@float. This is the hook the embedded article.cls's figure*/table* reach; the
% emulated classes (revtex, IEEEtran …) that lack a starred form reach the same
% \gotex@dblfloat through the substrate alias (amssubstrate.go). A real bundled class
% that defines its own \@dblfloat overrides this default and keeps its own.
\def\@float#1{\par\medskip\begingroup\centering\def\@captype{#1}\@ifnextchar[\@gobbleopt\relax}
\def\end@float{\par\endgroup\medskip}
\def\@dblfloat#1{\gotex@dblfloat{#1}}
\def\end@dblfloat{\end@float}
\def\usecounter#1{}
% \twocolumn / \onecolumn are Go primitives (see twocolumn.go / primitives.go): under
% the two-column opt-in they switch the page column mode, otherwise they gobble the
% optional [span] and do nothing (the historical stub behaviour).
\def\@gobbleopt[#1]{}
% ── \[ \] as robust commands (space-suffixed internal names) ─────────────────
% In real LaTeX \[ and \] are robust: \[ expands to \protect\[<space>, and the
% space-suffixed control words hold the actual body (\relax\ifmmode\@badmath\else
% $$\fi). The engine drives display math from a \[ primitive instead, but class code
% introspects the robust form: amsart's amsthm QED patch does
%   \expandafter\ifx\csname[ \endcsname\relax \expandafter\@tempa\[\@nil\[ \else …\fi
% to splice \def\@currenvir{displaymath} into \['s body by splitting it at its $. When
% \csname[ \endcsname is undefined the then-branch runs \@tempa\[\@nil\[, which — with
% \[ a non-expandable primitive rather than a $$-bearing macro — mis-scans and leaks a
% stray $…\def\@currenvir{displaymath} into the stream, opening math that swallows the
% following input (a lone \input body after \maketitle). Defining the space-suffixed
% forms makes \ifx…\relax false, so the else-branch patches the (inert) decoy \[<space>
% by splitting its real $$ body, and the display-math primitive \[ is never touched.
\expandafter\def\csname[ \endcsname{\relax\ifmmode\@badmath\else$$\fi}
\expandafter\def\csname] \endcsname{\relax\ifmmode$$\else\@badmath\fi}
% ── generic list (best-effort: each item on its own line with its label) ─────
% \list carries the vertical space LaTeX's list machinery puts around it —
% \topsep above and below — which \@trivlist alone does not. Without it every
% environment a class builds on \list came out with no separation at all: the
% embedded article.cls defines description as \list{}{…}, and its items sat a bare
% baseline apart (13.6pt) where real LaTeX gives 25.5 above, 22.5 between and 25.5
% below. itemize/enumerate never showed this because the kernel macros in latex.go
% serve them and already add \topsep themselves.
%
% The space goes on \list, NOT on \@trivlist: a theorem and amsart's author block
% run through \@trivlist directly and already match the reference.
\def\list#1#2{\par\addvspace\topsep\@trivlist}
% \endlist ends the innermost list by ending its trivlist, as ltlists.dtx does —
% \def\endlist{\global\advance\@listdepth\m@ne \endtrivlist}. That chain is what a
% class hooks: beamer patches \endtrivlist to run \beamer@closeitem, which closes the
% overlay wrappers (\begin{actionenv}\begin{uncoverenv}\begin{altenv}) that its LAST
% \item left open — every earlier item is closed by the NEXT \item. With \endlist a bare
% \par those three stayed open past \end{itemize}, and every \end after them closed one
% group too high.
\def\endlist{\endtrivlist\par\addvspace\topsep}
% \trivlist opens a group so a real class's redefined \trivlist (amsart's
% \maketitle author block: \trivlist … \item\relax … \endtrivlist, which calls
% \@trivlist) contains its material and CLOSES cleanly at \endtrivlist instead of
% leaving an undefined \@trivlist that swallows the rest of the document. \list is
% deliberately left ungrouped here (the corpus's article itemize/enumerate are
% sensitive to \list's exact behavior); only the trivlist path is scoped.
% \@trivlist clears \@itemlabel: a trivlist item (amsart's \maketitle author block
% \item\relax, and a theorem's \trivlist) carries NO bullet — real \trivlist gives
% a bare \item an empty label. Scoped by the \begingroup so \@itemlabel reverts to
% its bullet default (for a stray \item outside any real itemize) at \endtrivlist.
\def\@trivlist{\par\begingroup\@listfirsttrue\def\@itemlabel{}}
\def\trivlist{\@trivlist}
\def\endtrivlist{\endgroup\par}
\def\@itemlabel{\textbullet}
\def\item{\@ifnextchar[{\@gotexitem}{\@gotexitem[\@itemlabel]}}
% \@iteminterspace puts \itemsep between items and nothing before the first (the
% \@listfirst flag \@trivlist raises). Without it every \list-built environment ran
% its items a bare baseline apart: description gave 13.6pt where real LaTeX gives
% 22.5. A trivlist with a single \item — a theorem, amsart's author block — is
% unaffected, the flag suppressing the space on the first one.
\def\@gotexitem[#1]{\par\@iteminterspace\noindent#1\ }
`
