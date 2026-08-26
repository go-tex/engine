// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A \vbox is INTERNAL VERTICAL mode: text inside it is a paragraph, broken to
// \hsize, and the box stacks the resulting LINES.
//
// The engine used to build a \vbox the way it builds an \hbox — restricted
// horizontal mode — so its text never reached the paragraph builder. \vbox{\hbox{X}}
// rendered because X was already boxed; \vbox{X} stacked the letters as if each were
// a line and painted none of them. Every \vbox{ text } lost its text, including the
// body of a beamer frame, which the class builds as \vbox\bgroup … \egroup.

// vboxOf runs src and returns box register 0.
func vboxOf(t *testing.T, src string) *boxNode {
	t.Helper()
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(src); err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	b := e.box[0]
	if b == nil {
		t.Fatalf("box 0 is void after %q", src)
	}
	return b
}

// countGlyphs totals the character nodes anywhere inside a box.
func countGlyphs(n node) int {
	b, ok := n.(*boxNode)
	if !ok || b == nil {
		if _, isChar := n.(charNode); isChar {
			return 1
		}
		return 0
	}
	total := 0
	for _, c := range b.list {
		total += countGlyphs(c)
	}
	return total
}

func TestVboxKeepsItsText(t *testing.T) {
	b := vboxOf(t, `\setbox0=\vbox{ZORGLUB}`)
	if got := countGlyphs(b); got != 7 {
		t.Errorf("\\vbox{ZORGLUB} holds %d glyphs, want 7", got)
	}
	// The text is a LINE — a horizontal box inside the vertical list — not seven
	// vertically stacked letters.
	lines := 0
	for _, n := range b.list {
		if lb, ok := n.(*boxNode); ok && lb.kind == hbox {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("\\vbox{ZORGLUB} holds %d lines, want 1", lines)
	}
}

func TestVboxBgroupFormKeepsItsText(t *testing.T) {
	// \vbox\bgroup … \egroup: the form beamer uses to open a frame's body. It cannot
	// be grabbed as a token list, so the builder has to read it from the input.
	b := vboxOf(t, `\setbox0=\vbox\bgroup ZORGLUB\egroup`)
	if got := countGlyphs(b); got != 7 {
		t.Errorf("\\vbox\\bgroup form holds %d glyphs, want 7", got)
	}
}

func TestVboxBreaksItsParagraphToHsize(t *testing.T) {
	// A measure too narrow for the text produces several lines — proof the paragraph
	// builder ran, not just that the characters survived.
	b := vboxOf(t, `\hsize=40pt\setbox0=\vbox{aaa bbb ccc ddd eee fff ggg hhh}`)
	lines := 0
	for _, n := range b.list {
		if lb, ok := n.(*boxNode); ok && lb.kind == hbox {
			lines++
		}
	}
	if lines < 2 {
		t.Errorf("a paragraph in a 40pt \\vbox broke into %d line(s), want at least 2", lines)
	}
}

func TestVboxSeveralParagraphs(t *testing.T) {
	b := vboxOf(t, `\setbox0=\vbox{AAA\par BBB\par}`)
	if got := countGlyphs(b); got != 6 {
		t.Errorf("two paragraphs hold %d glyphs, want 6", got)
	}
}

func TestVboxOfBoxesStillWorks(t *testing.T) {
	// The case that worked before must keep working.
	b := vboxOf(t, `\setbox0=\vbox{\hbox{AB}\hbox{CD}}`)
	if got := countGlyphs(b); got != 4 {
		t.Errorf("\\vbox of two hboxes holds %d glyphs, want 4", got)
	}
}

func TestVboxTextRenders(t *testing.T) {
	// End to end: the text of a \vbox reaches the page.
	pages, err := CompileToSVGPages([]byte(
		`\documentclass{article}\begin{document}AVANT\par\setbox0=\vbox{ZORGLUB}\box0\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	all := strings.Join(pages, "")
	for _, w := range []string{"AVANT", "ZORGLUB"} {
		if !strings.Contains(all, w) {
			t.Errorf("%q is missing from the rendered page", w)
		}
	}
}

// ── \@currenvir ─────────────────────────────────────────────────────────────

func TestBeginRecordsCurrentEnvironment(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\newenvironment{myenv}{}{}\begin{myenv}\makeatletter\message{[\@currenvir]}\makeatother\end{myenv}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "[myenv]") {
		t.Errorf(`\@currenvir = %q, want it to contain "[myenv]"`, out)
	}
}

func TestCurrentEnvironmentComparesEqualToAName(t *testing.T) {
	// The comparison beamer actually makes: \ifx\@currenvir\<a macro holding the
	// name>. It only holds if the recorded body carries the same catcodes as the
	// class's own \def, which is why the name is retokenised under the current ones.
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\newenvironment{myenv}{}{}\makeatletter\def\thename{myenv}\makeatother` +
		`\begin{myenv}\makeatletter\ifx\@currenvir\thename\message{[EGAL]}\else\message{[DIFFERENT]}\fi\makeatother\end{myenv}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "[EGAL]") {
		t.Errorf(`\ifx\@currenvir\thename said %q, want [EGAL]`, out)
	}
}

func TestBeginStaysExpandable(t *testing.T) {
	// \begin must expand cleanly inside \message / \edef: the engine's tests observe
	// environments that way, and an assignment in its expansion would print as
	// literal text. This is why \@currenvir is recorded from \begin's Go-side probe
	// rather than with a \def in its body.
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\newenvironment{note}{[B]}{[E]}\message{\begin{note}mid\end{note}}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, `\def`) {
		t.Errorf("\\begin leaked an assignment into its expansion: %q", out)
	}
	if !strings.Contains(out, "[B]mid[E]") {
		t.Errorf("expansion = %q, want it to contain [B]mid[E]", out)
	}
}
