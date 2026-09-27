// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A maths space the layer does not know, translated to the one it does at the SAME
// width. Every value is amsmath.sty:170-178, which states each in both units — .1667em
// is 3/18, .2222em is 4/18, .2777em is 5/18 — so the mapping is exact.
//
// \> is the one that matters: plain TeX's medium space, which LaTeX spells \: because
// \> is tabbing there. 2157 papers of the 22127 harvested write it.
//
// The test is an EQUIVALENCE against the target command, not a width: a width pins
// today's font, while "\> is the medium space" is what the sources say.
func TestAMathSpaceSynonymIsItsTarget(t *testing.T) {
	for _, c := range [][2]string{
		{`$a\>b$`, `$a\:b$`},
		{`$a\thinspace b$`, `$a\,b$`},
		{`$a\negthinspace b$`, `$a\!b$`},
		{`$a\medspace b$`, `$a\:b$`},
		{`$a\thickspace b$`, `$a\;b$`},
	} {
		got, want := mathGeom2(t, ``, c[0]), mathGeom2(t, ``, c[1])
		if got.width != want.width || got.height != want.height || got.depth != want.depth {
			t.Errorf("%s = %d/%d/%d, %s gives %d/%d/%d", c[0], got.width, got.height,
				got.depth, c[1], want.width, want.height, want.depth)
		}
	}
}

// Each one must also MOVE the box, in the direction its width says. The equivalence
// above would hold just as well if both sides silently produced nothing.
func TestAMathSpaceSynonymActuallySpaces(t *testing.T) {
	plain := mathGeom2(t, ``, `$a b$`).width
	for _, c := range []struct {
		cmd   string
		wider bool
	}{
		{`\>`, true}, {`\thinspace`, true}, {`\medspace`, true}, {`\thickspace`, true},
		// A negative space must NARROW: it is −3mu, and the whole point of the
		// mathSpace work is that its sign used to flip.
		{`\negthinspace`, false},
	} {
		got := mathGeom2(t, ``, `$a`+c.cmd+` b$`).width
		if c.wider && got <= plain {
			t.Errorf(`$a%s b$ is %d wide, not more than $a b$'s %d`, c.cmd, got, plain)
		}
		if !c.wider && got >= plain {
			t.Errorf(`$a%s b$ is %d wide, not less than $a b$'s %d — a negative space `+
				`must narrow`, c.cmd, got, plain)
		}
	}
}

// \negmedspace and \negthickspace are deliberately NOT mapped: go-tex/math's table has
// a single negative width, −3mu, so −4mu and −5mu cannot be expressed. Mapping them to
// \! would be a silently wrong width, which is worse than the command being reported —
// a reported command is one the census can rank, a wrong width is invisible.
//
// Asserted so a later edit does not add them by symmetry with the five above.
func TestTheNegativeMediumAndThickSpacesStayUnmapped(t *testing.T) {
	for _, n := range []string{"negmedspace", "negthickspace"} {
		if _, ok := mathSpaceSynonym[n]; ok {
			t.Errorf(`\%s is mapped: go-tex/math has no -4mu or -5mu, so this is a `+
				`wrong width rather than a translation`, n)
		}
	}
}

// Text mode is untouched. \thinspace and \negthinspace are text commands too
// (format.go:45-46), and \, is \let to \thinspace there, so a \def would have been
// circular — which is why this is a maths-source rewrite.
func TestTheSynonymsDoNotReachTextMode(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`a\thinspace b\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.mathDropped) != 0 {
		t.Errorf("text-mode \\thinspace reached the math layer: %v", e.mathDropped)
	}
	if got := pageChars(e); got != "ab" {
		t.Errorf("page = %q, want %q — the text-mode command must still set its space", got, "ab")
	}
}
