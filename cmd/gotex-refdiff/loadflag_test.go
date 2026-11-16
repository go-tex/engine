// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"strings"
	"testing"
)

// ⛔ loadFlag exists because main has to decide whether to measure BEFORE run
// parses anything, and that decision is the one thing in this tool no test of
// run can reach. So it gets its own witnesses — and one on EACH SIDE of its
// boundary, because a stated rule ("it recognises these four spellings and
// nothing else") is only as good as the case that catches it being too wide.
//
// What the two sides cost if they are wrong:
//
//   - too NARROW — an operator's `-under-load "why"` is not seen, the gate
//     refuses a deliberate measurement, and the reason never reaches the report.
//   - too WIDE — something that is not the flag is read as a reason, and a
//     measurement runs on a busy machine with a stamp nobody wrote.
func TestLoadFlagReadsEverySpellingAndNothingElse(t *testing.T) {
	const why = "corpus run queued behind a CI build"
	accept := []struct {
		name string
		args []string
	}{
		{"single dash, separate value", []string{"-under-load", why}},
		{"double dash, separate value", []string{"--under-load", why}},
		{"single dash, equals", []string{"-under-load=" + why}},
		{"double dash, equals", []string{"--under-load=" + why}},
		// ⚠ Not only in first position: the operator types it where it falls.
		{"after other flags", []string{"-n", "40", "-seed", "7", "-under-load", why}},
		{"before other flags", []string{"-under-load=" + why, "-n", "40"}},
	}
	for _, c := range accept {
		if got := loadFlag(c.args); got != why {
			t.Errorf("%s: loadFlag = %q, want %q — a deliberate override that is not "+
				"read means the gate refuses a run the operator authorised", c.name, got, why)
		}
	}

	reject := []struct {
		name string
		args []string
	}{
		{"no flag at all", []string{"-n", "40"}},
		{"empty arguments", nil},
		// A value is REQUIRED: `-under-load` alone is not an override. run's own
		// FlagSet rejects it a moment later, so the right answer here is "no
		// override" and not "an override with an empty reason".
		{"flag with no value", []string{"-n", "40", "-under-load"}},
		{"equals with no reason", []string{"-under-load="}},
		// ⛔ The too-wide side. A prefix match on "-under-load" alone would take
		// all of these, and the last one would hand a measurement a stamp that
		// says nothing about why it was taken.
		{"a longer flag name", []string{"-under-loading", why}},
		{"a longer name with equals", []string{"-under-loadx=" + why}},
		{"the reason without the flag", []string{why}},
	}
	for _, c := range reject {
		if got := loadFlag(c.args); got != "" {
			t.Errorf("%s: loadFlag = %q, want \"\" — anything read as an override here "+
				"lets a measurement run on a busy machine under a reason nobody wrote",
				c.name, got)
		}
	}
}

// And the control the two loops above cannot give: that the flag run DECLARES is
// spelled the same as the one main READS. They are two literals in two functions,
// and the day they diverge both loops above still pass while `-under-load` stops
// working — declared by one, acted on by the other, agreed on by neither.
func TestRunDeclaresTheFlagMainReads(t *testing.T) {
	var out, errb bytes.Buffer
	// -h makes run print its defaults and exit; the flag must appear there,
	// because that listing is where an operator learns the spelling.
	if code := run([]string{"-h"}, &out, &errb); code != 2 {
		t.Fatalf("-h returned %d, want 2", code)
	}
	all := out.String() + errb.String()
	if !strings.Contains(all, "-under-load") {
		t.Errorf("run's own flag listing does not mention -under-load, so the flag main "+
			"reads is not the flag run declares:\n%s", all)
	}
}
