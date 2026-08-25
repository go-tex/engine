// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"testing"
)

// \meaning of an allocated register said "undefined", which is what a name with
// no meaning at all reports. A file that asks \ifx\foo\undefined about one got
// the right answer for the wrong reason, and anyone reading a trace was told the
// register did not exist.
//
// Measured against a real LaTeX (tectonic): \newcount gives \count196, \newdimen
// gives \dimen139, \newtoks gives \toks16, and \newread and \newbox — which TeX
// allocates with \chardef — give \char"2 and \char"33. The numbers are whatever
// the format has reached; the FORM is what has to match.
func TestMeaningOfAllocatedRegisters(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\documentclass{article}` +
		`\newcount\za\newdimen\zd\newtoks\zt\newread\zr\newbox\zb\newskip\zs` +
		`\message{[\meaning\za][\meaning\zd][\meaning\zt][\meaning\zr][\meaning\zb][\meaning\zs]}`)
	if err != nil {
		t.Fatal(err)
	}
	want := regexp.MustCompile(`^\[\\count\d+\]\[\\dimen\d+\]\[\\toks\d+\]\[\\char"[0-9A-F]+\]\[\\char"[0-9A-F]+\]\[\\skip\d+\]$`)
	if got := trimNL(out); !want.MatchString(got) {
		t.Errorf("obtenu %s\nattendu la forme [\\count<n>][\\dimen<n>][\\toks<n>][\\char\"<n>][\\char\"<n>][\\skip<n>]", got)
	}
}

// A register still has a meaning, so \ifx against an undefined name must say so.
func TestAllocatedRegisterIsNotUndefined(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\documentclass{article}\newcount\za` +
		`\message{[\ifx\za\undefinedzz OUI\else NON\fi]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[NON]" {
		t.Errorf("un registre alloué se compare comme indéfini : %s", got)
	}
}

// A font-selecting name reports "select font <name>", as TeX does. TeX names the
// font FILE there (select font cmr10); the engine names the control sequence it
// bound, which is the only name it has for a face given through Options.
func TestMeaningOfAFont(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\message{[\meaning\rm]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[select font rm]" {
		t.Errorf("obtenu %s, attendu [select font rm]", got)
	}
}
