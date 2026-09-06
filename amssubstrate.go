// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file adds the extra plain-TeX substrate a real amsart.cls (and its siblings)
// leans on beyond what article.cls needed: a handful of low-level TeX primitives
// (\ignorespaces, \unskip, \accent, \insert, the mode conditionals \ifmmode/…, the
// aux-file writers \openout/\write, …) plus the register-and-macro layer
// AMSClassSubstrate. amsart is math-heavy and uses the plain-TeX kernel far more
// directly than article: it assigns \brokenpenalty/\hfuzz/\tolerance, allocates
// inserts (\newinsert\copyins), and drives its title/mark machinery through token
// registers (see toks.go). Everything here is best-effort — a primitive whose only
// effect is on fine spacing or the .aux file is accepted and made a no-op so the
// class runs and typesets title, sections and math rather than aborting or leaking
// its arguments onto the page.
//
// What is NOT here: the token registers themselves (toks.go), the class kernel
// (classkernel.go / kernelhelpers.go), and the high-level class machinery
// (classprims.go's LaTeX2eClassLead). This layer sits on top of all of those and is
// loaded last (see LoadLaTeX).

// loadAMSPrims registers the extra low-level TeX primitives amsart uses. Called
// from New after loadToksPrims.
func (e *Engine) loadAMSPrims() {
	// \ignorespaces: expand and drop the following run of space tokens. amsart's
	// \andify emits it to trim the space after the last author.
	e.prim("ignorespaces", func(e *Engine) {
		for {
			t, ok := e.getXToken()
			if !ok {
				return
			}
			if t.cs_ || t.cat != catSpace {
				e.back(t)
				return
			}
		}
	})
	// \unskip / \unpenalty / \unkern: remove the last glue/penalty/kern from the
	// current list. The engine does not expose list surgery here; used only to trim
	// trailing space, so an accepted no-op is close enough.
	e.prim("unskip", func(e *Engine) {})
	e.prim("unpenalty", func(e *Engine) {})
	e.prim("unkern", func(e *Engine) {})
	e.prim("nointerlineskip", func(e *Engine) {})
	e.prim("removelastskip", func(e *Engine) {})
	// \immediate: a prefix to \write/\openout/\closeout; a no-op on its own.
	e.prim("immediate", func(e *Engine) {})
	// \aftergroup<token>: hold the token back until the current group closes, then
	// insert it (see endGroup). A package finishes what it built inside a box this
	// way — every TikZ node ends by \aftergroup-ing the code that closes it.
	e.prim("aftergroup", func(e *Engine) {
		if t, ok := e.getNext(); ok {
			e.afterGroupToken(t)
		}
	})
	// \accent<code>: set an accent over the next char. Used only in amsart's \dh/\dj
	// glyphs (themselves replaced when amsfonts is absent). Consume the code number.
	e.prim("accent", func(e *Engine) { e.scanInt() })
	// \fontdimen<n><font>: a font parameter. Read as a value (\fontdimen2\font) in a
	// couple of spacing macros off the critical path; consume the index and the font
	// token and contribute nothing.
	e.prim("fontdimen", func(e *Engine) {
		e.scanInt()
		if t, ok := e.getXToken(); !ok {
			return
		} else if t.cs_ && t.cs == "font" { // \fontdimen2\font (the real \font primitive)
			return
		} else {
			e.back(t)
		}
	})
	// \insert<n>{material}: add to an insert class (footnotes, amsart's \copyins).
	// The engine has its own footnote model; accept the register number and the
	// braced material and drop it here.
	e.prim("insert", func(e *Engine) {
		e.scanInt()
		e.skipOptSpace()
		if t, ok := e.getNext(); ok {
			if t.cat == catBegin && !t.cs_ {
				e.grabGroup()
			} else {
				e.back(t)
			}
		}
	})
	// \unhbox / \unvbox / \unhcopy / \unvcopy live in boxcmds.go.
	e.installUnbox()
	// aux-file writers: \newwrite\cs allocates a stream (a count is enough), and
	// \openout / \write / \closeout are accepted and dropped (the engine's two-pass
	// TOC/label machinery does not use TeX write streams).
	e.prim("newwrite", func(e *Engine) {
		name := e.scanCSName()
		if name == "" || e.allocCnt >= 256 {
			return
		}
		e.define(name, &meaning{kind: mCountRef, code: e.allocCnt}, false)
		e.allocCnt++
	})
	// \newread\cs allocates an input stream the same way; the engine reads no
	// TeX streams, but a package that allocates one (pgf's \r@pgf@reada) must not
	// see \newread undefined.
	// \openin / \closein / \read / \ifeof live in readstreams.go.
	e.installReadStreams()
	// \newread\cs allocates an input stream. TeX allocates it with \chardef, so
	// the handle IS the stream number — \meaning of one is \char"2. Allocating it
	// as a count register instead made \openin\cs read the register's VALUE,
	// which is zero until something writes it, so every stream a document opened
	// collided on number 0.
	e.prim("newread", func(e *Engine) {
		name := e.scanCSName()
		if name == "" || e.allocRead >= maxReadStreams {
			return
		}
		e.define(name, &meaning{kind: mCharDef, code: e.allocRead}, false)
		e.allocRead++
	})
	// \openout<n>=<filename> opens an output stream. The engine writes no auxiliary
	// files, but the primitive still has to CONSUME its operand: undefined, the
	// filename was left in the input and TYPESET. beamer opens three streams for its
	// .nav/.toc/.snm files as the document begins, so every talk carried a page
	// reading "texput.nav texput.toc texput.snm".
	//
	// The grammar is tex.web §1352 ("Implement \openout"):
	//
	//	new_write_whatsit(open_node_size);   { scan_four_bit_int for \openout }
	//	scan_optional_equals; scan_file_name;
	//
	// and scan_file_name is §526: skip leading blanks, then take every CHARACTER
	// token (get_x_token, so an expandable cs contributes the characters it expands
	// to) until either a space — which is consumed — or a non-character token, which
	// is pushed BACK (back_input). More precisely still, §516 more_name ends the name
	// on a space and on nothing else. The engine's scanFileName already reads exactly
	// that, so it is reused rather than re-derived.
	//
	// Two deliberate departures from the reference, both toward leniency:
	//   - TeX scans the stream number with scan_four_bit_int and errors outside 0..15;
	//     this takes any integer, as the neighbouring \closeout already does.
	//   - TeX appends a whatsit to the current list and acts on it at \shipout unless
	//     \immediate precedes; nothing is written here either way, so no node is made.
	e.prim("openout", func(e *Engine) { e.doOpenout() })
	e.prim("closeout", func(e *Engine) { e.doCloseout() })
	e.prim("write", func(e *Engine) { e.doWrite() })
	// mode conditionals. During class load and normal vertical/horizontal text the
	// engine is never in math mode; \ifhmode/\ifvmode track whether a paragraph is
	// open, \ifinner is always outer, and \ifvoid/\ifhbox/\ifvbox query a box
	// register (every unused register is void).
	e.prim("ifmmode", func(e *Engine) { e.doIf(false) })
	e.prim("ifhmode", func(e *Engine) { e.doIf(e.inPar) })
	e.prim("ifvmode", func(e *Engine) { e.doIf(!e.inPar) })
	e.prim("ifinner", func(e *Engine) { e.doIf(false) })
	e.prim("ifvoid", func(e *Engine) { e.doIf(e.getBox(e.scanInt()) == nil) })
	e.prim("ifhbox", func(e *Engine) { e.scanInt(); e.doIf(false) })
	e.prim("ifvbox", func(e *Engine) { e.scanInt(); e.doIf(false) })
}

// skipWriteFilename consumes an \openout right-hand side: an optional '=' then the
// file name up to (and consuming) a terminating \relax or a space, without
// typesetting it. Names in amsart are like `\jobname.toc\relax`.
func (e *Engine) skipWriteFilename() {
	e.scanEquals()
	for {
		t, ok := e.getXToken()
		if !ok {
			return
		}
		if t.cs_ { // a control sequence ends the file name (\relax) — consume and stop
			return
		}
		if t.cat == catSpace {
			return
		}
	}
}

// AMSClassSubstrate is the register-and-macro layer amsart builds on beyond the
// article substrate: plain-TeX scratch registers, the penalty/spacing parameters a
// class assigns, and best-effort no-ops for NFSS font selection and a few text
// commands. Loaded last by LoadLaTeX.
const AMSClassSubstrate = `
\catcode64=11
% ── token scratch registers (see toks.go) ───────────────────────────────────
% \toks@ is plain TeX's \toks0; \@temptokena / \@emptytoks are LaTeX's named
% scratch registers (the latter is kept permanently empty as a "clear" source).
\toksdef\toks@=0
\newtoks\@temptokena
\newtoks\@emptytoks
% ── plain-TeX scratch count / dimen / skip (\count@ = \count255, etc.) ───────
\countdef\count@=255
\dimendef\dimen@=0
\dimendef\dimen@i=1
\dimendef\dimen@ii=2
\skipdef\skip@=0
% ── penalty / spacing parameters a class assigns ────────────────────────────
\newcount\brokenpenalty
\newcount\tolerance \tolerance=10000
\newcount\hbadness
\newcount\vbadness
\newcount\hyphenpenalty
\newcount\finalhyphendemerits
\newdimen\hfuzz
\newdimen\vfuzz
\newdimen\emergencystretch
\newdimen\displaywidth
\newdimen\displayindent
\newdimen\predisplaysize
\newdimen\mathsurround
\newdimen\lineskiplimit
\newdimen\normallineskiplimit
% \jot is the extra leading between the rows of a multi-line display. latex.ltx
% ALLOCATES AND SETS IT (\newdimen\jot / \jot=3pt, l.11172-11173); allocating it
% and leaving it zero made every reader have to guess whether a zero meant "unset"
% or "this document wants none".
\newdimen\jot
\jot=3pt
\newdimen\hangafter
\newskip\tabskip
\newskip\spaceskip
\newskip\xspaceskip
\newskip\parfillskip
\newskip\normalbaselineskip
% \lastskip reads the last glue on the current list. The engine does not expose
% list surgery, and every use is a spacing tweak off the critical path (amsart's
% footnote \advance\skip@-\lastskip, and \removelastskip), so a permanently zero
% skip register is a safe stand-in — reads yield 0, the subtraction a no-op.
\newskip\lastskip
\newtoks\everydisplay
\newtoks\everymath
% list / equation scratch counters and skips amsart references directly
\newcount\@eqcnt
\newcount\@enumdepth
\newcount\@itemdepth
\newskip\@topsep
\newskip\@topsepadd
% \spacefactor: TeX updates it per character; the engine does not, so model it as
% a plain count seeded at 1000. \@addpunct only reads \ifnum\spacefactor>\@m, which
% stays false — punctuation is added normally.
\newcount\spacefactor \spacefactor=1000
\newdimen\prevdepth
% \newinsert allocates an insert class; a count register is a sufficient stand-in
% (its number then indexes \skip/\dimen/\box scratch, as in \skip\copyins=1.5pc).
\let\newinsert\newcount
% \strutbox: a (here empty) box a class measures; \strut/\lastbox are best-effort.
\newbox\strutbox
\def\strut{}
\def\lastbox{\hbox{}}
\def\vrulefil{}
% ── NFSS font selection: no real series/shape machinery, accept and drop ────
% \selectfont installs the font the current family/series/shape calls for. This
% engine has one text face per family rather than NFSS's tables, so it
% re-selects the roman face — which is what matters in practice: a package
% switches to \nullfont around material it wants measured but not set, and
% brings the real font back with \selectfont. Without that, everything typeset
% afterwards is set in a font with no characters (pgf does this for every
% picture, which left every node's text empty).
\def\selectfont{\gotex@selectbasefont}
\def\f@shape{n}
\def\f@series{m}
\def\f@family{}
\def\fontsize#1#2{}
% \@setfontsize keeps the exact shape it always had — a macro with three
% undelimited arguments — so nothing about how a class's tokens are consumed
% changes. It now puts the font at the size the class states (\gotex@fontsizeat),
% which is how a size table drives \tiny…\Huge; the size is read against the one
% the class states for \normalsize, so Options.Size still picks the body size and
% the table gives the ratios. It still REPORTS the \normalsize pair separately:
% the leading, which is the base \baselinestretch is measured against, and the
% size, which is the 100% the rest of the table is stated against.
\def\@setfontsize#1#2#3{\ifx#1\normalsize\gotex@classnormalsize{#2}\fi\gotex@fontsizeat{#2}{#3}\ifx#1\normalsize\gotex@notefontsize{#3}\fi}
\def\fontencoding#1{}
\def\fontfamily#1{}
\def\fontseries#1{}
\def\fontshape#1{}
\def\usefont#1#2#3#4{}
\def\try@load@fontshape{}
\def\check@mathfonts{}
% amsgen.sty's two shorthands (amsgen.sty:39-40), which every AMS-derived class
% uses everywhere:
%
%	\let\@xp=\expandafter
%	\let\@nx=\noexpand
%
% Undefined, \@xp is skipped and the \expandafter it stands for never happens, so
% the assignment it was steering hits the wrong token: an AMS class's own
%
%	\@xp\gdef\csname r@tocindent\@tempa\endcsname{0pt}
%
% then defines nothing and spills "0pt" and the name that followed it onto the
% page. In the arXiv reference set that cost one paper its whole body — one page
% of engine internals against tectonic's 29.
\let\@xp\expandafter
\let\@nx\noexpand
\def\@parboxrestore{}
\def\displaystyle{}
\def\textstyle{}
\def\scriptstyle{}
\def\m@th{}
\def\jobname{texput}
\def\@auxout{-1}
% \protected@edef / \protected@xdef must keep a \protect'd (robust) token from
% expanding while the body is being expanded — real LaTeX makes \protect
% momentarily unexpandable. Plain \edef/\xdef here expands right through \protect,
% so a robust command whose replacement text mentions itself or another robust
% command runs away and swallows the document: bm.sty's
% \protected@edef\bm#1{\bm{#1}}, and imsart's
% \protected@xdef\@thanks{…\protect\thanks@thefnmark…\protect\orig@footnotetext…}.
% Make \protect write itself and the following token literally, run the (x)edef
% (which reads the control sequence, its parameter text and the body that follow),
% then restore \protect once the assignment is done.
\def\@unexpandable@protect{\noexpand\protect\noexpand}
\def\gotex@restore@protect{\let\protect\gotex@@protect}
\def\protected@edef{\let\gotex@@protect\protect\let\protect\@unexpandable@protect
  \afterassignment\gotex@restore@protect\edef}
\def\protected@xdef{\let\gotex@@protect\protect\let\protect\@unexpandable@protect
  \afterassignment\gotex@restore@protect\xdef}
% \DeclareTextCommand\cs{encoding}{body}: amsart uses only the no-optional-arg
% form; bind \cs to its body and ignore the encoding.
\def\DeclareTextCommand#1#2#3{\def#1{#3}}
\def\DeclareTextSymbol#1#2#3{}
\def\DeclareTextCommandDefault#1{\def#1}
% \MakeTextUppercase behaves like \MakeUppercase (defined in the class kernel).
\def\MakeTextUppercase{\MakeUppercase}
% \textup: upright text — no series/shape machinery, so identity. Defining it (rather
% than leaving it undefined and gobbled) is what lets amsart's \@seccntformat
% (\textup{…the section number…}) actually emit the number in a heading.
\def\textup#1{#1}
\def\textsc#1{#1}
\def\textmd#1{#1}
\def\textnormal#1{#1}
% \mark{…}: page marks feed running heads the engine does not build; drop the mark.
\def\mark#1{}
% \/ : italic correction — a zero-width adjustment the engine does not need.
\def\/{}
% \hyphenation{…}: exception list — nothing to record here.
\def\hyphenation#1{}
% Sectioning support for classes that define their OWN \@startsection→\@sect (amsart)
% rather than using the engine's built-in \@startsection (article/report/book). Real
% LaTeX's \@hangfrom sets a hanging indent with the label #1 followed by body #2; the
% engine has no hanging-indent primitive, so typeset label then body on one paragraph.
% \@xsect finishes the heading (its argument is the after-skip, already applied), so a
% \par is enough to let the following text start its own paragraph.
% Display headings call \@hangfrom{label}{body}; the engine has no hanging indent,
% so typeset label then body on one line and clear \@svsechd so \@xsect does not
% re-emit it. Run-in headings instead defer the whole heading to \@svsechd, which
% \@sect \def's and real LaTeX fires from the next \everypar; the engine has no live
% everypar, so \@xsect runs \@svsechd itself (then clears it). This makes amsart
% headings visible whether \@sect chose the display or the run-in path.
\let\@svsechd\relax
\def\@hangfrom#1#2{\global\let\@svsechd\relax\par\noindent#1#2}
\def\@xsect#1{\ifx\@svsechd\relax\else\@svsechd\global\let\@svsechd\relax\fi\par}
\def\@afterheading{}
% \@ifclasswith{class}{opt}{then}{else}: the class-file analogue of
% \@ifpackagewith, consulting the option list the loader recorded in opt@class.cls.
\def\@ifclasswith#1#2{\@ifundefined{opt@#1.cls}{\@secondoftwo}{\@ifinlist{#2}{\@nameuse{opt@#1.cls}}\ifin@\expandafter\@firstoftwo\else\expandafter\@secondoftwo\fi}}
% ── amsthm counter-representation hooks (normally from amsmath.sty) ──────────
% amsart.cls's \@xthm builds a within-numbered theorem's printed number as
%   \the<thm> := \the<within>\@thmcountersep\@thmcounter{<thm>}
% (see amsart.cls \@xthm). \@thmcountersep is the separator between the parent
% and child number ("." → "Theorem 1.1"), and \@thmcounter{c} yields \arabic{c}
% kept unexpanded (\noexpand) so the \xdef freezes "\arabic{c}" into \the<thm> and
% it re-evaluates at every use. amsmath.sty defines both; the engine stubs amsmath,
% so without these the two control sequences would survive verbatim into \the<thm>
% and print literally ("1\@thmcountersep\@thmcounter{thm}") instead of "1.1".
% \providecommand so a real amsmath, if ever supplied, still wins.
\providecommand{\@thmcountersep}{.}
\providecommand{\@thmcounter}[1]{\noexpand\arabic{#1}}
% figure* and table* are the two-column forms. A real class defines them through
% \@dblfloat; a class this engine emulates itself (revtex, IEEEtran, acmart,
% elsarticle …) had no starred form at all, so \begin{figure*}[t] resolved to \relax
% and typeset its own placement key — every such float in the arXiv corpus carried a
% stray "[t]" or "[h]" on the page, in 23 papers for figure* and 11 for table*.
% They route through \@dblfloat (classprims.go), the same hook the embedded article.cls
% uses for its own figure*/table*: in two-column mode \@dblfloat → \gotex@dblfloat
% typesets the float at the FULL text width and places it as a band spanning BOTH columns
% across the top of a page (\@dblfloat / \@topnewpage), and in one column it sets the
% ordinary one-column float (\@float). \endfigure*/\endtable* are \end@dblfloat to pair.
% They are installed at \begin{document} and ONLY if nothing else has: a class that
% defines its own figure* must keep it, and one that declares it with
% \newenvironment would fail outright against a name already taken (acmart does,
% and defining these too early cost that paper 14 of its 18 pages).
\AtBeginDocument{%
  \@ifundefined{figure*}{%
    \expandafter\long\expandafter\def\csname figure*\endcsname{\@dblfloat{figure}}%
    \expandafter\long\expandafter\def\csname endfigure*\endcsname{\end@dblfloat}%
  }{}%
  \@ifundefined{table*}{%
    \expandafter\long\expandafter\def\csname table*\endcsname{\@dblfloat{table}}%
    \expandafter\long\expandafter\def\csname endtable*\endcsname{\end@dblfloat}%
  }{}%
}
\catcode64=11
`
