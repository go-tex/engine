// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \textsf and \sffamily were set in the roman: the default family bound no
// sans face, and fonts/lm carried none (go-tex/engine#877). tulmss.fd's m/n is
// LMSans10-Regular at the body size and has the same optical sizes as the
// slant. The test asks the optical table which design a size lands on, and
// that the engine binds the sans face by default.
func TestSansTableFollowsTulmssFd(t *testing.T) {
	cases := []struct {
		pt   float64
		name string
	}{
		{7, "lmsans8-regular"},
		{8.5, "lmsans9-regular"},
		{9.49, "lmsans9-regular"},
		{9.5, "lmsans10-regular"},
		{10.95, "lmsans10-regular"},
		{11, "lmsans12-regular"},
		{14.4, "lmsans12-regular"},
		{15, "lmsans17-regular"},
		{24, "lmsans17-regular"},
	}
	for _, c := range cases {
		if _, name := lmSansSizes.pick(c.pt); name != c.name {
			t.Errorf("sans at %gpt picks %s, want %s", c.pt, name, c.name)
		}
	}
	if opticalOf(lmSansRegular) == nil {
		t.Error("the embedded sans face has no optical table")
	}
}

// The default engine binds \sf, and \textsf typesets glyphs from the sans face,
// not the roman: the two renderings of the same word are different drawings.
func TestTextsfIsNotTheRoman(t *testing.T) {
	render := func(body string) string {
		e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		return strings.Join(e.RenderPages(e.renderMargin(0)), "")
	}
	roman, sans := render(`Sans`), render(`\textsf{Sans}`)
	if roman == sans {
		t.Error(`\textsf{Sans} renders exactly as the roman: no sans face is bound`)
	}
	if render(`\sffamily Sans`) != sans {
		t.Error(`\sffamily and \textsf render differently`)
	}
}
