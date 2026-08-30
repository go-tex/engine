// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A nested math environment carries its OWN & and \\. \begin{bmatrix}…&…\\…
// \end{bmatrix} inside an align cell is one matrix, not four cells over two rows —
// but brace depth does not see it, a matrix being unbraced, so the collector cut
// every matrix into fragments and the maths layer refused each one:
//
//	texmath: \begin{bmatrix} without \end   " = \begin {bmatrix} \mathrm {mat}…"
//
// 622 of the 4172 formulas the 200-paper arXiv corpus drops are that fault.
func TestAlignKeepsNestedEnvironmentsWhole(t *testing.T) {
	for _, c := range []struct{ nom, body string }{
		{"bmatrix", `S &= \begin{bmatrix} a & b \\ c & d \end{bmatrix} \\ &= X`},
		{"cases", `f(x) &= \begin{cases} 1 & x>0 \\ 0 & x\le 0 \end{cases} \\ &= g(x)`},
		{"array imbriqué", `A &= \left(\begin{array}{cc} 1 & 2 \\ 3 & 4 \end{array}\right)`},
	} {
		e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}\begin{document}`+
			`\begin{align*}`+c.body+`\end{align*}\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: la couche maths a refusé %v", c.nom, e.mathDropped)
		}
	}
}

// The align's own \\ and & must still separate: the guard is nesting, not blindness.
func TestAlignStillSeparatesItsOwnRows(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}\begin{document}`+
		`\begin{align}a &= b \\ c &= d\end{align}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if got != "(1)(2)" {
		t.Errorf("les numéros d'équation sont %q, want %q (deux lignes numérotées)", got, "(1)(2)")
	}
}

// TeX counts IMPLICIT braces in align_state as well (tex.web §7492-7493): \bgroup
// opens a group as surely as {. The cell scanner is that counter, so a & inside
// \bgroup…\egroup belongs to the group, not to the alignment — the same protection
// a nested matrix gets from the \bgroup that \@array opens (latex.ltx:12101).
func TestAlignCountsImplicitBraces(t *testing.T) {
	e, err := NewDocument(Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	e.push(alignToks(`a &= \bgroup x & y\egroup \\ c &= d\end{align}`))
	rows := e.collectAlignBody("align")
	if len(rows) != 2 {
		t.Fatalf("%d ligne(s), want 2", len(rows))
	}
	if n := len(rows[0].cells); n != 2 {
		t.Errorf("la première ligne a %d cellules, want 2: le & protégé par \\bgroup en a coupé une de trop", n)
	}
}

// alignToks tokenises a cell scanner's input, with & as the alignment tab (category
// 4) that a real document's \begin{align} gives it — tokenizeTeX makes it an
// ordinary character, which no alignment would ever split on.
func alignToks(src string) []tok {
	var out []tok
	rs := []rune(src)
	for i := 0; i < len(rs); i++ {
		switch c := rs[i]; {
		case c == '\\':
			j := i + 1
			for j < len(rs) && (rs[j] >= 'a' && rs[j] <= 'z' || rs[j] >= 'A' && rs[j] <= 'Z') {
				j++
			}
			if j == i+1 { // a control symbol, \\ among them
				j++
			}
			out = append(out, csTok(string(rs[i+1:j])))
			i = j - 1
		case c == '&':
			out = append(out, chTok('&', catAlign))
		case c == '{':
			out = append(out, chTok('{', catBegin))
		case c == '}':
			out = append(out, chTok('}', catEnd))
		case c == ' ':
			out = append(out, chTok(' ', catSpace))
		default:
			out = append(out, chTok(c, catOther))
		}
	}
	return out
}
