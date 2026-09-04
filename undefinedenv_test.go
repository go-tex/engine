// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// An undefined environment's body is typeset with ordinary category codes, which is
// right for a prose wrapper and ruinous for a code block: a lone $ opens math mode
// and the scan then runs past the environment's own \end, eating the rest of the
// document while it looks for the closing $.
func TestUndefinedCodeEnvironmentDoesNotSwallowTheDocument(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := "AVANT\n\\begin{jlcode}\npath = \"$write_dir/x\"\n\\end{jlcode}\nAPRES\\par"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "APRES") {
		t.Errorf("the text after the block was swallowed: %q", txt)
	}
	if !strings.Contains(strings.ReplaceAll(txt, " ", ""), `path="$write_dir/x"`) {
		t.Errorf("the code line did not survive verbatim: %q", txt)
	}
}

// A prose environment the engine does not know keeps the behaviour it had: its body
// is typeset as ordinary text, not as a code block.
func TestUndefinedProseEnvironmentIsStillProse(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{monenv}du texte $x+y$ ordinaire\end{monenv}APRES\par`); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "APRES") || !strings.Contains(txt, "ordinaire") {
		t.Errorf("a balanced body must be left alone: %q", txt)
	}
	// Set as prose, the maths is rendered, so its source is not in the text.
	if strings.Contains(txt, "$x+y$") {
		t.Errorf("the body was set verbatim rather than as prose: %q", txt)
	}
}
