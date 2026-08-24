// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \#, \% and \& are \chardef tokens under LaTeX, not macros. Measured against a
// real LaTeX (tectonic): \meaning\# is \char"23, \meaning\% is \char"25 and
// \meaning\& is \char"26, and \edef\x{\#} leaves \x as macro:->\# — the token
// survives, because a \chardef token cannot be expanded.
//
// That is not a detail of \meaning. pgf builds each SVG fragment with \edef and
// rebinds \# to a raw catcode-11 # only while the fragment is written out, which
// is how a shading's fill:url(#pgfsh7) gets its reference. While these were
// ordinary macros the \edef expanded them early, every gradient reference came
// out as fill:url(\char 35\relax pgfsh7), and no shading in any document
// referred to anything.
func TestEscapedCharsAreChardefTokens(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\message{[\meaning\#][\meaning\%][\meaning\&]}` +
		`\edef\x{\#\%\&}\message{[\meaning\x]}`)
	if err != nil {
		t.Fatal(err)
	}
	// TeX separates one \message from the next with a space.
	const want = `[\char"23][\char"25][\char"26] [macro:->\#\%\&]`
	if got := trimNL(out); got != want {
		t.Errorf("obtenu  %s\nattendu %s", got, want)
	}
}

// Being unexpandable must not stop them reaching the page as the characters they
// stand for.
func TestEscapedCharsStillTypeset(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{\#\%\&}`); err != nil {
		t.Fatal(err)
	}
	if got := boxChars(e.box[0]); got != "#%&" {
		t.Errorf("les caractères composés sont %q, attendu \"#%%&\"", got)
	}
}

// The idiom pgf relies on: rebind the token for the length of one expansion.
func TestEscapedHashCanBeRebound(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run("{\\catcode`\\#=11 \\gdef\\rawhash{#}}" +
		`\edef\frag{url(\#pgfsh7)}` +
		`{\let\#\rawhash\edef\done{\frag}\message{[\meaning\done]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); !strings.Contains(got, `url(#pgfsh7)`) {
		t.Errorf("le fragment développé est %s, attendu qu'il contienne url(#pgfsh7)", got)
	}
}
