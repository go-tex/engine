// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// go-tex/engine#788: running out of registers is a DECISION this engine already
// made — \newcount past the limit allocates nothing and the name stays
// undefined, which TestNewcounterExhaustion documents. What was missing is that
// nothing said so: the run completed normally and whatever used the register
// read 0 or vanished.
//
// ⛔ THE CONTROL IS THE SAME COMMAND ONE ALLOCATION EARLIER. A test that only
// drives the pool full would pass just as well against a tally incremented on
// every \newcount, so each case asserts the count BEFORE the ceiling too.
func TestARefusedRegisterAllocationIsReported(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(e *Engine)
		src   string
		kind  string
	}{
		{`\newcount`, func(e *Engine) { e.allocCnt = registerPoolSize - 1 }, `\newcount\aa \newcount\bb`, `\count`},
		{`\newdimen`, func(e *Engine) { e.allocDim = registerPoolSize - 1 }, `\newdimen\aa \newdimen\bb`, `\dimen`},
		{`\newskip`, func(e *Engine) { e.allocSkp = registerPoolSize - 1 }, `\newskip\aa \newskip\bb`, `\skip`},
		{`\newbox`, func(e *Engine) { e.allocBox = registerPoolSize - 1 }, `\newbox\aa \newbox\bb`, `\box`},
		{`\newcounter`, func(e *Engine) { e.allocCnt = registerPoolSize }, `\newcounter{zz}`, `\count`},
		{`\newtheorem`, func(e *Engine) { e.allocCnt = registerPoolSize }, `\newtheorem{zz}{Z}`, `\count`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := buildEngine(Options{Lenient: true}, true)
			if err != nil {
				t.Fatal(err)
			}
			c.setup(e)
			if _, err := e.Run(c.src); err != nil {
				t.Fatalf("Run: %v", err)
			}
			got := e.Diagnostics().RegistersExhausted
			if got[c.kind] == 0 {
				t.Errorf("a refused %s allocation was not reported: %v", c.name, got)
			}
			// ⛔ AND IT MUST NOT COUNT THE ONES THAT SUCCEEDED. Two \newcount with
			// one register left is exactly one refusal, never two.
			if n := got[c.kind]; n > 1 {
				t.Errorf("%s reported %d refusals, want 1: an allocation that SUCCEEDED was tallied", c.name, n)
			}
		})
	}
}

// A document that never runs out reports nothing at all: an empty map, not a
// map of zeroes. Without this the field would read as "always present", and a
// reader cannot tell a quiet run from an unreported one.
func TestADocumentWithRegistersToSpareReportsNoRefusal(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\newcount\aa \newdimen\bb \newskip\cc \newbox\dd \newcounter{ee}`); err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().RegistersExhausted; len(got) != 0 {
		t.Errorf("RegistersExhausted = %v on a document with registers to spare, want empty", got)
	}
}

// The refused name stays UNDEFINED, which is the behaviour the report exists to
// explain. Asserting it here keeps the two together: if the allocator ever
// starts handing out a register past the ceiling, this test says so rather than
// leaving a report describing something that no longer happens.
func TestARefusedAllocationLeavesTheNameUndefined(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	e.allocCnt = registerPoolSize
	if _, err := e.Run(`\newcount\zz`); err != nil {
		t.Fatal(err)
	}
	if m := e.eq["zz"]; m != nil && m.kind == mCountRef {
		t.Errorf("\\zz became a register past the ceiling: %+v", m)
	}
	if e.Diagnostics().RegistersExhausted[`\count`] == 0 {
		t.Error("the refusal that left it undefined was not reported")
	}
}
