// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \@classoptionslist holds the options given to \documentclass. ltclass.dtx starts it
// (and \@raw@classoptionslist) at \relax and has \documentclass overwrite them on the
// FIRST class load:
//
//	\ifx\@classoptionslist\relax
//	  \protected@xdef\@classoptionslist{\zap@space#2 \@empty}%
//	  \gdef\@raw@classoptionslist{#2}%
//
// Leaving it undefined was not merely a blank. \@for over an undefined control
// sequence does not iterate over nothing — it SWALLOWS what follows. beamer runs that
// loop unguarded in \beamer@filterclassoptions and again in \ProcessOptionsBeamer, so
// any theme built on \ProcessOptionsBeamer took the rest of the document with it.

func clsRun(t *testing.T, src string) string {
	t.Helper()
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return out
}

func TestClassOptionListRecordsTheOptions(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"no options", `\documentclass{article}\makeatletter\message{[\@classoptionslist]}`, "[]"},
		{"one option", `\documentclass[11pt]{article}\makeatletter\message{[\@classoptionslist]}`, "[11pt]"},
		{"several", `\documentclass[11pt,a4paper]{article}\makeatletter\message{[\@classoptionslist]}`, "[11pt,a4paper]"},
		{"raw list too", `\documentclass[11pt]{article}\makeatletter\message{[\@raw@classoptionslist]}`, "[11pt]"},
	} {
		if got := clsRun(t, c.src); !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q, want it to contain %q", c.name, got, c.want)
		}
	}
}

func TestForOverTheClassOptionListDoesNotSwallow(t *testing.T) {
	// The shape beamer uses, unguarded. Before \@classoptionslist existed, the loop
	// ate everything after it.
	src := `\documentclass{article}\makeatletter` +
		`\@for\CurrentOption:=\@classoptionslist\do{\message{[tour]}}` +
		`\makeatother\message{[apres]}`
	if got := clsRun(t, src); !strings.Contains(got, "[apres]") {
		t.Errorf("got %q — \\@for over \\@classoptionslist swallowed what followed", got)
	}
}

func TestClassOptionListIsSetOnlyByTheFirstClass(t *testing.T) {
	// ltclass.dtx guards the assignment with \ifx\@classoptionslist\relax, so a class
	// that loads another class does not overwrite the document's own list.
	src := `\documentclass[11pt]{article}\documentclass[12pt]{article}` +
		`\makeatletter\message{[\@classoptionslist]}`
	if got := clsRun(t, src); !strings.Contains(got, "[11pt]") {
		t.Errorf("got %q, want the FIRST class's options [11pt]", got)
	}
}
