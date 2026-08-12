// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// Math commands that go-tex/math v0.4.0 added — in-math font switches, named
// operators, \operatorname, modular, and long arrows — now render instead of the
// engine dropping the equation (SkippedCommands stays empty for these).
func TestMathCommandsRender(t *testing.T) {
	cases := []string{
		`$x^{\rm th}$`,                     // in-math \rm font switch
		`${\bf v}\cdot w$`,                 // in-math \bf
		`$\operatorname{argmax}$`,          // \operatorname
		`$\operatorname*{argmax}_i$`,       // starred, with a limit
		`$a \bmod b$`,                      // \bmod
		`$c \pmod n$`,                      // \pmod → (mod n)
		`$A \Longrightarrow B$`,            // long arrow
		`$\log x + \sin y + \lim_{n} a_n$`, // named operators
	}
	for _, src := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.lenient = true // so a still-unknown command would be recorded, not fatal
		if _, err := e.Run(src); err != nil {
			t.Errorf("%s: unexpected error %v", src, err)
			continue
		}
		if len(e.SkippedCommands()) != 0 {
			t.Errorf("%s: expected to render, but these were dropped: %v", src, e.SkippedCommands())
		}
	}
}
