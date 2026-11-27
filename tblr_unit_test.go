// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// tblrTestToks tokenises s the way the unit tests need: braces, spaces, others.
func tblrTestToks(s string) []tok {
	var out []tok
	for _, r := range s {
		switch r {
		case '{':
			out = append(out, chTok(r, catBegin))
		case '}':
			out = append(out, chTok(r, catEnd))
		case ' ':
			out = append(out, chTok(r, catSpace))
		default:
			out = append(out, chTok(r, catOther))
		}
	}
	return out
}

func tblrTestStr(ts []tok) string {
	var b strings.Builder
	for _, t := range ts {
		b.WriteRune(t.ch)
	}
	return b.String()
}

func TestTblrUnbrace(t *testing.T) {
	for in, want := range map[string]string{
		"{ab}":    "ab",
		" {a b} ": "a b",
		"{a}{b}":  "{a}{b}", // two groups, not one
		"ab":      "ab",
		"{a{b}}":  "a{b}",
		"{}":      "",
	} {
		if got := tblrTestStr(tblrUnbrace(tblrTestToks(in))); got != want {
			t.Errorf("tblrUnbrace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTblrGroupEnd(t *testing.T) {
	for _, c := range []struct {
		in   string
		from int
		want int
	}{{"{a{b}c}d", 0, 7}, {"{a}{b}", 0, 3}, {"{a}{b}", 3, 6}, {"{ab", 0, 3}} {
		if got := tblrGroupEnd(tblrTestToks(c.in), c.from); got != c.want {
			t.Errorf("tblrGroupEnd(%q, %d) = %d, want %d", c.in, c.from, got, c.want)
		}
	}
}

func TestParseTblrInner(t *testing.T) {
	bare := parseTblrInner(tblrTestToks("X[l]X[r]"))
	if got := tblrTestStr(bare.colspec); got != "X[l]X[r]" || bare.hlines {
		t.Errorf("bare colspec = %q hlines=%v", got, bare.hlines)
	}
	k := parseTblrInner(tblrTestToks("colspec = {|l|r|}, hlines, vlines, width = 5cm"))
	if got := tblrTestStr(k.colspec); got != "|l|r|" || !k.hlines || !k.vlines || tblrTestStr(k.width) != "5cm" {
		t.Errorf("keyed = %q %v %v %q", got, k.hlines, k.vlines, tblrTestStr(k.width))
	}
}

func TestPlaceSplitNilBox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.placeSplit(nil) // must not panic
}

// A rule between rows is wrapped with the skips too, or \centering would centre
// the rows and leave the rules at the left margin.
func TestLongtableRulesKeepCentering(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	body := ""
	for i := 0; i < 10; i++ {
		body += "r & x\\\\\n\\hline\n"
	}
	src := `\documentclass{article}
\usepackage{longtable}
\begin{document}
\centering
\begin{longtable}{|l|l|}
\hline
` + body + `\end{longtable}
\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	rules, wrapped := 0, 0
	for _, p := range e.Pages() {
		for _, top := range p.list {
			tb, ok := top.(*boxNode)
			if !ok {
				continue
			}
			for _, n := range tb.list {
				switch c := n.(type) {
				case ruleNode:
					rules++
				case *boxNode:
					if len(c.list) > 0 {
						if _, g := c.list[0].(glueNode); g {
							wrapped++
						}
					}
				}
			}
		}
	}
	if rules > 1 {
		t.Errorf("%d bare rules left unwrapped on a centred table", rules)
	}
	if wrapped < 15 {
		t.Errorf("wrapped = %d, want rows and rules both (about 21)", wrapped)
	}
}

// Without stretchy skips nothing is wrapped: the rows stay the boxes the table
// built, and the page is not littered with one extra hbox per row.
func TestLongtableRowsUnwrappedByDefault(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\documentclass{article}
\usepackage{longtable}
\begin{document}
\begin{longtable}{ll}
` + tblrRows(20) + `\end{longtable}
\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	for _, p := range e.Pages() {
		for _, top := range p.list {
			tb, ok := top.(*boxNode)
			if !ok {
				continue
			}
			for _, n := range tb.list {
				if b, ok := n.(*boxNode); ok && len(b.list) > 0 {
					if _, g := b.list[0].(glueNode); g {
						t.Fatalf("a row is wrapped in skips although they have no stretch")
					}
				}
			}
		}
	}
}

// Under \centering the skips have stretch, and every row after the first is
// wrapped with them one by one, as contribute wraps a lone box.
func TestLongtableRowsKeepCentering(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\documentclass{article}
\usepackage{longtable}
\begin{document}
\centering
\begin{longtable}{ll}
` + tblrRows(20) + `\end{longtable}
\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	wrapped := 0
	for _, p := range e.Pages() {
		for _, top := range p.list {
			tb, ok := top.(*boxNode)
			if !ok {
				continue
			}
			for _, n := range tb.list {
				if b, ok := n.(*boxNode); ok && len(b.list) > 0 {
					if _, g := b.list[0].(glueNode); g {
						wrapped++
					}
				}
			}
		}
	}
	if wrapped < 10 {
		t.Errorf("only %d rows carry the \\centering skips, want most of 20", wrapped)
	}
}
