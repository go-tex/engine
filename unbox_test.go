// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// flatChars walks nested boxes as well as the top level, which is how a file
// that accumulates material a piece at a time ends up storing it.
func flatChars(n node) string {
	switch v := n.(type) {
	case charNode:
		return string(v.ch)
	case *boxNode:
		var sb strings.Builder
		for _, c := range v.list {
			sb.WriteString(flatChars(c))
		}
		return sb.String()
	}
	return ""
}

func boxContents(b *boxNode) string {
	if b == nil {
		return ""
	}
	var sb strings.Builder
	for _, n := range b.list {
		sb.WriteString(flatChars(n))
	}
	return sb.String()
}

// \unhbox unpacks a register onto the list being built instead of placing it as
// one item, and voids the register; \unhcopy leaves the register alone. That is
// how a file appends to a box without nesting a box inside a box —
// \setbox0=\hbox{\unhbox0 <more>} — and pgf builds a path's nodes exactly that
// way. While these primitives consumed the register number and dropped the
// contents, every node on a path but the last one disappeared.
func TestUnboxUnpacksARegister(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"accumulation sur soi", `\setbox0=\hbox{A}\setbox0=\hbox{\unhbox0 B}\setbox1=\hbox{\unhbox0}`, "AB"},
		{"trois fois", `\setbox0=\hbox{A}\setbox0=\hbox{\unhbox0 B}\setbox0=\hbox{\unhbox0 C}\setbox1=\hbox{\unhbox0}`, "ABC"},
		{"le registre est vidé", `\setbox0=\hbox{A}\setbox2=\hbox{\unhbox0}\setbox1=\hbox{\unhbox0 Z}`, "Z"},
		{"\\unhcopy ne vide pas", `\setbox0=\hbox{A}\setbox1=\hbox{\unhcopy0 \unhcopy0}`, "AA"},
		{"registre vide : rien", `\setbox1=\hbox{\unhbox3 X}`, "X"},
		{"une vbox ne s'ouvre pas avec \\unhbox", `\setbox0=\vbox{A}\setbox1=\hbox{\unhbox0 X}`, "X"},
		{"contenu imbriqué conservé", `\setbox0=\hbox{\hbox{A}}\setbox1=\hbox{\unhbox0 B}`, "AB"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.SetFont(spMock{})
			if _, err := e.Run(c.src); err != nil {
				t.Fatal(err)
			}
			if got := boxContents(e.box[1]); got != c.want {
				t.Errorf("%s\n  obtenu %q, attendu %q", c.src, got, c.want)
			}
		})
	}
}

// \unvbox is the vertical pair, and refuses an hbox the same way round.
func TestUnvboxUnpacksAVerticalRegister(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"accumulation verticale", `\setbox0=\vbox{A}\setbox0=\vbox{\unvbox0 B}\setbox1=\vbox{\unvbox0}`, "AB"},
		{"\\unvcopy ne vide pas", `\setbox0=\vbox{A}\setbox1=\vbox{\unvcopy0 \unvcopy0}`, "AA"},
		{"une hbox ne s'ouvre pas avec \\unvbox", `\setbox0=\hbox{A}\setbox1=\vbox{\unvbox0 X}`, "X"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.SetFont(spMock{})
			if _, err := e.Run(c.src); err != nil {
				t.Fatal(err)
			}
			if got := boxContents(e.box[1]); got != c.want {
				t.Errorf("%s\n  obtenu %q, attendu %q", c.src, got, c.want)
			}
		})
	}
}
