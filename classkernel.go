// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file holds the LaTeX2e "class kernel" substrate: the mechanical constants,
// registers, flags, font aliases and no-op declarations that real .cls / .sty
// files (article.cls, book.cls, …) are built on top of but that carry no
// algorithm of their own. It is authored in pure TeX and loaded by LoadLaTeX
// right after LaTeX2eKernelHelpers (kernelhelpers.go), so a class file can be
// \input without tripping on an "undefined control sequence" for a plain
// register or a font-switch alias.
//
// SCOPE / non-goals. This layer defines only the *mechanical* gap between the
// kernel-helper layer and a class file: number/length constants (\p@, \@M, the
// \@vpt … size numbers), scratch and page/layout/list/float registers, the
// \if@… boolean flags, the NFSS font-switch aliases (mapped onto the engine's
// existing \rm/\bf/\it/… no-ops — there is no real font-series/shape machinery),
// and best-effort no-ops for the declaration commands a class preamble runs
// (\NeedsTeXFormat, \DeclareRobustCommand, \markboth, \addcontentsline, …).
//
// It deliberately does NOT define the algorithmic pieces a class relies on —
// \@startsection / \@sect / \list / \@setfontsize / \@float / \secdef / … —
// nor the low-level TeX primitives \ifdim / \divide / \long / \newbox /
// \leavevmode / \everypar / \sfcode / \hb@xt@. Those are provided elsewhere in
// the engine (Go primitives or the sectioning/list layer); stubbing them here
// would collide.
//
// Register layout is best-effort: allocating a name via \newdimen / \newskip /
// \newcount makes an assignment to it (or a \setlength on it) a valid store, so
// a class's `\setlength\leftmargini{...}` or `\itemsep=...` completes; the
// numbers do not (yet) drive real page layout.
//
// Glue idiom. \@plus / \@minus / \@width are defined here exactly as real LaTeX
// (as the bare keywords). The engine's glue scanner now expands while matching the
// `plus` / `minus` keywords and supports TeX's factor×internal-dimen products, so
// the classic LaTeX rubber-length idiom `\z@ \@plus 3\p@` assembles its full glue
// (0pt plus 3pt) — see classkernel_test.go.

// LaTeX2eClassKernel is the LaTeX2e class-kernel substrate, loaded by LoadLaTeX
// right after LaTeX2eKernelHelpers.
const LaTeX2eClassKernel = `
\catcode64=11
% ── numeric / length constants ──────────────────────────────────────────────
% \p@ = 1pt as a dimen (so 3\p@, \setlength\x\p@ read it). \z@ / \z@skip already
% come from the kernel-helper layer (kernelhelpers.go) as a \newdimen / \newskip.
\newdimen\p@ \p@=1pt
% \@plus / \@minus / \@width / \@height / \@depth: the plain-TeX/LaTeX keyword
% macros, defined exactly as real latex.ltx (bare keywords). The engine's glue
% scanner expands while matching keywords, so the \z@ \@plus 3\p@ idiom assembles
% its full rubber glue (0pt plus 3pt).
\def\@width{width}
\def\@height{height}
\def\@depth{depth}
\def\@plus{plus}
\def\@minus{minus}
% Big integer constants used as penalties / limits. Real LaTeX uses
% \mathchardef; \newcount avoids any \chardef range question and \the still
% prints the value.
\newcount\@M \@M=10000
\newcount\@m \@m=1000
\newcount\@Mi \@Mi=10001
\newcount\@Mii \@Mii=10002
\newcount\@Miii \@Miii=10003
\newcount\@Miv \@Miv=10004
% Font-size number macros consumed by \@setfontsize (\@xpt → 10, …).
\def\@vpt{5}
\def\@vipt{6}
\def\@viipt{7}
\def\@viiipt{8}
\def\@ixpt{9}
\def\@xpt{10}
\def\@xipt{10.95}
\def\@xiipt{12}
\def\@xivpt{14}
\def\@xviipt{17}
\def\@xxpt{20}
\def\@xxvpt{25}
% ── scratch registers ───────────────────────────────────────────────────────
% \@tempdima/b, \@tempcnta/b, \@tempskipa/b already come from the kernel-helper
% layer; only \@tempdimc and the scratch box are added here. The engine has no
% \newbox (a class-kernel algorithmic piece owned elsewhere); \@tempboxa is
% allocated with \newsavebox so \sbox\@tempboxa / \usebox\@tempboxa are valid.
\newdimen\@tempdimc
\newsavebox{\@tempboxa}
% ── page / layout dimens ────────────────────────────────────────────────────
% \paperwidth/\paperheight are geometry-package option keys in the engine, not
% registers; allocate them so a class may \setlength them. \parindent /
% \baselineskip / \hsize / \vsize are engine parameters (untouched).
\newdimen\paperwidth
\newdimen\paperheight
\newdimen\hangindent
\newdimen\overfullrule
\newdimen\maxdepth
\newskip\topskip
\newskip\parskip
\newskip\lineskip
\newskip\normallineskip
\newdimen\arraycolsep
\newdimen\arrayrulewidth
\newdimen\doublerulesep
\newdimen\tabcolsep
\newdimen\tabbingsep
\newdimen\fboxrule
\newdimen\fboxsep
\newcount\col@number
% ── list dimens / skips ─────────────────────────────────────────────────────
\newdimen\leftmargin
\newdimen\leftmargini
\newdimen\leftmarginii
\newdimen\leftmarginiii
\newdimen\leftmarginiv
\newdimen\leftmarginv
\newdimen\leftmarginvi
\newdimen\rightmargin
\newdimen\labelwidth
\newdimen\labelsep
\newdimen\itemindent
\newdimen\listparindent
\newskip\itemsep
\newskip\parsep
\newskip\topsep
\newskip\partopsep
% ── display skips ───────────────────────────────────────────────────────────
\newskip\abovedisplayskip
\newskip\belowdisplayskip
\newskip\abovedisplayshortskip
\newskip\belowdisplayshortskip
% ── standard vertical-space amounts (\bigskip/\medskip/\smallskip macros come
% from the Plain layer; only the amount registers are added). ─────────────────
\newskip\bigskipamount
\newskip\medskipamount
\newskip\smallskipamount
% ── float dimens / registers ────────────────────────────────────────────────
% \footins / \@mpfootins are inserts in real LaTeX (\newinsert); the engine has
% no inserts, so allocate them as \newcount pointing at high, otherwise-unused
% register numbers, keeping \skip\footins / \dimen\footins valid scratch stores
% that never collide with a \newskip/\newdimen allocation.
\newskip\floatsep
\newskip\textfloatsep
\newskip\intextsep
\newskip\dblfloatsep
\newskip\dbltextfloatsep
\newdimen\footnotesep
\newcount\footins \footins=255
\newcount\@mpfootins \@mpfootins=254
\newskip\@fptop
\newskip\@fpbot
\newskip\@fpsep
\newskip\@dblfptop
\newskip\@dblfpbot
\newskip\@dblfpsep
% ── penalty / counter registers ─────────────────────────────────────────────
\newcount\clubpenalty
\newcount\widowpenalty
\newcount\interlinepenalty
\newcount\predisplaypenalty
\newcount\postdisplaypenalty
\newcount\@lowpenalty \@lowpenalty=51
\newcount\@medpenalty \@medpenalty=151
\newcount\@highpenalty \@highpenalty=301
\newcount\@beginparpenalty
\newcount\@endparpenalty
\newcount\@itempenalty
\newcount\@secpenalty
\newcount\@clubpenalty
\newcount\@topnum
\newcount\c@secnumdepth \c@secnumdepth=3
\newcount\c@footnote
\newcount\day \day=1
\newcount\month \month=1
\newcount\year \year=2026
% ── plain-TeX \loop … \repeat ────────────────────────────────────────────────
% Packages build iteration with \loop〈body〉\repeat where \repeat is \let to \fi.
% The \let is essential beyond execution: it makes the conditional-skip that TeX
% performs over a FALSE \if branch treat \repeat as a closing \fi, so a body such
% as \loop\ifnum…\repeat inside a skipped branch stays balanced. Without \repeat
% defined, a skipped \ifnum…\repeat never closes and the skip overruns the
% matching \else/\fi, swallowing everything after it — the acl.sty 0-page bug.
\def\loop#1\repeat{\def\iterate{#1\relax\expandafter\iterate\fi}\iterate\let\iterate\relax}
\let\repeat\fi
% ── \if@ boolean flags (\newif comes from the kernel-helper layer) ───────────
% \@settopoint rounds a length down to a whole point, which the size option
% files apply to the lengths they compute. Without it a class silently drops
% those calls.
\def\@settopoint#1{\divide#1\p@\multiply#1\p@}
% \voidb@x is the kernel's permanently empty box: nothing is ever put in it, so
% \setbox<n>=\box\voidb@x is how a package empties a box register. Without it
% that idiom reads register 0 instead and steals whatever is in it.
\newbox\voidb@x
% NFSS's maths versions (\mathversion{bold} and friends) and the font-shape
% switches a class uses around its headings. This engine has one face per family,
% so a version switch has nothing to select; the argument is consumed.
\def\mathversion#1{}
\def\DeclareMathVersion#1{}
\def\SetMathAlphabet@#1#2#3#4#5#6{}
\def\fontfamily#1{\@gobbleone@nil}
\def\fontseries#1{\@gobbleone@nil}
\def\fontshape#1{\@gobbleone@nil}
\def\fontsize#1#2{}
\def\@gobbleone@nil{}
\def\usefont#1#2#3#4{}
% Line numbering (the lineno package, which several conference classes turn on
% and off around their front matter). This engine does not number lines, so the
% switches are accepted and do nothing.
\def\linenumbers{}
\def\nolinenumbers{}
\def\runninglinenumbers{}
\def\modulolinenumbers#1{}
\def\setrunninglinenumbers{}
% \protected@write writes to an auxiliary file with fragile commands shielded.
% This engine keeps its cross-reference and table-of-contents information in
% memory rather than in .aux files, so the write itself has nothing to do — but
% a class calls it directly, and an undefined one stops the document.
\def\protected@write#1#2#3{}
\def\protected@edef#1#2{\edef#1{#2}}
\def\protected@xdef#1#2{\xdef#1{#2}}
% NFSS's font declarations. This engine has one face per family rather than
% NFSS's encoding/family/series/shape tables, so a declaration cannot install
% what it asks for — but a real paper's preamble is full of them, and an
% undefined one stops the document before its first line. They are accepted and
% their arguments consumed. A maths alphabet is worth more than that: the command
% it declares is defined, and a family that names a blackboard-bold font gets the
% blackboard alphabet the maths layer really has, so \mathbbm{N} still reads as
% the set of naturals.
\def\DeclareMathAlphabet#1#2#3#4#5{\gotex@declmathalpha#1{#3}}
\def\gotex@declmathalpha#1#2{%
  \def\gotex@fam{#2}%
  \def\gotex@bbm{bbm}\def\gotex@bbold{bbold}\def\gotex@dsrom{dsrom}%
  \ifx\gotex@fam\gotex@bbm \def#1##1{\mathbb{##1}}%
  \else\ifx\gotex@fam\gotex@bbold \def#1##1{\mathbb{##1}}%
  \else\ifx\gotex@fam\gotex@dsrom \def#1##1{\mathbb{##1}}%
  \else\def#1##1{##1}%
  \fi\fi\fi}
\def\SetMathAlphabet#1#2#3#4#5#6{}
\def\DeclareSymbolFont#1#2#3#4#5{}
\def\SetSymbolFont#1#2#3#4#5#6{}
\def\DeclareSymbolFontAlphabet#1#2{}
\def\DeclareFontFamily#1#2#3{}
\def\DeclareFontShape#1#2#3#4#5#6{}
\def\DeclareFontEncoding#1#2#3{}
\def\DeclareFontSubstitution#1#2#3#4{}
\def\DeclareTextFontCommand#1#2{\def#1##1{{#2##1}}}
% Which engine is this? A document asks before it chooses how to include a
% graphic, which font machinery to load, or which encoding package to use — and
% an unanswered question is a hard error, since \ifpdf…\else…\fi then has no
% conditional to match. This engine is an e-TeX-capable engine that writes PDF
% directly and reads UTF-8 source, so it answers the pdfTeX-shaped questions yes
% (that is the branch whose packages it emulates) and the XeTeX/LuaTeX ones no.
\newif\ifpdf\pdftrue
\newif\ifPDFTeX\PDFTeXtrue
\newif\ifpdftex\pdftextrue
\newif\ifetex\etextrue
\newif\ifeTeX\eTeXtrue
\newif\ifxetex
\newif\ifXeTeX
\newif\ifluatex
\newif\ifLuaTeX
\newif\ifvtex
\newif\ifVTeX
\newif\ifptex
\newif\ifuptex
\newif\ifptexng
\newif\ifalephtex
\newif\ifTUTeX
\newif\if@twocolumn
\newif\if@twoside
\newif\if@compatibility
\newif\if@noskipsec
\newif\if@mparswitch
\newif\if@nobreak
\newif\if@minipage
\newif\if@titlepage
\newif\if@openbib
\newif\if@restonecol
\newif\if@afterindent
% list-machinery booleans (latex.ltx's \list/\trivlist/\item everypar hook): a
% class's real \trivlist/\@item toggles these, and if \@nmbrlistfalse etc. are
% undefined the list never closes cleanly and swallows the following input (seen
% with amsart's \maketitle author block, which runs \trivlist before the body).
\newif\if@nmbrlist
\newif\if@newlist
\newif\if@noparitem
\newif\if@noparlist
\newif\if@inlabel
% Set the flags a class expects to have a definite state.
\@compatibilityfalse
\@twocolumnfalse
\@twosidefalse
\@mparswitchfalse
\@nobreakfalse
\@minipagefalse
\@titlepagefalse
\@openbibfalse
\@restonecolfalse
\@noskipsecfalse
\@afterindentfalse
% ── NFSS font-switch aliases (no real series/shape machinery; map onto the
% engine's existing \rm/\bf/\it/\sl/\tt/\sf no-op switches). ───────────────────
\def\normalfont{\rm}
\def\rmfamily{\rm}
\def\sffamily{\sf}
\def\ttfamily{\tt}
\def\bfseries{\bf}
\def\mdseries{}
\def\itshape{\it}
\def\slshape{\sl}
\def\scshape{}
\def\upshape{}
% Math alphabets: the class body may name them; define as identity text wrappers
% (harmless — user math is handled wholesale by the math layer).
\def\mathrm#1{#1}
\def\mathbf#1{#1}
\def\mathit#1{#1}
\def\mathsf#1{#1}
\def\mathtt#1{#1}
\def\mathcal#1{#1}
\def\mathnormal#1{#1}
% \mathds (dsfont) is double-struck — the same blackboard-bold role as \mathbb,
% which the math layer knows. dsfont.sty's \DeclareMathAlphabet machinery can't run
% here, so alias it: in math the retry path expands \mathds{X} -> \mathbb{X}.
\def\mathds#1{\mathbb{#1}}
% amsmath's stackable accents (\Tilde \Bar \Hat …) render as the ordinary accents
% the math layer knows; the retry path expands e.g. \Bar{x} -> \bar{x}.
\def\Tilde{\tilde}
\def\Bar{\bar}
\def\Hat{\hat}
\def\Check{\check}
\def\Acute{\acute}
\def\Grave{\grave}
\def\Dot{\dot}
\def\Ddot{\ddot}
\def\Breve{\breve}
\def\Vec{\vec}
% \sfrac{a}{b} (xfrac, slanted fraction) approximated as an inline a/b; the math
% layer has no slanted-fraction primitive but this keeps the equation rendering.
\def\sfrac#1#2{#1\!/#2}
% amsmath's \implies / \impliedby are long double arrows the math layer knows.
\def\implies{\Longrightarrow}
\def\impliedby{\Longleftarrow}
% \bm (bold math) is the same bold symbol as amsmath's \boldsymbol, which the math
% layer knows; the retry path expands \bm{x} -> \boldsymbol{x}.
\def\bm{\boldsymbol}
% \scaleto{obj}{height} (scalerel) rescales obj to a height the math layer has no
% notion of; render obj at its natural size (the scaling is cosmetic).
\def\scaleto#1#2{#1}
\def\@fontswitch#1#2{#2}
\def\@nomath#1{}
% \not@math@alphabet{switch}{mathversion} guards an NFSS font switch against use in
% math mode: it consumes the switch and its math form and, in math mode only,
% warns. It MUST grab both arguments even in text mode — the standard
% \DeclareRobustCommand*{\bfseries}{\not@math@alphabet\bfseries\mathbf …} redefines
% \bfseries with a reference to itself as the first argument; undefined,
% \not@math@alphabet is skipped and that \bfseries re-executes, recursing forever
% and swallowing the document (incl_settings.tex's \bfseries redefinition does
% exactly this).
\def\not@math@alphabet#1#2{\relax\ifmmode\@nomath#1\fi}
% \DeclareOldFontCommand\rm{\normalfont\rmfamily}{\mathrm}: real LaTeX binds the
% one-token font command to a text and a math form. In this engine \rm/\bf/\it/…
% are ALREADY the font switches, and \normalfont/\rmfamily are defined above as
% aliases back to them, so honouring the class's redefinition would make
% \rm → \normalfont\rmfamily → \rm loop. It is therefore a no-op: the engine's
% own \rm/\bf/… (and the \mathrm/… math wrappers above) are kept.
\def\DeclareOldFontCommand#1#2#3{}
% ── no-op / best-effort declaration commands ────────────────────────────────
% \DeclareRobustCommand / \CheckCommand behave like \newcommand (a Go primitive);
% \@star@or@long (kernel-helper layer) consumes an optional leading '*'.
\def\DeclareRobustCommand{\@star@or@long\newcommand}
\def\CheckCommand{\@star@or@long\newcommand}
\def\MakeRobust#1{}
% \NeedsTeXFormat{fmt}[date] / \ProvidesFile/\ProvidesClass/\ProvidesPackage
% {name}[date]: eat the required group then, only when a '[' actually follows,
% the optional [date]. \@gobbleoptonly (\def\@gobbleoptonly[#1]{}) comes from the
% mini-LaTeX layer; \@ifnextbracket leaves the '[' in place for it to consume,
% and takes \relax (eating nothing more) when no bracket follows — so trailing
% text after \NeedsTeXFormat{LaTeX2e} is NOT swallowed.
\def\NeedsTeXFormat#1{\@ifnextbracket\@gobbleoptonly\relax}
% \gotexeatdate eats an optional [<date>] if one follows. The package loader runs
% it after every class/package file (see loadTeXFile).
\def\gotexeatdate{\@ifnextbracket\@gobbleoptonly\relax}
\def\ProvidesFile#1{\@ifnextbracket\@gobbleoptonly\relax}
\def\ProvidesClass#1{\@ifnextbracket\@gobbleoptonly\relax}
\def\ProvidesPackage#1{\@ifnextbracket\@gobbleoptonly\relax}
% ── counter-format / case helpers (\@Roman/\@alph/\@Alph already defined) ────
\def\@arabic#1{\number#1}
\def\@roman#1{\romannumeral#1}
% \MakeUppercase/\MakeLowercase expand their argument first, then case-shift it:
% \uppercase only acts on explicit character tokens, so a control sequence such as
% \contentsname or amsart's \shorttitle must be expanded to its letters before it
% can be shifted. Skipping this both left cs arguments un-shifted and, via amsart's
% \altucnm idiom (\MakeTextUppercase{\toks@{#1}}\edef#1{\the\toks@}), made
% \shorttitle self-referential — an infinite loop when the running head expanded it.
\def\MakeUppercase#1{\edef\@MakeCase@a{#1}\uppercase\expandafter{\@MakeCase@a}}
\def\MakeLowercase#1{\edef\@MakeCase@a{#1}\lowercase\expandafter{\@MakeCase@a}}
% ── running heads / marks (no page-head machinery here: accept and drop) ─────
\def\markboth#1#2{}
\def\markright#1{}
\def\@mkboth#1#2{}
\def\leftmark{}
\def\rightmark{}
% ── contents recording (the engine owns its own TOC; accept and drop) ────────
\def\addcontentsline#1#2#3{}
\def\addtocontents#1#2{}
\def\addvspace#1{\vskip#1}
\def\addpenalty#1{}
\def\nobreakspace{\space}
% ── diagnostics not already routed by the kernel-helper layer ────────────────
\def\@font@warning#1{\message{Font Warning: #1}}
% ── misc structural no-ops ──────────────────────────────────────────────────
\def\null{\hbox{}}
% LaTeX makes the three escaped characters that stand for nothing but themselves
% into \chardef tokens rather than macros: under a real LaTeX, \meaning\# is
% \char"23. The difference that matters is that a \chardef token is
% UNEXPANDABLE, so it survives \edef and can be \let to something else for the
% length of one expansion. pgf depends on exactly that — it builds an SVG
% fragment with \edef and rebinds \# to a raw catcode-11 # while the fragment is
% written out, which is how a shading's fill:url(#pgfsh7) gets its reference. As
% expandable macros these turned into \char 35\relax inside pgf's \edef and every
% gradient reference in the output was dead text.
\chardef\#=35
\chardef\%=37
\chardef\&=38
% ── text symbols (simple literal glyphs) ────────────────────────────────────
\def\textbullet{•}
\def\textendash{–}
\def\textemdash{—}
\def\textasteriskcentered{*}
\def\textperiodcentered{·}
% Symbols whose ASCII form has a special catcode are produced with \char so they
% survive in any context (\ ~ ^ _ { } are esc/active/sup/sub/begin/end):
\def\textbackslash{\char92\relax}
\def\textasciitilde{\char126\relax}
\def\textasciicircum{\char94\relax}
\def\textunderscore{\char95\relax}
\def\textbraceleft{\char123\relax}
\def\textbraceright{\char125\relax}
% Ordinary-catcode ASCII and Unicode glyphs:
\def\textbar{|}
\def\textless{<}
\def\textgreater{>}
\def\textquotesingle{'}
\def\textquotedbl{"}
\def\textquoteleft{‘}
\def\textquoteright{’}
\def\textquotedblleft{“}
\def\textquotedblright{”}
\def\textdagger{†}
\def\textdaggerdbl{‡}
\def\textsection{§}
\def\textparagraph{¶}
\def\textregistered{®}
\def\textcopyright{©}
\def\texttrademark{™}
\def\textdegree{°}
\def\textpm{±}
\def\textmu{µ}
\def\textbardbl{‖}
\catcode64=11
`
