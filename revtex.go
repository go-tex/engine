// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// RevtexAuthorBlock is a pragmatic revtex4 / revtex4-1 / revtex4-2 title-block
// emulation, loaded for a revtex document class when the real revtex .cls is not
// bundled with the paper (the common case — revtex is not embedded).
//
// The article-shaped fallback leaves \affiliation and revtex's other author-list
// commands undefined. That is the single most frequent "undefined control
// sequence" across the arXiv corpus (~11% of papers): it strict-fails, and in
// lenient mode it silently DROPS the whole author / affiliation block — the
// names, affiliations and contact lines vanish, because each undefined command
// swallows nothing while its braced argument lands in a discarded preamble area.
//
// This accumulates those commands into a centred block that \maketitle emits, so
// the content survives. It does NOT reproduce revtex's superscript-address layout
// (authors grouped under shared, numbered affiliations); the goal is faithful
// CONTENT, not that geometry. But the block is kept COMPACT — all authors on one
// wrapped paragraph (comma-joined), all affiliations/contacts on one wrapped
// italic paragraph (semicolon-joined) — rather than one line per command. A
// one-line-per-command block is far TALLER than real revtex's grouped format for a
// many-author paper, which pushes the whole body down and wrecks the word-POSITION
// match against the reference even though every word is present (measured: 2605.12538,
// 7 authors, went to 5.8 layout-divergence at 94.5% recall). Comma/semicolon joining
// keeps the block height near revtex's, recovering that layout without losing content.
// revtex prints NO "Abstract" heading — checked against the real class through
// tectonic, on the minimal document and on a corpus paper: page one goes straight
// from the affiliations to the abstract's own text. The generic \abstract
// (latex.go) centres one, which is right for article and wrong here, so this does
// not carry it over.
//
// The ABSTRACT is captured, not typeset where it stands. In revtex the abstract is
// written BEFORE \maketitle and emitted BY it, after the authors; the generic
// \abstract (latex.go) sets it in place, so the playground and every rendered revtex
// paper carried its abstract ABOVE its own title. It is collected into a box with
// \global\setbox — local would be restored by the \endgroup that \end{abstract}
// runs, and the abstract would vanish — and \maketitle places it. An abstract that
// comes AFTER \maketitle (some papers do) is emitted as soon as it closes, so the
// order is right either way and nothing is ever dropped.
//
// \author accumulates here — revtex allows several, each followed by its own
// \affiliation — rather than overwriting as the base article \author does. It runs at
// document time (from \documentclass), where @ is an "other" character, so it opens
// with \makeatletter for its @-commands.
const RevtexAuthorBlock = `
\makeatletter
\def\@revtexauthors{}
\def\@revtexaffils{}
\def\author#1{\g@addto@macro\@revtexauthors{#1, }}
\def\affiliation#1{\g@addto@macro\@revtexaffils{#1; }}
\def\collaboration#1{\g@addto@macro\@revtexauthors{#1, }}
\def\noaffiliation{}
\def\altaffiliation{\@ifnextchar[\@revtexaltopt\@revtexaltmand}
\def\@revtexaltopt[#1]#2{\g@addto@macro\@revtexaffils{#1#2; }}
\def\@revtexaltmand#1{\g@addto@macro\@revtexaffils{#1; }}
\def\email{\@ifnextchar[\@revtexcontopt\@revtexcontmand}
\def\homepage{\@ifnextchar[\@revtexcontopt\@revtexcontmand}
\def\@revtexcontopt[#1]#2{\g@addto@macro\@revtexaffils{#1#2; }}
\def\@revtexcontmand#1{\g@addto@macro\@revtexaffils{#1; }}
\def\preprint#1{}
\def\pacs#1{}
\def\keywords#1{}
\newbox\@revtexabsbox
\newif\if@revtexmadetitle
\long\def\abstract{\global\setbox\@revtexabsbox\vbox\bgroup
  \leftskip=20pt \rightskip=20pt \small}
\def\endabstract{\egroup\if@revtexmadetitle\@revtexputabstract\fi}
\def\@revtexputabstract{\par\bigskip\noindent\box\@revtexabsbox\par}
\long\def\maketitle{\par\begin{center}{\large\bfseries\@title\par}\medskip{\@revtexauthors\par}\smallskip{\itshape\@revtexaffils\par}\end{center}\@revtexputabstract\@revtexmadetitletrue\par\bigskip\gotex@revtexbodytwocol}
\makeatother
`

// isRevtexClass reports whether name is one of the revtex document classes.
func isRevtexClass(name string) bool {
	switch name {
	case "revtex4", "revtex4-1", "revtex4-2":
		return true
	}
	return false
}

// loadRevtexEmulation injects the revtex title-block emulation.
func (e *Engine) loadRevtexEmulation() error { return e.LoadFormat(RevtexAuthorBlock) }
