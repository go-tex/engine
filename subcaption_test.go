// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \captionof{figure}{X} used OUTSIDE a float behaves like \caption: it steps
// \c@figure, prints "Figure 1: X" and freezes \@currentlabel, so a following
// \label resolves to the plain figure number "1". \captionof{table}{Y} uses the
// independent table counter and prints "Table 1: Y".
func TestCaptionofOutsideFloat(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(
		`\noindent\captionof{figure}{X}\label{cf}` +
			`\captionof{table}{Y}\label{ct}`); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"Figure1:X", "Table1:Y"} {
		if !strings.Contains(txt, want) {
			t.Errorf("captionof text %q not found in %q", want, txt)
		}
	}
	if got := e.labels["cf"]; got != "1" {
		t.Errorf("captionof figure label = %q, want %q", got, "1")
	}
	if got := e.labels["ct"]; got != "1" {
		t.Errorf("captionof table label = %q, want %q", got, "1")
	}
}

// Two \subcaptionbox panels in a figure number (a), (b); each panel prints its
// lettered sub-caption "(a) …" and a \label inside resolves to parent+letter —
// "1a", "1b" — the figure number the pending \caption will assign plus the
// letter. The trailing main \caption prints "Figure 1: …" and resolves to "1".
func TestSubcaptionboxLetteringAndLabels(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(
		`\begin{figure}` +
			`\subcaptionbox{First}{\rule{20pt}{20pt}\label{sa}}` +
			`\subcaptionbox{Second}{\rule{20pt}{20pt}\label{sb}}` +
			`\caption{Panels}\label{mainfig}` +
			`\end{figure}`); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"(a)First", "(b)Second", "Figure1:Panels"} {
		if !strings.Contains(txt, want) {
			t.Errorf("subcaption text %q not found in %q", want, txt)
		}
	}
	for k, want := range map[string]string{"sa": "1a", "sb": "1b", "mainfig": "1"} {
		if got := e.labels[k]; got != want {
			t.Errorf("label %q = %q, want %q", k, got, want)
		}
	}
}

// \c@subfigure is zeroed when a new figure starts, so the second figure's panels
// restart at (a) and reference the second figure's number ("2a"). This exercises
// the additive re-\def of the figure environment opener.
func TestSubfigureCounterResetsPerFigure(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(
		`\begin{figure}` +
			`\subcaptionbox{A}{\rule{10pt}{10pt}\label{f1a}}` +
			`\caption{One}\end{figure}` +
			`\begin{figure}` +
			`\subcaptionbox{B}{\rule{10pt}{10pt}\label{f2a}}` +
			`\caption{Two}\end{figure}`); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"f1a": "1a", "f2a": "2a"} {
		if got := e.labels[k]; got != want {
			t.Errorf("label %q = %q, want %q", k, got, want)
		}
	}
}

// \subfloat is the subfig-package spelling: the sub-caption is the OPTIONAL
// argument. \subfloat[cap]{content} letters and labels like \subcaptionbox, and
// \subfloat{content} (no optional) still steps the counter with an empty caption.
func TestSubfloatAlias(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(
		`\begin{figure}` +
			`\subfloat[Cap]{\rule{10pt}{10pt}\label{sf1}}` +
			`\subfloat{\rule{10pt}{10pt}\label{sf2}}` +
			`\caption{X}\end{figure}`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mvlText(e.mvl), "(a)Cap") {
		t.Errorf("subfloat optional caption not typeset: %q", mvlText(e.mvl))
	}
	for k, want := range map[string]string{"sf1": "1a", "sf2": "1b"} {
		if got := e.labels[k]; got != want {
			t.Errorf("label %q = %q, want %q", k, got, want)
		}
	}
}

// \captionsetup accepts and ignores its options, in both the bare {options} form
// and the [float]{options} form, gobbling the arguments so nothing leaks into the
// typeset output and no error is raised.
func TestCaptionsetupGobbled(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(
		`\noindent\captionsetup{format=hang,labelfont=bf}` +
			`\captionsetup[figure]{skip=6pt}Z`); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if txt != "Z" {
		t.Errorf("captionsetup leaked into output: %q, want %q", txt, "Z")
	}
}

// Malformed calls (missing arguments hitting end of input) must surface as an
// engine error, never a Go panic.
func TestSubcaptionErrorBranchesNoPanic(t *testing.T) {
	cases := []string{
		`\captionof{figure}`,   // missing caption text
		`\subcaptionbox{only}`, // missing content group
		`\subcaptionbox{a}{b}`, // well-formed but outside a figure
		`\captionsetup`,        // missing option group
		`\subfloat`,            // missing everything
	}
	for _, src := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on %q: %v", src, r)
				}
			}()
			e := New()
			e.LoadLaTeX()
			e.SetFont(spMock{})
			_, _ = e.Run(src) // an error is fine; a panic is not
		}()
	}
}
