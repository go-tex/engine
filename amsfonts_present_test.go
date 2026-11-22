// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// amsart.cls:380 loads U/msa/m/n through \fontencoding{U}\fontfamily{msa}
// \try@load@fontshape and refuses the class when \U/msa/m/n is still undefined:
// "Package amsfonts not installed, or version too old?". Both font commands were
// no-ops, so every amsart document stopped there (go-tex/engine#876).
func TestAmsartFindsTheAMSSymbolFonts(t *testing.T) {
	empty := glyphPaths(t, ``)
	// The class's own load defined the name globally; undefined, the probe is six glyphs.
	if got := glyphPaths(t, `\makeatletter\@ifundefined{U/msa/m/n}{UUUUUU}{U}\makeatother`); got != empty+1 {
		t.Errorf("U/msa/m/n is undefined after \\documentclass{amsart}: %d glyph paths, want %d", got, empty+1)
	}
}

// glyphPaths counts the glyphs a body typesets under amsart.
func glyphPaths(t *testing.T, body string) int {
	t.Helper()
	e, err := compile([]byte(`\documentclass{amsart}\begin{document}`+body+`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	return strings.Count(strings.Join(e.RenderPages(e.renderMargin(0)), ""), "<path")
}

// The family that was asked for decides what loads: msa and msb define their
// shape, any other family or another encoding leaves it undefined, as a missing
// .fd file does. The probe typesets one glyph when the name is defined and six
// when it is not, on top of what an empty body already carries.
func TestTryLoadFontshapeDefinesOnlyTheAMSFamilies(t *testing.T) {
	empty := glyphPaths(t, ``)
	cases := []struct {
		name, enc, fam, probe string
		defined               bool
	}{
		{"msa", "U", "msa", "U/msa/m/n", true},
		{"msb", "U", "msb", "U/msb/m/n", true},
		{"another family", "U", "xyz", "U/xyz/m/n", false},
		{"msa in another encoding", "OT1", "msa", "OT1/msa/m/n", false},
		{"msb in another encoding", "OT1", "msb", "OT1/msb/m/n", false},
	}
	for _, c := range cases {
		body := `\makeatletter\begingroup\fontencoding{` + c.enc + `}\fontfamily{` + c.fam +
			`}\try@load@fontshape\endgroup\@ifundefined{` + c.probe + `}{UUUUUU}{U}\makeatother`
		want := empty + 6
		if c.defined {
			want = empty + 1
		}
		if got := glyphPaths(t, body); got != want {
			t.Errorf("%s: %d glyph paths, want %d (defined=%v)", c.name, got, want, c.defined)
		}
	}
}
