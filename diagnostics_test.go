// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// Diagnostics surfaces the silent-swallow alarms a lenient render otherwise hides:
// a runaway that tripped, and groups left open at the end of the document.
func TestDiagnosticsRunawayAndOpenGroups(t *testing.T) {
	// A self-referential macro trips the runaway guard.
	e := New()
	e.LoadLaTeX()
	e.lenient = true
	e.Run(`\def\lp{\lp}\lp`)
	if d := e.Diagnostics(); !d.Runaway {
		t.Error("Diagnostics did not flag the runaway")
	}

	// An unclosed group is left open at the end.
	e2 := New()
	e2.LoadLaTeX()
	e2.lenient = true
	e2.Run(`before {\bgroup unclosed`)
	if d := e2.Diagnostics(); d.OpenGroups == 0 {
		t.Error("Diagnostics did not report the open groups")
	}
}

// Internal markers (the page-cap tally) are lifted out of Skipped into their own
// flag, not reported as a fake undefined command.
func TestDiagnosticsHidesInternalMarkers(t *testing.T) {
	e := New()
	e.skippedCS = map[string]int{"realcmd": 3, "gotex@pagelimit": 1}
	d := e.Diagnostics()
	if _, ok := d.Skipped["gotex@pagelimit"]; ok {
		t.Error("internal marker leaked into Skipped")
	}
	if d.Skipped["realcmd"] != 3 {
		t.Error("real skipped command missing")
	}
	if !d.PageCapHit {
		t.Error("PageCapHit not set from the marker")
	}
}
