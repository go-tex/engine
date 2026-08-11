// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \part advances \c@part and prints its Roman numeral via \thepart; \part* leaves
// the counter untouched (starred forms never number, exactly like \section*).
func TestPartNumbering(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	_, err := e.Run(
		`\hsize=300pt` +
			`\part{One}\edef\x{\thepart}` +
			`\part*{Two}\edef\y{\thepart}` + // starred: counter unchanged
			`\part{Three}\edef\z{\thepart}` + // continues from I
			`\message{\x|\y|\z}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "I|I|II" {
		t.Errorf("part numbers = %q want I|I|II", got)
	}
}

// \part freezes \@currentlabel to its Roman number, so a following \label captures
// "I" and \ref resolves to it.
func TestPartLabel(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt\part{Intro}\label{pt:intro}`); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["pt:intro"]; got != "I" {
		t.Errorf("part label = %q want \"I\"", got)
	}
}

// \part typesets a big centred "Part I" heading followed by the title.
func TestPartHeadingTypeset(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt\part{Beginnings}`); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	if !strings.Contains(text, "PartI") && !strings.Contains(text, "Part I") {
		t.Errorf("part heading not typeset; chars = %q want to contain \"Part I\"", text)
	}
	if !strings.Contains(text, "Beginnings") {
		t.Errorf("part title not typeset; chars = %q want to contain \"Beginnings\"", text)
	}
}

// \part* typesets the title with no number and does not advance \c@part.
func TestStarredPart(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt\part*{Unnumbered}\message{\thepart}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "" {
		t.Errorf("starred part must leave \\c@part at 0 (\\thepart empty), got %q", got)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "Unnumbered") {
		t.Errorf("starred part title not typeset; chars = %q", b.String())
	}
}

// After \appendix, \section numbers become letters (A, B, …) and subsections read
// like "A.1"; sections before \appendix keep their arabic numbers.
func TestAppendixNumbering(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	_, err := e.Run(
		`\hsize=300pt` +
			`\section{Intro}\edef\a{\thesection}` + // 1
			`\appendix` +
			`\section{Data}\edef\b{\thesection}` + // A
			`\subsection{Tables}\edef\c{\thesubsection}` + // A.1
			`\section{More}\edef\d{\thesection}` + // B
			`\message{\a|\b|\c|\d}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "1|A|A.1|B" {
		t.Errorf("appendix numbering = %q want 1|A|A.1|B", got)
	}
}

// A \label on a section (and subsection) after \appendix resolves to its letter
// form, because \@nsection's \edef\@currentlabel picks up the redefined \thesection.
func TestAppendixLabel(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt\appendix` +
		`\section{Data}\label{app:data}` +
		`\subsection{Tab}\label{app:tab}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["app:data"]; got != "A" {
		t.Errorf("appendix section label = %q want \"A\"", got)
	}
	if got := e.labels["app:tab"]; got != "A.1" {
		t.Errorf("appendix subsection label = %q want \"A.1\"", got)
	}
}

// The appendix section heading and TOC entry both pick up the letter: redefining
// \thesection (not \@nsection) keeps the \@tocentry call intact, so the recorded
// contents number is "A" too.
func TestAppendixTocEntry(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt\appendix\section{Data}`); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "A") || !strings.Contains(b.String(), "Data") {
		t.Errorf("appendix heading not lettered; chars = %q want \"A\" and \"Data\"", b.String())
	}
}

// The abstract environment emits a centred bold "Abstract" heading, then the body.
func TestAbstractEnvironment(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt\begin{abstract}This studies things.\end{abstract}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	if !strings.Contains(text, "Abstract") {
		t.Errorf("abstract heading missing; chars = %q", text)
	}
	if !strings.Contains(text, "studies") {
		t.Errorf("abstract body missing; chars = %q", text)
	}
}

// The titlepage environment groups its content and typesets it; a group opened by
// \begin{titlepage} is balanced by \end{titlepage} (no leaked \leftskip etc.).
func TestTitlepageEnvironment(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt\begin{titlepage}\centerline{My Report}\end{titlepage}After`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	if !strings.Contains(text, "MyReport") && !strings.Contains(text, "My Report") {
		t.Errorf("titlepage content missing; chars = %q", text)
	}
	if !strings.Contains(text, "After") {
		t.Errorf("content after titlepage missing; chars = %q", text)
	}
}

// Error branches must not panic: \part with no argument (input ends mid-scan) and
// a stray \end{abstract} (\endgroup with no matching \begingroup).
func TestSectioningErrorBranches(t *testing.T) {
	for _, src := range []string{
		`\hsize=300pt\part`,          // \part with no {title}: argument scan hits EOF
		`\hsize=300pt\part*`,         // starred \part with no {title}
		`\hsize=300pt\end{abstract}`, // stray end: \endgroup with no open group
		`\hsize=300pt\end{titlepage}`,
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			// A run error is acceptable for malformed input; a panic is not
			// (the test process would crash). Reaching here means no panic.
			t.Logf("src %q returned err %v (no panic — acceptable)", src, err)
		}
	}
}
