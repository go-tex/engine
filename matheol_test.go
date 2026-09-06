// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// An escape at the END OF A LINE is the empty control sequence in TeX (tex.web
// §354); this engine names it after the \endlinechar that follows. Emitted verbatim
// into go-tex/math source it became "\<CR>", the maths layer refused the command,
// and the engine dropped the WHOLE equation. A real appendix line ends
//
//	c_{T} = \left(...\right) \sum_{\{S_{T}\}} \alpha_{T} \
//	\quad \rightarrow \quad ...
//
// and that paper lost 15 formulas to it.
func TestEscapeAtEndOfLineDoesNotDropTheEquation(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run("AVANT $a + \\\nb$ APRES\\par"); err != nil {
		t.Fatal(err)
	}
	if n := e.Diagnostics().MathDropped; len(n) != 0 {
		t.Errorf("the equation was dropped whole: %v", n)
	}
	// The prose around it is untouched either way — the loss is invisible there,
	// which is what made it worth a diagnostic.
	if txt := glyphString(e.mvl); !strings.Contains(txt, "AVANT") {
		t.Errorf("the surrounding text was disturbed: %q", txt)
	}
}

// The encoder is shared, so the same holds for a display and for an equation body.
func TestEscapeAtEndOfLineInDisplay(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run("\\[ a + \\\nb \\]\\par"); err != nil {
		t.Fatal(err)
	}
	if n := e.Diagnostics().MathDropped; len(n) != 0 {
		t.Errorf("the display was dropped: %v", n)
	}
}

// A real control sequence is still encoded: this must not silence ordinary commands.
func TestOrdinaryCommandStillReachesTheMathLayer(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`$\alpha + \beta$\par`); err != nil {
		t.Fatal(err)
	}
	if n := len(e.mvl); n == 0 {
		t.Error("the formula produced nothing")
	}
}
