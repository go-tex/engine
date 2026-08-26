// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A comment discards the rest of its line INCLUDING the line's end.
//
// tex.web §347 sends any_state_plus(comment) to <Finish line, goto switch>, which
// is §350: loc := limit+1. §362 puts the \endlinechar at buffer[limit], so
// loc := limit+1 steps PAST it. The line's end is therefore discarded with the
// rest of the line, positionally, WHATEVER its catcode.
//
// Stopping on the end-of-line and letting the next scan classify it was invisible
// while \catcode`\^^M was 5. It stopped being invisible the moment a package
// changed that catcode: beamer reads a line verbatim under \catcode`\^^M=12, and
// every %-terminated line in that group left a category-12 character to be
// TYPESET — one stray page per beamer document.

func cmRun(t *testing.T, src string) string {
	t.Helper()
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return out
}

func TestCommentEatsTheLineEndWhateverItsCatcode(t *testing.T) {
	// ^^M as an ordinary character: the end of a commented line must still vanish.
	out := cmRun(t, "\\begingroup\\catcode`\\^^M=12 %\n\\endgroup\\message{[end]}")
	if !strings.Contains(out, "[end]") {
		t.Fatalf("got %q, want [end]", out)
	}
	if strings.ContainsRune(out, '\r') {
		t.Errorf("a category-12 line end survived the comment: %q", out)
	}
}

func TestCommentJoinsTheNextLine(t *testing.T) {
	// The classic reason to write %: no interword space across the break.
	if out := cmRun(t, "\\message{[foo%\nbar]}"); !strings.Contains(out, "[foobar]") {
		t.Errorf("got %q, want [foobar]", out)
	}
}

func TestBlankLineAfterCommentStillBreaksTheParagraph(t *testing.T) {
	// §347 new_line+car_ret emits \par: the comment eats its OWN line end, and the
	// empty line that follows is what breaks the paragraph. Consuming the comment's
	// line end must not cost that \par.
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run("A%\n\nB"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Two paragraphs on the page, not one.
	paras := 0
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			paras++
		}
	}
	e2, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e2.Run("A%\nB"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	joined := 0
	for _, n := range e2.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			joined++
		}
	}
	if paras <= joined {
		t.Errorf("a blank line after a comment gave %d line(s) and no blank line gave %d — the \\par was lost", paras, joined)
	}
}
