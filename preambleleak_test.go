// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// Nothing between \documentclass and \begin{document} is typeset — LaTeX refuses
// text there outright ("Missing \begin{document}") — so lenient recovery from an
// undefined command may discard its arguments in the preamble exactly as it does
// inside a package. It did not, and the leaked text OPENED A PAGE.
//
// Corpus paper 2304.01951 \input{macros.tex} from its preamble, and macros.tex
// carries \pgfdeclarelayer{background} and \pgfsetlayers{background,main}, both
// undefined with GOTEX_PGF off. Their arguments were typeset, that text began a
// page, and the paper's real title page became page 2 — page 1 holding the words
// "background" and "background,main" and nothing else.
func TestPreambleDoesNotTypesetAnUndefinedCommandsArguments(t *testing.T) {
	const src = `\documentclass{article}` +
		`\pgfdeclarelayer{background}\pgfsetlayers{background,main}` +
		`\begin{document}BODYWORD\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// The commands are still REPORTED — discarding an argument must not hide that
	// the command was not understood.
	for _, name := range []string{"pgfdeclarelayer", "pgfsetlayers"} {
		if e.SkippedCommands()[name] == 0 {
			t.Errorf("\\%s was not reported as skipped", name)
		}
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if !strings.Contains(svg, "<path") {
		t.Fatal("rendered no glyph paths at all")
	}
	// The leak is legible in the source layer our SVG carries, which is why this
	// asserts on it rather than on a glyph count: a repeated glyph is not one path
	// per character.
	for _, leaked := range []string{"background,main"} {
		if strings.Contains(svg, leaked) {
			t.Errorf("preamble leaked %q onto the page", leaked)
		}
	}
}

// A file with NO \documentclass is a fragment — a body someone \inputs — and has no
// preamble at all, so nothing in it may be discarded. Getting that wrong reads as a
// smaller corpus: 546 of 7990 undefined commands in one census came from two such
// files compiled alone (measure/TOOLS.md, go-tex/engine#395).
func TestAFragmentHasNoPreambleToDiscardIn(t *testing.T) {
	// No \documentclass, so hasClass stays false and the document-mode rule applies:
	// the argument is CONTENT and is typeset.
	const src = `\undefinedmacro{KEEPTHISWORD}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if e.SkippedCommands()["undefinedmacro"] == 0 {
		t.Error("\\undefinedmacro was not reported")
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if !strings.Contains(svg, "KEEPTHISWORD") {
		t.Error("a fragment's content was discarded as if it were a preamble")
	}
}
