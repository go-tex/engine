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
// This accumulates those commands into one centred block that \maketitle emits,
// so the content survives. It does NOT reproduce revtex's superscript-address
// layout (authors grouped under shared, numbered affiliations); the goal is
// faithful CONTENT, not that geometry. \author accumulates here — revtex allows
// several, each followed by its own \affiliation — rather than overwriting as the
// base article \author does. It runs at document time (from \documentclass), where
// @ is an "other" character, so it opens with \makeatletter for its @-commands.
const RevtexAuthorBlock = `
\makeatletter
\def\@revtexauthblock{}
\def\author#1{\g@addto@macro\@revtexauthblock{{#1}\par}}
\def\affiliation#1{\g@addto@macro\@revtexauthblock{{\itshape #1}\par}}
\def\collaboration#1{\g@addto@macro\@revtexauthblock{{\bfseries #1}\par}}
\def\noaffiliation{}
\def\altaffiliation{\@ifnextchar[\@revtexaltopt\@revtexaltmand}
\def\@revtexaltopt[#1]#2{\g@addto@macro\@revtexauthblock{{\itshape #1#2}\par}}
\def\@revtexaltmand#1{\g@addto@macro\@revtexauthblock{{\itshape #1}\par}}
\def\email{\@ifnextchar[\@revtexcontopt\@revtexcontmand}
\def\homepage{\@ifnextchar[\@revtexcontopt\@revtexcontmand}
\def\@revtexcontopt[#1]#2{\g@addto@macro\@revtexauthblock{{#1#2}\par}}
\def\@revtexcontmand#1{\g@addto@macro\@revtexauthblock{{#1}\par}}
\def\preprint#1{}
\def\pacs#1{}
\def\keywords#1{}
\long\def\maketitle{\par\begin{center}{\large\bfseries\@title\par}\medskip\@revtexauthblock\end{center}\par\bigskip}
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
