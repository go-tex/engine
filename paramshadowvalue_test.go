// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"sort"
	"strings"
	"testing"
)

// ⛔ A \newcount in the macro layers can SHADOW a TeX parameter, and nothing says
// so. namedInt prefers a count register bound to the name and only falls back to
// texIntParams, so `\newcount\widowpenalty` makes both `\the\widowpenalty` and the
// engine read a fresh register holding ZERO — the parameter table's value becomes
// unreachable while every surface still looks consistent.
//
// Measured against tectonic on \documentclass[11pt]{book} before the fix:
//
//	\clubpenalty      ours 0      reference 10000 (\@afterheading)
//	\widowpenalty     ours 0      reference 150
//	\predisplaypenalty ours 0     reference 10000
//	\brokenpenalty    ours 0      reference 100
//	\displaywidowpenalty ours 50  reference 50   ← the one name NOT \newcount'd
//
// So the invariant is not structural but about VALUES: a freshly loaded engine
// must agree with its own parameter table. A layer that wants a different value
// says so here with its reason, which is what the existing exceptions do.
func TestLoadedEngineAgreesWithItsIntParameterTable(t *testing.T) {
	// Names the macro layers deliberately set to something else, with why.
	exceptions := map[string]string{
		// TeX sets the date at start-up (tex.web §241: "fix_date_and_time"), so a
		// loaded engine reads today, not the table's placeholder zero. Three names,
		// one reason — and \time is not here because the table does not list it.
		"day": "today's date", "month": "today's date", "year": "today's date",
	}

	e := New()
	e.LoadLaTeX()

	var bad []string
	for name, want := range texIntParams {
		if _, ok := exceptions[name]; ok {
			continue
		}
		if got := e.namedInt(name); got != want {
			bad = append(bad, "\\"+name+": engine reads "+itoa(got)+", table says "+itoa(want))
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d parameter(s) shadowed or re-set by the macro layers:\n  %s\n"+
			"A \\newcount of a parameter's own name allocates a register that hides it. "+
			"Remove the allocation, or list the name in exceptions with the reason.",
			len(bad), strings.Join(bad, "\n  "))
	}
	// ⛔ Both halves of the guard: that it looked at the table at all, and that it
	// can see a planted shadow. Without these a table that failed to load, or a
	// namedInt that ignored registers, would pass silently.
	if len(texIntParams) < 20 {
		t.Fatalf("only %d integer parameters in the table — the table, not the engine, is what passed", len(texIntParams))
	}
	probe := New()
	probe.LoadLaTeX()
	if _, err := probe.Run(`\newcount\widowpenalty`); err != nil {
		t.Fatal(err)
	}
	if probe.namedInt("widowpenalty") == texIntParams["widowpenalty"] {
		t.Fatal("a planted \\newcount\\widowpenalty did not change what the engine reads: the check cannot detect a shadow")
	}
}
