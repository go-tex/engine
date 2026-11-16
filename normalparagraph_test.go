// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ A PARAGRAPH SHAPE LASTS ONE PARAGRAPH, and tex.web says so in a procedure
// of four lines (:20756-20761):
//
//	procedure normal_paragraph;
//	begin if looseness<>0 then eq_word_define(int_base+looseness_code,0);
//	if hang_indent<>0 then eq_word_define(dimen_base+hang_indent_code,0);
//	if hang_after<>1 then eq_word_define(int_base+hang_after_code,1);
//	if par_shape_ptr<>null then eq_define(par_shape_loc,shape_ref,null);
//	end;
//
// called from nine places, `vmode+par_end` among them (:21152) — that is every
// \par. So `\hangindent=20pt` followed by two paragraphs hangs the FIRST and
// leaves the second alone.
//
// ⚠ THIS BECAME A DEFECT THE MOMENT THE PARAMETER STARTED WORKING. While
// \hangindent was accept-and-ignore, never resetting it cost nothing and no test
// could have noticed. #795 made it shape the line breaker, and the same omission
// now hangs every following paragraph — a latent bug that one change turned
// load-bearing, which is why the reset lands beside the feature rather than
// after it.
//
// ⚠ Most LaTeX uses survive without this, because an assignment inside a group
// is restored by the save stack and \@hangfrom and friends are grouped. What is
// NOT survivable is the top-level assignment this test makes.
func TestAShapeLastsOneParagraph(t *testing.T) {
	lineText := func(t *testing.T, src string) [][]string {
		t.Helper()
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("buildEngine: %v", err)
		}
		if _, err := e.Run(src); err != nil {
			t.Fatalf("Run: %v", err)
		}
		var lines []string
		for _, n := range e.mvl {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			var s []rune
			for _, c := range b.list {
				if ch, ok := c.(charNode); ok {
					s = append(s, ch.ch)
				}
			}
			if len(s) > 0 {
				lines = append(lines, string(s))
			}
		}
		return [][]string{lines}
	}

	// The same second paragraph, once after a hung first paragraph and once
	// after an ordinary one. \hangindent is set ONCE, at the top level.
	const second = `Typography affects every line of every paper, and a measure that is wrong by a centimetre is wrong on every line it sets.\par`
	hung := lineText(t, `\documentclass{article}\begin{document}\hsize=120pt \parindent=0pt `+
		`\hangindent=24pt \hangafter=1 First paragraph, which asks for the shape.\par `+
		second+`\end{document}`)
	plain := lineText(t, `\documentclass{article}\begin{document}\hsize=120pt \parindent=0pt `+
		`First paragraph, which asks for nothing.\par `+
		second+`\end{document}`)

	// Compare the LAST lines of each run: those belong to the second paragraph,
	// which asked for no shape in either document.
	tailOf := func(ls []string, n int) []string {
		if len(ls) < n {
			return ls
		}
		return ls[len(ls)-n:]
	}
	a, b := tailOf(hung[0], 3), tailOf(plain[0], 3)
	t.Logf("after a hung paragraph: %q", a)
	t.Logf("after a plain one:      %q", b)
	if len(hung[0]) < 4 || len(plain[0]) < 4 {
		t.Fatalf("the witness needs both paragraphs to run to several lines; got %d and %d lines",
			len(hung[0]), len(plain[0]))
	}
	if !sameStrings(a, b) {
		t.Errorf("the second paragraph is set differently depending on whether the FIRST "+
			"asked for a hanging indent:\n after hung  = %q\n after plain = %q\n"+
			"tex.web's normal_paragraph (:20756-20761) zeroes \\hangindent at every \\par, "+
			"so a shape lasts exactly one paragraph", a, b)
	}
}

// And the parameters themselves must read back at TeX's defaults once the
// paragraph that used them is over, since a document may ask.
func TestTheShapeParametersAreBackAtTheirDefaultsAfterAPar(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	var indent, after, loose int
	var probed bool
	e.prim("gotex@probeafterpar", func(e *Engine) {
		probed = true
		after, loose = e.namedInt("hangafter"), e.namedInt("looseness")
		if e.hangIndentReg >= 0 && e.hangIndentReg < len(e.dimen) {
			indent = e.dimen[e.hangIndentReg]
		}
	})
	src := `\documentclass{article}\begin{document}\makeatletter\hsize=120pt ` +
		`\hangindent=24pt \hangafter=3 \looseness=2 Some text.\par ` +
		`\gotex@probeafterpar\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if !probed {
		t.Fatal("the probe never ran")
	}
	if indent != 0 {
		t.Errorf("\\hangindent reads %d sp after \\par, want 0 (tex.web:20758)", indent)
	}
	if after != 1 {
		t.Errorf("\\hangafter reads %d after \\par, want 1 (tex.web:20759)", after)
	}
	if loose != 0 {
		t.Errorf("\\looseness reads %d after \\par, want 0 (tex.web:20757)", loose)
	}
}
