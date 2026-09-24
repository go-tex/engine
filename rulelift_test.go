// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// The lift of \rule[lift]{width}{height} moves the rule ACROSS the baseline; it
// does not make it taller. latex.ltx:11900-11907 says which dimension takes it:
//
//	\def\@rule[#1]#2#3{%
//	  \leavevmode
//	  \hbox{%
//	    \setlength\@tempdima{#1}%   lift
//	    \setlength\@tempdimb{#2}%   width
//	    \setlength\@tempdimc{#3}%   height
//	    \advance\@tempdimc\@tempdima
//	    \vrule\@width\@tempdimb\@height\@tempdimc\@depth-\@tempdima}}
//
// height = #3 + lift, depth = -lift. Both signs were the wrong way round, which
// for a NEGATIVE lift — the only kind that does anything visible — added the lift
// to the height and asked for a negative depth the packer clamped to zero. A
// corpus paper writes \rule[-\textheight/2]{1ex}{\textheight}, meant to straddle
// the baseline by half a page each way; it packed one and a half pages ABOVE it.
func TestRuleLiftMatchesTectonic(t *testing.T) {
	for _, c := range []struct {
		src                   string
		wantHeight, wantDepth int // in pt
	}{
		// The reference case: 6pt of rule, 2pt of it below the baseline.
		{`\rule[-2pt]{3pt}{6pt}`, 4, 2},
		// A raised rule: LaTeX asks for a NEGATIVE depth here, and says so.
		{`\rule[2pt]{3pt}{6pt}`, 8, -2},
		// No lift is the common form and must be untouched.
		{`\rule[0pt]{3pt}{6pt}`, 6, 0},
		{`\rule{3pt}{6pt}`, 6, 0},
	} {
		r := runRule(t, c.src)
		if got, want := r.height, c.wantHeight*unity; got != want {
			t.Errorf("%s: height = %d sp, want %d (%dpt)", c.src, got, want, c.wantHeight)
		}
		if got, want := r.depth, c.wantDepth*unity; got != want {
			t.Errorf("%s: depth = %d sp, want %d (%dpt)", c.src, got, want, c.wantDepth)
		}
		// Whatever the lift, the rule keeps the height it was asked for.
		if got, want := r.height+r.depth, 6*unity; got != want {
			t.Errorf("%s: height+depth = %d sp, want %d (the rule is 6pt tall)", c.src, got, want)
		}
	}
}

// runRule runs src and returns the first ruleNode it places.
func runRule(t *testing.T, src string) ruleNode {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	var find func([]node) (ruleNode, bool)
	find = func(ns []node) (ruleNode, bool) {
		for _, n := range ns {
			switch c := n.(type) {
			case ruleNode:
				return c, true
			case *boxNode:
				if r, ok := find(c.list); ok {
					return r, true
				}
			}
		}
		return ruleNode{}, false
	}
	r, ok := find(e.mvl)
	if !ok {
		t.Fatalf("%q: no ruleNode placed", src)
	}
	return r
}
