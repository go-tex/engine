// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// The engine's own dimension parameters — \hsize, \vsize, \parindent,
// \baselineskip — are parameters rather than registers, so the register save path
// never covered them and every assignment to one was permanent. TeX restores them
// at the end of the group that made the assignment, like any other parameter; only
// \global outlives the group.
//
// The consequence was not subtle. A package that narrows the measure inside a box
// left that width in force for the rest of the document: beamer writes its headline
// and footline as \hbox to\@tempdima{\textwidth=\@tempdima …}, so one template ran
// with a scratch dimension and every following page inherited it.
//
// Checked against real TeX: \hsize=100pt {\hsize=50pt}\showthe\hsize prints 100.0pt.

func runScope(t *testing.T, src string) *Engine {
	t.Helper()
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(src); err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return e
}

func TestEngineDimenParamsRestoredAtGroupEnd(t *testing.T) {
	e := runScope(t, `\hsize=100pt \vsize=300pt \parindent=7pt \baselineskip=13pt `+
		`{\hsize=50pt \vsize=70pt \parindent=1pt \baselineskip=2pt }`)
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{`\hsize`, e.hsize, 100 * unity},
		{`\vsize`, e.vsize, 300 * unity},
		{`\parindent`, e.parindent, 7 * unity},
		{`\baselineskip`, e.baselineskip, 13 * unity},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d after the group, want %d", c.name, c.got, c.want)
		}
	}
}

func TestEngineDimenParamsGlobalSurvivesGroup(t *testing.T) {
	e := runScope(t, `\hsize=100pt \vsize=300pt {\global\hsize=50pt \global\vsize=70pt }`)
	if e.hsize != 50*unity {
		t.Errorf(`\global\hsize = %d after the group, want %d`, e.hsize, 50*unity)
	}
	if e.vsize != 70*unity {
		t.Errorf(`\global\vsize = %d after the group, want %d`, e.vsize, 70*unity)
	}
}

func TestEngineDimenParamsGlobalInsideNestedGroups(t *testing.T) {
	// The global assignment has to drop the outer group's saved value too, or the
	// outer group puts the old one back on the way out.
	e := runScope(t, `\hsize=100pt {\hsize=80pt {\global\hsize=50pt }}`)
	if e.hsize != 50*unity {
		t.Errorf(`\hsize = %d, want %d`, e.hsize, 50*unity)
	}
}

func TestEngineDimenParamsAdvanceIsScoped(t *testing.T) {
	e := runScope(t, `\hsize=100pt {\advance\hsize by 30pt }`)
	if e.hsize != 100*unity {
		t.Errorf(`\advance\hsize inside a group leaked: %d, want %d`, e.hsize, 100*unity)
	}
	e = runScope(t, `\hsize=100pt {\global\advance\hsize by 30pt }`)
	if e.hsize != 130*unity {
		t.Errorf(`\global\advance\hsize = %d, want %d`, e.hsize, 130*unity)
	}
}

func TestEngineDimenParamsMultiplyDivideAreScoped(t *testing.T) {
	e := runScope(t, `\hsize=100pt {\multiply\hsize by 3 }\vsize=90pt {\divide\vsize by 3 }`)
	if e.hsize != 100*unity {
		t.Errorf(`\multiply\hsize leaked: %d, want %d`, e.hsize, 100*unity)
	}
	if e.vsize != 90*unity {
		t.Errorf(`\divide\vsize leaked: %d, want %d`, e.vsize, 90*unity)
	}
}

func TestEngineDimenParamsRestoredThroughBegingroup(t *testing.T) {
	e := runScope(t, `\parindent=7pt \begingroup\parindent=0pt \endgroup`)
	if e.parindent != 7*unity {
		t.Errorf(`\parindent = %d, want %d`, e.parindent, 7*unity)
	}
}

func TestEngineDimenParamsUnscopedAtTopLevel(t *testing.T) {
	// With no group open there is nothing to restore to, and nothing is recorded.
	e := runScope(t, `\hsize=100pt \hsize=42pt `)
	if e.hsize != 42*unity {
		t.Errorf(`\hsize = %d, want %d`, e.hsize, 42*unity)
	}
	if len(e.save) != 0 {
		t.Errorf("save stack = %d entries at top level, want 0", len(e.save))
	}
}

func TestSetlengthOnEngineDimenIsScoped(t *testing.T) {
	// \setlength reaches the same parameters and must be scoped the same way:
	// \setlength\textwidth{...} inside a box must not resize the rest of the
	// document. (\textwidth is \let to \hsize, \textheight to \vsize.)
	e := runScope(t, `\hsize=100pt {\setlength\textwidth{40pt}}`)
	if e.hsize != 100*unity {
		t.Errorf(`\setlength\textwidth leaked out of the group: %d, want %d`, e.hsize, 100*unity)
	}
	e = runScope(t, `\vsize=300pt {\addtolength\textheight{-50pt}}`)
	if e.vsize != 300*unity {
		t.Errorf(`\addtolength\textheight leaked: %d, want %d`, e.vsize, 300*unity)
	}
	// NOT asserted here: \global\setlength. The engine's \global reads ONE token and
	// dispatches on it, so a prefix in front of a MACRO is dropped rather than kept
	// pending across the expansion, as TeX keeps it. That is a separate gap in the
	// prefix machinery, older than this scoping fix and untouched by it.
}
