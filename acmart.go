// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// AcmartMetadata stubs acmart's top-matter METADATA commands, loaded for
// \documentclass{acmart} when the real acmart.cls is not bundled (it is not
// embedded, so acmart otherwise runs the amsart-shaped emulation). These commands
// carry bibliographic metadata — copyright, DOI, conference, CCS concepts — that
// acmart records and typesets in its own strip; the emulation has none of that, so
// left undefined they strict-fail and, lenient, typeset their arguments as body
// text (\setcopyright is the corpus's 3rd most common undefined control sequence,
// and the CCSXML block dumps a wall of raw XML). Gobbling them keeps the
// document's real content clean. A bundled acmart.cls defines them itself.
//
// Loaded with @ already an "other" character (document time), so it opens with
// \makeatletter for its @-helpers.
const AcmartMetadata = `
\makeatletter
\def\setcopyright#1{}
\def\copyrightyear#1{}
\def\acmYear#1{}
\def\acmMonth#1{}
\def\acmDay#1{}
\def\acmDOI#1{}
\def\acmISBN#1{}
\def\acmPrice#1{}
\def\acmVolume#1{}
\def\acmNumber#1{}
\def\acmArticle#1{}
\def\acmArticleSeq#1{}
\def\acmJournal#1{}
\def\acmBooktitle#1{}
\def\acmSubmissionID#1{}
\def\acmBadgeL{\@ifnextchar[\@acmbadgeopt\@acmbadgemand}
\def\@acmbadgeopt[#1]#2{}
\def\@acmbadgemand#1{}
\let\acmBadgeR\acmBadgeL
\def\citestyle#1{}
\def\settopmatter#1{}
\def\titlenote#1{}
\def\authornote#1{}
\def\subtitle#1{}
\def\orcid#1{}
\def\authorsaddresses#1{}
\def\startPage#1{}
\def\terms#1{}
\def\keywords#1{}
\def\received{\@ifnextchar[\@acmrecvopt\@acmrecvmand}
\def\@acmrecvopt[#1]#2{}
\def\@acmrecvmand#1{}
\def\ccsdesc{\@ifnextchar[\@ccsdescopt\@ccsdescmand}
\def\@ccsdescopt[#1]#2{}
\def\@ccsdescmand#1{}
\def\acmConference{\@ifnextchar[\@acmconfopt\@acmconfmand}
\def\@acmconfopt[#1]#2#3#4{}
\def\@acmconfmand#1#2#3{}
% acmart author block: like revtex, acmart uses \author + \affiliation + \email
% (each optionally bracketed), and \affiliation wraps \institution / \city /
% \country / … sub-fields. The article emulation's \author overwrites and knows
% none of these, so accumulate them into a centred block \maketitle emits, and
% make the sub-fields identity wrappers so the affiliation text survives. Kept
% COMPACT — authors comma-joined on one wrapped paragraph, affiliations/emails
% semicolon-joined on one wrapped italic paragraph — rather than one line per
% command: a tall per-command block pushes the body down and breaks the word-
% position match against the reference (see revtex.go for the measured effect).
\def\@acmauthors{}
\def\@acmaffils{}
\def\author{\@ifnextchar[\@acmauthopt\@acmauthmand}
\def\@acmauthopt[#1]#2{\g@addto@macro\@acmauthors{#2, }}
\def\@acmauthmand#1{\g@addto@macro\@acmauthors{#1, }}
\def\affiliation{\@ifnextchar[\@acmaffopt\@acmaffmand}
\def\@acmaffopt[#1]#2{\g@addto@macro\@acmaffils{#2; }}
\def\@acmaffmand#1{\g@addto@macro\@acmaffils{#1; }}
\def\additionalaffiliation#1{\g@addto@macro\@acmaffils{#1; }}
\def\email{\@ifnextchar[\@acmemailopt\@acmemailmand}
\def\@acmemailopt[#1]#2{\g@addto@macro\@acmaffils{#2; }}
\def\@acmemailmand#1{\g@addto@macro\@acmaffils{#1; }}
\def\institution#1{#1 }
\def\department#1{#1 }
\def\city#1{#1 }
\def\country#1{#1 }
\def\state#1{#1 }
\def\region#1{#1 }
\def\streetaddress#1{#1 }
\def\postcode#1{#1 }
\def\position#1{#1 }
\long\def\maketitle{\par\begin{center}{\large\bfseries\@title\par}\medskip{\@acmauthors\par}\smallskip{\itshape\@acmaffils\par}\end{center}\par\bigskip}
\makeatother
`

// loadAcmartMetadata injects the acmart metadata stubs and excludes the CCSXML
// concepts block (a raw-XML environment acmart reads mechanically) so its body is
// not typeset.
func (e *Engine) loadAcmartMetadata() error {
	e.registerExcludedComment("CCSXML")
	return e.LoadFormat(AcmartMetadata)
}
