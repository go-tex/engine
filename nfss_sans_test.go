// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// The sans family follows the series and shape switches: \sffamily\bfseries is the
// sans BOLD face a font package bound (libertine's Biolinum, which acmart sets its
// titles in), not the roman bold. Mock faces whose width encodes their identity make
// the choice observable without a font file.
func TestSansFamilyFollowsSeriesAndShape(t *testing.T) {
	const (
		rm, bf, sf, sfbf, sfit, it = 5, 7, 6, 9, 10, 11
	)
	build := func(t *testing.T, sans bool) *Engine {
		t.Helper()
		e, err := buildEngine(Options{}, true)
		if err != nil {
			t.Fatal(err)
		}
		e.bindFont("rm", famMock{w: rm})
		e.bindFont("bf", famMock{w: bf})
		e.bindFont("it", famMock{w: it})
		e.bindFont("sl", famMock{w: it})
		e.bindFont("sf", famMock{w: sf})
		if sans {
			e.bindFont(sansBoldName, famMock{w: sfbf})
			e.bindFont(sansItalicName, famMock{w: sfit})
		}
		e.SetFont(famMock{w: rm})
		return e
	}
	for _, tc := range []struct {
		name string
		src  string
		want int
		sans bool
	}{
		{"sffamily", `\sffamily x`, sf, true},
		{"sffamily then bfseries", `\sffamily\bfseries x`, sfbf, true},
		{"bfseries then sffamily keeps the series", `\bfseries\sffamily x`, sfbf, true},
		{"sffamily then itshape", `\sffamily\itshape x`, sfit, true},
		{"slanted is the sans italic", `\sffamily\slshape x`, sfit, true},
		{"textbf of textsf", `\textbf{\textsf{x}}`, sfbf, true},
		{"textsf of bfseries", `\textsf{\bfseries x}`, sfbf, true},
		{"mdseries returns to the regular sans", `\sffamily\bfseries\mdseries x`, sf, true},
		{"rmfamily leaves the sans family", `\sffamily\bfseries\rmfamily x`, bf, true},
		{"bf is a roman switch", `\sffamily\bf x`, bf, true},
		{"sf resets the series", `\bfseries\sf x`, sf, true},
		{"normalfont leaves the sans family", `\sffamily\normalfont x`, rm, true},
		{"a roman bold stays roman", `\bfseries x`, bf, true},
		// A package with no sans bold: the roman bold, as before this change.
		{"no sans bold bound", `\sffamily\bfseries x`, bf, false},
		{"no sans italic bound", `\sffamily\itshape x`, it, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := build(t, tc.sans)
			if _, err := e.Run(`\setbox0=\hbox{{` + tc.src + `}}`); err != nil {
				t.Fatal(err)
			}
			if got := e.box[0].width; got != tc.want*unity {
				t.Errorf("%s: width %d sp, want %d pt (%d sp)", tc.src, got, tc.want, tc.want*unity)
			}
		})
	}
}

// libertine binds its sans family beside the roman one: Linux Biolinum regular, bold
// and italic. The host serves stand-in bytes under the real file names, each pair
// distinct, so the test needs neither the bundle nor the network.
func TestLibertineBindsItsSansFaces(t *testing.T) {
	served := map[string][]byte{
		"LinLibertine_R.otf":  lmMonoRegular,
		"LinLibertine_RB.otf": lmRomanBold,
		"LinLibertine_RI.otf": lmRomanItalic,
		"LinBiolinum_R.otf":   lmSansRegular,
		"LinBiolinum_RB.otf":  lmRomanRegular,
		"LinBiolinum_RI.otf":  lmRomanItalic,
	}
	e, err := buildEngine(Options{Resolve: func(name string) ([]byte, bool) {
		b, ok := served[name]
		return b, ok
	}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\usepackage{libertine}`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sf", sansBoldName, sansItalicName} {
		if m := e.eq[name]; m == nil || m.kind != mFont || m.font == nil {
			t.Errorf("%s not bound to a face after \\usepackage{libertine}: %+v", name, m)
		}
	}
	sf, bold := e.eq["sf"], e.eq[sansBoldName]
	if sf != nil && bold != nil && faceKey(sf.font) == faceKey(bold.font) {
		t.Error("the sans bold is the sans regular: LinBiolinum_RB.otf was not the file read")
	}
}
