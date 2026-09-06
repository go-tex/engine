// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// lineno's \modulolinenumbers takes an OPTIONAL [n]. Declared with a mandatory #1
// the stub grabbed the "[" as its argument and left "5]" in the input, which was
// then TYPESET — in the preamble, where it opened a page of its own.
func TestModulolinenumbersEatsItsOptionalArgument(t *testing.T) {
	for _, src := range []string{
		`\modulolinenumbers[5]X\par`,
		`\modulolinenumbers{5}X\par`, // a real paper writes the braced form too
		`\modulolinenumbers X\par`,
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Fatalf("Run(%q): %v", src, err)
		}
		if txt := mvlText(e.mvl); txt != "X" {
			t.Errorf("%s left %q on the page, want X alone", src, txt)
		}
	}
}

// The bare form must not swallow what follows it: \gotex@maybegroup eats a {..}
// only when one is really there.
func TestModulolinenumbersDoesNotSwallowTheNextGroup(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\modulolinenumbers AVANT APRES\par`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "AVANT") || !strings.Contains(txt, "APRES") {
		t.Errorf("text after the switch was swallowed: %q", txt)
	}
}
