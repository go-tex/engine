// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// fontspec without \setmainfont sets Latin Modern, which is the face the engine sets:
// the warning "\usepackage{fontspec} wanted a font named by \setmainfont" was false
// for it (go-tex/engine#370). A \setmainfont naming another face is the real request,
// and the warning then carries that face's name.
func TestFontspecWarnsOnlyForAFaceItLacks(t *testing.T) {
	for _, c := range []struct {
		name, pre string
		want      string // "" = no warning
	}{
		{"fontspec alone", `\usepackage{fontspec}`, ""},
		{"Latin Modern named", `\usepackage{fontspec}\setmainfont{Latin Modern Roman}`, ""},
		{"Latin Modern with options", `\usepackage{fontspec}\setmainfont[Ligatures=TeX]{LM Roman 10}`, ""},
		{"another face", `\usepackage{fontspec}\setmainfont[Ligatures=TeX]{Linux Libertine O}`, "Linux Libertine O"},
	} {
		e, err := compile([]byte(`\documentclass{article}`+c.pre+`\begin{document}Hello.\end{document}`),
			Options{})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, warned := e.Diagnostics().FontsSubstituted["fontspec"]
		if c.want == "" && warned {
			t.Errorf("%s: warned that fontspec wanted %q; Latin Modern is what is set", c.name, got)
		}
		if c.want != "" && got != c.want {
			t.Errorf("%s: fontspec wanted %q, want %q", c.name, got, c.want)
		}
	}
}
