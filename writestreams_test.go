// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"strings"
	"testing"
)

// A document that writes a file and reads it again must get its own text back.
// \openout / \write / \closeout used to consume their operands and write nowhere,
// so the \input that followed found nothing. beamer renders a [fragile] frame
// exactly this way — it copies the frame's body out verbatim, line by line, then
// \inputs it with the catcodes restored.

func TestWriteStreamRoundTrip(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\immediate\openout3=zz-round.vrb
\immediate\write3{\noexpand\message{[relu]}}%
\immediate\closeout3
\input{zz-round.vrb}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "[relu]") {
		t.Errorf("got %q, want the written line to come back through \\input", out)
	}
	if _, err := os.Stat("zz-round.vrb"); !os.IsNotExist(err) {
		t.Errorf("the engine created a file beside the document: %v", err)
	}
}

// tex.web §1369-1370: \write runs its token list through the scanner, "expanding
// macros and \the and \number", and token_show(def_ref); print_ln writes the
// EXPANDED list followed by exactly one newline. So one \write is one line, and
// what lands in the file is the expansion, not the literal tokens.
func TestWriteExpandsItsListAndWritesOneLine(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\count5=42 \def\mot{expanse}
\immediate\openout3=zz-exp.vrb
\immediate\write3{\noexpand\message{[\mot\the\count5]}}%
\immediate\write3{\noexpand\message{[deuxieme ligne]}}%
\immediate\closeout3
\input{zz-exp.vrb}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "[expanse42]") {
		t.Errorf("got %q, want the list expanded before it is written", out)
	}
	if !strings.Contains(out, "[deuxieme ligne]") {
		t.Errorf("got %q, want each \\write on its own line — the second one was lost", out)
	}
}

// A \write to a stream that is not open is discarded rather than typeset. TeX sends
// it to the log, which this engine does not keep.
func TestWriteToAClosedStreamIsDiscarded(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\immediate\write9{perdu}\message{[suite]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, "perdu") {
		t.Errorf("got %q, want the write to a closed stream to produce nothing", out)
	}
	if !strings.Contains(out, "[suite]") {
		t.Errorf("got %q, want what follows the \\write to survive", out)
	}
}

// A stream that is never \closeout is not readable: TeX has not finished the file.
func TestAnUnclosedStreamIsNotReadable(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\immediate\openout3=zz-open.vrb
\immediate\write3{\noexpand\message{[pas ferme]}}%
\input{zz-open.vrb}\message{[suite]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, "[pas ferme]") {
		t.Errorf("got %q, want an unclosed stream to stay unreadable", out)
	}
	if !strings.Contains(out, "[suite]") {
		t.Errorf("got %q, want the document to carry on past the failed \\input", out)
	}
}

// \openout on a stream that is already open starts the file again: what was written
// before belongs to the file that stream held, and is dropped with it.
func TestReopeningAStreamStartsANewFile(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\immediate\openout3=zz-re.vrb
\immediate\write3{\noexpand\message{[premier]}}%
\immediate\openout3=zz-re.vrb
\immediate\write3{\noexpand\message{[second]}}%
\immediate\closeout3
\input{zz-re.vrb}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, "[premier]") {
		t.Errorf("got %q, want the abandoned file's text to be gone", out)
	}
	if !strings.Contains(out, "[second]") {
		t.Errorf("got %q, want the reopened file's text", out)
	}
}
