// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file holds the LaTeX2e low-level kernel helper macros — the internal
// \@-prefixed control sequences that real .cls / .sty files are built on top of
// (\@empty, \@gobble, \@namedef, \@ifundefined, \@for, \g@addto@macro, \newif,
// the \PackageWarning family, the \AtBeginDocument hook, …). They are authored in
// pure TeX on top of the engine's existing primitives (see primitives.go) and
// loaded by LoadLaTeX immediately after MiniLaTeXKernel, so that a later
// definition here wins over the placeholder no-ops in latex.go (\AtBeginDocument
// / \AtEndDocument were previously argument-gobbling no-ops; the versions below
// make their hooks fire).
//
// Deliberately NOT defined here: the option-processing layer
// (\DeclareOption / \ProcessOptions / \ExecuteOptions / \CurrentOption /
// \PassOptionsToPackage). That is implemented in Go by the class/package loader
// and would collide with these definitions.
//
// ── Engine capabilities relied on (verified against primitives.go) ───────────
//   * \def/\edef/\gdef/\xdef with DELIMITED parameter text (matchParams).
//   * \csname…\endcsname; an UNDEFINED name resolves to \relax (doCsname), which
//     is what makes \@ifundefined work.
//   * \expandafter, \noexpand, \string, \let, \ifx (meaning-compared), \if,
//     \ifnum, \else/\fi, \chardef, \newcount/\newdimen/\newskip.
//   * \@ifstar (peeks '*') and \@ifnextbracket (peeks '[') — the ONLY look-ahead
//     primitives; the engine has NO \futurelet, NO \escapechar, NO toks
//     registers, and NO \scantokens (see the LIMITATIONS notes below).
//
// ── CONTRACT with the Go class/package loader (read this before wiring Go) ────
//
//   \@ifpackageloaded / \@ifclassloaded / \@ifpackagewith / \@ifl@aded consult a
//   loaded-package registry that the Go loader must populate. The convention is
//   the standard LaTeX one:
//
//     * When package "foo" (file foo.sty) is loaded, the Go loader MUST define a
//       non-empty macro named  ver@foo.sty  — e.g. via the engine's \@namedef:
//           \@namedef{ver@foo.sty}{<date/version or any non-empty text>}
//       (\ProvidesPackage does exactly this in real LaTeX.)
//     * When class "bar" (file bar.cls) is loaded, define  ver@bar.cls  likewise.
//     * To make \@ifpackagewith work, the Go loader SHOULD also record the
//       comma-separated option list the package was called with in a macro named
//           opt@foo.sty
//       (again via \@namedef). If it is absent, \@ifpackagewith falls back to its
//       else-branch (i.e. "not loaded with those options").
//
//   If the Go side records none of these, the *loaded predicates simply report
//   "not loaded" (safe else-branch) rather than crashing.
//
//   \AtEndOfPackage / \AtEndOfClass append tokens to the macros
//   \@endofpackagehook / \@endofclasshook. The Go loader, after \input-ing a
//   package/class file, MUST execute and then reset that hook, e.g.:
//       run  \@endofpackagehook   then   \let\@endofpackagehook\@empty
//   (These are single shared accumulators — reset between loads, exactly as the
//   Go loader already scopes a package's other state.)
//
//   \AtBeginDocument / \AtEndDocument are fully wired HERE (no Go needed): they
//   append to \@begindocumenthook / \@enddocumenthook, which are executed by the
//   \document / \enddocument macros (redefined below) at \begin{document} /
//   \end{document}.

// LaTeX2eKernelHelpers is the LaTeX2e low-level kernel helper layer, loaded by
// LoadLaTeX right after MiniLaTeXKernel.
const LaTeX2eKernelHelpers = `
\catcode64=11
% ── expansion primitives / gobblers / selectors ─────────────────────────────
\def\@empty{}
\long\def\@iden#1{#1}
\long\def\@firstofone#1{#1}
\long\def\@gobble#1{}
\long\def\@gobbletwo#1#2{}
\long\def\@gobblethree#1#2#3{}
\long\def\@gobblefour#1#2#3#4{}
\long\def\@firstoftwo#1#2{#1}
\long\def\@secondoftwo#1#2{#2}
% LaTeX kernel while-loops. \@whilenum <test> \do {<body>} repeats <body> while
% the \ifnum test holds; \@whiledim is the \ifdim analogue; \@whilesw <switch>\fi
% {<body>} loops on a boolean switch. Classes drive frontmatter box splitting and
% list machinery with these (e.g. imsart's \close@fm \@whiledim…\vsplit loop);
% defined exactly as latex.ltx so the delimited \do argument parses correctly.
\long\def\@whilenum#1\do #2{\ifnum #1\relax #2\relax\@iwhilenum{#1\relax #2\relax}\fi}
\long\def\@iwhilenum#1{\ifnum #1\expandafter\@iwhilenum\else\expandafter\@gobble\fi{#1}}
\long\def\@whiledim#1\do #2{\ifdim #1\relax#2\@iwhiledim{#1\relax#2}\fi}
\long\def\@iwhiledim#1{\ifdim #1\expandafter\@iwhiledim\else\expandafter\@gobble\fi{#1}}
\long\def\@whilesw#1\fi#2{#1#2\@iwhilesw{#1#2}\fi}
\long\def\@iwhilesw#1\fi{#1\expandafter\@iwhilesw\else\@gobbletwo\fi{#1}\fi}
% \@nil / \@nnil : pure delimiter tokens for list-scanning macros. Their meaning
% is never executed; \@nnil's body is the single token \@nil so that a loop macro
% \def'd to \@nil compares \ifx-equal to \@nnil (meaning-equality of bodies).
\def\@nil{}
\def\@nnil{\@nil}
% ── \csname-based (re)naming ────────────────────────────────────────────────
\def\@namedef#1{\expandafter\def\csname #1\endcsname}
\def\@nameuse#1{\csname #1\endcsname}
% ── numeric / length constants used pervasively by the kernel ───────────────
% Plain-TeX register constants. \z@ is BOTH the number 0 and the dimen 0pt (as in
% plain.tex it is a \dimen); \@ne/\tw@/\thr@@ are \chardef'd small integers.
\newcount\m@ne \m@ne=-1
\chardef\@ne=1
\chardef\tw@=2
\chardef\thr@@=3
\newdimen\z@ \z@=0pt
\newskip\z@skip \z@skip=0pt plus0pt minus0pt
\newskip\@tempskipa
\newskip\@tempskipb
\newdimen\@tempdima
\newdimen\@tempdimb
\newcount\@tempcnta
\newcount\@tempcntb
% ── \newif (the engine has no \newif primitive) ─────────────────────────────
% \@stripif\iffoo expands to the letters "foo": \string\iffoo gives the six
% catcode-12 chars \ i f f o o (the engine has no \escapechar, so \string always
% keeps the leading backslash), and \@gobblethree drops the "\if" prefix. \newif
% then \let's the switch false and builds \footrue / \foofalse via \csname.
\def\@stripif#1{\expandafter\@gobblethree\string#1}
\def\newif#1{%
  \let#1\iffalse
  \expandafter\def\csname\@stripif#1true\endcsname{\let#1\iftrue}%
  \expandafter\def\csname\@stripif#1false\endcsname{\let#1\iffalse}}
\newif\if@tempswa
\newif\ifin@
% xcolor's own switches, which packages built on it read: xxcolor — beamer's
% colour layer — toggles \ifglobalcolors around every colour it sets.
\newif\ifglobalcolors
\newif\ifXC@keepwhite
% ── definability / undefined tests ──────────────────────────────────────────
% \@ifundefined{name}{then}{else}: \csname name\endcsname is \relax when the name
% is undefined (doCsname), so \ifx…\relax selects the branch. NOTE the standard
% LaTeX side effect: after the test an undefined name becomes \relax (no longer
% "undefined") — matching real latex.ltx.
% (One line: any space/newline in the body would leak into the branch that runs
% inside a \message, so it is written without stray space tokens.)
\def\@ifundefined#1{\expandafter\ifx\csname #1\endcsname\relax\expandafter\@firstoftwo\else\expandafter\@secondoftwo\fi}
% \@ifdefinable\name{def}: run {def} if \name is currently undefinable-safe (i.e.
% \relax/undefined); otherwise a no-op (LIMITATION: real LaTeX raises an error and
% also rejects a handful of reserved names — the engine's \newcommand path is a Go
% primitive and does its own checking, so this is only a best-effort fallback).
\long\def\@ifdefinable#1#2{%
  \expandafter\ifx\csname\expandafter\@gobble\string#1\endcsname\relax
    #2%
  \fi}
% ── list / string dissection ────────────────────────────────────────────────
\def\@car#1#2\@nil{#1}
\def\@cdr#1#2\@nil{#2}
% \zap@space<text> \@empty strips ALL spaces from <text>; the trailing \@empty is
% the sentinel (\zap@space a b c\@empty -> abc).
\def\zap@space#1 #2{#1\ifx#2\@empty\else\expandafter\zap@space\fi#2}
% \@backslashchar / \@percentchar : a single catcode-12 "\" / "%" usable inside
% \edef (\string keeps the backslash, \@gobble removes it).
\edef\@backslashchar{\expandafter\@gobble\string\\}
\edef\@percentchar{\expandafter\@gobble\string\%}
% ── two-argument expansion + substring membership (\in@) ─────────────────────
\def\@expandtwoargs#1#2#3{\edef\reserved@a{\noexpand#1{#2}{#3}}\reserved@a}
\def\reserved@a{}
% \@ifinlist{item}{comma-list} sets \ifin@ true iff <item> (after \edef expansion)
% equals one of the comma-separated entries of <comma-list> (also \edef-expanded
% first, so a macro like \opt@foo.sty is spread into its entries). A self-contained
% scanner terminated by the \@inliststop sentinel — it always halts.
%
% LIMITATION: the classic substring \in@ from latex.ltx is NOT provided. Its
% definition builds a macro whose parameter text is self-delimited by its own name
% (\def\in@@#1<sub>#2#3\in@@{…}); this engine's delimited-parameter matcher loops
% on that construction. \@ifinlist covers the comma-option-list case that
% \@ifpackagewith needs; packages that call \in@ directly are the option-processing
% layer's concern (handled in Go by the lead).
\def\@inliststop{\@inliststop}
\def\@ifinlist#1#2{\in@false\edef\@inlistwant{#1}\edef\@inlisttmp{#2}\expandafter\@inlist@scan\@inlisttmp,\@inliststop,}
\def\@inlist@scan#1,{\def\@inlistcur{#1}\ifx\@inlistcur\@inliststop\else\ifx\@inlistcur\@inlistwant\in@true\fi\expandafter\@inlist@scan\fi}
% ── appending tokens to a macro ─────────────────────────────────────────────
% \g@addto@macro (global) / \@addto@macro (local): append #2 to the body of the
% macro #1. Uses an \expandafter chain instead of \toks@ (the engine has no toks
% registers): \expandafter…\def\expandafter#1\expandafter{#1#2} redefines #1 to be
% its old body followed by #2.
\def\@addto@macro#1#2{\expandafter\def\expandafter#1\expandafter{#1#2}}
\long\def\g@addto@macro#1#2{\expandafter\gdef\expandafter#1\expandafter{#1#2}}
% ── list iteration (\@for over a comma list, \@tfor over tokens) ─────────────
% Authentic latex.ltx definitions (they need only \ifx, \def, \expandafter, the
% \@nil/\@nnil/\@empty sentinels and delimited parameters — all available).
\long\def\@fornoop#1\@@#2#3{}
\long\def\@for#1:=#2\do#3{%
  \expandafter\def\expandafter\@fortmp\expandafter{#2}%
  \ifx\@fortmp\@empty \else
    \expandafter\@forloop#2,\@nil,\@nil\@@#1{#3}\fi}
\long\def\@forloop#1,#2,#3\@@#4#5{\def#4{#1}\ifx #4\@nnil \else
       #5\def#4{#2}\ifx #4\@nnil \else#5\@iforloop #3\@@#4{#5}\fi\fi}
\long\def\@iforloop#1,#2\@@#3#4{\def#3{#1}\ifx #3\@nnil
       \expandafter\@fornoop \else
    #4\relax\expandafter\@iforloop\fi#2\@@#3{#4}}
\def\@tfor#1:=#2\do#3{%
  \def\@fortmp{#2}\ifx\@fortmp\@empty \else
    \expandafter\@tforloop#2\@nil\@nil\@@#1{#3}\fi}
\long\def\@tforloop#1#2\@@#3#4{\def#3{#1}\ifx #3\@nnil
       \expandafter\@fornoop \else
    #4\relax\expandafter\@tforloop\fi#2\@@#3{#4}}
\long\def\@break@tfor#1\@@#2#3{\fi\fi}
% ── star/long dispatch ──────────────────────────────────────────────────────
% \@star@or@long\cmd peeks for a '*' (via \@ifstar) and runs \cmd either way, and
% sets \l@ngrel@x to the prefix the definition should carry — \relax after a star,
% \long without one, exactly as the LaTeX kernel does. \long is an accepted no-op
% prefix here (the engine does not model the \par-in-argument restriction), but the
% MEANING of \l@ngrel@x is read: etoolbox's \newrobustcmd branches on
% \ifx\l@ngrel@x\relax to decide between \protected and \protected\long, so a
% \l@ngrel@x that is neither sends every starred command down the long branch.
\let\l@ngrel@x\relax
\def\@star@or@long#1{\@ifstar{\let\l@ngrel@x\relax#1}{\let\l@ngrel@x\long#1}}
% \@ifnextchar is a native primitive (real \ifx-based look-ahead over any target
% token, including control sequences) — see the engine's primitive table.
% \@testopt{<cmd>}{<default>} runs <cmd> on a following [optional argument], or on
% [<default>] when there is none — the kernel's optional-argument dispatcher, which
% every \newcommand-with-a-default and etoolbox's \newrobustcmd are built on. The
% default is BRACED so a multi-token default arrives as one argument.
\long\def\@testopt#1#2{\@ifnextchar[{#1}{#1[{#2}]}}
% \@protected@testopt\cmd is the same dispatcher guarded by \protect: in the real
% kernel the guard chooses between typesetting and writing the command to a file.
% This engine has one \protect meaning (typesetting — nothing is written to .aux
% for a later run), so the guard is always true and the argument, which only the
% write branch uses, is dropped.
\def\@typeset@protect{}
\def\@protected@testopt#1{\@testopt}
% ── \@argdef / \@yargdef: build an n-argument definition ─────────────────────
% These are the kernel's own definition builders, and packages call them directly
% — etoolbox's \newrobustcmd routes every command it defines through \@argdef (no
% optional argument) or \@yargdef (with one). They were missing, so etoolbox
% defined NOTHING: \mode, on which every line of beamer stands, simply did not
% exist, and the class's own \mode<all> printed "<all>" onto the page.
%
% Written exactly as the LaTeX kernel writes them (read back from a real TeX with
% \meaning). \@yargd@f assembles the parameter text #1#2…#n by matching against a
% ready-made run of nine parameters, and stops at the requested count using a
% parameter text that ends in "#{" — the argument runs up to the opening brace,
% which stays behind to open the body.
\long\def\@argdef#1[#2]#3{\@ifdefinable#1{\@yargdef#1\@ne{#2}{#3}}}
\long\def\@reargdef#1[#2]{\@yargdef#1\@ne{#2}}
\long\def\@yargdef#1#2#3{%
  \ifx#2\tw@
    \def\reserved@b##11{[####1]}%
  \else
    \let\reserved@b\@gobble
  \fi
  \expandafter\@yargd@f\expandafter{\number#3}#1}
\long\def\@yargd@f#1#2{%
  \def\reserved@a##1#1##2##{\expandafter\def\expandafter#2\reserved@b##1#1}%
  \l@ngrel@x\reserved@a 0##1##2##3##4##5##6##7##8##9###1}
% \kernel@ifnextchar is the kernel's own name for \@ifnextchar (the kernel keeps a
% private copy so a package that redefines \@ifnextchar cannot break the kernel).
% beamer's overlay decoder calls it by that name.
% \maxdimen is plain TeX's largest dimension. Package code uses it as "no limit"
% (\vbox to\maxdimen, \dimen0=\maxdimen), so its absence is not a rounding matter:
% the assignment simply does not happen.
\newdimen\maxdimen \maxdimen=16383.99998pt
% \@cons\list{\item} appends an \@elt-separated item to a LaTeX list macro, with
% \@elt made harmless while the list is rebuilt.
\def\@cons#1#2{\begingroup\let\@elt\relax\xdef#1{#1\@elt #2}\endgroup}
% \@onelevel@sanitize\cs rewrites \cs's content as ordinary characters, which is
% how the kernel makes a name safe to compare or write out. \strip@prefix drops
% the "macro:->" that \meaning prints in front of the body.
\def\strip@prefix#1>{}
\def\@onelevel@sanitize#1{\edef#1{\expandafter\strip@prefix\meaning#1}}
\let\kernel@ifnextchar\@ifnextchar
% \in@{<a>}{<b>} sets \ifin@ true iff the token list <a> occurs inside <b>. The
% kernel builds it as a delimited-match macro; keyval and beamer's option handling
% both ask it.
\def\in@#1#2{%
  \def\in@@##1#1##2##3\in@@{%
    \ifx\in@##2\in@false\else\in@true\fi}%
  \in@@#2#1\in@\in@@}
% \@makeother makes a character ordinary — how a package reads text verbatim — and
% \dospecials is plain TeX's list of the characters that need it.
\def\@makeother#1{\catcode` + "`" + `#1=12\relax}
\def\dospecials{\do\ \do\\\do\{\do\}\do\$\do\&\do\#\do\^\do\_\do\%\do\~}
% \@onlypreamble{\cmd} records a command as preamble-only. The list is kept, so
% code that walks \@preamblecmds finds what it expects; the engine does not
% disable the commands at \begin{document}, which only costs a worse error message
% for a document that misuses one.
\def\@preamblecmds{}
\def\@onlypreamble#1{%
  \expandafter\gdef\expandafter\@preamblecmds\expandafter{\@preamblecmds\do#1}}
\long\def\@xargdef#1[#2][#3]#4{%
  \@ifdefinable#1{%
    \expandafter\def\expandafter#1\expandafter{%
      \expandafter\@protected@testopt\expandafter#1%
      \csname\string#1\endcsname{#3}}%
    \expandafter\@yargdef\csname\string#1\endcsname\tw@{#2}{#4}}}
% ── logging / diagnostics: never abort a .sty, route everything to \message ──
% \MessageBreak/\protect/\on@line/\@spaces are the tokens warnings interpolate;
% keep them harmless inside the \message expansion. \@ehc/\@ehd are the "error
% help" tokens callers pass as the last argument of \PackageError/\ClassError.
\def\MessageBreak{ }
\def\protect{}
\def\on@line{}
\def\@spaces{}
\def\@ehc{}
\def\@ehd{}
\def\wlog#1{\message{#1}}
\long\def\typeout#1{\message{#1}}
\def\PackageWarning#1#2{\message{Package #1 Warning: #2}}
\def\PackageWarningNoLine#1#2{\message{Package #1 Warning: #2}}
\def\PackageInfo#1#2{\message{Package #1 Info: #2}}
\def\PackageError#1#2#3{\message{Package #1 Error: #2}}
\def\ClassWarning#1#2{\message{Class #1 Warning: #2}}
\def\ClassWarningNoLine#1#2{\message{Class #1 Warning: #2}}
\def\ClassInfo#1#2{\message{Class #1 Info: #2}}
\def\ClassError#1#2#3{\message{Class #1 Error: #2}}
\def\@latex@warning#1{\message{LaTeX Warning: #1}}
\def\@latex@warning@no@line#1{\message{LaTeX Warning: #1}}
\def\@latex@info#1{\message{LaTeX Info: #1}}
\def\@latex@info@no@line#1{\message{LaTeX Info: #1}}
\def\@latex@error#1#2{\message{LaTeX Error: #1}}
\def\@warning#1{\message{Warning: #1}}
% ── begin/end-document and package/class hooks ──────────────────────────────
% \AtBeginDocument / \AtEndDocument accumulate tokens fired by \document /
% \enddocument (redefined here to run the hook; the originals in MiniLaTeXKernel
% only toggled the @ catcode / added \vfill). \AtEndOfPackage / \AtEndOfClass
% accumulate into hooks the Go loader runs+resets after each load (see CONTRACT).
\def\@begindocumenthook{}
\def\@enddocumenthook{}
\def\@endofpackagehook{}
\def\@endofclasshook{}
% These take NO argument of their own, exactly as latex.ltx writes them:
% \def\AtBeginDocument{\g@addto@macro\@begindocumenthook}. The braced code is
% grabbed by \g@addto@macro, which is \long (checked against real LaTeX:
% "\long macro:#1#2->\begingroup\toks@\expandafter{#1#2}\xdef#1{\the\toks@}…").
% Taking the argument here instead made a NON-long macro read it, so a hook whose
% code spans a blank line tripped the \par check of tex.web §392.
\def\AtBeginDocument{\g@addto@macro\@begindocumenthook}
\def\AtEndDocument{\g@addto@macro\@enddocumenthook}
\def\AtEndOfPackage{\g@addto@macro\@endofpackagehook}
\def\AtEndOfClass{\g@addto@macro\@endofclasshook}
% \document / \enddocument run BOTH the classic \AtBeginDocument / \AtEndDocument
% accumulators and the named hooks of the 2020 format (see hooks.go), in the order
% the real format uses: begindocument/before, the \AtBeginDocument code, then
% begindocument/end, then the document environment's own env/document/begin.
\def\document{\catcode64=12 \gotex@applybaselinestretch
  \UseHook{begindocument/before}\@begindocumenthook
  \UseHook{begindocument}\UseHook{begindocument/end}\UseHook{env/document/begin}}
% \enddocument also plants the LastPage label the lastpage package is loaded for,
% so \pageref{LastPage} — the "page 1 of N" every report and CV wants — names the
% final page. It is placed before the closing \vfill so it falls ON that page, and
% after the closing \par, so the last paragraph has been contributed to the page
% before the marker is taken. Planting it unconditionally rather than behind
% \usepackage{lastpage} costs one unused label and covers the documents that
% inherit it from a class.
%
% Only the PAGE half is used. Real lastpage writes \newlabel{LastPage}{{}{\thepage}},
% so a \ref{LastPage} there is empty where ours gives the last section number;
% emptying \@currentlabel to match cost 348 glyphs off the last page of corpus
% paper 2401.17012 — content that, it turns out, only survives BY passing through
% \@currentlabel. Reported separately; a label nobody \refs is not worth it.
\def\enddocument{\UseHook{env/document/end}\@enddocumenthook\UseHook{enddocument}%
  \UseHook{enddocument/afterlastpage}\UseHook{enddocument/afteraux}%
  \UseHook{enddocument/info}\UseHook{enddocument/end}%
  \par\label{LastPage}\vfill\penalty-10000 }
% ── loaded-package / loaded-class registry (see CONTRACT above) ─────────────
\def\@ifl@aded#1#2{\@ifundefined{ver@#2.#1}\@secondoftwo\@firstoftwo}
% \@ptionlist{<file>} expands to the options that <file> was loaded with (the Go
% loader records them in opt@<file>, see the CONTRACT above); empty if it was not.
\def\@ptionlist#1{\@ifundefined{opt@#1}\@empty{\csname opt@#1\endcsname}}
\def\@ifpackageloaded#1{\@ifl@aded{sty}{#1}}
\def\@ifclassloaded#1{\@ifl@aded{cls}{#1}}
% \@ifpackagewith{pkg}{opts}{then}{else}: true iff every option in {opts} is in
% the recorded option list opt@pkg.sty. If that list was never recorded (Go side
% did not populate it), fall back to the else-branch.
% \@ifpackagewith{pkg}{opt}: true iff <opt> is one of the options recorded for the
% package in opt@pkg.sty. LIMITATION: checks a SINGLE option (the dominant real
% use); a multi-option {opt} is compared as one string and will not match.
\def\@ifpackagewith#1#2{\@ifundefined{opt@#1.sty}{\@secondoftwo}{\@ifinlist{#2}{\@nameuse{opt@#1.sty}}\ifin@\expandafter\@firstoftwo\else\expandafter\@secondoftwo\fi}}
\catcode64=11
`

// (LoadLaTeX in latex.go is extended to load this layer after MiniLaTeXKernel.)
