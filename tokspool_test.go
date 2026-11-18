// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"strings"
	"testing"
)

// ⭐ THE TOKEN-REGISTER POOL IS eTeX's TOO, AND ITS EXHAUSTION IS REPORTED.
//
// #844 raised \count, \dimen, \skip and \box to registerPoolSize and grew them
// on demand. `toks` was a separate [][]tok allocated flat at 256, so the 247th
// \newtoks was refused — with a bare `return`: no report, exit 0, and \the on
// the never-defined name TYPESET the control sequence instead of its contents.
//
// Measured against tectonic at 7dec596, 250 \newtoks then write and read back
// the 250th:
//
//	             reference   before
//	\toks 250    ALFA250     \zzt250
//	\toks 1      BRAVO1      BRAVO1
//	\count 250   4242        4242     <- the control: #844 raised that pool
//
// go-tex/engine#849.
//
// ⛔⛔ EACH ROW BELOW IS MUTATED AGAINST ITS OWN GUARD, not against one. A break
// check on one row is not a break check on the table: the first version of this
// file was called break-checked after reverting the `\newtoks` bound and running
// TWO of the four tests, which says nothing about the two whose guards that
// mutation does not touch. Verified one at a time:
//
//	test                                    guard reverted              result
//	NewtoksReachesPastTeX82sCeiling         \newtoks's bound            RED
//	AnExhaustedTokenPoolSaysSo              noteRegisterExhausted call  RED
//	AHighTokenRegisterStoresAndReadsBack    setToks's growPool          RED
//	AnUnwrittenTokenRegisterIsNotOutOfRange noteBadRegisterIndex call   RED
//
// ⭐ And the two middle rows are why that matters: reverting the `\newtoks` bound
// leaves AHighTokenRegister GREEN (its guard is setToks) and reverting setToks
// leaves AnUnwritten GREEN (its guard is the note call). Either mutation alone
// would have "confirmed" a table in which half the rows were untested. The
// neighbouring session found exactly this in its own gate, in a row that read a
// value both branches share.
func TestNewtoksReachesPastTeX82sCeiling(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// ⛔ ARABIC NAMES. A witness for this first used \romannumeral and wrote to
	// "zztcl" meaning the 250th — 250 is "ccl" and "cl" is 150, so it exercised a
	// register well inside the old bound and came back green from both engines.
	// An index built by hand instead of read proves nothing.
	var b strings.Builder
	b.WriteString(`\makeatletter\newcount\n \n=0 `)
	b.WriteString(`\loop\advance\n by1\relax\expandafter\newtoks\csname zzt\the\n\endcsname\ifnum\n<250 \repeat`)
	b.WriteString(`\csname zzt250\endcsname={ALFA250}`)
	b.WriteString(`\csname zzt1\endcsname={BRAVO1}`)
	b.WriteString(`\message{[\the\csname zzt250\endcsname][\the\csname zzt1\endcsname]}\makeatother`)
	if _, err := e.Run(b.String()); err != nil {
		t.Fatal(err)
	}
	msg := e.Diagnostics().Messages
	if !strings.Contains(msg, "[ALFA250]") {
		t.Errorf("the 250th token register did not store: %q\n"+
			"A register whose \\newtoks was refused is never defined, so \\the "+
			"typesets the control sequence's own NAME where its content belongs.", msg)
	}
	// The control: register 1 is inside TeX82's range and must be unaffected, or
	// the witness is measuring the loop rather than the bound.
	if !strings.Contains(msg, "[BRAVO1]") {
		t.Errorf("the FIRST token register did not store either: %q — "+
			"the loop itself is broken and this test proves nothing about the pool", msg)
	}
}

// A high register reached directly, without the allocator: \toks<n> grows the
// pool through growPool exactly as \count<n> does.
func TestAHighTokenRegisterStoresAndReadsBack(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\toks4000={CHARLIE}\message{[\the\toks4000]}`); err != nil {
		t.Fatal(err)
	}
	if msg := e.Diagnostics().Messages; !strings.Contains(msg, "[CHARLIE]") {
		t.Errorf("\\toks4000 did not read back: %q", msg)
	}
}

// ⛔⛔ AND THE EXHAUSTION IS REPORTED, which is the half this issue is really
// about. Diagnostics.RegistersExhausted has DOCUMENTED a `\toks` key since #830
// and nothing ever wrote it, so a consumer was told the refusal was reported.
// Documenting a key the engine cannot emit is worse than documenting none: it
// answers the question instead of leaving it open.
//
// The allocator is moved to the ceiling rather than called 32758 times: what is
// under test is the refusal path, and a loop long enough to reach it honestly
// would cost seconds per run to assert the same two facts.
func TestAnExhaustedTokenPoolSaysSo(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.allocToks = registerPoolSize
	if _, err := e.Run(`\newtoks\zzover`); err != nil {
		t.Fatal(err)
	}
	if m := e.eq["zzover"]; m != nil && m.kind == mToksRef {
		t.Errorf("\\zzover became a register past the ceiling: %+v", m)
	}
	if got := e.Diagnostics().RegistersExhausted[`\toks`]; got == 0 {
		t.Errorf("the refused \\newtoks was not reported; RegistersExhausted = %v.\n"+
			"The `\\toks` key is documented on that field (engine.go) — a key the "+
			"engine cannot emit tells a consumer the exhaustion IS reported.",
			e.Diagnostics().RegistersExhausted)
	}
}

// A legal-but-unwritten high register is not an error: it reads empty and is
// NOT tallied as out of range. Only an index no pool can hold is.
func TestAnUnwrittenTokenRegisterIsNotOutOfRange(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\message{[\the\toks5000]}`); err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().RegistersOutOfRange; len(got) != 0 {
		t.Errorf("RegistersOutOfRange = %v for \\toks5000, which is inside the pool "+
			"and merely unwritten", got)
	}
	// And one that truly cannot exist IS tallied.
	if _, err := e.Run(fmt.Sprintf(`\message{[\the\toks%d]}`, registerPoolSize+1)); err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().RegistersOutOfRange[`\toks`]; got == 0 {
		t.Errorf("an index past registerPoolSize was not tallied: %v",
			e.Diagnostics().RegistersOutOfRange)
	}
}
