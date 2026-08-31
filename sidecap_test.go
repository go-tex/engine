// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// sidecap's SCfigure/SCtable become plain figure/table floats: the caption is
// numbered ("Figure 1: …", "Table 1: …") and NONE of the two leading optional
// arguments ([relwidth][pos]) leak onto the page, for zero, one and two optionals.
func TestSCfigureFloatsAndDropsOptionals(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `Before ` +
		`\begin{SCfigure}[0.5][t]\caption{Fig cap}\end{SCfigure}` + // two optionals
		`\begin{SCfigure*}\caption{Star cap}\end{SCfigure*}` + // none
		`\begin{SCtable}[h]\caption{Tab cap}\end{SCtable}` + // one optional
		` After`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)

	// Numbered float captions (figures share one counter, tables another).
	for _, want := range []string{"Figure1:Figcap", "Figure2:Starcap", "Table1:Tabcap"} {
		if !strings.Contains(txt, want) {
			t.Errorf("SC float caption %q missing: %q", want, txt)
		}
	}
	// The optional arguments did not leak.
	for _, garbage := range []string{"0.5", "[t]", "[h]", "][", "0.5]"} {
		if strings.Contains(txt, garbage) {
			t.Errorf("SC float leaked optional %q: %q", garbage, txt)
		}
	}
	if !strings.Contains(txt, "Before") || !strings.Contains(txt, "After") {
		t.Errorf("surrounding prose lost: %q", txt)
	}
}
