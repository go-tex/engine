// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// ElsarticleFrontMatter emulates the front matter of elsarticle.cls (3.3, 2020/11/20)
// for a paper that does not bundle the class — which is nearly all of them.
//
// Without it the article fallback leaves \ead, \cortext, \fntext, \tnotetext, \tnoteref,
// \fnref, \corref, \address, \affiliation and \journal undefined: in lenient mode their
// arguments were typeset raw, as "[c1]Corresponding author" in the body, and the title
// block carried no author marks, no footnotes at the foot of page one, no e-mail line and
// no "Preprint submitted to" footer. The line numbers below are elsarticle.cls's own.
//
// What is ported, nearly verbatim:
//
//	\title                       :120      \tnotetext/\tnoteref/\tnotemark  :130-148, :1355
//	\fntext/\fnref/\fnmark       :154-169  \cortext/\corref/\cormark          :171-190
//	\ead, \emailauthor, \urlauthor :192-233  \author                          :235-282
//	\address                     :636-648  \affiliation (key=value)           :564-633
//	abstract and keyword boxes   :650-723  \printFirstPageNotes               :728-751
//	\pprintMaketitle             :760-790  \finalMaketitle, \MaketitleBox     :801-826
//	\maketitle                   :887-984  \ps@pprintTitle                    :998-1022
//	\journal                     :1277     \nonumnote                         :150
//
// What differs, on purpose:
//
//   - The class defers a label to the NEXT run through the .aux file (\elsLabel writes
//     \Newlabel, \ead writes \emailauthor), so a first run prints the default mark. The
//     engine renders once, so a label is defined when its \tnotetext / \fntext / \cortext /
//     \address is READ, counting the same way the class does, and \ead calls \emailauthor
//     directly. The result is the second-run output, which is what the reference
//     (tectonic) prints.
//   - \affiliation's keys are read here by a comma/equals splitter, not keyval, because
//     the kernel gobbles \setkeys. Keys that set punctuation (op, ap, cp, ...) are
//     ignored; the default ", " after each value is used. An unknown key is dropped.
//   - 1p/3p (the CRC "final" layouts) use \finalMaketitle in one column; 5p runs it as
//     \twocolumn[\finalMaketitle] (:353-364), the title box spanning both columns. The
//     \vsplit of the long-title option and the highlights / graphical-abstract pages
//     are not reproduced.
//   - \usepackage{hyperref}'s title/author hooks (:284-289) are not needed here.
const ElsarticleFrontMatter = `
\newif\ifpreprint \preprinttrue
\newif\ifnopreprintline \nopreprintlinefalse
\newif\ifdoubleblind \doubleblindfalse
\def\elsarticletitlealign{center}
\let\comma\@empty
\let\tnotesep\@empty
\let\@title\@empty
\def\useelstitle{}
\def\title#1{\g@addto@macro\@title{#1%
       \global\let\tnoteref\@gobble}%
       \g@addto@macro\useelstitle{#1}}
\def\elsRef#1{\@ifundefined{X@#1}{0}{\csname X@#1\endcsname}}
\def\gotex@els@label#1#2{\expandafter\xdef\csname X@#1\endcsname{#2}}
\newcount\gotex@els@tn \newcount\gotex@els@fn \newcount\gotex@els@cn \newcount\gotex@els@an
\let\@tnotemark\@empty
\def\tnotemark[#1]{\ifdoubleblind\else\@for\mytmark:=#1\do{%
        \expandafter\ifcase\elsRef{\mytmark}\or$^{\star}$\or
       $^{,\star\star}$\fi
    }\fi}
\def\tnoteref#1{\tnotemark[{#1}]}
\let\@tnotes\@empty
\newcounter{tnote}
\def\tnotetext[#1]#2{\global\advance\gotex@els@tn\@ne
  \gotex@els@label{#1}{\the\gotex@els@tn}%
  \g@addto@macro\@tnotes{%
    \refstepcounter{tnote}%
    \def\thefootnote{\ifcase\c@tnote\or⋆\or⋆⋆\fi}%
    \footnotetext{#2}}}
\let\@nonumnotes\@empty
\def\nonumnote#1{\g@addto@macro\@nonumnotes{%
     \let\thefootnote\relax\footnotetext{#1}}}
\newcounter{fnote}
\def\thefnote{\arabic{fnote}}
\def\fnmark[#1]{\let\comma\@empty
    \def\@fnmark{\@for\@@fnmark:=#1\do{%
    \edef\fnotenum{\@ifundefined{X@\@@fnmark}{1}{\elsRef{\@@fnmark}}}%
  \unskip\comma\fnotenum\let\comma,}}%
}
\def\fnref#1{\fnmark[#1]}
\let\@fnotes\@empty\let\@fnmark\@empty
\def\fntext[#1]#2{\global\advance\gotex@els@fn\@ne
  \gotex@els@label{#1}{\the\gotex@els@fn}%
  \g@addto@macro\@fnotes{%
    \refstepcounter{fnote}%
    \def\thefootnote{\thefnote}%
    \global\setcounter{footnote}{\c@fnote}%
    \footnotetext{#2}}}
\def\cormark[#1]{\edef\cnotenum{\elsRef{#1}}%
    \unskip\textsuperscript{\sep\ifcase\cnotenum\or
       ∗\or∗∗\fi\hspace{-1pt}}\let\sep=,}
\let\@cormark\@empty
\let\@cornotes\@empty
\newcounter{cnote}
\def\cortext[#1]#2{\global\advance\gotex@els@cn\@ne
  \gotex@els@label{#1}{\the\gotex@els@cn}%
  \g@addto@macro\@cornotes{%
    \refstepcounter{cnote}%
    \def\thefootnote{\ifcase\c@cnote\or∗\or
    ∗∗\fi}%
    \footnotetext{#2}}}
\let\@corref\@empty
\def\corref#1{\edef\cnotenum{\elsRef{#1}}%
    \edef\@corref{\ifcase\cnotenum\or
         ∗\or∗∗\fi\hskip-1pt}}
\def\resetTitleCounters{\c@cnote=0
   \c@fnote=0 \c@tnote=0 \c@footnote=0}
\let\eadsep\@empty
\def\@elseads{}
\let\@elsuads\@empty
\def\hashchar{\expandafter\@gobble\string\~}
\def\underscorechar{\expandafter\@gobble\string\_}
\def\lbracechar{\expandafter\@gobble\string\{}
\def\rbracechar{\expandafter\@gobble\string\}}
\def\@eadauthor{}
\long\def\gotex@els@emailcall#1{\expandafter\gotex@els@emailcallb\expandafter{\@eadauthor}{#1}}
\long\def\gotex@els@emailcallb#1#2{\emailauthor{#2}{#1}}
\long\def\gotex@els@urlcall#1{\expandafter\gotex@els@urlcallb\expandafter{\@eadauthor}{#1}}
\long\def\gotex@els@urlcallb#1#2{\urlauthor{#2}{#1}}
\gdef\ead{\@ifnextchar[{\@uad}{\@ead}}
\gdef\@ead#1{\bgroup
   \def\_{\underscorechar}%
   \def\{{\lbracechar}%
   \def~{\hashchar}%
   \def\}{\rbracechar}%
   \edef\gotex@els@mail{#1}%
   \expandafter\gotex@els@emailcall\expandafter{\gotex@els@mail}%
   \egroup
}
\newcounter{ead}
\gdef\emailauthor#1#2{\stepcounter{ead}%
     \g@addto@macro\@elseads{\raggedright%
      \let\corref\@gobble\def\@@tmp{#1}%
      \eadsep{\ttfamily\expandafter\strip@prefix\meaning\@@tmp}
      (#2)\def\eadsep{\unskip,\space}}%
}
\gdef\@uad[#1]#2{\bgroup
   \def~{\hashchar}%
   \def\_{\underscorechar}%
   \def\}{\rbracechar}%
   \edef\gotex@els@mail{#2}%
   \expandafter\gotex@els@urlcall\expandafter{\gotex@els@mail}%
  \egroup
}
\gdef\urlauthor#1#2{\g@addto@macro\@elsuads{\let\corref\@gobble%
     \def\@@tmp{#1}\raggedright\eadsep
     {\ttfamily\expandafter\strip@prefix\meaning\@@tmp}\space(#2)%
     \def\eadsep{\unskip,\space}}%
}
\def\elsauthors{}
\def\useauthors{}
\def\elsprelimauthors{}
\def\pprinttitle{}
\let\authorsep\@empty
\let\prelimauthorsep\@empty
\let\sep\@empty
\newcounter{author}
\def\author{\@ifnextchar[{\@@author}{\@author}}
\def\@@author[#1]#2{%
    \g@addto@macro\elsprelimauthors{%
     \prelimauthorsep#2%
     \def\prelimauthorsep{\unskip,\space}}%
    \g@addto@macro\elsauthors{%
    \def\baselinestretch{1}%
    \authorsep#2\unskip\textsuperscript{%
      \@for\@@affmark:=#1\do{%
       \edef\affnum{\@ifundefined{X@\@@affmark}{1}{\elsRef{\@@affmark}}}%
     \unskip\sep\affnum\let\sep=,}%
      \ifx\@fnmark\@empty\else\unskip\sep\@fnmark\let\sep=,\fi
      \ifx\@corref\@empty\else\unskip\sep\@corref\let\sep=,\fi
      }%
    \def\authorsep{\unskip,\space}%
    \global\let\sep\@empty\global\let\@corref\@empty
    \global\let\@fnmark\@empty}%
    \gdef\@eadauthor{#2}%
    \g@addto@macro\useauthors{#2; }%
}
\def\@author#1{%
    \g@addto@macro\elsprelimauthors{%
     \prelimauthorsep#1%
     \def\prelimauthorsep{\unskip,\space}}%
    \g@addto@macro\elsauthors{\normalsize%
    \def\baselinestretch{1}%
    \upshape\authorsep#1\unskip\textsuperscript{%
      \ifx\@fnmark\@empty\else\unskip\sep\@fnmark\let\sep=,\fi
      \ifx\@corref\@empty\else\unskip\sep\@corref\let\sep=,\fi
      }%
    \def\authorsep{\unskip,\space}%
    \global\let\@fnmark\@empty
    \global\let\@corref\@empty    \global\let\sep\@empty}%
    \gdef\@eadauthor{#1}%
    \g@addto@macro\useauthors{#1; }%
}
\def\elsaddress{}
\def\addsep{\par\vskip6pt}
\newcounter{affn}
\renewcommand\theaffn{\alph{affn}}
\def\address{\@ifnextchar[{\@@address}{\@address}}
\long\def\@@address[#1]#2{%
    \global\advance\gotex@els@an\@ne
    \gotex@els@label{#1}{\@alph\gotex@els@an}%
    \g@addto@macro\elsaddress{%
    \def\baselinestretch{1}%
     \refstepcounter{affn}%
     \xdef\@currentlabel{\theaffn}%
    \textsuperscript{\theaffn}#2\par}}
\long\def\@address#1{\g@addto@macro\elsauthors{%
    \def\baselinestretch{1}%
    \addsep\footnotesize\itshape#1\def\addsep{\par\vskip6pt}%
    \def\authorsep{\par\vskip8pt}}}
\gdef\gotex@els@affvalues{}
\def\gotex@els@affadd#1{\xdef\gotex@els@affvalues{\unexpanded\expandafter{\gotex@els@affvalues}#1}}
\def\gotex@els@affeach#1{%
  \gotex@els@affsplit#1==\@nil}
\def\gotex@els@affsplit#1=#2=#3\@nil{%
  \edef\gotex@els@k{\zap@space#1 \@empty}%
  \def\gotex@els@v{#2}%
  \@ifundefined{gotex@els@isk@\gotex@els@k}{}{%
    \@ifundefined{gotex@els@iscountry@\gotex@els@k}%
      {\expandafter\gotex@els@affadd\expandafter{\gotex@els@v,\space}}%
      {\expandafter\gotex@els@affadd\expandafter{\gotex@els@v\space}}%
  }}
\@for\gotex@els@x:=organization,addressline,city,postcode,state,or,ad,ci,po,st,o,a,c,p,s,cy,country\do{%
  \expandafter\def\csname gotex@els@isk@\gotex@els@x\endcsname{}}
\expandafter\def\csname gotex@els@iscountry@cy\endcsname{}
\expandafter\def\csname gotex@els@iscountry@country\endcsname{}
\long\def\gotex@els@affall#1{\gdef\gotex@els@affvalues{}%
  \@for\gotex@els@item:=#1\do{\expandafter\gotex@els@affeach\expandafter{\gotex@els@item}}}
\def\affiliation{\@ifnextchar[{\@@affiliation}{\@affiliation}}
\long\def\@@affiliation[#1]#2{%
    \global\advance\gotex@els@an\@ne
    \gotex@els@label{#1}{\@alph\gotex@els@an}%
    \gotex@els@affall{#2}%
    \g@addto@macro\elsaddress{%
    \def\baselinestretch{1}%
    \refstepcounter{affn}%
    \xdef\@currentlabel{\theaffn}%
    \textsuperscript{\theaffn}}%
    \expandafter\g@addto@macro\expandafter\elsaddress\expandafter{\gotex@els@affvalues\par}}
\long\def\@affiliation#1{%
    \gotex@els@affall{#1}%
    \g@addto@macro\elsauthors{\def\baselinestretch{1}}%
    \expandafter\g@addto@macro\expandafter\elsaddress\expandafter{\gotex@els@affvalues\par}}
\newbox\absbox
\let\@elsarticleabstitle\@empty
\def\abstracttitle#1{\gdef\@elsarticleabstitle{#1}}
\abstracttitle{Abstract}
\def\abstract{\global\setbox\absbox=\vbox\bgroup
  \hsize=\textwidth\def\baselinestretch{1}%
  \noindent\unskip\textbf{\@elsarticleabstitle}%
 \par\medskip\noindent\unskip\ignorespaces}
\def\endabstract{\egroup}
\newbox\keybox
\let\@elsarticlekwdtitle\@empty
\def\keywordtitle#1{\gdef\@elsarticlekwdtitle{#1}}
\def\keywordtitlesep#1{\gdef\@elsarticlekeywordtitlesep{#1}}
\keywordtitle{Keywords}
\keywordtitlesep{:\ }
\def\keyword{%
  \def\sep{\unskip, }%
  \def\MSC{\@ifnextchar[{\@MSC}{\@MSC[2000]}}%
  \def\@MSC[##1]{\par\leavevmode\hbox {\it ##1~MSC:\space}}%
  \def\PACS{\par\leavevmode\hbox {\it PACS:\space}}%
  \def\JEL{\par\leavevmode\hbox {\it JEL:\space}}%
  \global\setbox\keybox=\vbox\bgroup\hsize=\textwidth
  \normalsize\normalfont\def\baselinestretch{1}%
  \parskip\z@
  \noindent\textit{\@elsarticlekwdtitle\@elsarticlekeywordtitlesep}%
  \raggedright
  \ignorespaces}
\def\endkeyword{\par \egroup}
\def\printFirstPageNotes{%
\ifdoubleblind
\else
  \ifx\@tnotes\@empty\else\@tnotes\fi
  \ifx\@nonumnotes\@empty\else\@nonumnotes\fi
  \ifx\@cornotes\@empty\else\@cornotes\fi
  \ifx\@elseads\@empty\relax\else
   \let\thefootnote\relax
   \footnotetext{\ifnum\theead=1\relax
      \textit{Email address:\space}\else
      \textit{Email addresses:\space}\fi
     \@elseads}\fi
  \ifx\@elsuads\@empty\relax\else
   \let\thefootnote\relax
   \footnotetext{\textit{URL:\space}%
     \@elsuads}\fi
\fi
  \ifx\@fnotes\@empty\else\@fnotes\fi
}
\long\def\pprintMaketitle{\clearpage
  \resetTitleCounters
  \def\baselinestretch{1}%
  \printFirstPageNotes
  \begin{\elsarticletitlealign}%
 \thispagestyle{pprintTitle}%
   \def\baselinestretch{1}%
    \Large\@title\par\vskip18pt%
    \ifdoubleblind
      \vspace*{2pc}
    \else
      \normalsize\elsauthors\par\vskip10pt
      \footnotesize\itshape\elsaddress\par\vskip36pt
    \fi
    \hrule\vskip12pt
    \ifvoid\absbox\else\unvbox\absbox\par\vskip10pt\fi
    \ifvoid\keybox\else\unvbox\keybox\par\vskip10pt\fi
    \hrule\vskip12pt
    \end{\elsarticletitlealign}%
    \gdef\thefootnote{\arabic{footnote}}%
  }
\long\def\MaketitleBox{%
  \resetTitleCounters
  \def\baselinestretch{1}%
  \begin{\elsarticletitlealign}%
   \def\baselinestretch{1}%
    \Large\@title\par\vskip18pt
  \ifdoubleblind
    \vspace*{2pc}
  \else
    \normalsize\elsauthors\par\vskip10pt
    \footnotesize\itshape\elsaddress\par\vskip36pt
  \fi
    \hrule\vskip12pt
    \ifvoid\absbox\else\unvbox\absbox\par\vskip10pt\fi
    \ifvoid\keybox\else\unvbox\keybox\par\vskip10pt\fi
    \hrule\vskip12pt
    \end{\elsarticletitlealign}%
}
\long\def\finalMaketitle{%
  \resetTitleCounters
  \def\baselinestretch{1}%
   \MaketitleBox
   \thispagestyle{pprintTitle}%
  \gdef\thefootnote{\arabic{footnote}}%
  }
\def\maketitle{\ifpreprint\pprintMaketitle\else
  \finalMaketitle\printFirstPageNotes\gdef\thefootnote{\arabic{footnote}}\fi}
\let\@elsarticlemyfooter\@empty
\let\@elsarticlemyfooteralign\@empty
\def\@elsarticlemyfooteralignleft{L}
\def\@elsarticlemyfooteralignright{R}
\def\@elsarticlemyfooteraligncenter{C}
\def\myfooter[#1]#2 {\gdef\@elsarticlemyfooteralign{#1}\gdef\@elsarticlemyfooter{#2}}
\def\myfooterfont#1{\gdef\@myfooterfont{#1}}
\myfooterfont{\footnotesize\itshape}
\def\ps@pprintTitle{%
     \let\@oddhead\@empty
     \let\@evenhead\@empty
     \def\@oddfoot
       {\hbox to \textwidth%
        {\ifnopreprintline\relax\else
        \@myfooterfont%
         \ifx\@elsarticlemyfooteralign\@elsarticlemyfooteraligncenter%
           \hfil\@elsarticlemyfooter\hfil%
         \else%
         \ifx\@elsarticlemyfooteralign\@elsarticlemyfooteralignleft%
           \@elsarticlemyfooter\hfill{}%
         \else%
         \ifx\@elsarticlemyfooteralign\@elsarticlemyfooteralignright%
           {}\hfill\@elsarticlemyfooter%
         \else%
               Preprint submitted to \ifx\@journal\@empty%
                 Elsevier%
            \else\@journal\fi\hfill\@date\fi%
         \fi%
         \fi%
         \fi%
         }
       }%
     \let\@evenfoot\@oddfoot}
\def\journal#1{\gdef\@journal{#1}}
\let\@journal\@empty
`

// loadElsarticleEmulation splices the elsarticle front matter into the input and applies
// the class options that choose between its title layouts: final, 1p, 3p and 5p switch
// the preprint layout off (elsarticle.cls:72, :79-83), nopreprintline and doubleblind are
// flags (:71, :107), lefttitle left-aligns the title block (:100).
//
// It must be called BEFORE article is loaded: a splice runs ahead of the splices already
// pending, so the front matter then executes AFTER article.cls and overrides its \title,
// \author and \maketitle, as elsarticle.cls (which \LoadClass{article} first) does. It is
// spliced as a package, with @ a letter, because LoadFormat would run it in the middle of
// whatever file is being read.
func (e *Engine) loadElsarticleEmulation(opts []string) {
	var b strings.Builder
	b.WriteString(ElsarticleFrontMatter)
	for _, o := range opts {
		switch strings.TrimSpace(o) {
		case "5p":
			// elsarticle.cls:353-364: for jtype 5 \maketitle is
			// \twocolumn[\finalMaketitle]\printFirstPageNotes, so the title block spans
			// both columns at the top of page 1 and the body flows in two columns below it.
			b.WriteString("\\preprintfalse\n" +
				"\\def\\maketitle{\\twocolumn[\\finalMaketitle]\\printFirstPageNotes" +
				"\\gdef\\thefootnote{\\arabic{footnote}}}\n")
		case "final", "1p", "3p":
			b.WriteString("\\preprintfalse\n")
		case "preprint", "review":
			b.WriteString("\\preprinttrue\n")
		case "nopreprintline":
			b.WriteString("\\nopreprintlinetrue\n")
		case "doubleblind":
			b.WriteString("\\doubleblindtrue\n")
		case "lefttitle":
			b.WriteString("\\def\\elsarticletitlealign{flushleft}\n")
		}
	}
	e.loadTeXFile([]byte(b.String()), "gotex-elsarticle", ".sty", nil)
}
