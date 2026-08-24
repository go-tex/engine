// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"
import "testing"

// boxChars is the text a box holds, which is where a typeset number lands.
func boxChars(b *boxNode) string {
	if b == nil {
		return ""
	}
	var sb strings.Builder
	for _, n := range b.list {
		if c, ok := n.(charNode); ok {
			sb.WriteRune(c.ch)
		}
	}
	return sb.String()
}

// The two grouping queries are read as internal integers, so every route to a
// number has to reach them: \the, \number and \ifnum alike. Each case is set
// inside a box one level deeper than the level under test, and the box's own
// level is accounted for in what it expects.
func TestCurrentGroupQueries(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"boîte seule", `\the\currentgrouplevel`, "1"},
		{"une accolade", `{\the\currentgrouplevel}`, "2"},
		{"deux accolades", `{{\the\currentgrouplevel}}`, "3"},
		{"refermée", `{}\the\currentgrouplevel`, "1"},
		{"begingroup", `\begingroup\the\currentgrouplevel\endgroup`, "2"},
		{"via \\number", `{\number\currentgrouplevel}`, "2"},
		{"via \\ifnum", `{\ifnum\currentgrouplevel>1 A\else B\fi}`, "A"},
		{"type: boîte", `\the\currentgrouptype`, "2"},
		{"type: accolade", `{\the\currentgrouptype}`, "1"},
		{"type: begingroup", `\begingroup\the\currentgrouptype\endgroup`, "14"},
		{"type après fermeture", `{}\the\currentgrouptype`, "2"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.SetFont(spMock{})
			e.Run(`\setbox0=\hbox{` + c.src + `}`)
			if got := boxChars(e.box[0]); got != c.want {
				t.Errorf("%s : obtenu %q, attendu %q", c.src, got, c.want)
			}
		})
	}
}

// At the outermost level, with no group open, both queries read zero.
func TestCurrentGroupAtBottom(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.Run(`\count0=\currentgrouplevel \count1=\currentgrouptype`)
	if e.count[0] != 0 || e.count[1] != 0 {
		t.Errorf("au sommet : niveau=%d type=%d, attendu 0 et 0", e.count[0], e.count[1])
	}
}

// Written where no number is being read, each query contributes its digits, so
// a file can put the level straight into a message or into the page.
func TestCurrentGroupWrittenDirectly(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"le niveau, écrit tel quel", `\setbox0=\hbox{{\currentgrouplevel}}`, "2"},
		{"le type, écrit tel quel", `\setbox0=\hbox{\begingroup\currentgrouptype\endgroup}`, "14"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.SetFont(spMock{})
			e.Run(c.src)
			if got := boxChars(e.box[0]); got != c.want {
				t.Errorf("%s : obtenu %q, attendu %q", c.src, got, c.want)
			}
		})
	}
}
