// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// TestAcmartMetadataAndAuthors: \documentclass{acmart} (no bundled .cls) loads the
// metadata stubs and author-block emulation. The bibliographic metadata commands
// and the CCSXML block are gobbled (no spurious text); the title, the accumulated
// authors, and each \affiliation's \institution/\city/\country survive.
func TestAcmartMetadataAndAuthors(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass[sigconf]{acmart}
\setcopyright{acmlicensed}\acmDOI{10.1145/ZZDOI}\citestyle{acmauthoryear}
\settopmatter{printacmref=false}\received{ZZRECV}\received[revised]{ZZREV}
\acmConference[ZZC]{ZZConf}{ZZDate}{ZZLoc}
\begin{CCSXML}
<ccs2012> ZZXMLGARBAGE </ccs2012>
\end{CCSXML}
\ccsdesc[500]{ZZCCS}\keywords{ZZKW}
\begin{document}
\title{RealTitle}
\author{AuthorOne}
\affiliation{\institution{InstOne}\city{CityOne}\country{CtryOne}}
\email{one@x.edu}
\author{AuthorTwo}
\affiliation{\institution{InstTwo}}
\maketitle
RealBody.
\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("acmart run: %v", err)
	}
	txt := treeText(e)
	for _, want := range []string{
		"RealTitle", "AuthorOne", "InstOne", "CityOne", "CtryOne",
		"one@x.edu", "AuthorTwo", "InstTwo", "RealBody",
	} {
		if !strings.Contains(txt, want) {
			t.Errorf("acmart render missing %q\ngot: %q", want, txt)
		}
	}
	for _, notwant := range []string{"ZZDOI", "ZZRECV", "ZZREV", "ZZXMLGARBAGE", "ZZCCS", "ZZKW", "acmlicensed", "ZZConf"} {
		if strings.Contains(txt, notwant) {
			t.Errorf("acmart render leaked gobbled metadata %q", notwant)
		}
	}
}
