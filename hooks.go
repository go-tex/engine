// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file holds two things a package written after 2020 asks about before it
// does anything else: HOW OLD THE FORMAT IS, and the HOOK system that a modern
// format offers instead of patching kernel macros.
//
// The question is not academic. etoolbox ends with
//
//	\IfFormatAtLeastTF{2020-10-01}
//	  {\newrobustcmd*{\AtEndPreamble}{\AddToHook{begindocument/before}}%
//	   … \endinput}
//	  {}
//
// and then, in the branch for an OLD format, prepends code to \document and
// \patchcm ds \enddocument. With no \IfFormatAtLeastTF at all the engine fell
// through to that old branch, whose \patchcmd cannot match this kernel's macros:
// the patches failed, their code leaked into the page, and the rest of the
// document was swallowed. beamer's compatibility layer branches the same way.
//
// Answering "yes, at least 2020-10-01" is both the truthful answer — the hook
// interface below is exactly what that release introduced — and the one that puts
// packages on the code path that asks the format for a service rather than
// rewriting the format's internals.
//
// ── What the hook layer models, and what it does not ─────────────────────────
//
// A hook is a named token list that the format executes at a defined moment. The
// engine stores one macro per hook (gotex@hook@<name>) and runs it at that
// moment. Modelled:
//
//   - \NewHook / \NewReversedHook / \ProvideHook / \NewMirroredHookPair — declare.
//   - \AddToHook{<name>}[<label>]{<code>} — append; the hook need not exist yet.
//   - \AddToHookNext{<name>}{<code>} — code that fires at the NEXT use only.
//   - \UseHook / \UseOneTimeHook — run (and clear the next-use list).
//   - \RemoveFromHook, \IfHookEmptyTF, \ShowHook / \LogHook, \ActivateGenericHook.
//   - The document hooks are WIRED: begindocument/before, begindocument,
//     begindocument/end and env/document/begin all fire at \begin{document};
//     enddocument and its four sub-hooks fire at \end{document}.
//   - package/<name>/after and file/<name>/after fire from the package loader
//     when that package or file finishes loading (see loadTeXFile).
//
// NOT modelled, deliberately:
//
//   - ORDERING. \DeclareHookRule and the [<label>] argument are accepted and
//     ignored; code runs in the order it was added. \RemoveFromHook therefore
//     empties the whole hook rather than one label's contribution.
//   - cmd/<name>/before and cmd/<name>/after. Those ask the format to inject code
//     into the BODY of an existing command, which needs the command's argument
//     count to place the "after" part. The engine records such a hook and never
//     fires it. The one caller that reaches this in a beamer talk is the
//     pdfpages integration, which needs pdfpages itself.
//   - Reversed hooks run in the order added, like ordinary ones (ordering again).
const LaTeXHooks = `\catcode64=11
% ── format version ──────────────────────────────────────────────────────────
% \fmtversion is the format's release date. The engine states the release whose
% interfaces it offers: the hook layer below is the 2020-10-01 one. A package
% asking for anything later takes its pre-hook branch, which is the safe
% direction — it asks for less.
\def\fmtversion{2020-10-01}
\def\fmtname{gotex}
% \@ifl@t@r{<a>}{<b>}{<then>}{<else>}: <then> iff date <a> is at least date <b>.
% Both spellings of a LaTeX date are read — 1994/06/01 and 2020-10-01 — because
% the format switched from slashes to dashes and package code still uses both.
% This is the kernel's own two-stage parse: strip the slashes, then the dashes,
% leaving a comparable eight-digit number. Checked against real TeX, including
% the boundary case: a date is "at least" itself.
\def\@parse@version#1/#2/#3#4#5\@nil{\@parse@version@dash#1-#2-#3#4\@nil}
% The TRAILING SPACE after #4 is load-bearing, and the real kernel has it: the
% expansion is read as a <number>, and TeX keeps scanning digits until something
% that is not one arrives. Without the space the next token is \expandafter,
% which the scan expands — and the conditional that follows unravels (verified
% against real TeX, which produces literal "\relax {Y}{N}" for that shape).
\def\@parse@version@dash#1-#2-#3#4#5\@nil{\if\relax#2\relax\else#1\fi#2#3#4 %
}
\let\@parse@version@\@parse@version
\def\@ifl@t@r#1#2{%
  \ifnum\expandafter\@parse@version#1//00\@nil<\expandafter\@parse@version#2//00\@nil
    \expandafter\@secondoftwo
  \else
    \expandafter\@firstoftwo
  \fi}
\def\IfFormatAtLeastTF{\@ifl@t@r\fmtversion}
\def\IfFormatAtLeastT#1#2{\IfFormatAtLeastTF{#1}{#2}{}}
\def\IfFormatAtLeastF#1#2{\IfFormatAtLeastTF{#1}{}{#2}}
% ── hooks ───────────────────────────────────────────────────────────────────
% A hook is the macro gotex@hook@<name>; its next-use companion is
% gotex@hooknext@<name>. Declaring a hook is just making sure both exist, so
% \AddToHook to an undeclared hook works exactly as it does in the real format.
\def\gotex@hookinit#1{%
  \@ifundefined{gotex@hook@#1}{\@namedef{gotex@hook@#1}{}}{}%
  \@ifundefined{gotex@hooknext@#1}{\@namedef{gotex@hooknext@#1}{}}{}}
\def\NewHook#1{\gotex@hookinit{#1}}
\let\NewReversedHook\NewHook
\let\ProvideHook\NewHook
\def\NewMirroredHookPair#1#2{\gotex@hookinit{#1}\gotex@hookinit{#2}}
\let\NewReversedMirroredHookPair\NewMirroredHookPair
\def\ActivateGenericHook#1{\gotex@hookinit{#1}}
\def\DeclareHookRule#1#2#3#4{}
\def\DeclareDefaultHookLabel#1{}
\def\DeclareHookrule#1#2#3#4{}
% \AddToHook{<name>}[<label>]{<code>}: the label is read so it cannot reach the
% page, then dropped — this engine does not order a hook's contributions.
\def\AddToHook#1{\@ifnextchar[{\gotex@addtohook@lbl{#1}}{\gotex@addtohook{#1}{}}}
\def\gotex@addtohook@lbl#1[#2]{\gotex@addtohook{#1}{#2}}
\long\def\gotex@addtohook#1#2#3{%
  \gotex@hookinit{#1}%
  \expandafter\gotex@gaddto\csname gotex@hook@#1\endcsname{#3}}
\long\def\AddToHookNext#1#2{%
  \gotex@hookinit{#1}%
  \expandafter\gotex@gaddto\csname gotex@hooknext@#1\endcsname{#2}}
% \gotex@gaddto\cs{code} appends code to \cs globally, keeping what is there.
\long\def\gotex@gaddto#1#2{%
  \expandafter\gdef\expandafter#1\expandafter{#1#2}}
% \UseHook runs the hook and then its next-use list, clearing the latter.
\def\UseHook#1{%
  \@ifundefined{gotex@hook@#1}{}{%
    \csname gotex@hook@#1\endcsname
    \csname gotex@hooknext@#1\endcsname
    \@namedef{gotex@hooknext@#1}{}}}
\let\UseOneTimeHook\UseHook
\def\RemoveFromHook#1{\@ifnextchar[{\gotex@remhook@lbl{#1}}{\gotex@remhook{#1}}}
\def\gotex@remhook@lbl#1[#2]{\gotex@remhook{#1}}
\def\gotex@remhook#1{\@namedef{gotex@hook@#1}{}}
\def\IfHookEmptyTF#1{%
  \@ifundefined{gotex@hook@#1}{\@firstoftwo}{%
    \expandafter\ifx\csname gotex@hook@#1\endcsname\@empty
      \expandafter\@firstoftwo\else\expandafter\@secondoftwo\fi}}
\def\ShowHook#1{}
\def\LogHook#1{}
\def\DebugHooksOn{}
\def\DebugHooksOff{}
\catcode64=12\relax
`
