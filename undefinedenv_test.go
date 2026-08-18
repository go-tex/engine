// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// An undefined environment is a silent gap: \begin{env} expands to
// \csname env\endcsname, and \csname turns a missing \env into \relax — so the
// body is typeset in whatever mode was current and no undefined COMMAND is ever
// tallied. Diagnostics().UndefinedEnvs must surface it, while a DEFINED
// environment (here itemize) must not appear. Without the \gotex@checkenv hook on
// \begin this reports zero undefined envs and so fails.
func TestDiagnosticsUndefinedEnvironment(t *testing.T) {
	src := []byte(`\documentclass{article}` +
		`\begin{document}` +
		`\begin{itemize}\item a\end{itemize}` + // DEFINED: must NOT be tallied
		`\begin{nosuchenv}body\end{nosuchenv}` + // UNDEFINED: must be tallied
		`\end{document}`)
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile errored: %v", err)
	}
	d := e.Diagnostics()
	if d.UndefinedEnvs["nosuchenv"] == 0 {
		t.Errorf("undefined environment nosuchenv not tallied; UndefinedEnvs=%v", d.UndefinedEnvs)
	}
	if _, ok := d.UndefinedEnvs["itemize"]; ok {
		t.Errorf("defined environment itemize wrongly tallied as undefined; UndefinedEnvs=%v", d.UndefinedEnvs)
	}
	if _, ok := d.UndefinedEnvs["document"]; ok {
		t.Errorf("document environment wrongly tallied as undefined; UndefinedEnvs=%v", d.UndefinedEnvs)
	}
}

// Two occurrences of the same undefined environment count twice.
func TestDiagnosticsUndefinedEnvironmentCounts(t *testing.T) {
	src := []byte(`\documentclass{article}` +
		`\begin{document}` +
		`\begin{foo}a\end{foo}\begin{foo}b\end{foo}\begin{bar}c\end{bar}` +
		`\end{document}`)
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile errored: %v", err)
	}
	d := e.Diagnostics()
	if got := d.UndefinedEnvs["foo"]; got != 2 {
		t.Errorf("foo tallied %d times, want 2; UndefinedEnvs=%v", got, d.UndefinedEnvs)
	}
	if got := d.UndefinedEnvs["bar"]; got != 1 {
		t.Errorf("bar tallied %d times, want 1; UndefinedEnvs=%v", got, d.UndefinedEnvs)
	}
}

// A normal document that uses only DEFINED environments (and a user's own
// \newenvironment) reports zero undefined envs — no false positives.
func TestDiagnosticsNoUndefinedEnvironments(t *testing.T) {
	src := []byte(`\documentclass{article}` +
		`\newenvironment{mybox}{\par}{\par}` +
		`\begin{document}` +
		`\begin{itemize}\item a\item b\end{itemize}` +
		`\begin{center}centered\end{center}` +
		`\begin{quote}quoted\end{quote}` +
		`\begin{mybox}custom\end{mybox}` +
		`\end{document}`)
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile errored: %v", err)
	}
	if d := e.Diagnostics(); len(d.UndefinedEnvs) != 0 {
		t.Errorf("defined-only document reported undefined envs: %v", d.UndefinedEnvs)
	}
}
