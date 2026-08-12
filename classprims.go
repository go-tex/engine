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
	e.prim("ifdim", func(e *Engine) { e.doIf(e.evalIfdim()) })
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
	// \everypar{toks} / \everypar={toks}: the paragraph-start token list. The engine
	// has no everypar hook; accept and discard the assignment so a class can set it.
	e.prim("everypar", func(e *Engine) {
		e.scanEquals()
		e.readBraceToksRaw()
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
	e.prim("long", func(e *Engine) {})
	// \outer: like \long, an accepted no-op prefix.
	e.prim("outer", func(e *Engine) {})
	// \endinput: stop reading the current file. In the splice model the remaining
	// file text is already queued; accepting it as a no-op is close enough (real
	// files put \endinput at end of file).
	e.prim("endinput", func(e *Engine) {})
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
% The five layout arguments are accepted (only a little vertical space is used). The
% unstarred form NUMBERS the heading: when level<=secnumdepth it steps the name's
% counter and prefixes \the<name>; the starred form is unnumbered. The optional
% [toc-title] is accepted and ignored.
\def\@startsection#1#2#3#4#5#6{\par\@ifstar{\@gxsec{#1}{#2}{#6}\@gxstar}{\@gxsec{#1}{#2}{#6}\@gxnum}}
\def\@gxsec#1#2#3#4{\@ifnextchar[{\@gxsecopt{#1}{#2}{#3}{#4}}{\@gxsecplain{#1}{#2}{#3}{#4}}}
\def\@gxsecopt#1#2#3#4[#5]#6{#4{#1}{#2}{#3}{#6}}
\def\@gxsecplain#1#2#3#4#5{#4{#1}{#2}{#3}{#5}}
\def\@gxstar#1#2#3#4{\@gxhead{#3}{#4}}
\def\@gxnum#1#2#3#4{\ifnum#2>\c@secnumdepth\@gxhead{#3}{#4}\else\refstepcounter{#1}\@tocentry{toc}{#2}{\csname the#1\endcsname}{#4}\@gxhead{#3}{\csname the#1\endcsname\quad#4}\fi}
\def\@gxhead#1#2{\par\medskip\noindent{#1 #2}\par}
\def\@afterheading{}
\def\secdef#1#2{\@ifstar{#2}{#1}}
\def\@dottedtocline#1#2#3#4#5{}
\def\@textsuperscript#1{#1}
% \@thanks accumulates \thanks footnotes for \maketitle; \thanks is best-effort
% (its note is dropped). Defining them keeps a real class's \maketitle from
% aborting on \@thanks. (\@starttoc is a Go primitive bridging to the engine's
% two-pass contents table, so it is not defined here.)
\def\@thanks{}
\def\thanks#1{}
% ── float environments (figure/table via the class's \@float) ────────────────
% \@float{type}[placement] starts a centred float block and records \@captype so
% \caption numbers it; \end@float closes it. \@dblfloat is the two-column (figure*/
% table*) form, handled the same way (single column).
\def\@float#1{\par\medskip\begingroup\centering\def\@captype{#1}\@ifnextchar[\@gobbleopt\relax}
\def\end@float{\par\endgroup\medskip}
\def\@dblfloat#1{\@float{#1}}
\def\end@dblfloat{\end@float}
\def\usecounter#1{}
\def\twocolumn{\@ifnextchar[{\@gobbleopt}{}}
\def\onecolumn{}
\def\@gobbleopt[#1]{}
% ── generic list (best-effort: each item on its own line with its label) ─────
\def\list#1#2{\par}
\def\endlist{\par}
\def\trivlist{\par}
\def\endtrivlist{\par}
\def\item{\@ifnextchar[{\@gotexitem}{\@gotexitem[\textbullet]}}
\def\@gotexitem[#1]{\par\noindent#1\ }
`
