// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// geometry's twocolumn key sets the class's \@twocolumn switch
// (geometry.sty:576-579). sn-jnl's iicol option loads geometry with it
// (sn-jnl.cls:238-249) and \maketitle then branches on \if@twocolumn
// (sn-jnl.cls:803-807) to run \twocolumn[\@maketitle]; with the key ignored the
// switch stayed false and 68 corpus papers were set in one column.
func TestGeometryTwocolumnKeySetsTheSwitch(t *testing.T) {
	probe := `\makeatletter\hsize=1pt \if@twocolumn\hsize=100pt \fi`
	for _, c := range []struct {
		name, pre string
		want      string
	}{
		{"flag", `\usepackage[twocolumn]{geometry}`, "100pt"},
		{"true", `\usepackage[twocolumn=true]{geometry}`, "100pt"},
		{"false", `\usepackage[twocolumn,twocolumn=false]{geometry}`, "1pt"},
		{"onecolumn", `\usepackage[twocolumn,onecolumn]{geometry}`, "1pt"},
		{"absent", `\usepackage{geometry}`, "1pt"},
	} {
		e := runGeom(t, `\documentclass{article}`+c.pre+probe)
		if want := texSP(t, c.want); e.hsize != want {
			t.Errorf("%s: hsize = %d, want %d (\\if@twocolumn read %s)", c.name, e.hsize, want, c.want)
		}
	}
}
