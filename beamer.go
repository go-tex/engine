// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// BeamerClassKernel is the built-in emulation of the beamer presentation class.
// beamer is built on pgf/tikz (out of scope here) and far too large to load its real
// .cls, so the engine EMULATES its document structure instead: every frame becomes a
// page, frame titles become headings, blocks render as titled paragraphs, and overlay
// specifications (\pause, <1->, \only, \uncover, …) are shown STATICALLY — all the
// material of a frame at once. Themes and pgf styling are gobbled. The goal is that a
// beamer talk RENDERS its content as a sequence of pages, not pixel-fidelity to a
// themed slide. Loaded by doDocumentClass when \documentclass{beamer} is seen.
const BeamerClassKernel = `\makeatletter
% ── overlay specifications <…>, shown statically ───────────────────────────────
\long\def\bmr@eatov<#1>{}
\def\pause{\@ifnextchar<\bmr@eatov\relax}
% \only/\uncover/\visible/\onslide/\invisible: an optional <ov>, then reveal {content}
\def\only{\@ifnextchar<{\bmr@ovarg}{\@firstofone}}
\long\def\bmr@ovarg<#1>{\@ifnextchar\bgroup{\@firstofone}{}}
\let\uncover\only \let\visible\only \let\onslide\only \let\invisible\only
% \alt<ov>{A}{B} → A (the "active" branch)
\def\alt{\@ifnextchar<{\bmr@altov}{\@firstoftwo}}
\long\def\bmr@altov<#1>#2#3{#2}
% \alert<ov>{X} → bold X
\def\alert{\@ifnextchar<{\bmr@alertov}{\bmr@doalert}}
\long\def\bmr@alertov<#1>{\bmr@doalert}
\long\def\bmr@doalert#1{\textbf{#1}}
% \item may carry a leading <ov>
\let\bmr@item\item
\def\item{\@ifnextchar<{\bmr@itemov}{\bmr@item}}
\def\bmr@itemov<#1>{\bmr@item}
% ── frame titles ───────────────────────────────────────────────────────────────
\long\def\bmr@ft#1{\par\bigskip\noindent{\Large\bfseries #1}\par\nobreak\smallskip}
\def\frametitle{\@ifnextchar<{\bmr@ftov}{\bmr@ft}}
\long\def\bmr@ftov<#1>{\bmr@ft}
\long\def\framesubtitle#1{\noindent{\large\itshape #1}\par\smallskip}
% ── frame environment: each frame is a page ────────────────────────────────────
\newenvironment{frame}{\clearpage\bmr@frameopt}{\par}
\def\bmr@frameopt{\@ifnextchar<\bmr@frameov\bmr@framebrk}
\def\bmr@frameov<#1>{\bmr@framebrk}
\def\bmr@framebrk{\@ifnextchar[{\bmr@fbrk}{\bmr@fttl}}
\def\bmr@fbrk[#1]{\bmr@fttl}
\def\bmr@fttl{\@ifnextchar\bgroup{\bmr@dottl}{}}
\long\def\bmr@dottl#1{\frametitle{#1}\@ifnextchar\bgroup{\bmr@dosub}{}}
\long\def\bmr@dosub#1{\framesubtitle{#1}}
% \frame{…} command form, e.g. \frame{\titlepage}
\def\frame{\@ifnextchar[{\bmr@framecmdo}{\bmr@framecmd}}
\long\def\bmr@framecmdo[#1]#2{\clearpage #2\par}
\long\def\bmr@framecmd#1{\clearpage #1\par}
% ── blocks ─────────────────────────────────────────────────────────────────────
\newenvironment{block}[1]{\par\medskip\noindent{\bfseries #1}\par\nobreak}{\par\medskip}
\newenvironment{alertblock}[1]{\par\medskip\noindent{\bfseries #1}\par\nobreak}{\par\medskip}
\newenvironment{exampleblock}[1]{\par\medskip\noindent{\bfseries #1}\par\nobreak}{\par\medskip}
% ── columns (command form \column{width}; env is a plain wrapper) ───────────────
\newenvironment{columns}{\par}{\par}
\def\column{\@ifnextchar[{\bmr@colo}{\bmr@col}}
\def\bmr@colo[#1]#2{\par}
\def\bmr@col#1{\par}
% ── title page ─────────────────────────────────────────────────────────────────
\def\titlepage{\begin{center}{\LARGE\bfseries\@title\par}\bigskip{\large\@author\par}\medskip{\@date\par}\end{center}\par}
\def\maketitle{\frame{\titlepage}}
\def\subtitle#1{}
\def\institute{\@ifnextchar[{\bmr@insto}{\@gobble}}
\def\bmr@insto[#1]#2{}
% ── theming / templates / pgf: gobble ──────────────────────────────────────────
\def\usetheme{\@ifnextchar[{\bmr@1o}{\@gobble}}
\def\bmr@1o[#1]#2{}
\let\usecolortheme\usetheme \let\usefonttheme\usetheme
\let\useinnertheme\usetheme \let\useoutertheme\usetheme
\def\setbeamertemplate{\@ifnextchar[{\bmr@sbt@o}{\bmr@sbt}}
\def\bmr@sbt#1{\@ifnextchar[{\bmr@sbt@ao}{\@gobble}}
\def\bmr@sbt@ao[#1]#2{}
\def\bmr@sbt@o[#1]#2{}
\def\setbeamercolor{\@ifnextchar*{\bmr@sbc@s}{\@gobbletwo}}
\def\bmr@sbc@s*{\@gobbletwo}
\def\setbeamerfont{\@gobbletwo}
\def\setbeamersize{\@gobble}
\def\setbeamercovered#1{}
\def\beamertemplatenavigationsymbolsempty{}
\def\logo#1{}
\def\beamerdefaultoverlayspecification{\@gobble}
% ── other beamer commands: gobble (they add no visible slide content here) ──────
% \note<ov>[opt]{text} — speaker notes, hidden in presentation mode
\def\note{\@ifnextchar<{\bmr@note@ov}{\bmr@note@a}}
\def\bmr@note@ov<#1>{\bmr@note@a}
\def\bmr@note@a{\@ifnextchar[{\bmr@note@o}{\@gobble}}
\def\bmr@note@o[#1]{\@ifnextchar\bgroup\@gobble{}}
% \mode<..> and \mode<..>{..} — presentation/article mode switches
\def\mode{\@ifnextchar<{\bmr@eatov}{}}
\def\againframe{\@ifnextchar<{\bmr@again@ov}{\@gobble}}
\def\bmr@again@ov<#1>{\@ifnextchar[{\bmr@again@o}{\@gobble}}
\def\bmr@again@o[#1]{\@gobble}
\def\transdissolve{\@ifnextchar<\bmr@eatov\relax}
\let\transboxin\transdissolve \let\transboxout\transdissolve \let\transblindshorizontal\transdissolve
\let\transblindsvertical\transdissolve \let\transwipe\transdissolve \let\transsplitverticalin\transdissolve
\let\transglitter\transdissolve \let\transduration\transdissolve
\def\movie{\@ifnextchar[{\bmr@2o}{\@gobbletwo}}
\def\bmr@2o[#1]#2#3{}
\let\sound\movie \let\animate\pause \let\animatevalue\@gobblethree
\def\hyperlink{\@ifnextchar<{\bmr@hl@ov}{\bmr@hl}}
\def\bmr@hl@ov<#1>#2#3{#3}
\def\bmr@hl#1#2{#2}
\def\beamerbutton#1{[#1]}
\def\beamergotobutton#1{[#1]}
\def\beamerreturnbutton#1{[#1]}
\def\beamerskipbutton#1{[#1]}
\def\metroset#1{}
\def\insertframenumber{}
\def\insertsectionhead{}
\def\metropolis@disablegreenboxes{}
\makeatother
`

// loadBeamer installs the beamer emulation on top of the standard class kernel.
func (e *Engine) loadBeamer() error { return e.LoadFormat(BeamerClassKernel) }
