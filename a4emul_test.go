// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// a4.sty is NTG's and may only be distributed with the NTG classes, so it is not
// bundled: the engine emulates it (latex.go, \gotex@afour). a4wide.sty (LPPL 1 or
// later) is bundled and loads a4 with \RequirePackage, so it runs on the same
// emulation.
//
// The expected strings below were produced by the ORIGINAL a4.sty v1.2g (and
// a4wide.sty 1994/08/30) run by this engine at the commit that still bundled
// them (origin/main before this change); the a4wide 10pt row also equals what
// tectonic 0.15 prints (424.58624pt, see a4wide_test.go). Each string is
// textwidth/textheight/oddsidemargin/evensidemargin/topmargin/marginparwidth.
func TestA4EmulationMatchesTheOriginalPackage(t *testing.T) {
	for _, c := range []struct{ name, pre, want string }{
		{"a4-10", `\documentclass[10pt]{article}\usepackage{a4}`, "361.34999pt/646.0pt/39.74872pt/39.74872pt/0.0pt/72.26999pt"},
		{"a4-11", `\documentclass[11pt]{article}\usepackage{a4}`, "375.80377pt/636.60028pt/32.52127pt/32.52127pt/0.0pt/72.26999pt"},
		{"a4-12", `\documentclass[12pt]{article}\usepackage{a4}`, "411.93877pt/621.0pt/14.45377pt/14.45377pt/0.0pt/57.81621pt"},
		{"a4-10-twoside", `\documentclass[10pt,twoside]{article}\usepackage{a4}`, "361.34999pt/646.0pt/39.74872pt/54.2025pt/0.0pt/72.26999pt"},
		{"a4-11-twoside", `\documentclass[11pt,twoside]{article}\usepackage{a4}`, "375.80377pt/636.60028pt/32.52127pt/46.97505pt/0.0pt/72.26999pt"},
		{"a4-12-twoside", `\documentclass[12pt,twoside]{article}\usepackage{a4}`, "411.93877pt/621.0pt/14.45377pt/28.90755pt/0.0pt/57.81621pt"},
		{"a4-10-wide", `\documentclass[10pt]{article}\usepackage[widemargins]{a4}`, "325.215pt/646.0pt/39.74872pt/39.74872pt/0.0pt/108.40498pt"},
		{"a4-11-wide", `\documentclass[11pt]{article}\usepackage[widemargins]{a4}`, "339.66878pt/636.60028pt/32.52127pt/32.52127pt/0.0pt/108.40498pt"},
		{"a4-12-wide", `\documentclass[12pt]{article}\usepackage[widemargins]{a4}`, "361.34999pt/621.0pt/14.45377pt/14.45377pt/0.0pt/108.40498pt"},
		{"a4-10-twoside-wide", `\documentclass[10pt,twoside]{article}\usepackage[widemargins]{a4}`, "325.215pt/646.0pt/39.74872pt/90.3375pt/0.0pt/108.40498pt"},
		{"a4-12-twoside-wide", `\documentclass[12pt,twoside]{article}\usepackage[widemargins]{a4}`, "361.34999pt/621.0pt/14.45377pt/79.49632pt/0.0pt/108.40498pt"},
		{"a4-10-twoside-wide-rev", `\documentclass[10pt,twoside]{article}\reversemarginpar\usepackage[widemargins]{a4}`, "325.215pt/646.0pt/90.3375pt/39.74872pt/0.0pt/108.40498pt"},
		{"a4-10-wide-rev", `\documentclass[10pt]{article}\reversemarginpar\usepackage[widemargins]{a4}`, "325.215pt/646.0pt/75.88371pt/75.88371pt/0.0pt/108.40498pt"},
		{"a4-10-cmd-wide", `\documentclass[10pt]{article}\usepackage{a4}\WideMargins`, "325.215pt/646.0pt/39.74872pt/39.74872pt/0.0pt/108.40498pt"},
		{"a4wide-10", `\documentclass[10pt]{article}\usepackage{a4wide}`, "424.58624pt/646.0pt/18.06749pt/18.06749pt/0.0pt/54.2025pt"},
		{"a4wide-11", `\documentclass[11pt]{article}\usepackage{a4wide}`, "442.65375pt/636.60028pt/9.03374pt/9.03374pt/0.0pt/54.2025pt"},
		{"a4wide-12", `\documentclass[12pt]{article}\usepackage{a4wide}`, "460.72124pt/621.0pt/0.0pt/0.0pt/0.0pt/54.2025pt"},
		{"a4wide-10-twoside", `\documentclass[10pt,twoside]{article}\usepackage{a4wide}`, "424.58624pt/646.0pt/18.06749pt/18.06749pt/0.0pt/54.2025pt"},
	} {
		e := runGeom(t, c.pre+`\edef\a@wit{\the\textwidth/\the\textheight/\the\oddsidemargin/\the\evensidemargin/\the\topmargin/\the\marginparwidth}\begin{document}`)
		if got := macroBody(e, "a@wit"); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
