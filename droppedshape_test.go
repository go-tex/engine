// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// hangIndentDrops counts anything the census still says about \hangindent.
func hangIndentDrops(t *testing.T, body string) (int, *Engine) {
	t.Helper()
	src := `\documentclass{article}\begin{document}` + body + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for k, v := range e.SkippedCommands() {
		if strings.Contains(k, `\hangindent`) {
			n += v
		}
	}
	return n, e
}

// ⭐ THE DROP IS GONE, AND SO IS ITS REPORT.
//
// This file used to assert the opposite: \hangindent was declared, settable,
// readable back and consulted by nothing, so the engine reported it as a
// silently dropped feature — "a feature silently dropped is worse than one
// refused", which was the right thing to do while it was true.
//
// It is no longer true. tex.web §847-850 takes one measure per line, and the
// line breaker can now express that (go-typeset/linebreak v1.10.0's Measure), so
// paragraphShape builds a HangIndent and the paragraph is broken on it. A census
// entry saying the engine drops the parameter would now be a false report, which
// is worse than either the drop or the silence — see hangindent_test.go for the
// witness that the words actually move.
func TestASetHangIndentIsNoLongerReportedAsDropped(t *testing.T) {
	n, _ := hangIndentDrops(t, `\hangindent=4em Some text that asks for a hanging indent.`)
	if n != 0 {
		t.Errorf(`\hangindent=4em was reported %d time(s) as dropped, but the engine now `+
			`honours it: the census is lying about its own behaviour`, n)
	}
}

// The control that keeps the test above from passing on an engine whose census
// has simply stopped working: a command the engine really does skip must still
// be reported.
func TestTheCensusStillReportsSomethingItReallyDrops(t *testing.T) {
	src := `\documentclass{article}\begin{document}\thiscommanddoesnotexist text\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.SkippedCommands()) == 0 {
		t.Error("an undefined command produced no entry in the skip census: the reporting " +
			"channel is dead, so the zero above says nothing about \\hangindent")
	}
}

// ⛔⛔ THE REGISTER CACHE MUST SURVIVE A REBINDING, AND IT MATTERS MORE NOW THAN
// IT DID.
//
// The natural place to cache \hangindent's register is the end of
// loadTeXParams, beside \endlinechar and \escapechar. It would be WRONG:
// classkernel.go runs afterwards and declares \newdimen\hangindent, rebinding
// the name to a different register. A value cached earlier names a register
// nothing writes to — the same load-order trap that took \prevdepth (#438) and
// \hangafter (#759), silently, both times.
//
// ⭐ When this cache only fed a census, a stale value meant a report that never
// fired. Now it feeds paragraphShape, so a stale value means EVERY \hangindent
// IS SILENTLY IGNORED AGAIN — the defect comes back, wearing the same clothes,
// and the only thing standing in front of it is this test.
func TestTheWatchedRegisterIsTheBindingThatWon(t *testing.T) {
	_, e := hangIndentDrops(t, `\hangindent=4em text`)
	m := e.eq["hangindent"]
	if m == nil {
		t.Fatal(`\hangindent is not defined at all`)
	}
	if m.kind != mDimenRef {
		t.Fatalf(`\hangindent is kind %d, not a dimen register`, int(m.kind))
	}
	if e.hangIndentReg != m.code {
		t.Errorf("the engine reads dimen register %d but \\hangindent resolves to %d: "+
			"the cache was taken before the binding that won, so every hanging indent a "+
			"document asks for is invisible to the line breaker", e.hangIndentReg, m.code)
	}
}

// …and a document may rebind it itself, which is the same trap one level up.
func TestADocumentsOwnRebindingIsFollowed(t *testing.T) {
	_, e := hangIndentDrops(t, `\newdimen\hangindent \hangindent=4em text`)
	if m := e.eq["hangindent"]; m != nil && e.hangIndentReg != m.code {
		t.Errorf("after the document's own \\newdimen the engine still reads %d, "+
			"but \\hangindent is register %d", e.hangIndentReg, m.code)
	}
}

// The register index must never default to 0: register 0 is a real register
// somebody else holds, and a zero default would make every assignment to it
// shape a paragraph that asked for nothing.
func TestTheUnboundRegisterIsNotZero(t *testing.T) {
	e := New()
	if e.hangIndentReg == 0 {
		t.Error("hangIndentReg defaults to 0, which is a REAL register: every " +
			"assignment to dimen 0 would hang-indent a paragraph")
	}
}
