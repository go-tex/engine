// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A box register is scoped like any other quantity: \setbox inside a group is
// restored when the group closes, \global\setbox is not. But the voiding that
// comes from READING a register with \box or \unhbox is permanent — the box is
// used up, not borrowed. All four measured against a real TeX (tectonic):
//
//	\setbox0=\hbox{AAAA}{\setbox0=\hbox{B}}           → box 0 is still AAAA
//	\setbox1=\hbox{AAAA}{\global\setbox1=\hbox{B}}    → box 1 is B
//	\setbox2=\hbox{AAAA}{\setbox3=\box2 }             → box 2 is VOID
//	\setbox4=\hbox{AAAA}{\setbox5=\hbox{\unhbox4}}    → box 4 is VOID
//
// Registers used to be unscoped here, which is what lost every tikz node but the
// last: a node voids \tikz@figbox inside the group that collects its text, and
// the nodes already accumulated there have to come back when that group closes.
func TestBoxRegistersAreScoped(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{AAAA}{\setbox0=\hbox{B}}` +
		`\setbox1=\hbox{AAAA}{\global\setbox1=\hbox{B}}` +
		`\setbox2=\hbox{AAAA}{\setbox3=\box2 }` +
		`\setbox4=\hbox{AAAA}{\setbox5=\hbox{\unhbox4}}`); err != nil {
		t.Fatal(err)
	}
	if got := boxContents(e.box[0]); got != "AAAA" {
		t.Errorf("\\setbox dans un groupe : box 0 = %q, attendu \"AAAA\" (l'affectation locale doit être restaurée)", got)
	}
	if got := boxContents(e.box[1]); got != "B" {
		t.Errorf("\\global\\setbox : box 1 = %q, attendu \"B\" (l'affectation globale doit survivre)", got)
	}
	if e.box[2] != nil {
		t.Error("\\box lu dans un groupe : box 2 devrait rester vide après le groupe")
	}
	if e.box[4] != nil {
		t.Error("\\unhbox dans un groupe : box 4 devrait rester vide après le groupe")
	}
}

// Nesting: the value restored is the one in force one level out, not the one
// from before every group.
func TestBoxScopeRestoresOneLevel(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{A}` +
		`{\setbox0=\hbox{B}{\setbox0=\hbox{C}}\global\setbox1=\copy0 }` +
		`\global\setbox2=\copy0 `); err != nil {
		t.Fatal(err)
	}
	if got := boxContents(e.box[1]); got != "B" {
		t.Errorf("au niveau intérieur, box 0 = %q, attendu \"B\"", got)
	}
	if got := boxContents(e.box[2]); got != "A" {
		t.Errorf("de retour au sommet, box 0 = %q, attendu \"A\"", got)
	}
}

// \sbox and \savebox are \setbox in LaTeX clothing and are scoped with it.
func TestSboxIsScoped(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\newsavebox{\zz}\sbox{\zz}{A}{\sbox{\zz}{B}}\global\setbox1=\copy\zz`); err != nil {
		t.Fatal(err)
	}
	if got := boxContents(e.box[1]); got != "A" {
		t.Errorf("\\sbox dans un groupe : %q, attendu \"A\"", got)
	}
}

// A register number outside the range the engine keeps is accepted and dropped
// rather than allowed to index past the end — the same on the scoped path, the
// global one and a destructive read.
func TestBoxRegisterOutOfRange(t *testing.T) {
	for _, src := range []string{
		`\setbox300=\hbox{A}`,
		`\global\setbox300=\hbox{A}`,
		`\setbox-1=\hbox{A}`,
		`\setbox0=\box300 `,
		`\setbox0=\hbox{\unhbox300 X}`,
	} {
		e := New()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Errorf("%s : %v", src, err)
		}
	}
}
