// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \url, \nolinkurl, \href and \hypertarget read their argument RAW from the source,
// so ~, %, #, _ and & keep their literal value. Only the SOURCE can be read that way:
// when something is pending on the input stack the argument is coming from an
// expansion, and the base text at the mouth's position is whatever follows the macro
// that produced it.
//
// beamer builds such an argument — its navigation lays a \hypertarget per frame whose
// name it composes — and with an outer theme that draws a head or foot line the
// target is emitted while a [fragile] frame's body is being copied out verbatim. The
// braced group eaten from the source was the frame's own: \textbf{gras} reached
// \jobname.vrb as \textbf.

func TestRawBracedArgFromAnExpansionDoesNotEatTheSource(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\def\pose{\hypertarget{ancre}{}}` +
		`\pose {\message{[groupe]}}\message{[fin]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := trimNL(out); got != "[groupe] [fin]" {
		t.Errorf("= %q, want the braced group that follows to survive", got)
	}
}

// The same, with the argument itself built by the expansion.
func TestRawBracedArgBuiltByAMacro(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\def\nom{ancre}\def\pose{\hypertarget{\nom}{}}` +
		`\pose {\message{[groupe]}}\message{[fin]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := trimNL(out); got != "[groupe] [fin]" {
		t.Errorf("= %q, want the braced group that follows to survive", got)
	}
}

// Read straight from the source, the argument is still taken raw: a % does not start
// a comment and a ~ is not the \nobreakspace tie.
func TestRawBracedArgFromTheSourceStaysRaw(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\hsize=300pt\url{http://x/a~b%c_d#e&f}\message{[apres]}`); err != nil {
		t.Fatalf("Run: %v", err)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	for _, want := range []string{"~", "%", "_", "#", "&"} {
		if !strings.Contains(svg, want) {
			t.Errorf("the URL lost %q — it was not read raw", want)
		}
	}
}
