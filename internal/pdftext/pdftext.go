// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package pdftext finds and runs the text extractor the measuring tools share.
//
// ⛔ ONE EXTRACTOR, OR THE TOOLS MEASURE THE EXTRACTOR. gotex-refdiff scores a
// paper by what pdftotext reads off two PDFs; anything else that reads a PDF
// and is compared with that score has to read it the SAME way, through the same
// binary found by the same rule. A second tool that reached for a different
// extractor would report differences that are the extractors', and it would
// report them in the same units.
//
// What lives here is therefore the part that must not diverge: which binary,
// how it is found, and the pkgx project pin. What does NOT live here is what
// each tool makes of the text — refdiff's word recall, its genuine-empty versus
// tool-error distinction — because those are its scoring semantics, not
// extraction.
package pdftext

import (
	"context"
	"os/exec"
	"time"
)

// PopplerProject is the pkgx project id that unambiguously provides poppler's
// command-line tools. On some machines a bare `pkgx pdftotext` fails with
// MultipleProjects (ambiguous between poppler.freedesktop.org and
// freedesktop.org/poppler-qt5); pinning the project resolves it so the pkgx
// fallback runs the extractor instead of silently producing no text.
const PopplerProject = "poppler.freedesktop.org"

// PkgxArgs builds the argument list after `pkgx` to run tool name with
// toolArgs, pinning the poppler project for poppler's ambiguous tools
// (pdftotext, pdftoppm) so pkgx resolves a single package rather than erroring.
// Other tools are passed through unpinned.
func PkgxArgs(name string, toolArgs ...string) []string {
	args := make([]string, 0, len(toolArgs)+2)
	if name == "pdftotext" || name == "pdftoppm" {
		args = append(args, "+"+PopplerProject)
	}
	args = append(args, name)
	return append(args, toolArgs...)
}

// CaptureRunner returns a function that runs a text-producing tool (pdftotext)
// on a PDF, writing to stdout ("-"), and returns the captured text. opts are
// passed to the tool BEFORE the file, which is where pdftotext takes its
// options.
//
// ⚠ THE PATH-OR-pkgx DECISION IS MADE ONCE, not per call, so a genuinely
// failing tool is not silently retried through pkgx — a retry would turn "this
// binary is broken" into "this page has no text".
//
// ⭐ opts EXISTS FOR `-layout`, AND THAT IS NOT A COSMETIC CHOICE. Without it
// pdftotext reads a two-column page in a single stream and FUSES the end of one
// column's line to the start of another's: `disFigure`, `participaentire`,
// `quaThe` are real tokens from one revtex paper. Which boundaries fuse depends
// on where the lines break, so a line-breaking change moves them — and a score
// built on these tokens partly measures the extractor's column handling rather
// than the engine's. Callers that compare two layouts should say -layout and
// find out.
func CaptureRunner(name string, opts ...string) func(timeout time.Duration, pdf string) (string, error) {
	usePkgx := false
	if _, err := exec.LookPath(name); err != nil {
		usePkgx = true
	}
	return func(timeout time.Duration, pdf string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		args := append(append([]string{}, opts...), pdf, "-")
		var cmd *exec.Cmd
		if usePkgx {
			cmd = exec.CommandContext(ctx, "pkgx", PkgxArgs(name, args...)...)
		} else {
			cmd = exec.CommandContext(ctx, name, args...)
		}
		out, err := cmd.Output()
		return string(out), err
	}
}
