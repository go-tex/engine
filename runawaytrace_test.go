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
