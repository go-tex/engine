package engine

import (
	"bytes"
	"strings"
	"testing"
)

// A runaway is the hardest failure this engine reports to act on: the error names
// the line the document DIED on, which is almost never the line at fault. Loading
// pgf's parser module said "6:15" — \end{document} — for a loop that started in
// pgfkeys hundreds of expansions earlier. With the trace on, the looping macro
// names itself on the first line of the report.
func TestRunawayTraceNamesTheLoop(t *testing.T) {
	saveOn, saveOut := traceRunaway, traceOut
	defer func() { traceRunaway, traceOut = saveOn, saveOut }()
	var buf bytes.Buffer
	traceRunaway, traceOut = true, &buf

	e := New()
	e.SetFont(spMock{})
	// \loopy expands to itself: the shape of pgfkeys' \pgfkeys@mainstop sentinel
	// when something expands it instead of matching it as a delimiter.
	e.Run(`\def\loopy{\loopy}\loopy`)

	got := buf.String()
	if !strings.Contains(got, `\loopy`) {
		t.Errorf("report does not name the looping macro:\n%s", got)
	}
	// Runs are collapsed, so the report stays short enough to read.
	if !strings.Contains(got, "x") || strings.Count(got, "\n") > 20 {
		t.Errorf("report is not collapsed (%d lines):\n%s", strings.Count(got, "\n"), got)
	}
	if !strings.Contains(got, "FIRST expansions") || !strings.Contains(got, "LAST expansions") {
		t.Errorf("report lacks one of its two halves:\n%s", got)
	}
}

// With the trace off (the default) nothing is written and nothing is recorded.
func TestRunawayTraceSilentByDefault(t *testing.T) {
	saveOn, saveOut := traceRunaway, traceOut
	defer func() { traceRunaway, traceOut = saveOn, saveOut }()
	var buf bytes.Buffer
	traceRunaway, traceOut = false, &buf

	e := New()
	e.SetFont(spMock{})
	e.Run(`\def\loopy{\loopy}\loopy`)
	if buf.Len() != 0 {
		t.Errorf("wrote %q with the trace off", buf.String())
	}
	if e.trace != nil || e.traceHead != nil {
		t.Errorf("recorded a trace with the trace off")
	}
}

// The expansion trace says WHICH macros were spinning; it does not say what they
// were spinning ON, and a loop in a lookahead is decided entirely by the token in
// front of it. On a corpus paper whose TikZ \matrix truncated the document, the
// expansion list showed \pgfutil@ifnextchar cycling and nothing else — reading
// the input stack named the site in one run: pgfmath's unit test on top, three
// pending \fi below it, and the base buffer sitting on the space before
// \tapeblock inside the \matrix (go-tex/engine#410).
func TestRunawayTraceShowsWhatTheMouthWasReading(t *testing.T) {
	saveOn, saveOut := traceRunaway, traceOut
	defer func() { traceRunaway, traceOut = saveOn, saveOut }()
	var buf bytes.Buffer
	traceRunaway, traceOut = true, &buf

	e := New()
	e.SetFont(spMock{})
	// The loop reads nothing, so the source pointer stays where it started and the
	// report must still show the text sitting in front of it.
	e.Run(`\def\loopy{\loopy}\loopy MARKERTEXT`)

	got := buf.String()
	if !strings.Contains(got, "INPUT the mouth was about to read") {
		t.Errorf("report does not show the input stack:\n%s", got)
	}
	if !strings.Contains(got, "MARKERTEXT") {
		t.Errorf("report does not show the source the reader is stopped on:\n%s", got)
	}
	// It must stay readable: the whole report, both halves and the stack, short
	// enough to look at without scrolling.
	if n := strings.Count(got, "\n"); n > 30 {
		t.Errorf("report is %d lines, too long to read:\n%s", n, got)
	}
}
