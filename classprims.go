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
		if name == "" {
			return
		}
		if e.allocBox >= registerPoolSize {
			e.noteRegisterExhausted(`\box`)
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
	e.prim("leavevmode", func(e *Engine) { e.leaveVMode() })
	// \everypar{toks} / \everypar=<toks|\toksreg>: the paragraph-start token list,
	// fired by beginParagraph. The value may be a braced group or another toks
	// register (amsart uses \everypar\dth@everypar), so read it like any toks
	// assignment; the setting is group-scoped so a list restores it at \endgroup.
	e.prim("everypar", func(e *Engine) {
		e.setEverypar(e.readToksValue())
	})
	// \sfcode<charcode>=<int>: the space-factor code of a character. It is READ
	// back too (\the\sfcode`\. and \ifnum\sfcode`\.=\@m, which is how LaTeX asks
	// whether \frenchspacing is in force), so it is stored, group-scoped, and
	// consulted by adjustSpaceFactor. See spacefactor.go.
	e.prim("sfcode", func(e *Engine) {
		r := rune(e.scanInt())
		e.scanEquals()
		e.setSfcode(r, e.scanInt(), false)
	})
	// \spacefactor: TeX's space_factor as an assignable internal integer. \@ is
	// \spacefactor\@m — the one thing that makes a period after a capital end a
	// sentence — and \- is \spacefactor\sfcode`\- (latex.ltx:6617).
	e.prim("spacefactor", func(e *Engine) {
		e.scanEquals()
		v := e.scanInt()
		if len(e.groups) > 0 {
			e.save = append(e.save, saveItem{kind: saveSpaceFactor, oldi: e.spaceFactor})
		}
		e.spaceFactor = v
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
			e.setCount(i, e.getCount(i)/v, global)
		}
		return
	}
	if i, ok := e.dimenIndex(); ok {
		e.skipByKeyword()
		if v := e.scanInt(); v != 0 {
			e.setDimen(i, e.getDimen(i)/v, global)
		}
		return
	}
	if i, ok := e.skipIndex(); ok {
		e.skipByKeyword()
		if v := e.scanInt(); v != 0 {
			g := e.getSkip(i)
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
% ⛔ AND IT CARRIES \@secpenalty, which it did not: ltsect.dtx:331 is
% \addpenalty\@secpenalty\addvspace\@tempskipa, and \@secpenalty is -300
% (:229-230). A heading contributed NO penalty node at all, so the page builder
% had no reason to prefer the break a reader would choose — above the heading —
% over one two lines into its text. Measured with \showlists against the
% reference, go-tex/engine#691. The penalty goes BEFORE the space, which is also
% the order the reference's own list shows.
%
% ⚠ The space stays a \vskip and is deliberately NOT changed to \addvspace,
% although the reference uses it: \addvspace takes the MAXIMUM of adjacent skips
% where \vskip adds them, that moves the corpus on its own, and it is
% go-tex/engine#690 with its own pull request and its own measured regression.
% One defect per change.
%
% ⚠ \@sect and \@ssect below get the same line. They are not reached THROUGH
% this macro — this one goes via \@gxsec (see the note on them) — so a class
% calling LaTeX's continuations directly would otherwise keep the old silence,
% and adding it in both places cannot double the penalty.
\def\@startsection#1#2#3#4#5#6{\par
  \@tempskipa\@tsbefore{#1}{#4}\relax
  \@afterindenttrue
  \ifdim\@tempskipa<\z@ \@tempskipa-\@tempskipa \@afterindentfalse\fi
  \addpenalty\@secpenalty
  \vskip\@tempskipa
  \@ifstar{\@gxsec{#1}{#2}{#6}{#5}\@gxstar}{\@gxsec{#1}{#2}{#6}{#5}\@gxnum}}
\def\@gxsec#1#2#3#4#5{\@ifnextchar[{\@gxsecopt{#1}{#2}{#3}{#4}{#5}}{\@gxsecplain{#1}{#2}{#3}{#4}{#5}}}
\def\@gxsecopt#1#2#3#4#5[#6]#7{#5{#1}{#2}{#3}{#4}{#7}}
\def\@gxsecplain#1#2#3#4#5#6{#5{#1}{#2}{#3}{#4}{#6}}
\def\@gxstar#1#2#3#4#5{\@gxhead{\@tsstyle{#1}{#3}}{\@tstitle{#1}{#5}}{\@tsafter{#1}{#4}}}
% A sectioning command sets the running MARK, which is how a head shows the
% section it is on: \@sect runs \csname #1mark\endcsname{#7} (latex.ltx:12796 and
% :12806, both the display and the run-in heading). An unnumbered-but-not-starred
% heading marks too, so the call sits in BOTH branches of the secnumdepth test —
% and in the numbered one it comes AFTER \refstepcounter, since the mark carries
% \the<name> and must see the section it names. The starred form does not mark, and
% \@gxstar accordingly leaves it alone.
%
% The marks default to \@gobble (latex.ltx:12870-12872), so this costs nothing
% until a page style redefines them; \ps@headings does
% \def\sectionmark##1{\markright{\MakeUppercase{…##1}}}.
\def\@gxnum#1#2#3#4#5{\ifnum#2>\c@secnumdepth\csname #1mark\endcsname{#5}\@gxhead{\@tsstyle{#1}{#3}}{\@tstitle{#1}{#5}}{\@tsafter{#1}{#4}}\else\refstepcounter{#1}\csname #1mark\endcsname{#5}\@tocentry{toc}{#2}{\csname the#1\endcsname}{#5}\@gxhead{\@tsstyle{#1}{#3}}{\@tslabel{#1}\@tstitle{#1}{#5}}{\@tsafter{#1}{#4}}\fi}
% titlesec hooks (\titleformat / \titlespacing store per-name macros, see
% classkernel.go). With nothing stored they expand to the class's own argument, so
% a document without titlesec sets exactly what it did before.
\def\@tsstyle#1#2{\@ifundefined{@tsfmt@#1}{#2}{\csname @tsfmt@#1\endcsname}}
\def\@tsbefore#1#2{\@ifundefined{@tsbef@#1}{#2}{\csname @tsbef@#1\endcsname}}
\def\@tsafter#1#2{\@ifundefined{@tsaft@#1}{#2}{\ifx\@tsrunin\csname @tsshape@#1\endcsname-\fi\csname @tsaft@#1\endcsname}}
\def\@tslabel#1{\@ifundefined{@tslab@#1}{\csname the#1\endcsname\quad}{\csname @tslab@#1\endcsname}}
\def\@tstitle#1#2{\@ifundefined{@tsbc@#1}{#2}{\csname @tsbc@#1\endcsname{#2}}}
% \@gxhead{style}{title}{afterskip}: set the heading, then its trailing space. A
% positive afterskip is a display heading — end the line and skip that far down; a
% negative one is a run-in heading — leave horizontal space and let the body text
% follow on the same line (\paragraph/\subparagraph).
%
% ⛔⛔ The \par goes INSIDE the style group, as LaTeX's own \@sect has it:
%
%	\begingroup #6\relax \@hangfrom{…\@svsec}\interlinepenalty\@M #8\@@par \endgroup
%
% so the heading's paragraph ends under the heading's \baselineskip and not the
% body's. It was outside the group, and that cost 5.978pt per heading,
% measured on a witness whose two ends are both body text so poppler's word-box
% bias cancels (AVANTSEC→APRESSEC: 48.819 against the reference's 54.795).
%
% ⭐ That error was CANCELLING most of another one of the opposite sign: 1ex was
% half the font size instead of the x-height, 16% too big, which made each
% ex-stated heading skip 4.00pt too generous (see exheight.go). The visible
% symptom was only 1.99pt per section, and fixing either alone makes headings
% worse than leaving both — go-tex/engine#620 has the arithmetic:
% −1.989 + 5.978 − 4.00 ≈ 0.
% ⛔⛔ NO SPACE BETWEEN #1 AND #2. It read {#1 <space> #2}, and that space is a real
% space TOKEN: #1 is a parameter, not a control word, so nothing absorbs it.
% Expanded in horizontal mode at the very start of the heading it became
% interword glue, and EVERY sectioning heading in every document began that far
% right of the margin. LaTeX has none — ltsect.dtx:418 is
% \@hangfrom{\hskip #3\relax\@svsec}. Measured against tectonic, leftmost glyph
% in points from the page edge:
%
%	heading           font    reference   before   after
%	\section          14.35   133.77      140.95   133.77
%	\subsection       11.96   133.77      139.75   133.77
%	\subsubsection     9.96   133.77      138.86   133.77
%	body lines         9.96   133.77      133.77   133.77
%
% The right edges land with them (185.66/185.67, 190.40/190.41, 206.51/206.51).
% go-tex/engine#833.
%
% ⚠ STILL MISSING, and deliberately not fixed here: \@startsection's INDENT
% argument. LaTeX passes it (\hskip #3 above); \@gxhead is only handed
% {style}{title}{afterskip} and never sees it. It is \z@ in every standard class,
% so nothing in the corpus moves, but a class that indents its headings is
% unobeyed. That is a signature change and wants its own witness.
\def\@gxhead#1#2#3{\@tempskipa#3\relax
  \ifdim\@tempskipa<\z@
    \noindent{#1#2}\hskip-\@tempskipa
  \else
% ⛔ \nobreak BETWEEN THE HEADING'S \par AND ITS AFTERSKIP. ltsect.dtx:483-493,
% \@xsect's display-heading branch, verbatim:
%
%	\ifdim \@tempskipa>\z@
%	    \par \nobreak
%	    \vskip \@tempskipa
%	    \@afterheading
%
% That \nobreak is a \penalty 10000 on the VERTICAL list, and it is what forbids
% a page break between a heading and the first line of its text — a typographic
% fault this engine could commit and the reference cannot. #691 measured the
% reference's list as hbox(heading), \penalty 10000, \glue 9.913(afterskip); ours
% had the box and the glue with nothing between them.
%
% ⚠ It belongs in the POSITIVE branch only. When the afterskip is negative the
% heading runs in beside its text and the reference takes the else branch
% instead, which sets \@nobreakfalse — there is no vertical break to forbid
% because there is no vertical space.
    \noindent{#1#2\par}\nobreak\vskip\@tempskipa
    \@afterheading
  \fi}
% \@sect and \@ssect: LaTeX's own sectioning continuations, which \@startsection
% calls once it has read the star, the optional toc title and the title. This
% engine's \@startsection goes through \@gxsec instead — but a CLASS may call
% \@sect directly, and the copernicus/EGU family does:
%
%	\def\section{...\@dblarg{\@sect{section}{1}{\z@}{...}{...}{...}}}
%
% Undefined, lenient mode skipped the command WITH ITS EIGHT ARGUMENTS, so every
% heading of such a document vanished — 2304.06058 came out with not one of its
% fifteen section titles, against a reference that numbers them 1 to 9.
%
% They are defined in terms of the same helpers \@startsection uses, so a heading
% set through them is spaced and numbered exactly like one set the other way. A
% class that defines its own (amsart does) overrides these, since the kernel is
% loaded first.
\def\@sect#1#2#3#4#5#6[#7]#8{\par
  \@tempskipa\@tsbefore{#1}{#4}\relax
  \@afterindenttrue
  \ifdim\@tempskipa<\z@ \@tempskipa-\@tempskipa \@afterindentfalse\fi
  \addpenalty\@secpenalty
  \vskip\@tempskipa
  \@gxnum{#1}{#2}{#6}{#5}{#8}}
\def\@ssect#1#2#3#4#5{\par
  \@tempskipa#2\relax
  \@afterindenttrue
  \ifdim\@tempskipa<\z@ \@tempskipa-\@tempskipa \@afterindentfalse\fi
  \addpenalty\@secpenalty
  \vskip\@tempskipa
  \@gxhead{#4}{#5}{#3}}
% \@afterheading (latex.ltx \@xsect): when the heading's beforeskip was negative
% (\@afterindentfalse) the paragraph after it is not indented — \section in article
% and report, where tectonic sets the first paragraph flush with the margin. The
% engine indented it, 15pt of width on the first line of every section.
%
% ⛔ NOW THE KERNEL'S, ltsect.dtx:615-627 (v1.1h), verbatim. The stand-in was
% \if@afterindent\else\gotexnoindentnext\fi: it removed the indent and did nothing
% else. LaTeX's also clears \@nobreak at the first paragraph, and amsart's \@sect
% sets \global\@nobreaktrue relying on exactly that. With the stand-in, \@nobreak
% stayed true through the section's whole first paragraph, so the next
% \@startsection took its \if@nobreak branch and dropped \addvspace: an amsart
% \subsection after a section's first paragraph sat .5\linespacing (5.98pt) too
% close to it, measured against tectonic. The first line after a heading also
% gets \clubpenalty\@M, the second paragraph \@clubpenalty back.
\def\@afterheading{%
  \@nobreaktrue
  \everypar{%
    \if@nobreak
      \@nobreakfalse
      \clubpenalty \@M
      \if@afterindent \else
        {\setbox\z@\lastbox}%
      \fi
    \else
      \clubpenalty \@clubpenalty
      \everypar{}%
    \fi}}
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
% ⛔ Un \thanks VIDE ne produit rien. eptcs.cls écrit \thanks\relax, et en faire
% une note donnait une marque et une ligne de pied que la référence n'a pas:
% 2311.08855 passait de 19 pages à 20. \relax autant que le vide.
\def\gotex@threlax{\relax}
\def\thanks#1{\def\gotex@th{#1}%
  \ifx\gotex@th\@empty\else\ifx\gotex@th\gotex@threlax\else
  \g@addto@macro\@thanks{\footnote{#1}}\fi\fi}
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
% wrapfig's \begin{wrapfigure}[<lines>]{<position>}{<width>} … and \begin{wraptable}.
% wrapfig.sty:40-43:
%
%	\def\wrapfloat#1{\def\@captype{#1}\@ifnextchar[\WF@wr{\WF@wr[]}}
%	\def\wrapfigure{\wrapfloat{figure}}
%	\def\wraptable{\wrapfloat{table}}
%
% The real thing narrows the paragraph beside the float so the text WRAPS around it.
% This engine cannot reshape a paragraph, so the three arguments are consumed and the
% body becomes an ordinary centred float of the stated WIDTH: the figure and its
% numbered caption are where they belong in the sequence, and the text that would have
% run alongside it runs below instead.
%
% Undefined, \begin{wrapfigure} resolved to \relax through \csname, the POSITION and the
% WIDTH were typeset on the page, and the body was set as running prose — a figure and
% its caption in the middle of a sentence. Eight corpus papers use it, seventeen times,
% and two use \wraptable.
%
% The width is honoured through a \parbox rather than ignored, because a wrapfigure is
% narrow by construction (0.3-0.5\textwidth is the norm) and a full-width float would
% make every one of them cost more vertical space than the real one does.
\def\wrapfigure{\@ifnextbracket\gotex@wrapfloatopt\gotex@wrapfloat}
\def\wraptable{\@ifnextbracket\gotex@wraptabopt\gotex@wraptab}
\def\gotex@wrapfloatopt[#1]#2#3{\@float{figure}\parbox{#3}\bgroup}
\def\gotex@wrapfloat#1#2{\@float{figure}\parbox{#2}\bgroup}
\def\gotex@wraptabopt[#1]#2#3{\@float{table}\parbox{#3}\bgroup}
\def\gotex@wraptab#1#2{\@float{table}\parbox{#2}\bgroup}
\def\endwrapfigure{\egroup\end@float}
\def\endwraptable{\egroup\end@float}
% \usecounter makes a \list NUMBERED (ltlists.dtx:1083): its items step #1 through
% \refstepcounter (see \gotex@listitem). It was a no-op, so thebibliography's
% \usecounter{enumiv} numbered nothing and \bibitem kept its own counter beside it.
\newif\if@noitemarg
\def\usecounter#1{\@nmbrlisttrue\def\@listctr{#1}\setcounter{#1}\z@}
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
%
% ⛔ AND \list THREW ITS SETTINGS AWAY (\def\list#1#2{\@trivlist}), which is where every
% \list-based environment states its margins — article.cls:
%
%	\newenvironment{quote}{\list{}{\rightmargin\leftmargin}\item\relax}{\endlist}
%
% so quote, quotation and verse had NO indentation and description no bold label.
% Measured against tectonic (testdata/parity/listmargin.tex): quote 133.77–477.50
% where the reference sets 158.67–452.57, verse's two lines merged into one.
% go-tex/engine#587; the \list body is #646's, re-measured on today's main.
%
% This is ltlists.dtx:552-573 in its own order: the per-depth defaults run FIRST
% (\@listi sets \leftmargin\leftmargini), then the settings argument, which can
% therefore override them; the indentation is a PARAGRAPH SHAPE, not \leftskip
% (\@trivlist zeroes \leftskip, :613).
%
% Two pieces of \@trivlist's are put here rather than in our \@trivlist, which theorems
% and the author block also use and which match the reference as they are:
%   - \@setpar{{\@@par}} (ltlists.dtx:617-625, without the missing-\item loop count):
%     the primitive \par runs in a group of its own, so its end-of-paragraph reset of
%     \parshape (tex.web:20760) is undone and the next paragraph keeps the list's shape.
%     That needs \parshape to be group-scoped, which it is now (setParShape).
%   - the item: \gotex@listitem lays the label out as \@item does (ltlists.dtx:1037-1052).
\newcount\@listdepth \@listdepth=0
\newdimen\@totalleftmargin \@totalleftmargin=0pt
\def\@toodeep{}% LaTeX's "Too deeply nested" (lterror.dtx:718); the depth is capped below
\def\@setpar#1{\def\par{#1}\def\@par{#1}}% ltpar.dtx:167
\def\@mklab#1{\hfil #1}% ltlists.dtx:902
\let\makelabel\@mklab
\def\list#1#2{%
  \ifnum \@listdepth >5\relax \@toodeep \else \global\advance\@listdepth\@ne \fi
  \rightmargin\z@ \listparindent\z@ \itemindent\z@
  \csname @list\romannumeral\the\@listdepth\endcsname
  \def\@itemlabel{#1}%
  \let\makelabel\@mklab
  \@nmbrlistfalse
  #2\relax
  \@trivlist
  % Again, after: OUR \@trivlist empties \@itemlabel (LaTeX's \trivlist does, its
  % \@trivlist does not), and without this every thebibliography entry lost its
  % [N] — the label is \@biblabel{\@arabic\c@enumiv}, the list's #1.
  \def\@itemlabel{#1}%
  \let\@gotexitem\gotex@listitem
  % \item as \list has it (ltlists.dtx, \item): no [label] raises \@noitemarg, which
  % is what lets a numbered list step its counter for that item and not for one
  % that names its own label.
  \def\item{\@ifnextchar[\gotex@listitem{\@noitemargtrue\gotex@listitem[\@itemlabel]}}%
  % What \@setpar{{\@@par}} defines, written out: this engine stops a non-\long
  % argument at anything MEANING \par, \@@par included, so passing {\@@par}
  % through \@setpar abandoned the call. tex.web stops at the \par TOKEN only
  % (:8035) — that fix is ready but held, because it unmasks a chain of acmart
  % heading defects (go-tex/engine#859); this does not depend on it.
  \def\par{{\@@par}}\def\@par{{\@@par}}%
  \parskip\parsep \parindent\listparindent
  \advance\linewidth -\rightmargin
  \advance\linewidth -\leftmargin
  \advance\@totalleftmargin \leftmargin
  \parshape \@ne \@totalleftmargin \linewidth
  \ignorespaces}
% \gotex@listitem is \@item's label layout (ltlists.dtx:1037-1052): back out of the
% indentation by \labelwidth and \labelsep, set the label in a box \labelwidth wide
% (its natural width when wider), step forward by \labelsep, with \itemindent first.
% For a quote's empty label the four cancel and the text starts at the list's left
% margin; description's \itemindent -\leftmargin puts its bold label at the page
% margin while the wrapped lines hang at the list's.
%
% ⛔ AND THE LABEL WAITS FOR THE PARAGRAPH, as \@item's does (ltlists.dtx:940-1017):
% the item stays in VERTICAL mode, the label goes into \@labels, and \everypar puts
% it at the start of the paragraph once one begins, taking the indentation box back
% off with \lastbox. Starting the paragraph here instead (a first draft did) typeset
% the space after "\item\relax", which LaTeX drops in vertical mode, and every first
% line came out one stretched interword space — 4.6 to 7.9bp — right of tectonic's.
% Left out of \@item: the page-break penalties (\@beginparpenalty, \@itempenalty,
% \@nbitem); the spacing between items is our \@iteminterspace.
\def\gotex@listitem[#1]{%
  \if@inlabel \indent\par \fi% a pending label with no text yet: give it its line
  \ifhmode \unskip\unskip\par \fi
  % The space, as \@item spaces it (ltlists.dtx:950-961) now that \list sets
  % \parskip\parsep: the paragraph's own \parskip brings the \parsep, so the first
  % item cancels it (\addvspace{-\parskip}, :956) and the others add \itemsep only
  % (:960). \@iteminterspace adds \parsep itself, which is right for itemize and
  % enumerate (they keep the outer \parskip) and counted it twice here.
  \if@listfirst \@listfirstfalse \vskip-\parskip \else \vskip\itemsep \fi
  \global\@inlabeltrue
  \everypar{%
    \if@inlabel
      \global\@inlabelfalse
      {\setbox\z@\lastbox \ifvoid\z@ \kern-\itemindent \fi}%
      \box\@labels
      \penalty\z@
    \fi
    \everypar{}}%
  % A numbered list steps its counter for an item with no [label] of its own
  % (ltlists.dtx:1024-1032), BEFORE the label is set, since the label reads it.
  \if@noitemarg \@noitemargfalse \if@nmbrlist \refstepcounter\@listctr \fi \fi
  \sbox\@tempboxa{\makelabel{#1}}%
  \global\setbox\@labels\hbox{%
    \unhbox\@labels
    \hskip\itemindent\hskip-\labelwidth\hskip-\labelsep
    \ifdim\wd\@tempboxa>\labelwidth \box\@tempboxa
    \else\hbox to\labelwidth{\unhbox\@tempboxa}\fi
    \hskip\labelsep}%
  \ignorespaces}
% \@centercr is \\ in centering environments and in verse (article.cls's verse does
% \let\\\@centercr): it ENDS THE PARAGRAPH (ltmiscen.dtx:1155-1187), so each verse
% line is a paragraph of its own, indented by \listparindent. Undefined, \\ was
% skipped and both verse lines of the witness were set as one.
\def\@centercr{\ifhmode \unskip\else \@nolnerr\fi
  \par\@ifstar{\nobreak\@xcentercr}\@xcentercr}
\def\@xcentercr{\addvspace{-\parskip}\@ifnextchar[\@icentercr\ignorespaces}
\def\@icentercr[#1]{\vskip #1\ignorespaces}
\providecommand\@nolnerr{}
% \endlist ends the innermost list by ending its trivlist, as ltlists.dtx does —
% \def\endlist{\global\advance\@listdepth\m@ne \endtrivlist}. That chain is what a
% class hooks: beamer patches \endtrivlist to run \beamer@closeitem, which closes the
% overlay wrappers (\begin{actionenv}\begin{uncoverenv}\begin{altenv}) that its LAST
% \item left open — every earlier item is closed by the NEXT \item. With \endlist a bare
% \par those three stayed open past \end{itemize}, and every \end after them closed one
% group too high.
\def\endlist{\global\advance\@listdepth\m@ne \endtrivlist}
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
% \@trivlist carries the vertical space, as latex.ltx does (l.11456): \@topsepadd
% is \topsep, plus \partopsep when the list starts in VERTICAL mode — the test comes
% before the \par, so a list opened mid-paragraph gets \topsep alone. \endtrivlist
% then closes with \addvspace\@topsepadd (through \@endparenv, l.11523).
%
% This was first written the other way round, with \addvspace\topsep on \list and
% nothing on \@trivlist, from measuring a description against tectonic. The
% measurement put it within 2.9pt and the source names that 2.9pt: \partopsep.
% ⭐ The space itself is \gotex@listtopspace (latex.go), which this line used to
% spell out. \itemize, \enumerate and \description each spelled it out too, and
% got it wrong — \par first, then \partopsep unconditionally — which cost 1.99pt
% at each end of every list that follows running text (#685). This copy was the
% CORRECT one, and having two is what let the other three diverge unnoticed, so
% there is one now and all four call it.
\def\@trivlist{\gotex@listtopspace\begingroup\@listfirsttrue\def\@itemlabel{}}
\def\trivlist{\@trivlist}
% \@endpetrue is \@endparenv's last word (ltlists.dtx), after the space: it is
% what makes \end run \@doendpe, so the next line is flush unless a blank line
% intervenes. \gotex@endparenv is the one copy every list end calls.
\def\gotex@endparenv{\addvspace\@topsepadd\@endpetrue}
% ⛔ \par BEFORE \endgroup (ltlists.dtx's \endtrivlist ends the last paragraph while
% still inside the list). Everything \list sets is scoped to the group \@trivlist
% opened here, and with \endgroup first the shape was gone before the last paragraph
% reached the breaker: a quote whose text ends without a \par came out at the full
% measure while the same quote with one was indented (#646).
\def\endtrivlist{\par\endgroup\gotex@endparenv}
\def\@itemlabel{\textbullet}
\def\item{\@ifnextchar[{\@gotexitem}{\@gotexitem[\@itemlabel]}}
% \@iteminterspace puts \itemsep between items and nothing before the first (the
% \@listfirst flag \@trivlist raises). Without it every \list-built environment ran
% its items a bare baseline apart: description gave 13.6pt where real LaTeX gives
% 22.5. A trivlist with a single \item — a theorem, amsart's author block — is
% unaffected, the flag suppressing the space on the first one.
\def\@gotexitem[#1]{\par\@iteminterspace\noindent#1\ }
`

// leaveVMode is \leavevmode: in vertical mode it starts a paragraph, so the box
// that follows attaches to a line instead of becoming a paragraph of its own.
// latex.ltx opens both \@iiiminipage (11853) and \@iiiparbox with it, which is
// what makes two side-by-side panels share a line.
//
// ⛔⛔ THE PARAGRAPH IS INDENTED. tex.web:21104-21109 lists the commands that, met in
// vertical mode, back the input up and start one:
//
//	vmode+letter, vmode+other_char, vmode+char_num, vmode+char_given,
//	vmode+math_shift, vmode+un_hbox, vmode+vrule,
//	vmode+accent, vmode+discretionary, vmode+hskip, vmode+valign,
//	vmode+ex_space, vmode+no_boundary:
//	  begin back_input; new_graf(true); end;
//
// and the argument is TRUE — indented. \leavevmode is \unhbox\voidb@x, which is the
// vmode+un_hbox case, so it indents like the rest. Passing false here made every
// \parbox and minipage at the start of a paragraph sit \parindent too far left:
// measured against tectonic on disc/indent.tex, the reference puts the box at
// x=148.71 and we put it at 133.77, the 15pt of \parindent exactly. A document that
// wants the box flush says \noindent, and that path was never the broken one.
func (e *Engine) leaveVMode() {
	if !e.inPar {
		e.beginParagraph(true)
	}
}
