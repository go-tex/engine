// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// TeX's four spacing commands are named by punctuation: \, \! \: \; — measured
// in math units, where 18mu is 1em, so \, is 3mu, \: is 4mu, \; is 5mu and \! is
// −3mu. All four were undefined, and \, is close to the most common command in a
// mathematical paper: seven documents out of an arXiv sample stopped at it.
//
// Measured against a real LaTeX (tectonic) at 10pt: \hbox{ab} is 10.56pt and
// \hbox{a\,b} is 12.22672pt, so \, contributes 1.66672pt — one sixth of an em.
func TestMathSpacingCommands(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\setbox0=\hbox{ab}` +
		`\setbox1=\hbox{a\,b}\setbox2=\hbox{a\:b}\setbox3=\hbox{a\;b}\setbox4=\hbox{a\!b}`); err != nil {
		t.Fatal(err)
	}
	if e.box[0] == nil {
		t.Fatal("la boîte de référence est vide")
	}
	base := e.box[0].width
	// 10pt text: 3mu = 1.66672pt, 4mu = 2.22214pt, 5mu = 2.77786pt.
	for _, c := range []struct {
		name string
		reg  int
		want int
	}{
		{`\,`, 1, 109230},
		{`\:`, 2, 145630},
		{`\;`, 3, 182050},
		{`\!`, 4, -109230},
	} {
		if e.box[c.reg] == nil {
			t.Errorf("%s : boîte vide", c.name)
			continue
		}
		if got := e.box[c.reg].width - base; got != c.want {
			t.Errorf("%s ajoute %d sp (%.5f pt), attendu %d sp (%.5f pt)",
				c.name, got, float64(got)/65536, c.want, float64(c.want)/65536)
		}
	}
}

// \, is \thinspace under another name, as it is in plain TeX and in LaTeX.
func TestThinSpaceAliases(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\message{[\ifx\,\thinspace OUI\else NON\fi][\ifx\!\negthinspace OUI\else NON\fi]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[OUI][OUI]" {
		t.Errorf("obtenu %s, attendu [OUI][OUI]", got)
	}
}
