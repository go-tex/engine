// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \@colht and \@colroom are LaTeX's page-height registers. A class that resizes
// the text block assigns them next to \vsize, and beamer's own
// \beamer@calculateheadfoot ends exactly that way:
//
//	\textheight=\paperheight
//	\advance\textheight by-\footheight
//	\advance\textheight by-\headheight
//	\@colht\textheight  \@colroom\textheight  \vsize\textheight
//
// Unallocated, \@colht did more than go missing. The skipped command left
// \textheight — which the engine \lets to \vsize — standing in vertical mode,
// where the \vsize primitive read the FOLLOWING token as its value and set the
// page height to ZERO. Every beamer frame then ran onto one endless page.

func TestColHeightRegistersAreAllocated(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\makeatletter\@colht=100pt\@colroom=90pt\makeatother`); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, c := range []struct {
		name string
		want int
	}{{"@colht", 100 * unity}, {"@colroom", 90 * unity}} {
		m := e.eq[c.name]
		if m == nil || m.kind != mDimenRef {
			t.Errorf(`\%s is not an allocated dimen (%v)`, c.name, m)
			continue
		}
		if got := e.dimen[m.code]; got != c.want {
			t.Errorf(`\%s = %d, want %d`, c.name, got, c.want)
		}
	}
}

func TestColHeightAssignmentLeavesVsizeAlone(t *testing.T) {
	// The exact shape that zeroed the page: \@colht\textheight with \textheight
	// \let to \vsize, followed by more input.
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	src := `\vsize=273pt\makeatletter\@colht\textheight\@colroom\textheight\vsize\textheight\makeatother\message{[after]}`
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if e.vsize != 273*unity {
		t.Errorf(`\vsize = %d after the \@colht/\@colroom sequence, want %d`, e.vsize, 273*unity)
	}
	// The trailing input must survive too: the old failure ate it as \vsize's value.
	if want := "[after]"; !strings.Contains(out, want) {
		t.Errorf("output %q lost %q — the sequence swallowed what followed", out, want)
	}
	// \@colht/\@colroom took the text height, as the class intended.
	for _, name := range []string{"@colht", "@colroom"} {
		m := e.eq[name]
		if m == nil {
			t.Fatalf(`\%s not allocated`, name)
		}
		if got := e.dimen[m.code]; got != 273*unity {
			t.Errorf(`\%s = %d, want %d`, name, got, 273*unity)
		}
	}
}
