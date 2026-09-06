// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// go-tex/math knows none of the physics package's commands, and an unknown command
// makes it refuse the WHOLE equation: one \grad costs the formula it stands in.
func TestPhysicsCommandsDoNotDropTheEquation(t *testing.T) {
	for _, src := range []string{
		`\usepackage{physics}$\grad p = 0$\par`,
		`\usepackage{physics}$\div \mathbf{u} = 0$\par`,
		`\usepackage{physics}$\vb{u}$\par`,
		`\usepackage{physics}$\norm{x}$\par`,
		`\usepackage{physics}$\pdv{u}{t}$\par`,
		`\usepackage{physics}$\pdv[2]{u}{r}$\par`,
		`\usepackage{physics}$\dv{e}{t}$\par`,
		`\usepackage{physics}$\laplacian u$\par`,
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Fatalf("Run(%q): %v", src, err)
		}
		if d := e.Diagnostics().MathDropped; len(d) != 0 {
			t.Errorf("%s dropped the equation: %v", src, d)
		}
	}
}

// Everything is gated on the document having ASKED for physics: \div is the
// division sign in plain TeX and the divergence only under physics, so an ungated
// table would corrupt every other document's formulas.
func TestPhysicsIsGatedOnThePackage(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`$a \div b$\par`); err != nil {
		t.Fatal(err)
	}
	// \div is a symbol go-tex/math knows, so this renders either way; what must NOT
	// happen is the substitution.
	if got, ok := e.resolvePhysics(`a \div b`, "div"); ok || got != `a \div b` {
		t.Errorf("physics substituted without the package: %q, ok=%v", got, ok)
	}
}
