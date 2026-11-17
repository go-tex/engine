// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ ZERO IS A VALUE A DOCUMENT CAN MEAN.
//
// floatSep, textFloatSep and dblTextFloatSep each used to read their register
// behind `if s.width > 0`, falling back to LaTeX's 12pt / 20pt / 20pt. That is a
// test for "did anyone set this" written as a test for "is this non-zero", and
// the two differ on exactly the value a document is most likely to write on
// purpose: \textfloatsep=0pt to butt a float against the text.
//
// ⚠ MEASURED against tectonic, which is how it was found. article at 11pt, a [t]
// float, the gap from the caption's baseline to the first text baseline below it:
//
//	\textfloatsep default   reference 33.13pt   ours 35.72pt
//	\textfloatsep=0pt       reference 13.21pt   ours 35.72pt   ← unchanged
//
// The reference dropped the 19.92pt it was told to drop and we did not move at
// all. After the fix our first text baseline moves 19.98pt, and the default case
// is where it was.
//
// The defaults now live in classkernel.go, where classes.dtx:1440-1442 puts them,
// so these readers have nothing to fall back to: with no class they are LaTeX's
// values, a class overwrites them, and zero is obeyed.
//
// ⚠ AND NOTHING IN THIS SUITE CAUGHT IT. The whole suite was green with the guard
// and is green without it — these three registers had no witness for the one
// value that distinguished the two readings.
func TestAFloatSkipSetToZeroIsHonoured(t *testing.T) {
	for _, c := range []struct {
		name string
		reg  string
		read func(*Engine) int
		def  int // LaTeX's default, which the kernel must now carry
	}{
		{"floatsep", "floatsep", (*Engine).floatSep, 12 * unity},
		{"textfloatsep", "textfloatsep", (*Engine).textFloatSep, 20 * unity},
		{"dbltextfloatsep", "dbltextfloatsep", (*Engine).dblTextFloatSep, 20 * unity},
	} {
		t.Run(c.name, func(t *testing.T) {
			// ⛔ THE DEFAULT FIRST, because a reader that always returned the
			// register would also pass the zero row below while silently giving
			// every class-less document 0pt instead of LaTeX's value. The kernel
			// carrying the default is half of this change and needs its own row.
			e := New()
			if err := e.LoadLaTeX(); err != nil {
				t.Fatal(err)
			}
			if got := c.read(e); got != c.def {
				t.Errorf("with no class loaded, \\%s = %d, want LaTeX's %d "+
					"(classes.dtx:1440-1442) — the kernel is not carrying the default",
					c.reg, got, c.def)
			}

			// A document's own value is read.
			e2 := New()
			if err := e2.LoadLaTeX(); err != nil {
				t.Fatal(err)
			}
			if _, err := e2.Run(`\` + c.reg + `=33pt`); err != nil {
				t.Fatal(err)
			}
			if got := c.read(e2); got != 33*unity {
				t.Errorf("\\%s=33pt reads back %d, want %d", c.reg, got, 33*unity)
			}

			// ⛔ AND ZERO. This is the row the old guard failed: 0pt made s.width
			// zero, the guard said "nobody set it", and the document's own value
			// was replaced by the default it had just overridden.
			e3 := New()
			if err := e3.LoadLaTeX(); err != nil {
				t.Fatal(err)
			}
			if _, err := e3.Run(`\` + c.reg + `=0pt`); err != nil {
				t.Fatal(err)
			}
			if got := c.read(e3); got != 0 {
				t.Errorf("\\%s=0pt reads back %d, want 0 — a document that butts a "+
					"float against the text is handed %dpt of glue it asked to remove",
					c.reg, got, got/unity)
			}
		})
	}
}
