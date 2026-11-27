// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

func colourEngine(t *testing.T, src string) *Engine {
	t.Helper()
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return e
}

// findRules collects every rule in a list, through the boxes that hold them.
func findRules(list []node) []ruleNode {
	var out []ruleNode
	for _, n := range list {
		switch c := n.(type) {
		case ruleNode:
			out = append(out, c)
		case *boxNode:
			if c != nil {
				out = append(out, findRules(c.list)...)
			}
		}
	}
	return out
}

// A rule is painted in the current colour. beamer's background canvas is
// \color{bg}\vrule width\paperwidth height\paperheight, and every beamer page came
// out black.
func TestARuleTakesTheCurrentColour(t *testing.T) {
	e := colourEngine(t, `\hbox{{\color{red}\vrule width 2pt height 2pt}\vrule width 3pt height 2pt}`)
	rules := findRules(e.mvl)
	if len(rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(rules))
	}
	if rules[0].color != 0xFF0000 || rules[1].color != 0 {
		t.Errorf("rule colours = %06X, %06X; want FF0000 then the default", rules[0].color, rules[1].color)
	}
	var sb strings.Builder
	ruleRect(&sb, 0, 0, 2, 2, rules[0].color)
	if !strings.Contains(sb.String(), `fill="#FF0000"`) && !strings.Contains(strings.ToLower(sb.String()), `fill="#ff0000"`) {
		t.Errorf("SVG rule = %q, want a red fill", sb.String())
	}
}

// xcolor expands a colour expression before it parses it; beamer names every
// colour through a macro (\colorlet{temp@fg}{\csname beamer@thc@fg\endcsname}).
func TestColorletExpandsItsExpression(t *testing.T) {
	e := colourEngine(t, `\def\foo{blue}\def\bar{red}\colorlet{b}{\foo}`+
		`\colorlet{c}{\csname foo\endcsname}\colorlet{d}{\bar!50!\foo}{\color{\foo}X}`)
	for name, want := range map[string]uint32{"b": 0x0000FF, "c": 0x0000FF, "d": 0x800080} {
		if got := e.colors[name]; got != want {
			t.Errorf("%s = %06X, want %06X", name, got, want)
		}
	}
	for _, c := range glyphColors(e.mvl) {
		if c != 0x0000FF {
			t.Errorf(`\color{\foo} set %06X, want blue`, c)
		}
	}
}

// A named colour is local to its group unless \globalcolorstrue: beamer's
// \usebeamercolor redefines fg and bg inside the box it colours.
func TestColorletIsLocalUnlessGlobalcolors(t *testing.T) {
	e := colourEngine(t, `\colorlet{fg}{black}`+
		`{\colorlet{fg}{red}\colorlet{fresh}{red}}`+
		`{\globalcolorstrue\colorlet{kept}{blue}}`+
		`{\definecolor{inner}{rgb}{0,1,0}}`)
	if got := e.colors["fg"]; got != 0 {
		t.Errorf("fg after the group = %06X, want black again", got)
	}
	if _, ok := e.colors["fresh"]; ok {
		t.Error("a colour first named inside a group outlived it")
	}
	if _, ok := e.colors["inner"]; ok {
		t.Error(`\definecolor inside a group outlived it`)
	}
	if got := e.colors["kept"]; got != 0x0000FF {
		t.Errorf("kept = %06X, want blue: \\globalcolorstrue makes it global", got)
	}
}

// "." is xcolor's name for the current colour.
func TestDotIsTheCurrentColour(t *testing.T) {
	e := colourEngine(t, `\color{red}\colorlet{now}{.}`)
	if got := e.colors["now"]; got != 0xFF0000 {
		t.Errorf("now = %06X, want red", got)
	}
}

func glyphColors(list []node) []uint32 {
	var out []uint32
	for _, n := range list {
		switch c := n.(type) {
		case charNode:
			out = append(out, c.color)
		case *boxNode:
			if c != nil {
				out = append(out, glyphColors(c.list)...)
			}
		}
	}
	return out
}
