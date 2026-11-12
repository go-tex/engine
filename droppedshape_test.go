// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// hangIndentDrops counts what the census says about a dropped hanging indent.
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

// ⛔ A FEATURE SILENTLY DROPPED IS WORSE THAN ONE REFUSED.
//
// \hangindent is declared, listed among the dimension parameters a document may
// set, assignable, and readable back — and nothing consults it. Measured against
// tectonic (#757): a paragraph asking for \hangindent=4em comes out flush left
// AND re-broken, because four ems more text per line moves where the lines end.
//
// It took an audit to find that. The engine already had a channel for saying so.
func TestASetHangIndentIsReported(t *testing.T) {
	n, _ := hangIndentDrops(t, `\hangindent=4em Some text that asks for a hanging indent.`)
	if n == 0 {
		t.Error(`a document set \hangindent=4em and the census says nothing: ` +
			`the engine drops it in silence, which is the defect`)
	}
}

// ⛔ ZERO IS NOT A DROP. \hangindent=0pt asks for no hanging indent, which is
// exactly what this engine does — honouring it by doing nothing is correct, and
// reporting it would be noise on every document, since LaTeX's own \@endparenv
// and friends zero it routinely.
func TestAZeroHangIndentIsNotReported(t *testing.T) {
	n, _ := hangIndentDrops(t, `\hangindent=0pt Some ordinary text.`)
	if n != 0 {
		t.Errorf(`\hangindent=0pt was reported %d time(s) as a drop; asking for no `+
			`indent is a request this engine HONOURS`, n)
	}
}

// The control, without which the two above could both pass on an engine that
// reports a drop for everything or for nothing in particular.
func TestADocumentThatSetsNoHangIndentReportsNone(t *testing.T) {
	n, _ := hangIndentDrops(t, `Some ordinary text with no paragraph shape at all.`)
	if n != 0 {
		t.Errorf(`a document that never mentions \hangindent was reported %d time(s)`, n)
	}
}

// ⛔⛔ THE REGISTER CACHE MUST SURVIVE A REBINDING, AND THIS IS THE TEST THAT
// SAYS SO.
//
// The natural place to cache \hangindent's register is the end of
// loadTeXParams, beside \endlinechar and \escapechar. It would be WRONG:
// classkernel.go runs afterwards and declares \newdimen\hangindent, rebinding
// the name to a different register. A value cached earlier names a register
// nothing writes to, so the census reports nothing for ever while looking
// perfectly alive — the same load-order trap that took \prevdepth (#438) and
// \hangafter (#759), silently, both times.
//
// This asserts the cache tracks the binding that WON: the register the name
// resolves to after everything has loaded is the one the engine watches.
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
		t.Errorf("the engine watches dimen register %d but \\hangindent resolves to %d: "+
			"the cache was taken before the binding that won, so every assignment a "+
			"document makes is invisible to the census", e.hangIndentReg, m.code)
	}
}

// …and a document may rebind it itself, which is the same trap one level up.
func TestADocumentsOwnRebindingIsFollowed(t *testing.T) {
	n, e := hangIndentDrops(t, `\newdimen\hangindent \hangindent=4em text`)
	if m := e.eq["hangindent"]; m != nil && e.hangIndentReg != m.code {
		t.Errorf("after the document's own \\newdimen the engine still watches %d, "+
			"but \\hangindent is register %d", e.hangIndentReg, m.code)
	}
	if n == 0 {
		t.Error("a document that re-declared and then set \\hangindent was not reported")
	}
}

// The register index must never default to 0: register 0 is a real register
// somebody else holds, and a zero default would report every assignment to it
// as a dropped hanging indent.
func TestTheUnboundRegisterIsNotZero(t *testing.T) {
	e := New()
	if e.hangIndentReg == 0 {
		t.Error("hangIndentReg defaults to 0, which is a REAL register: every " +
			"assignment to dimen 0 would be reported as a dropped hanging indent")
	}
}
