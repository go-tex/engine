// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// TestIsRevtexClass covers the revtex class predicate.
func TestIsRevtexClass(t *testing.T) {
	for _, n := range []string{"revtex4", "revtex4-1", "revtex4-2"} {
		if !isRevtexClass(n) {
			t.Errorf("isRevtexClass(%q) = false, want true", n)
		}
	}
	for _, n := range []string{"article", "amsart", "revtex", "revtex5", ""} {
		if isRevtexClass(n) {
			t.Errorf("isRevtexClass(%q) = true, want false", n)
		}
	}
}

// TestRevtexAuthorBlock: \documentclass{revtex4-2} (with no bundled .cls) loads the
// title-block emulation, so the author, affiliations and contact lines — which the
// article-shaped fallback drops on the undefined \affiliation — are typeset by
// \maketitle instead. The pure-metadata commands (\preprint/\pacs/\keywords) are
// gobbled, leaving no spurious text.
func TestRevtexAuthorBlock(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\documentclass[aps,prl,twocolumn]{revtex4-2}
\hsize=400pt
\begin{document}
\title{TheTitle}
\author{AuthorName}
\affiliation{AffilPlace}
\author{SecondAuthor}
\altaffiliation{AltAffil}
\email{me@x.edu}
\homepage[URL: ]{example.org}
\collaboration{Collab}
\noaffiliation
\preprint{PPNUM}\pacs{PACSNUM}\keywords{KWORD}
\maketitle
BodyText here.
\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("revtex run: %v", err)
	}
	txt := treeText(e)
	for _, want := range []string{
		"TheTitle", "AuthorName", "AffilPlace", "SecondAuthor",
		"AltAffil", "me@x.edu", "URL:", "example.org", "Collab", "BodyText",
	} {
		if !strings.Contains(txt, want) {
			t.Errorf("revtex render missing %q\ngot: %q", want, txt)
		}
	}
	for _, notwant := range []string{"PPNUM", "PACSNUM", "KWORD"} {
		if strings.Contains(txt, notwant) {
			t.Errorf("revtex render leaked gobbled metadata %q", notwant)
		}
	}
}
