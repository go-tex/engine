// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// XKeyvalSubstrate implements the part of xkeyval that a document class needs to
// declare and read its own options. It is a SUBSET, sized by what the corpus uses
// and no wider (see the list at the end of this comment for what is deliberately
// absent) — a speculative port of all 622 lines of xkeyval.tex would be code with
// no caller.
//
// Why it exists. Every command in the option machinery was undefined, so on an
// acmart paper:
//
//   - \ACM@format@nr was never set, and acmart drives its FONT SIZE and its
//     journal/conference geometry off \ifcase\ACM@format@nr (acmart.cls:199-245):
//     a sigconf paper is 9pt two-column, and without the number it got neither.
//   - the key lists were typeset. Page 1 of corpus paper 2304.01951 opened with
//     "+acmart.clsformat[]manuscript, acmsmall, acmlarge, ..." and eight lines of
//     [named]ACMBlue colour specifications.
//   - \define@boolkey+'s trailing {\PackageError...} group was left OPEN by the
//     lenient skip, sixteen times, so the preamble ran seventeen groups deep and
//     every \newcommand in it was group-local and rolled back (go-tex/engine#305).
//
// Naming. xkeyval stores a key as \<prefix>@<family>@<key> (xkeyval.tex:72-87,
// \XKV@makepf / \XKV@makehd; the prefix defaults to KV), so \define@key{acmart.cls}
// {screen} defines \KV@acmart.cls@screen, taking the value as its one argument.
// A [default] additionally defines \<header><key>@default (xkeyval.tex:145-148).
//
// Deliberately absent, because nothing in the corpus reaches them: key pointers
// (\savevalue/\usevalue), \presetkeys/\savekeys, the family SEARCH order of a
// multi-family \setkeys (the first family that defines the key wins here),
// \DeclareOptionX*'s catch-all beyond storing it, \XKV@depth save/restore, and
// the unused-option bookkeeping of \ProcessOptionsX (the engine has neither
// \@removeelement nor \@unusedoptionlist).
//
// @ is already a letter here — this loads inside LoadLaTeX, after
// LaTeX2eClassKernel — and LoadFormat does not save and restore catcodes, so this
// must NOT emit a \makeatother. See the same note on FloatPlacementSubstrate.
const XKeyvalSubstrate = `
% ── key naming (xkeyval.tex:72-87) ──────────────────────────────────────────
% \XKV@makepf{p} -> \XKV@prefix = "p@" (empty for an empty prefix);
% \XKV@makehd{f} -> \XKV@header = "\XKV@prefix f@". The default prefix is KV.
\def\gotex@xkv@makepf#1{%
  \def\gotex@xkv@t{#1}%
  \ifx\gotex@xkv@t\@empty\let\XKV@prefix\@empty\else\edef\XKV@prefix{#1@}\fi}
\def\gotex@xkv@makehd#1{%
  \def\gotex@xkv@t{#1}%
  \ifx\gotex@xkv@t\@empty\let\XKV@header\XKV@prefix
  \else\edef\XKV@header{\XKV@prefix#1@}\fi}

% Strip the spaces from a key name or a choice-list item, as \XKV@sp@def does:
% "sigplan, sigchi" must match the key "sigchi", not " sigchi". Only names and
% choices go through this, never a free-form value.
\def\gotex@xkv@trim#1#2{\edef#1{\zap@space#2 \@empty}}

% ── \define@key[prefix]{family}{key}[default]{code} (xkeyval.tex:151-160) ────
% The {code} is not read as an argument: the definition is left mid-\def so the
% brace group that follows in the source BECOMES the body, exactly as xkeyval
% does it.
\def\define@key{\@testopt\gotex@xkv@dk{KV}}
\def\gotex@xkv@dk[#1]#2{\gotex@xkv@makepf{#1}\gotex@xkv@makehd{#2}\gotex@xkv@dk@}
\def\gotex@xkv@dk@#1{%
  \@ifnextchar[{\gotex@xkv@dkd{#1}}{\gotex@xkv@dkn{#1}}}
\def\gotex@xkv@dkn#1{\expandafter\def\csname\XKV@header#1\endcsname##1}
\def\gotex@xkv@dkd#1[#2]{%
  \gotex@xkv@setdefault{#1}{#2}%
  \expandafter\def\csname\XKV@header#1\endcsname##1}
% \XKV@define@default (xkeyval.tex:145-148): \<header><key>@default runs the key
% with the default value.
\def\gotex@xkv@setdefault#1#2{%
  \expandafter\def\csname\XKV@header#1@default\expandafter\endcsname
    \expandafter{\csname\XKV@header#1\endcsname{#2}}}

% ── \setkeys[prefix]{families}{key=value,...} ───────────────────────────────
% A bare "key" runs \<header>key@default; "key=value" runs \<header>key{value}.
% An unknown key is silently ignored (xkeyval raises an error; a lenient engine
% must not stop a compile over a class option it does not model).
\def\setkeys{\@testopt\gotex@xkv@sk{KV}}
\def\gotex@xkv@sk[#1]#2#3{%
  \gotex@xkv@makepf{#1}%
  \def\gotex@xkv@fams{#2}%
  \gotex@xkv@forcomma{#3}\gotex@xkv@setone}
\def\gotex@xkv@setone#1{%
  \gotex@xkv@split#1==\@nil}
\def\gotex@xkv@split#1=#2=#3\@nil{%
  \gotex@xkv@trim\gotex@xkv@k{#1}%
  \def\gotex@xkv@v{#2}%
  \gotex@xkv@forcomma{\gotex@xkv@fams}\gotex@xkv@tryfam}
% Try one family: if it defines the key, run it and stop looking.
\def\gotex@xkv@tryfam#1{%
  \ifx\gotex@xkv@k\@empty\else
    \gotex@xkv@makehd{#1}%
    \ifx\gotex@xkv@v\@empty
      \expandafter\ifx\csname\XKV@header\gotex@xkv@k @default\endcsname\relax\else
        \csname\XKV@header\gotex@xkv@k @default\endcsname
        \let\gotex@xkv@k\@empty
      \fi
    \else
      \expandafter\ifx\csname\XKV@header\gotex@xkv@k\endcsname\relax\else
        \expandafter\gotex@xkv@runkey\expandafter{\gotex@xkv@v}%
        \let\gotex@xkv@k\@empty
      \fi
    \fi
  \fi}
\def\gotex@xkv@runkey#1{%
  \gotex@xkv@trim\gotex@xkv@w{#1}%
  \expandafter\expandafter\expandafter
    \csname\XKV@header\gotex@xkv@k\expandafter\endcsname\expandafter{\gotex@xkv@w}}

% A comma loop that survives an empty list and a trailing comma, and that is
% REENTRANT: the body travels as an argument rather than in a shared macro, so a
% key handler may itself loop (\setkeys walks the key list, and a choice key then
% walks its own choice list) without the inner loop eating the outer one's state.
\def\gotex@xkv@forcomma#1#2{%
  \edef\gotex@xkv@l{#1}%
  \ifx\gotex@xkv@l\@empty\else
    \expandafter\gotex@xkv@f@rcomma\expandafter{\expandafter#2\expandafter}\gotex@xkv@l,\@nil,%
  \fi}
\def\gotex@xkv@f@rcomma#1#2,{%
  \def\gotex@xkv@item{#2}%
  \ifx\gotex@xkv@item\@nnil
    \expandafter\@gobble
  \else
    \expandafter\@firstofone
  \fi
  {#1{#2}\gotex@xkv@f@rcomma{#1}}}

% ── \define@boolkey[pre]{fam}[macpre]{key}[default]{fn}{errfn} ──────────────
% xkeyval.tex:206-232. It creates \if<macpre><key> with \newif, and a key handler
% that checks the value against "true,false" and runs \<macpre><key><value> — so
% \define@boolkey+{acmart.cls}[@ACM@]{screen}[true]{..}{..} makes \if@ACM@screen
% and \@ACM@screentrue. The + form takes the second (error) function; without it
% a bad value is an error and only one function is read.
\def\define@boolkey{\gotex@xkv@plus\gotex@xkv@dbk}
\def\gotex@xkv@dbk{\@testopt\gotex@xkv@dbk@{KV}}
\def\gotex@xkv@dbk@[#1]#2{%
  \gotex@xkv@makepf{#1}\gotex@xkv@makehd{#2}%
  \@testopt\gotex@xkv@dbk@@\XKV@header}
\def\gotex@xkv@dbk@@[#1]#2{%
  \@ifnextchar[{\gotex@xkv@dbk@@@{#1}{#2}}{\gotex@xkv@dbk@@@{#1}{#2}[]}}
\def\gotex@xkv@dbk@@@#1#2[#3]#4{%
  \expandafter\newif\csname if#1#2\endcsname
  \def\gotex@xkv@t{#3}%
  \ifx\gotex@xkv@t\@empty\else\gotex@xkv@setdefault{#2}{#3}\fi
  % The error function is only there in the + form, and it is read from the
  % SOURCE — so the conditional must be closed before the grab, or the \fi lands
  % in the argument. This is xkeyval's own \XKV@afterelsefi / \XKV@afterfi idiom
  % (xkeyval.tex:207-212).
  \ifgotex@xkv@pl
    \expandafter\gotex@xkv@dbk@two
  \else
    \expandafter\gotex@xkv@dbk@one
  \fi
  {#1}{#2}{#4}}
\def\gotex@xkv@dbk@two#1#2#3#4{\gotex@xkv@d@fbool{#1}{#2}{#3}{#4}}
\def\gotex@xkv@dbk@one#1#2#3{\gotex@xkv@d@fbool{#1}{#2}{#3}{}}
\def\gotex@xkv@d@fbool#1#2#3#4{%
  \expandafter\def\csname\XKV@header#2\endcsname##1{%
    \def\gotex@xkv@bv{##1}%
    \expandafter\ifx\csname#1#2##1\endcsname\relax
      #4%
    \else
      \csname#1#2##1\endcsname
      #3%
    \fi}}

% ── \define@choicekey*+{fam}{key}[\val\nr]{choices}[default]{fn}{errfn} ─────
% xkeyval.tex:176-205 and \XKV@@ch@ckchoice, :288-300. \val takes the value and
% \nr its 0-based position in the choice list (-1 when absent). The star form
% matches case-insensitively; here the value is compared as given, which is what
% every corpus use needs — acmart's own \ExecuteOptionsX and the document's class
% options are written in the same case as the list.
\def\define@choicekey{\gotex@xkv@star{\gotex@xkv@plus\gotex@xkv@dck}}
\def\gotex@xkv@dck{\@testopt\gotex@xkv@dck@{KV}}
\def\gotex@xkv@dck@[#1]#2#3{%
  \gotex@xkv@makepf{#1}\gotex@xkv@makehd{#2}%
  \@testopt{\gotex@xkv@dck@@{#3}}{}}
\def\gotex@xkv@dck@@#1[#2]#3{%
  \def\gotex@xkv@bin{#2}%
  \def\gotex@xkv@choices{#3}%
  \@ifnextchar[{\gotex@xkv@dck@@@{#1}}{\gotex@xkv@dck@@@{#1}[]}}
\def\gotex@xkv@dck@@@#1[#2]#3{%
  \def\gotex@xkv@t{#2}%
  \ifx\gotex@xkv@t\@empty\else\gotex@xkv@setdefault{#1}{#2}\fi
  \ifgotex@xkv@pl
    \expandafter\gotex@xkv@dck@two
  \else
    \expandafter\gotex@xkv@dck@one
  \fi
  {#1}{#3}}
\def\gotex@xkv@dck@two#1#2#3{\gotex@xkv@d@fchoice{#1}{#2}{#3}}
\def\gotex@xkv@dck@one#1#2{\gotex@xkv@d@fchoice{#1}{#2}{}}
% The choice list AND the [\val\nr] pair are baked into the handler body. Holding
% either in a shared macro read at use time is wrong: acmart declares three choice
% keys, and the last one's \val\nr would then serve all three.
\def\gotex@xkv@d@fchoice#1#2#3{%
  \expandafter\gotex@xkv@d@fch@ice\expandafter{\gotex@xkv@choices}%
    {\XKV@header#1}{#2}{#3}}
\def\gotex@xkv@d@fch@ice#1#2#3#4{%
  \expandafter\gotex@xkv@d@fch@@ce\expandafter{\gotex@xkv@bin}{#1}{#2}{#3}{#4}}
\def\gotex@xkv@d@fch@@ce#1#2#3#4#5{%
  \expandafter\def\csname#3\endcsname##1{%
    \gotex@xkv@lookup{##1}{#2}{#1}%
    \ifnum\gotex@xkv@nr<\z@ #5\else #4\fi}}
% Walk the choice list, setting \gotex@xkv@nr to the 0-based index of the value
% (-1 if it is not there) and assigning the two [\val\nr] macros.
\newcount\gotex@xkv@nr
\newcount\gotex@xkv@i
\def\gotex@xkv@lookup#1#2#3{%
  \gotex@xkv@trim\gotex@xkv@want{#1}%
  \gotex@xkv@nr\m@ne \gotex@xkv@i\z@
  \gotex@xkv@forcomma{#2}\gotex@xkv@l@@kup
  \expandafter\gotex@xkv@assign\expandafter{\gotex@xkv@want}{#3}}
\def\gotex@xkv@l@@kup#1{%
  \gotex@xkv@trim\gotex@xkv@have{#1}%
  \ifnum\gotex@xkv@nr<\z@
    \ifx\gotex@xkv@have\gotex@xkv@want\gotex@xkv@nr\gotex@xkv@i\fi
  \fi
  \advance\gotex@xkv@i\@ne}
\def\gotex@xkv@assign#1#2{\gotex@xkv@ass@gn#2\@nil{#1}}
\def\gotex@xkv@ass@gn#1#2\@nil#3{%
  \def#1{#3}%
  \def\gotex@xkv@t{#2}%
  \ifx\gotex@xkv@t\@empty\else\edef#2{\the\gotex@xkv@nr}\fi}

% ── the * and + variant markers (xkeyval.tex:70-71, 97-100) ─────────────────
% \XKV@ifstar and \XKV@ifplus are the same shape; \XKV@testopta tests the star
% and \XKV@t@stopta the plus, so \define@choicekey*+ carries both.
\newif\ifgotex@xkv@st
\newif\ifgotex@xkv@pl
\def\gotex@xkv@star#1{%
  \@ifnextchar*{\gotex@xkv@sttrue\gotex@xkv@st@r{#1}}{\gotex@xkv@stfalse#1}}
\def\gotex@xkv@st@r#1*{#1}
\def\gotex@xkv@plus#1{%
  \@ifnextchar+{\gotex@xkv@pltrue\gotex@xkv@pl@s{#1}}{\gotex@xkv@plfalse#1}}
\def\gotex@xkv@pl@s#1+{#1}

% ── class/package options (xkeyval.sty:85-96) ──────────────────────────────
% \DeclareOptionX[pre]<fam>{key}[default]{code} is \define@key with the family
% defaulting to the current file, \ExecuteOptionsX is \setkeys, and
% \ProcessOptionsX applies the options the document actually asked for.
\def\DeclareOptionX{\gotex@xkv@star\gotex@xkv@dox}
\def\gotex@xkv@dox{%
  \ifgotex@xkv@st\expandafter\gotex@xkv@doxstar\else\expandafter\gotex@xkv@d@x\fi}
\long\def\gotex@xkv@doxstar#1{\def\gotex@xkv@catchall{#1}}
\def\gotex@xkv@d@x{\@testopt\gotex@xkv@d@x@{KV}}
\def\gotex@xkv@d@x@[#1]{%
  \@ifnextchar<{\gotex@xkv@d@x@fam{#1}}{\gotex@xkv@d@x@cur{#1}}}
% \@testopt{...}{} — a declared option ALWAYS gets a default, empty when the
% source gives none (xkeyval.sty:94). Without it a bare class option, which is
% how every document names its format, matches no @default and does nothing.
\def\gotex@xkv@d@x@fam#1<#2>#3{\@testopt{\define@key[#1]{#2}{#3}}{}}
\def\gotex@xkv@d@x@cur#1#2{%
  \gotex@xkv@thisfile
  \expandafter\gotex@xkv@d@x@run\expandafter{\gotex@xkv@f}{#1}{#2}}
\def\gotex@xkv@d@x@run#1#2#3{\@testopt{\define@key[#2]{#1}{#3}}{}}
\def\ExecuteOptionsX{\@testopt\gotex@xkv@eox{KV}}
\def\gotex@xkv@eox[#1]{%
  \@ifnextchar<{\gotex@xkv@eox@fam{#1}}{\gotex@xkv@eox@cur{#1}}}
\def\gotex@xkv@eox@fam#1<#2>#3{\setkeys[#1]{#2}{#3}}
\def\gotex@xkv@eox@cur#1#2{%
  \gotex@xkv@thisfile
  \expandafter\gotex@xkv@eox@run\expandafter{\gotex@xkv@f}{#1}{#2}}
\def\gotex@xkv@eox@run#1#2#3{\setkeys[#2]{#1}{#3}}
\def\ProcessOptionsX{\gotex@xkv@star\gotex@xkv@pox}
\def\gotex@xkv@pox{\@testopt\gotex@xkv@pox@{KV}}
\def\gotex@xkv@pox@[#1]{%
  \@ifnextchar<{\gotex@xkv@pox@fam{#1}}{\gotex@xkv@pox@cur{#1}}}
\def\gotex@xkv@pox@fam#1<#2>{\gotex@xkv@p@x@{#1}{#2}}
\def\gotex@xkv@pox@cur#1{%
  \gotex@xkv@thisfile
  \expandafter\gotex@xkv@pox@r@n\expandafter{\gotex@xkv@f}{#1}}
\def\gotex@xkv@pox@r@n#1#2{\gotex@xkv@p@x@{#2}{#1}}
% The options the document actually asked for. \@classoptionslist is the class's;
% a package's own list is not modelled (nothing in the corpus reads one).
\def\gotex@xkv@p@x@#1#2{%
  \edef\gotex@xkv@o{\@classoptionslist}%
  \expandafter\gotex@xkv@p@x@@\expandafter{\gotex@xkv@o}{#1}{#2}}
\def\gotex@xkv@p@x@@#1#2#3{\setkeys[#2]{#3}{#1}}
% The family a class or package declares its options under: "<name>.<ext>", as
% xkeyval's \XKV@@t@st@pte does (xkeyval.sty:76-83). If the engine leaves those
% empty while reading a class, every one of \DeclareOptionX, \ExecuteOptionsX and
% \ProcessOptionsX sees the SAME empty family, so they still agree with each
% other — which is all that matters for reading a document's class options.
\def\gotex@xkv@thisfile{\edef\gotex@xkv@f{\@currname.\@currext}}
% \define@cmdkey[pre]{fam}[macpre]{key}[default]{code}: the value is stored in
% \<macpre><key> before the code runs (xkeyval.tex:161-168).
\def\define@cmdkey{\gotex@xkv@star\gotex@xkv@dcmk}
\def\gotex@xkv@dcmk{\@testopt\gotex@xkv@dcmk@{KV}}
\def\gotex@xkv@dcmk@[#1]#2{%
  \gotex@xkv@makepf{#1}\gotex@xkv@makehd{#2}%
  \@testopt\gotex@xkv@dcmk@@{cmd}}
\def\gotex@xkv@dcmk@@[#1]#2{%
  \@ifnextchar[{\gotex@xkv@dcmk@@@{#1}{#2}}{\gotex@xkv@dcmk@@@{#1}{#2}[]}}
\def\gotex@xkv@dcmk@@@#1#2[#3]#4{%
  \def\gotex@xkv@t{#3}%
  \ifx\gotex@xkv@t\@empty\else\gotex@xkv@setdefault{#2}{#3}\fi
  \expandafter\def\csname\XKV@header#2\endcsname##1{%
    \expandafter\def\csname#1#2\endcsname{##1}#4}}
`
