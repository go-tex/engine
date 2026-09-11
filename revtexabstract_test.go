// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// In revtex the abstract is written BEFORE \maketitle and emitted BY it, after the
// authors. The generic \abstract sets it where it stands, so every rendered revtex
// paper carried its abstract ABOVE its own title — visible on the deployed
// playground, and on 2203.15077 whose page 1 opened with "Abstract Growing a flat
// lamina…" where the reference opens with "How to grow a flat leaf".
//
// It costs no page, so Σ cannot see it, which is why it survived.
func TestRevtexAbstractFollowsTheTitle(t *testing.T) {
	src := `\documentclass[reprint,aps]{revtex4-2}\begin{document}` +
		`\title{TITREICI}\author{AUTEURICI}\affiliation{AFFILIATIONICI}` +
		`\begin{abstract}RESUMEICI\end{abstract}\maketitle CORPSICI\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	order := []string{"TITREICI", "AUTEURICI", "AFFILIATIONICI", "RESUMEICI", "CORPSICI"}
	at := -1
	for _, want := range order {
		i := strings.Index(got, want)
		if i < 0 {
			t.Fatalf("%q is not on the page: %q", want, got)
		}
		if i < at {
			t.Errorf("%q comes before what should precede it — order is %v in %q", want, order, got)
		}
		at = i
	}
}

// An abstract written AFTER \maketitle (some papers do) is emitted as soon as it
// closes, so nothing is ever captured into a box that no one empties.
func TestRevtexAbstractAfterMaketitleIsStillSet(t *testing.T) {
	src := `\documentclass[reprint,aps]{revtex4-2}\begin{document}` +
		`\title{TITREICI}\author{AUTEURICI}\maketitle` +
		`\begin{abstract}RESUMEICI\end{abstract} CORPSICI\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "RESUMEICI") {
		t.Errorf("the abstract vanished: %q", got)
	}
}

// And revtex prints no "Abstract" heading: checked against the real class through
// tectonic, page one goes straight from the affiliations to the abstract's own
// text. The generic \abstract centres one, which is right for article and wrong
// here.
func TestRevtexPrintsNoAbstractHeading(t *testing.T) {
	src := `\documentclass[reprint,aps]{revtex4-2}\begin{document}` +
		`\title{TITREICI}\author{AUTEURICI}` +
		`\begin{abstract}RESUMEICI\end{abstract}\maketitle CORPSICI\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); strings.Contains(got, "Abstract") {
		t.Errorf("revtex printed an \"Abstract\" heading, which the real class does not: %q", got)
	}
}
