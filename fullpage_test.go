// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// fullpage (H. Partl / P. W. Daly) is a one-line geometry: the text block is the
// paper less a uniform margin on every side, 1in by default. Unemulated, a document
// kept its class margins — a book set 38 lines a page where its reference sets 44.
func TestFullpageSetsTheTextBlock(t *testing.T) {
	e := runGeom(t, `\usepackage{fullpage}`)
	if e.geom == nil {
		t.Fatal("fullpage left no geometry state")
	}
	inch := texSP(t, "1in")
	if e.geom.top != inch || e.geom.left != inch {
		t.Errorf("margins = %d/%d, want %d on every side", e.geom.top, e.geom.left, inch)
	}
	if want := e.geom.paperH - 2*inch; e.vsize != want {
		t.Errorf("vsize = %d, want %d (paper − 2in)", e.vsize, want)
	}
	if want := e.geom.paperW - 2*inch; e.hsize != want {
		t.Errorf("hsize = %d, want %d (paper − 2in)", e.hsize, want)
	}
}

// [cm] asks for 1.5cm instead.
func TestFullpageCmOption(t *testing.T) {
	e := runGeom(t, `\usepackage[cm]{fullpage}`)
	if e.geom == nil {
		t.Fatal("fullpage left no geometry state")
	}
	if want := texSP(t, "1.5cm"); e.geom.top != want {
		t.Errorf("top margin = %d, want %d", e.geom.top, want)
	}
}
