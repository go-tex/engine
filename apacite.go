// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// ApaciteStubs recovers the apacite package's reference-list macros, loaded for
// \usepackage{apacite} / \RequirePackage{apacite} (many APA journal classes —
// agujournal, sn-jnl — require it) when the real apacite.sty is NOT bundled with
// the paper. When it IS bundled the real file resolves and loads and these stubs
// are not used.
//
// apacite drives an APA-style .bbl. Each reference is a \bibitem whose body is
// built from an APACrefauthors environment (the author names) followed by
// content-bearing macros: \APACrefYearMonthDay{year}{month}{day}, the title
// (\APACrefatitle{Title}{title} — apacite typesets the sentence-case SECOND arg),
// \APACjournalVolNumPages{journal}{vol}{num}{pages}, plus a run of short string
// producers (\BBA -> "&", \BCBL -> ",", \BPBI -> ". ", \BOthers -> "et al.").
//
// Left undefined these do not merely vanish: the engine's lenient path GOBBLES an
// undefined control sequence's braced arguments (see skipUndefined), so
// \APACrefYearMonthDay{2022}{}{} takes "2022" with it and \APACrefatitle{T}{t}
// takes the whole title. The reference list then shows the author names alone and
// drops the year, title, journal and pages — the defect these stubs repair.
//
// The definitions are apacite's own (from apacite.sty), pared to the macros that
// appear in real .bbl files, so the recovered text matches what apacite renders.
// The goal is CONTENT recovery, not pixel-perfect APA punctuation: the words
// (authors, year, title, journal, volume/pages, URL, DOI) appear instead of being
// dropped or leaking macro names. It opens with \makeatletter for apacite's
// @-helpers (\@empty tests, \@connect@with@commas) and closes with \makeatother.
const ApaciteStubs = `
\makeatletter
% ── short string / connective producers ─────────────────────────────────────
% No arguments, so undefined they only omit a separator rather than dropping
% content; defined they restore apacite's punctuation between authors and fields.
\newcommand{\BBOP}{(}   % opening parenthesis
\newcommand{\BBCP}{)}   % closing parenthesis
\newcommand{\BBOQ}{}    % opening quote for an article title (empty in APA)
\newcommand{\BBCQ}{}    % closing quote for an article title
\newcommand{\BBAA}{\&}  % ampersand between the last two authors in the ref. list
\newcommand{\BBAB}{and} % "and" between authors in an in-text citation
\newcommand{\BAnd}{\&}  % for "Ed. and Trans."
\newcommand{\BBA}{\BBAA}% apacite picks \BBAA in the ref. list
\DeclareRobustCommand{\BPBI}{.~}% period between initials
\DeclareRobustCommand{\BHBI}{.-}% hyphen between initials
\newcommand{\BAP}{ }
\newcommand{\BBAY}{, }  % between author(s) and year
\newcommand{\BBYY}{, }
\newcommand{\BBC}{; }
\newcommand{\BBN}{, }
\newcommand{\BCBT}{,}   % comma between two authors
\newcommand{\BCBL}{,}   % comma before the last author when there are > 2
\newcommand{\BDBL}{, \dots{} }
\newcommand{\BIP}{in press}
\newcommand{\BOthers}[1]{et al.\hbox{}}
\newcommand{\BOthersPeriod}[1]{et al.\hbox{}}
\newcommand{\BIn}{In}
\newcommand{\Bby}{by}
\newcommand{\BED}{Ed.\hbox{}}
\newcommand{\BEDS}{Eds.\hbox{}}
\newcommand{\BTRANS}{Trans.\hbox{}}
\newcommand{\BTRANSS}{Trans.\hbox{}}
\newcommand{\BTRANSL}{trans.\hbox{}}
\newcommand{\BVOL}{Vol.\hbox{}}
\newcommand{\BVOLS}{Vols.\hbox{}}
\newcommand{\BNUM}{No.\hbox{}}
\newcommand{\BNUMS}{Nos.\hbox{}}
\newcommand{\BEd}{ed.\hbox{}}
\newcommand{\BCHAP}{chap.\hbox{}}
\newcommand{\BCHAPS}{chap.\hbox{}}
\newcommand{\BPG}{p.\hbox{}}
\newcommand{\BPGS}{pp.\hbox{}}
\newcommand{\BPP}{pp.\hbox{}}
\newcommand{\BTR}{Tech.\ Rep.\hbox{}}
\newcommand{\BPhD}{Doctoral dissertation}
\newcommand{\BUPhD}{Unpublished doctoral dissertation}
\newcommand{\BMTh}{Master's thesis}
\newcommand{\BUMTh}{Unpublished master's thesis}
\newcommand{\BAuthor}{Author}
\newcommand{\BOWP}{Original work published}
\newcommand{\BREPR}{Reprinted from}
\newcommand{\BAvailFrom}{Available from\ }
\newcommand{\BRetrieved}[1]{Retrieved {#1}, from\ }
\newcommand{\BRetrievedFrom}{Retrieved from\ }
\newcommand{\BMsgPostedTo}{Message posted to\ }
\newcommand{\bibnodate}{n.d.\hbox{}}
\let\Bem\emph
% ── the year ────────────────────────────────────────────────────────────────
\newcommand{\APACmonth}[1]{\ifcase #1\or January\or February\or March\or
    April\or May\or June\or July\or August\or September\or October\or
    November\or December\or Winter\or Spring\or Summer\or Fall\else
    {#1}\fi}
\newcommand{\APACyear}[1]{{#1}}
\newcommand{\APACexlab}[1]{{#1}}
\newcommand{\APACrefYear}[1]{{\BBOP}{#1}{\BBCP}}
\newcommand{\APACrefYearMonthDay}[3]{%
  {\BBOP}{#1}%           year; should not be empty
  \ifx\@empty#2\@empty
    \ifx\@empty#3\@empty
    \else
      \unskip, {#3}%     day
    \fi
  \else
    \unskip, {#2}%       month
    \ifx\@empty#3\@empty
    \else
      \unskip~{#3}%      day
    \fi
  \fi
  {\BBCP}%
}
% ── titles ──────────────────────────────────────────────────────────────────
% apacite typesets the sentence-case SECOND argument of an article title.
\newcommand{\APACrefatitle}[2]{#2}
\newcommand{\APACrefbtitle}[2]{\Bem{#2}}
\newcommand{\APACrefaetitle}[2]{[#2]}
\newcommand{\APACrefbetitle}[2]{[#2]}
% ── the citation tail: journal, volume, (number), pages ─────────────────────
\newcommand{\APACjournalVolNumPages}[4]{%
  \Bem{#1}%             journal
  \ifx\@empty#2\@empty\else\unskip, \Bem{#2}\fi%  volume
  \ifx\@empty#3\@empty\else\unskip({#3})\fi%      issue number
  \ifx\@empty#4\@empty\else\unskip, {#4}\fi%      pages
}
% ── publisher / institution / school / edition-volume-report ────────────────
\def\@connect@with@commas#1{%
  \def\@comma@space{\unskip, }%
  \let\@connect@string\relax
  \@for\@element@:=#1\do{%
     \ifx\@empty\@element@%
     \else
       \@connect@string\@element@%
       \let\@connect@string\@comma@space
     \fi
  }%
  \let\@connect@string\@undefined
  \let\@comma@space\@undefined
}
\newcommand{\APACaddressPublisher}[2]{%
  \ifx\@empty#1\@empty
    \ifx\@empty#2\@empty\else{#2}\fi%              publisher
  \else
    {#1}%                                          address
    \ifx\@empty#2\@empty\else\unskip: {#2}\fi%     address: publisher
  \fi
}
\let\APACaddressInstitution\APACaddressPublisher
\newcommand{\APACaddressSchool}[2]{\@connect@with@commas{{#1},{#2}}}
\newcommand{\APACtypeAddressSchool}[3]{\@connect@with@commas{{#1},{#2},{#3}}}
\newcommand{\APACbVolEdTR}[2]{%
  \ifx\@empty#1\@empty
    \ifx\@empty#2\@empty\else{(#2)}\fi
  \else
    ({#1}%
    \ifx\@empty#2\@empty\else\unskip; \@connect@with@commas{{#2}}\fi
    )%
  \fi
}
\newcommand{\APACrefnote}[1]{\ifx\@empty#1\@empty\else({#1})\fi}
\let\APAChowpublished\relax
% ── DOI, its prefix and the \doi wrapper (identity: print the DOI text) ──────
\newcommand{\doiprefix}{doi:\penalty0{}}
\providecommand{\doi}[1]{#1}
% ── metadata / back-reference hooks that must NOT typeset their argument ─────
\newcommand{\APACinsertmetastar}[1]{}
\newcommand{\PrintBackRefs}[1]{}
\newcommand{\CurrentBib}{}
% \citeauthoryear composes a \bibitem's author-year LABEL; the engine numbers the
% list and never typesets that optional argument, so it gobbles its three parts.
\newcommand{\citeauthoryear}[3]{}
% ── environments: the body is the content, so process it transparently ───────
\newenvironment{APACrefauthors}{}{}
\newenvironment{APACrefURL}[1][]{%
  \ifx\@empty#1\@empty
    \BRetrievedFrom
  \else
    \BRetrieved{#1}%
  \fi
}{}
\newenvironment{APACrefDOI}{\doiprefix}{}
\makeatother
`

// loadApaciteStubs installs the apacite reference-list macros (ApaciteStubs) when a
// paper that uses apacite does not bundle apacite.sty. Called from the package
// loader for \usepackage{apacite} / \RequirePackage{apacite} once the real file has
// failed to resolve, so a bundled apacite.sty still wins.
func (e *Engine) loadApaciteStubs() error {
	if e.loadedPackages == nil {
		e.loadedPackages = map[string]bool{}
	}
	if e.loadedPackages["apacite"] {
		return nil // already loaded (stubs or real file)
	}
	e.loadedPackages["apacite"] = true
	return e.LoadFormat(ApaciteStubs)
}
