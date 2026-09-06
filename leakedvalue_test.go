// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A command that does not consume its value leaves that value in the input, and the
// value is then TYPESET. In a preamble it opens a page of its own: two corpus papers
// began on a page carrying nothing but "==-=" repeated, their title pushed to page 2.
func TestAValueIsNotTypeset(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		// tex.web §1224: assign_font_dimen is find_font_dimen, scan_optional_equals,
		// scan_normal_dimen. IEEEtran writes three of these per font-size switch.
		{"fontdimen", `\fontdimen2\font=10pt\relax X\par`},
		{"fontdimen négatif", `\fontdimen3\font=-5pt\relax X\par`},
		// url.sty:200 declares \newmuskip\Urlmuskip; papers set it.
		{"Urlmuskip", `\Urlmuskip=0mu plus 1mu X\par`},
		// TeX integer parameters the table lacked.
		{"interdisplaylinepenalty", `\interdisplaylinepenalty=10000 X\par`},
		{"interfootnotelinepenalty", `\interfootnotelinepenalty=2500 X\par`},
		// latex.ltx:8533 sets \f@size to the size as a BARE NUMBER; IEEEtran writes
		// \setlength{\dimen}{\f@size pt}, and an undefined \f@size left the "pt".
		{"f@size", `\makeatletter\@setfontsize\normalsize{10}{12}\setlength{\@tempdima}{\f@size pt}\makeatother X\par`},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(c.src); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if txt := mvlText(e.mvl); txt != "X" {
			t.Errorf("%s left %q on the page, want X alone", c.name, txt)
		}
	}
}

// \fontdimen is also READ as a value, and a read is never followed by "=" — so
// nothing after it may be consumed.
func TestFontdimenReadConsumesNothing(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\dimen0=\fontdimen2\font APRES\par`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "APRES") {
		t.Errorf("a read swallowed what followed it: %q", txt)
	}
}
