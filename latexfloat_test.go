package engine

import (
	"strings"
	"testing"
)

// The table and figure environments number their captions from independent
// counters, \caption records the number under a following \label, and an
// optional [placement] argument is consumed (not typeset).
func TestFigureTableCaption(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(
		`\begin{table}[ht]\caption{First}\label{t1}\end{table}` +
			`\begin{table}\caption{Second}\label{t2}\end{table}` +
			`\begin{figure}[h]\caption{Plot}\label{f1}\end{figure}`); err != nil {
		t.Fatal(err)
	}
	// Independent counters : two tables → 1,2 ; one figure → 1.
	for k, want := range map[string]string{"t1": "1", "t2": "2", "f1": "1"} {
		if got := e.labels[k]; got != want {
			t.Errorf("label %q = %q, want %q", k, got, want)
		}
	}
	// Caption text : "Table N:" / "Figure N:" prefix then the argument.
	// Spaces are glue, so mvlText drops them.
	txt := mvlText(e.mvl)
	for _, want := range []string{"Table1:First", "Table2:Second", "Figure1:Plot"} {
		if !strings.Contains(txt, want) {
			t.Errorf("caption text %q not found in %q", want, txt)
		}
	}
	// The [ht] / [h] placement options must be consumed, never typeset.
	if strings.Contains(txt, "ht") || strings.Contains(txt, "[") {
		t.Errorf("placement option leaked into output : %q", txt)
	}
}
