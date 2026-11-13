// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"strings"
	"testing"
)

// The one paragraph the first two tests set, at \hsize=100pt — the measure
// tectonic was run at, and one at which THIS ENGINE hyphenates when justified.
// That second condition is the control: at \hsize=120pt our justified
// paragraph has no hyphen either, so a ragged paragraph with none proves
// nothing there.
const raggedProbeText = `Typography affects every line of every paper, and a ` +
	`measure that is wrong by a centimetre is wrong on every line it sets.`

func raggedProbe(t *testing.T, prelude string) string {
	t.Helper()
	src := `\documentclass{article}\begin{document}\hsize=100pt ` + prelude +
		raggedProbeText + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return pageChars(e)
}

// ⛔ \raggedright BROKE ITS LINES AS IF IT WERE JUSTIFIED, AND HYPHENATED THEM.
//
// tex.web §852 gives badness 0 and fitness `decent` to ANY line carrying
// infinite stretch. \raggedright's \rightskip is `0pt plus 1fil`, so under it
// TeX's FIRST pass — no hyphenation, at \pretolerance — succeeds on every
// paragraph it meets and the hyphenating pass is never reached. That is why
// ragged text is not hyphenated: not a rule about hyphens, a consequence of
// what an infinite background does to badness.
//
// go-typeset/linebreak carries one float of stretch and flattens a fil to a
// large finite number (its issue #12, item 1). The lines therefore came back
// with a large but FINITE badness, the first pass failed, and the paragraph was
// hyphenated exactly as if it were justified.
//
// ⭐ MEASURED, tectonic 0.17.0, this paragraph at \hsize=100pt, lmroman10 on
// both sides:
//
//	\raggedright   6 lines, 0 hyphens, no warning
//	justified      6 lines, 2 hyphens (`pa-`/`per,` and `time-`/`tre`)
//	               and one Overfull \hbox (1.172pt too wide)
//
// and before this change we set the ragged paragraph `… and a mea-sure …`,
// with two overfull lines of our own.
func TestRaggedTextIsNotHyphenated(t *testing.T) {
	ragged := raggedProbe(t, `\raggedright `)
	if n := strings.Count(ragged, "-"); n != 0 {
		t.Errorf("a \\raggedright paragraph came out with %d hyphen(s); tectonic "+
			"sets this paragraph ragged with none: %q", n, ragged)
	}
}

// ⚠⚠ AND THE JUSTIFIED PARAGRAPH MUST STILL HYPHENATE — the control, without
// which the test above is satisfied by an engine that has simply stopped
// hyphenating. Both the reference and this engine break `pa-`/`per` here, and
// this output is byte for byte what the commit before this one produced.
func TestJustifiedTextStillHyphenates(t *testing.T) {
	just := raggedProbe(t, ``)
	if !strings.Contains(just, "pa-") {
		t.Errorf("the justified paragraph lost its hyphen; both tectonic and the "+
			"previous commit break it `… of every pa-` / `per, and …`: %q", just)
	}
}

// ⛔⛔ CENTRED TEXT IS NOT HYPHENATED EITHER, AND THAT IS THE WHOLE POINT OF
// ASKING ABOUT THE BACKGROUND RATHER THAN ABOUT \raggedright.
//
// §852 is about a line carrying infinite stretch, not about which command put
// it there. \centering sets BOTH skips to 0pt plus 1fil, so its paragraphs take
// the same first pass and are not hyphenated.
//
// ⭐ MEASURED, tectonic 0.17.0, this paragraph inside \begin{center} at
// \hsize=100pt: 6 lines, 0 hyphens, no warning — character for character what
// it sets the same text \raggedright.
//
// ⚠ THIS TEST IS WHY raggedBackground ASKS `||` AND NOT `&&`. Narrowing it to a
// strictly right-ragged background (right skip infinite, left skip finite)
// excludes \centering, and buys back 3 words of corpus word-recall on two
// revtex papers — measured, on #772. It also makes the engine hyphenate centred
// text, which the reference does not. The corpus score is a proxy; this is the
// oracle.
func TestCentredTextIsNotHyphenated(t *testing.T) {
	src := `\documentclass{article}\begin{document}\hsize=100pt ` +
		`\begin{center}` + raggedProbeText + `\end{center}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); strings.Count(got, "-") != 0 {
		t.Errorf("a centred paragraph came out with %d hyphen(s); tectonic sets "+
			"this text centred with none: %q", strings.Count(got, "-"), got)
	}
}

// ⛔⛔ AND RAGGED TEXT IS STILL HYPHENATED WHEN THE LINE WOULD NOT OTHERWISE
// FIT. The rule is NOT "do not hyphenate when ragged": TeX's first pass
// succeeds because every line is good, and a line holding a word wider than the
// measure is not good at any \rightskip. It fails, the hyphenating pass runs,
// and the word is broken. tectonic 0.17.0, \raggedright at \hsize=60pt:
//
//	A rate of
//	incomprehen-
//	sibilities per
//	page is the
//	measure here.
//
// ⚠ THIS TEST DOES NOT DISTINGUISH THE WIDTH TEST FROM THE RATIO TEST, and the
// first version of its comment claimed it did. Replacing the overfull test with
// `Ratio < -1`, or removing it altogether, leaves this green: at a measure
// narrow enough to overfull a line the first pass fails before the question is
// asked. What it does guard is the rule itself — that \raggedright has not been
// turned into "never hyphenate", which is the plausible wrong fix.
//
// ⚠ WHAT IS ASSERTED IS THE HYPHEN AND THE OVERHANG, NOT THE BREAK POINT. We
// set `in-comprehen-sibilities` where the reference sets `incomprehen-`, with
// 0.10pt of overhang left over, and the commit before this one did exactly the
// same: that divergence is older than this change and is not what this test
// watches. An accepted unhyphenated first pass would show ~33pt here instead.
func TestRaggedTextStillHyphenatesAWordThatCannotFit(t *testing.T) {
	src := `\documentclass{article}\begin{document}\hsize=60pt \raggedright ` +
		`A rate of incomprehensibilities per page is the measure here.` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "comprehen-") {
		t.Errorf("a word wider than the ragged measure was not hyphenated; "+
			"tectonic breaks it `incomprehen-` / `sibilities`: %q", got)
	}
	if d := e.Diagnostics(); math.Abs(d.WorstOverfull-0.10) > 0.05 {
		t.Errorf("worst overhang = %.2fpt, want about 0.10 — the residue of our "+
			"own break point, and the same value the commit before this one "+
			"produced. Tens of points would mean the word was left unbroken",
			d.WorstOverfull)
	}
}
