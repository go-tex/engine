// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \openout<n>=<filename> opens an output stream. The engine writes no auxiliary
// files, but the primitive still has to CONSUME its operand. Undefined, the
// filename was left in the input and TYPESET: beamer opens three streams for its
// .nav/.toc/.snm files as the document begins, so every talk carried a page
// reading "texput.nav texput.toc texput.snm".

func TestOpenoutConsumesItsFileName(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"with =", `\openout5=texput.nav \message{[after]}`},
		{"without =", `\openout5 texput.nav \message{[after]}`},
		{"unexpandable cs ends the name", `\openout5=texput.nav\relax\message{[after]}`},
		{"stream from a register", `\newcount\nn \nn=7 \openout\nn=texput.toc \message{[after]}`},
	} {
		e, err := buildEngine(Options{}, true)
		if err != nil {
			t.Fatalf("buildEngine: %v", err)
		}
		out, err := e.Run(c.src)
		if err != nil {
			t.Fatalf("%s: Run: %v", c.name, err)
		}
		if strings.Contains(out, "texput") {
			t.Errorf("%s: the file name leaked into the output: %q", c.name, out)
		}
		if !strings.Contains(out, "[after]") {
			t.Errorf("%s: got %q, want it to contain [after] — the scan ate what followed", c.name, out)
		}
	}
}

// The two ends of a file name, straight from tex.web: §516 more_name ends the name
// on a SPACE and on nothing else, and §526 scan_file_name reads with get_x_token —
// so an EXPANDABLE control sequence contributes the characters it expands to and
// becomes part of the name, while an unexpandable one stops the scan and is pushed
// back (back_input) to be executed afterwards.
func TestOpenoutFileNameEndsAsTeXSaysItDoes(t *testing.T) {
	// Expandable: \zz expands to ZZ, which joins the name and must NOT be typeset.
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\def\zz{ZZ}\openout5=abc\zz \message{[after]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, "ZZ") {
		t.Errorf("an expandable cs in a file name reached the output: %q", out)
	}
	if !strings.Contains(out, "[after]") {
		t.Errorf("got %q, want [after]", out)
	}

	// Unexpandable: \message is not part of the name, so it still runs.
	e2, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out2, err := e2.Run(`\openout5=abc\message{[pushed-back]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out2, "[pushed-back]") {
		t.Errorf("got %q — an unexpandable cs was swallowed by the file name", out2)
	}
}

func TestOpenoutLeavesNoPage(t *testing.T) {
	// End to end: a document whose only content is stream bookkeeping renders the
	// page its text asks for, not the file names.
	pages, err := CompileToSVGPages([]byte(
		`\documentclass{article}\begin{document}\openout5=texput.nav \openout6=texput.toc VISIBLE\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	all := strings.Join(pages, "")
	if strings.Contains(all, "texput") {
		t.Errorf("a stream file name was typeset onto the page")
	}
	if !strings.Contains(all, "VISIBLE") {
		t.Errorf("the document's own text is missing from the page")
	}
}
