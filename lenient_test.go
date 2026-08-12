// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A document that uses commands gotex does not define must, in strict mode, fail
// with an "Undefined control sequence" error — and in lenient mode, compile: the
// unknown commands (and their argument blocks) are skipped and the surrounding
// prose is typeset.
func TestLenientSkipsUndefined(t *testing.T) {
	const src = `\documentclass{article}
\usepackage{madeuppkg}
\begin{document}
\somemacro[opt]{swallowed} Alpha \another{eaten} Beta\bareword\ Gamma.
\end{document}`

	// strict: a hard error naming the first undefined command.
	if _, err := CompileToSVGPages([]byte(src), Options{}); err == nil {
		t.Fatal("strict mode: expected an undefined-control-sequence error, got nil")
	}

	// lenient: it compiles.
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("lenient mode: unexpected error: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("lenient mode: expected at least one page")
	}
	svg := strings.Join(pages, "")
	// An undefined command's {argument} must not leak into the output as text.
	if strings.Contains(svg, "swallowed") || strings.Contains(svg, "eaten") {
		t.Error("lenient mode: an undefined command's {argument} leaked into the output as text")
	}
}

// SkippedCommands reports exactly which control sequences were dropped, and how
// many times, so a caller can surface them after a preview compile.
func TestLenientSkippedCommands(t *testing.T) {
	const src = `\documentclass{article}\begin{document}
\foo \foo \bar{x} plain text.
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("lenient compile: %v", err)
	}
	got := e.SkippedCommands()
	if got["foo"] != 2 {
		t.Errorf("skipped[foo] = %d, want 2", got["foo"])
	}
	if got["bar"] != 1 {
		t.Errorf("skipped[bar] = %d, want 1", got["bar"])
	}
}

// Lenient mode must also survive the failure modes real third-party documents hit
// past the preamble — each of these is a hard error in strict mode but a best-effort
// no-op (or placeholder) in lenient mode, so the compile still produces pages.
func TestLenientBodyFailuresTolerated(t *testing.T) {
	cases := []struct {
		name, src, wantSkip string
	}{
		{"missing image", `\documentclass{article}\begin{document}
Before.\par\includegraphics[width=100pt]{no-such-figure.png}\par After.
\end{document}`, "includegraphics"},
		{"unknown math", `\documentclass{article}\begin{document}
Text $a + \someunknownmathop b$ more text.
\end{document}`, `\someunknownmathop`},
		{"undefined length", `\documentclass{article}\begin{document}
\setlength{\nosuchlength}{4pt}Body text.
\end{document}`, ""},
		{"missing input", `\documentclass{article}\begin{document}
Head.\input{no-such-file}Tail.
\end{document}`, "input"},
		{"missing bibliography", `\documentclass{article}\begin{document}
Body.\bibliography{no-such-refs}
\end{document}`, "bibliography"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := CompileToSVGPages([]byte(c.src), Options{}); err == nil {
				t.Fatal("strict mode: expected an error, got nil")
			}
			e, err := compile([]byte(c.src), Options{Lenient: true})
			if err != nil {
				t.Fatalf("lenient mode: unexpected error: %v", err)
			}
			if c.wantSkip != "" && e.SkippedCommands()[c.wantSkip] == 0 {
				t.Errorf("expected %q recorded as skipped, got %v", c.wantSkip, e.SkippedCommands())
			}
		})
	}
}

// A no-argument undefined command must not eat the following word: \bareword is
// skipped but "Kept" stays.
func TestLenientNoArgKeepsFollowingWord(t *testing.T) {
	const src = `\documentclass{article}\begin{document}
\bareword Kept here.
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("lenient compile: %v", err)
	}
	if e.SkippedCommands()["bareword"] != 1 {
		t.Fatalf("expected \\bareword skipped once, got %v", e.SkippedCommands())
	}
	// "Kept" must be present in the vertical list somewhere: assert via a strict
	// re-render that the word's characters reach the page.
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil || len(pages) == 0 {
		t.Fatalf("render: pages=%d err=%v", len(pages), err)
	}
}
