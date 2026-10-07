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
	for _, c := range []struct{ name, src string }{
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
			t.Fatalf("%s: buildEngine: %v", c.name, err)
		}
		if _, err := e.Run(`\hsize=300pt` + c.src + `\message{[fin]}`); err != nil {
			t.Fatalf("%s: Run: %v", c.name, err)
		}
		if d := e.Diagnostics(); d.OpenGroups != 0 {
			t.Errorf("%s left %d group(s) open", c.name, d.OpenGroups)
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

// \list's SECOND argument is where every class states a list's margins, and it
// used to be thrown away: \def\list#1#2{\@trivlist}. article.cls defines
//
//	\newenvironment{quote}{\list{}{\rightmargin\leftmargin}\item\relax}{\endlist}
//
// so a quotation came out with NO indentation at all — not too little, none.
// Measured against tectonic on a 133.77–477.48 measure: the reference sets a
// quote from x=158.67 to 452.58, and we set it from 133.77 to 477.50. Now
// 158.68 to 452.59.
//
// What is asserted is the \leftskip each of the quote's lines opens with, which
// is what produces those numbers: \leftmargin, 2.5em, 25pt at this size.
func TestAListAppliesTheMarginsItsSettingsState(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\begin{document}
\noindent Ordinary text at the full measure, long enough to wrap onto a second line so the margin is plain to see.
\begin{quote}
A quotation long enough that it wraps onto a second line, so both of its margins can be measured against the ordinary text.
\end{quote}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var atMargin, indented int
	for _, p := range e.Pages() {
		for _, n := range p.list {
			b, ok := n.(*boxNode)
			if !ok || len(b.list) == 0 {
				continue
			}
			g, isGlue := b.list[0].(glueNode)
			switch {
			case !isGlue || g.spec.width == 0:
				atMargin++
			case spToPt(g.spec.width) == 25: // \leftmargini, 2.5em at 10pt
				indented++
			}
		}
	}
	if indented < 2 {
		t.Errorf("%d lines open with the 25pt \\leftmargin; the quotation's two lines should", indented)
	}
	if atMargin < 2 {
		t.Errorf("%d lines open at the margin; the ordinary paragraph's two should", atMargin)
	}
}

// \endtrivlist has to end the paragraph INSIDE the list's group. \leftskip is
// read when a paragraph is BROKEN, so ending the group first restores it and the
// list's last paragraph is set at the full measure. \itemize has always had the
// right order (latex.go:264) and that is why it indented while \list could not.
func TestEndTrivlistEndsTheParagraphInsideTheGroup(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}`); err != nil {
		t.Fatal(err)
	}
	m := e.eq["endtrivlist"]
	if m == nil {
		t.Fatal("\\endtrivlist is not defined")
	}
	body := e.toksToString(m.body)
	par, grp := strings.Index(body, `\par`), strings.Index(body, `\endgroup`)
	if par < 0 || grp < 0 {
		t.Fatalf("\\endtrivlist = %q, want both \\par and \\endgroup", body)
	}
	if par > grp {
		t.Errorf("\\endtrivlist closes its group before ending the paragraph: %q", body)
	}
}

// \itemindent moves an item's FIRST line relative to the list's left margin, and
// a class uses it to pull a label back out into the margin: article's
// description is \list{}{\labelwidth\z@ \itemindent-\leftmargin …}, so its term
// starts at the margin while the rest of the item hangs at \leftmargin.
// Measured against tectonic: the term at x=133.77 there, and it had moved to
// 158.68 — the hanging indent — when \list began applying its margins without
// also applying \itemindent.
//
// Exactly ONE line carries the pull-back: the item's first.
func TestAnItemsFirstLineTakesItemindent(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\begin{document}
\begin{description}
\item[Term] A description item long enough that it wraps onto a second line, so the hanging indent under the term can be told from the term's own line.
\end{description}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var pulled, hanging int
	for _, p := range e.Pages() {
		for _, n := range p.list {
			b, ok := n.(*boxNode)
			if !ok || len(b.list) == 0 {
				continue
			}
			g, isGlue := b.list[0].(glueNode)
			if !isGlue || spToPt(g.spec.width) != 25 { // \leftmargini at 10pt
				continue
			}
			hanging++
			if lineBacksUp(b, -25) {
				pulled++
			}
		}
	}
	if hanging < 2 {
		t.Fatalf("%d lines hang at the 25pt \\leftmargin; the item wrapped onto two", hanging)
	}
	if pulled != 1 {
		t.Errorf("%d of the %d lines carry the -25pt \\itemindent, want exactly 1 — the item's first", pulled, hanging)
	}
}

// lineBacksUp reports whether the line carries a glue of that width among the
// few nodes before its text — a negative one is a pull-back out of the margin.
func lineBacksUp(b *boxNode, width float64) bool {
	for i, n := range b.list {
		if i > 4 {
			return false
		}
		if g, ok := n.(glueNode); ok && spToPt(g.spec.width) == width {
			return true
		}
	}
	return false
}
