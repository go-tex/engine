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
	// \aftergroup<token>: defer the token to the enclosing group's end. The engine
	// has no after-group queue; consume the token (its deferred action is dropped)
	// so it is not executed early. Best-effort: amsart uses it for paragraph-shape
	// bookkeeping, not for output.
	e.prim("aftergroup", func(e *Engine) { e.getNext() })
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
	// \unhbox<n> / \unvbox<n>: unpack a box register onto the current list. Used
	// inside \insert groups (dropped above) and footnote assembly; consume the index.
	e.prim("unhbox", func(e *Engine) { e.scanInt() })
	e.prim("unvbox", func(e *Engine) { e.scanInt() })
	e.prim("unhcopy", func(e *Engine) { e.scanInt() })
	e.prim("unvcopy", func(e *Engine) { e.scanInt() })
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
	e.prim("openout", func(e *Engine) { e.scanInt(); e.skipWriteFilename() })
	e.prim("closeout", func(e *Engine) { e.scanInt() })
	e.prim("write", func(e *Engine) {
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
\newdimen\jot
\newdimen\hangafter
\newskip\tabskip
\newskip\spaceskip
\newskip\xspaceskip
\newskip\parfillskip
\newskip\normalbaselineskip
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
\def\selectfont{}
\def\f@shape{n}
\def\f@series{m}
\def\f@family{}
\def\fontsize#1#2{}
\def\@setfontsize#1#2#3{}
\def\fontencoding#1{}
\def\fontfamily#1{}
\def\fontseries#1{}
\def\fontshape#1{}
\def\usefont#1#2#3#4{}
\def\try@load@fontshape{}
\def\check@mathfonts{}
\def\@parboxrestore{}
\def\displaystyle{}
\def\textstyle{}
\def\scriptstyle{}
\def\m@th{}
\def\jobname{texput}
\def\@auxout{-1}
% \protected@edef ≈ \edef (\protect is already a harmless no-op in the kernel).
\let\protected@edef\edef
\let\protected@xdef\xdef
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
\catcode64=11
`
