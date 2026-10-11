// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \endinput does not read to the end of the spliced file — it SKIPS FORWARD to the
// marker the loader appended (classprims.go), so that trailing documentation is
// never processed. The marker is preceded by \makeatletter, which is what repairs
// the catcode of @ after a package that ends \makeatother; landing on the marker
// itself jumped straight over that repair.
//
// So \makeatother ALONE is clean (TestPackageEndingWithMakeatotherLeavesNothing
// Behind covers it) and \endinput ALONE is clean, and the two together printed the
// engine's own markers on the page:
//
//	endofpackagehookgotex@endload Bonjour
//
// Two real corpus papers carried that onto page 1 (go-tex/engine#606).
func TestAPackageEndingWithMakeatotherThenEndinputLeavesNothingBehind(t *testing.T) {
	for _, c := range []struct{ name, tail string }{
		{"\\makeatother then \\endinput", "\\makeatother\n\\endinput\n"},
		{"\\endinput alone", "\\endinput\n"},
		{"\\makeatother alone", "\\makeatother\n"},
		// Trailing prose after \endinput is exactly what the skip exists for: it
		// must still be skipped, and the repair must still happen.
		{"prose after \\endinput", "\\makeatother\n\\endinput\nTest file integrity: \\] ^_ x\n"},
	} {
		sty := []byte("\\ProvidesPackage{loc}\n\\makeatletter\n\\def\\@foo{x}\n" + c.tail)
		opt := Options{Lenient: true, Resolve: func(name string) ([]byte, bool) {
			if name == "loc.sty" {
				return sty, true
			}
			return nil, false
		}}
		e, err := compile([]byte(`\documentclass{article}\usepackage{loc}`+
			`\begin{document}Bonjour\end{document}`), opt)
		if err != nil {
			t.Fatalf("%s: compile: %v", c.name, err)
		}
		if got := pageChars(e); got != "Bonjour" {
			t.Errorf("%s: the page carries %q, want %q", c.name, got, "Bonjour")
		}
		// And the frame must have been popped, which is the other half of landing
		// on the repair: \@gotex@endload is read with @ a letter or not at all.
		if len(e.loadStack) != 0 {
			t.Errorf("%s: %d load frame(s) still stacked: \\@gotex@endload did not run",
				c.name, len(e.loadStack))
		}
	}
}

// And the sentinel has to be one of the tokens that STOP a runaway delimited
// argument (isFileEndSentinel), not only a place for \endinput to land. A scan
// that swallows it never makes @ a letter, so the hook behind it is read as \@
// plus letters again and the leak comes back — in a file that both ends
// \makeatother and leaves a delimiter unclosed.
//
// Removing it from the stop list alone is invisible to
// TestDelimitedScanStillStopsAtAFileEnd: the scan simply halts one token later,
// at \@endofpackagehook. That mutation survived the whole suite until this case
// existed.
func TestARunawayMustNotSwallowTheTailsSentinel(t *testing.T) {
	sty := []byte("\\ProvidesPackage{loc}\n\\makeatletter\\def\\eat#1\\@nil{}\n" +
		"\\makeatother\n\\eat with-no-delimiter-in-sight\n")
	opt := Options{Lenient: true, Resolve: func(name string) ([]byte, bool) {
		if name == "loc.sty" {
			return sty, true
		}
		return nil, false
	}}
	e, err := compile([]byte(`\documentclass{article}\usepackage{loc}`+
		`\begin{document}Bonjour\end{document}`), opt)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := pageChars(e); got != "Bonjour" {
		t.Errorf("the page carries %q, want %q", got, "Bonjour")
	}
	if len(e.loadStack) != 0 {
		t.Errorf("%d load frame(s) still stacked after a runaway", len(e.loadStack))
	}
}
