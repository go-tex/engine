// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \begin{env} … \end{env} is a GROUP. ltmiscen.dtx:
//
//	\protected\def\begin#1{… \begingroup\@endpefalse\reserved@a}
//	  where \reserved@a is \def\@currenvir{#1}… \csname #1\endcsname
//	\def\end#1{\csname end#1\endcsname\@checkend{#1}\expandafter\endgroup …}
//
// so \begingroup comes first and everything the environment defines — \@currenvir
// included — is local to it.

func TestEnvironmentIsAGroup(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\def\x{dehors}\newenvironment{env}{}{}` +
		`\begin{env}\def\x{dedans}\message{[\x]}\end{env}\message{[\x]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[dedans] [dehors]" {
		t.Errorf("= %q, want a definition made inside the environment to end with it", got)
	}
}

// \@currenvir names the environment being run and is restored when it ends. beamer
// picks between \begin{frame}…\end{frame} and the command form \frame{…} with
// \ifx\@currenvir\beamer@frametext, so a stale value sends the command form down the
// environment path.
func TestCurrentEnvIsRestored(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\newenvironment{env}{}{}\newenvironment{autre}{}{}` +
		`\message{[\@currenvir]}` +
		`\begin{env}\message{[\@currenvir]}\begin{autre}\message{[\@currenvir]}\end{autre}` +
		`\message{[\@currenvir]}\end{env}\message{[\@currenvir]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[] [env] [autre] [env] []" {
		t.Errorf("= %q, want \\@currenvir to follow the nesting", got)
	}
}

// A class may leave an environment early by closing its group itself, and then read
// \@currenvir to find it is no longer inside. beamer's fragile frame does exactly
// that: \beamer@checkforfragile ends with \endgroup% end environment, then calls
// \frame — which must take the COMMAND path.
func TestClosingTheGroupLeavesTheEnvironment(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\newenvironment{env}{\endgroup\message{[\@currenvir]}}{}` +
		`\begin{env}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[]" {
		t.Errorf("= %q, want \\endgroup to leave the environment", got)
	}
}

// Allocation is GLOBAL. ltplain.dtx's \e@alloc ends with
// `\global#2#6\allocationnumber`, and ltcounts.dtx's \@definecounter makes \cl@<c>,
// \p@<c> and \the<c> global too — so a counter declared inside an environment (or
// inside \begin{document}, which is one) is still there afterwards. \setcounter is
// global as well; the register's VALUE follows TeX's ordinary scoping, so the
// assignment here is explicitly \global.
func TestAllocationSurvivesTheEnvironment(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\newenvironment{env}{}{}` +
		`\begin{env}\newcounter{compte}\newcount\reg \global\reg=7 \setcounter{compte}{4}\end{env}` +
		`\message{[\arabic{compte}][\the\reg]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[4][7]" {
		t.Errorf("= %q, want the counter and the register to outlive the environment", got)
	}
}

// Every alignment entry is a group (tex.web §791: a template's u-part and v-part are
// inserted inside braces), so a font switch in one cell stops at that cell.
func TestAlignmentCellIsAGroup(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\hsize=300pt\begin{tabular}{ll}` +
		`\bfseries A & B \\` +
		`\end{tabular}`); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The table must have been built: a cell group that leaked would have been
	// reported as a stray brace by the box builder.
	if d := e.Diagnostics(); d.OpenGroups != 0 {
		t.Errorf("a tabular left %d group(s) open", d.OpenGroups)
	}
}

// An environment the engine implements in Go swallows its own \end, so \end — and
// with it the \endgroup — never runs. Each such environment closes the group itself;
// this checks the whole family at once.
func TestGoSideEnvironmentsCloseTheirGroup(t *testing.T) {
	for _, c := range []struct{ nom, src string }{
		{"tabular", `\begin{tabular}{ll}A & B\\\end{tabular}`},
		{"tabularx", `\begin{tabularx}{200pt}{lX}A & B\\\end{tabularx}`},
		{"verbatim", "\\begin{verbatim}\nbrut\n\\end{verbatim}"},
		{"equation", `\begin{equation}x\end{equation}`},
		{"align", `\begin{align}x&=y\end{align}`},
		{"minipage", `\begin{minipage}{100pt}texte\end{minipage}`},
		{"comment", `\excludecomment{comment}\begin{comment}rien\end{comment}`},
	} {
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("%s: buildEngine: %v", c.nom, err)
		}
		if _, err := e.Run(`\hsize=300pt` + c.src + `\message{[fin]}`); err != nil {
			t.Fatalf("%s: Run: %v", c.nom, err)
		}
		if d := e.Diagnostics(); d.OpenGroups != 0 {
			t.Errorf("%s left %d group(s) open", c.nom, d.OpenGroups)
		}
	}
}

// A group left open by an environment shows up as text later: the check above reads
// the count, this one reads the consequence.
func TestAnEnvironmentDoesNotLeakItsScope(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\newenvironment{env}{}{}\count0=1 ` +
		`\begin{env}\count0=2 \end{env}\message{[\the\count0]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); !strings.Contains(got, "[1]") {
		t.Errorf("= %q, want the register assignment to end with the environment", got)
	}
}

// TeX's alignment scanner EXPANDS as it looks for & and \cr, so a class may split a
// table from inside it. NeurIPS's style does:
//
//	\def\And{\end{tabular}\hfil\linebreak[0]\hfil\begin{tabular}[t]{c}…}
//
// used inside \begin{tabular}[t]{c}…\@author\end{tabular}, so \author{A \And B} carries
// an \end{tabular} two levels down — inside \@author, inside \And. Read raw, it never
// reached the body scanner and only surfaced while the CELL was being typeset, in the
// middle of a box: measured, a paper of 55 232 glyphs rendered 221.
func TestTabularSplitByAMacroCarryingItsEnd(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\hsize=300pt` +
		`\def\And{\end{tabular}\begin{tabular}{c}}` +
		`\def\auteurs{A \And B}` +
		`\begin{tabular}{c}\auteurs\end{tabular}` +
		`\message{[suite]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "[suite]") {
		t.Errorf("= %q, want the document to carry on past the split table", trimNL(out))
	}
	if d := e.Diagnostics(); d.OpenGroups != 0 {
		t.Errorf("a table split by \\And left %d group(s) open", d.OpenGroups)
	}
}

// \endlist ends the list's trivlist, as ltlists.dtx does:
//
//	\def\endlist{\global\advance\@listdepth\m@ne \endtrivlist}
//
// That chain is what a class hooks. beamer patches \endtrivlist to run
// \beamer@closeitem, which closes the overlay wrappers its LAST \item left open —
// every earlier item is closed by the next \item. With \endlist a bare \par those
// three environments stayed open past \end{itemize}, every \end after them closed one
// group too high, and the stack grew by two per slide: four lines of beamer left six
// groups open where TeX leaves none.
func TestListAndEndlistAreAGroupPair(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\count0=1 \list{}{}\count0=2 \message{[\the\count0]}\endlist\message{[\the\count0]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[2] [1]" {
		t.Errorf("= %q, want \\list … \\endlist to scope what it contains", got)
	}
	if d := e.Diagnostics(); d.OpenGroups != 0 {
		t.Errorf("\\list … \\endlist left %d group(s) open", d.OpenGroups)
	}
}
