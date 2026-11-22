package engine

import (
	"strings"
	"testing"
)

// \halign runs its entries the way tex.web does (§785-791): the row is not
// collected and expanded before it executes; each entry is read live, so an
// assignment in it is in force for what follows it. go-tex/engine#602.
//
// Every expectation below is TECTONIC's output for the same source, kept in
// go-tex-engine-capacity/unblock/halign/ref (w1..w5).

// w1: tectonic sets NEW; we set OLD, because the cs after \def was expanded to
// its old body while the row was collected.
func TestAnAssignmentInAnEntryActsBeforeTheMacrosAfterIt(t *testing.T) {
	got := runHalign(t, `\def\a{OLD}\noindent\halign{#\cr \def\a{NEW}\a\cr}`)
	if !strings.Contains(got, "NEW") || strings.Contains(got, "OLD") {
		t.Errorf("want NEW, got %q", got)
	}
}

// w2: a macro redefined by the entry and read from the v part. Tectonic: xNEW.
func TestAnEntryRedefinesAMacroTheTemplateUses(t *testing.T) {
	got := runHalign(t, `\def\a{OLD}\noindent\halign{#\a\cr \gdef\a{NEW}x\cr}`)
	if !strings.Contains(got, "xNEW") {
		t.Errorf("want xNEW, got %q", got)
	}
}

// w4: IEEEtran's \IEEEauthorblockN shape, a conditional that is a \let target and
// a row that ends from inside a macro. Tectonic: ALICE and BOB on the page.
func TestAnAuthorBlockShapeKeepsItsNames(t *testing.T) {
	src := `\makeatletter
\def\X{2.7ex}
\newif\ifprev \prevfalse
\def\blockN#1{\relax\gdef\X{0pt}%
\ifprev\expandafter\topline\fi\global\prevtrue\gdef\X{2.6ex}#1\crcr}
\def\topline{\cr\noalign{\vskip 0pt}}
\noindent\halign{&\hfil#\hfil\cr
\blockN{ALICE}\blockN{BOB}}`
	got := runHalign(t, src)
	for _, want := range []string{"ALICE", "BOB"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// w3: the same with IEEEtran's strut template, whose v part reads \X when the
// entry has already set it. Tectonic: ALICE and BOB.
func TestAnAuthorBlockWithAStrutTemplateKeepsItsNames(t *testing.T) {
	src := `\makeatletter
\newdimen\tmpd
\def\X{2.7ex}
\def\strutrule{\tmpd\X\relax\rule[-0.3\tmpd]{0pt}{\tmpd}}
\newif\ifprev \prevfalse
\def\blockN#1{\relax\normalfont\normalsize\gdef\X{0pt}%
\ifprev\expandafter\topline\fi\global\prevtrue\gdef\X{2.6ex}#1\crcr}
\def\topline{\cr\noalign{\vskip 0pt}}
\noindent\halign{&\relax\hfil\relax #\relax\hfil\strutrule\cr
\blockN{ALICE}\blockN{BOB}}`
	got := runHalign(t, src)
	for _, want := range []string{"ALICE", "BOB"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// w5: the plain-rows control, which already worked and must keep working.
func TestStrutTemplateWithPlainRowsStillWorks(t *testing.T) {
	src := `\makeatletter
\newdimen\tmpd
\def\X{2.7ex}
\def\strutrule{\tmpd\X\relax\rule[-0.3\tmpd]{0pt}{\tmpd}}
\noindent\halign{&\relax\hfil\relax #\relax\hfil\strutrule\cr
ALICE\cr BOB\cr}`
	got := runHalign(t, src)
	for _, want := range []string{"ALICE", "BOB"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// The brace trick (IEEEtran.cls:3377-3383): the backquoted } is READ, so
// align_state reaches 0 although no group closed, and the & that follows ends the
// entry with the { still open. Counting braces where groups are executed instead
// of where tokens are read loses the second entry (and the row).
func TestTheBraceTrickEndsAnEntryInsideAGroup(t *testing.T) {
	src := "\\noindent\\halign{&[#]\\cr {\\ifnum0=`}\\fi ALICE&{\\ifnum0=`}\\fi BOB\\ifnum0=`{\\fi}\\cr}"
	got := runHalign(t, src)
	if !strings.Contains(got, "[ALICE][BOB]") {
		t.Errorf("want [ALICE][BOB], got %q", got)
	}
}

// A brace pair inside an entry is balanced at the reader, so an & inside it is
// the entry's own text and the entry goes on: tex.web ends it only at state 0.
func TestAnAmpersandInsideBracesDoesNotEndTheEntry(t *testing.T) {
	got := runHalign(t, `\noindent\halign{[#]\cr \hbox{a&b}c\cr}`)
	if strings.Count(got, "[") != 1 || !strings.Contains(got, "c]") {
		t.Errorf("the & inside braces ended the entry: %q", got)
	}
}

// \halign\bgroup … \egroup (IEEEtran.cls:4701): the implicit brace opens the
// alignment, and its rows are built rather than typeset as source with a stray #.
func TestAnAlignmentOpenedWithBgroupBuildsItsRows(t *testing.T) {
	got := runHalign(t, `\noindent\halign\bgroup[#]\cr a\cr b\cr\egroup`)
	if !strings.Contains(got, "[a]") || !strings.Contains(got, "[b]") || strings.Contains(got, "#") {
		t.Errorf("want [a] and [b] and no #, got %q", got)
	}
}

// Input that ends after the last \cr, with no closing brace, ends the alignment
// there: no phantom row is built from the end of the input.
func TestAnAlignmentWhoseInputEndsAfterItsLastRowAddsNoRow(t *testing.T) {
	got := runHalign(t, `\noindent\halign{[#]\cr a\cr`)
	if strings.Count(got, "[") != 1 {
		t.Errorf("want one row, got %q", got)
	}
}

// A control sequence is not a brace, whatever its category field holds: unread
// moves align_state only for a real { or }.
func TestUnreadCountsOnlyRealBraces(t *testing.T) {
	c := &alignCtx{state: 5}
	// Each alone: a begin and an end would cancel if tried together.
	c.unread(tok{cs_: true, cs: "x", cat: catBegin})
	if c.state != 5 {
		t.Errorf("a control sequence with a begin category moved align_state to %d", c.state)
	}
	c.unread(tok{cs_: true, cs: "y", cat: catEnd})
	if c.state != 5 {
		t.Errorf("a control sequence with an end category moved align_state to %d", c.state)
	}
	c.unread(chTok('{', catBegin))
	if c.state != 4 {
		t.Errorf("unreading { left align_state at %d, want 4", c.state)
	}
	c.unread(chTok('}', catEnd))
	if c.state != 5 {
		t.Errorf("unreading } left align_state at %d, want 5", c.state)
	}
}
