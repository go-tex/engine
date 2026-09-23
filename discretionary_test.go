// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \discretionary{<pre>}{<post>}{<no-break>} (tex.web §1117) is a break OPPORTUNITY:
// TeX sets the THIRD list where no break is taken. It was undefined, so all three
// were typeset — the command is forgotten and the groups after it are ordinary
// material.
func TestDiscretionarySetsOnlyTheNoBreakText(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`mot\discretionary{PRE}{POST}{NOBREAK}suite\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	for _, leaked := range []string{"PRE", "POST"} {
		if strings.Contains(got, leaked) {
			t.Errorf("%s reached the page: %q", leaked, got)
		}
	}
	if !strings.Contains(got, "motNOBREAKsuite") {
		t.Errorf("page = %q, want motNOBREAKsuite", got)
	}
}

// Every corpus use is \discretionary{}{}{} — a .bbl letting a long DOI break without
// adding anything. Nothing may appear, and the break must NOT grow the hyphen a
// Liang break carries.
func TestDiscretionaryEmptyAddsNothing(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`A\discretionary{}{}{}B\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); got != "AB" {
		t.Errorf("page = %q, want AB alone", got)
	}
}

// discNode now carries its pre-break text instead of assuming a hyphen. Liang
// hyphenation must still set one: this pins that the change did not silently turn
// hyphenation off.
func TestLiangBreakStillCarriesItsHyphen(t *testing.T) {
	for _, c := range []struct {
		name string
		n    discNode
		want string
	}{
		{"Liang", discNode{penalty: 50, pre: "-"}, "-"},
		{"discretionary vide", discNode{penalty: 50}, ""},
	} {
		if c.n.pre != c.want {
			t.Errorf("%s: pre = %q, want %q", c.name, c.n.pre, c.want)
		}
	}
	// and the hyphenator itself still produces one
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=40pt \noindent hyphenation testing ` + strings.Repeat("difficulty ", 6) + `\par`); err != nil {
		t.Fatal(err)
	}
	var found bool
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case discNode:
				if c.pre == "-" {
					found = true
				}
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	if !found {
		t.Error("the hyphenator produced no break carrying a hyphen — Liang hyphenation is off")
	}
}
