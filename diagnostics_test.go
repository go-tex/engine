// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

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

// "Argument of \x has an extra }" is a RECOVERY, not a missing command: the macro
// exists and its argument grab was abandoned. Tallied in Skipped it was printed as
// a command NAME — "\Argument of \@authoropt has an extra }" offered as an
// undefined command — which is the mistake RunawayArgs was split out to stop, made
// a second time. It has its own field now, keyed by the macro and the line.
//
// The classification is what is tested, on the tally the recovery writes: a source
// that provokes it needs a real package's argument scan (measured on corpus paper
// 2312.03957, where pgf's \use@pgfmodule and the class's \@authoropt trip it), and
// pinning the message would be pinning the producer rather than the split.
func TestExtraBraceIsNotReportedAsAMissingCommand(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	e.expandingCS = "use@pgfmodule"
	e.curSrcLine = 661
	e.noteExtraBrace()
	e.skippedCS["realmissing"] = 3

	d := e.Diagnostics()
	if got := d.ExtraBrace["\\use@pgfmodule (line 661)"]; got != 1 {
		t.Errorf("ExtraBrace = %v, want the macro and its line", d.ExtraBrace)
	}
	if d.Skipped["realmissing"] != 3 {
		t.Errorf("a genuinely undefined command was lost from Skipped: %v", d.Skipped)
	}
	for k := range d.Skipped {
		if strings.Contains(k, "extra }") || strings.Contains(k, "Argument of") {
			t.Errorf("a recovery message is still tallied as an undefined command: %q", k)
		}
	}
}
