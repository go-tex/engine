package engine

import (
	"strings"
	"testing"
)

// formatPageNumber renders each page-number style correctly.
func TestFormatPageNumber(t *testing.T) {
	cases := []struct {
		n     int
		style byte
		want  string
	}{
		{1, 'a', "1"}, {42, 'a', "42"},
		{4, 'r', "iv"}, {4, 'R', "IV"},
		{1, 'l', "a"}, {26, 'l', "z"}, {27, 'l', "aa"}, {28, 'l', "ab"},
		{1, 'L', "A"}, {27, 'L', "AA"},
		{0, 'a', "1"}, // clamp to 1
	}
	for _, c := range cases {
		if got := formatPageNumber(c.n, c.style); got != c.want {
			t.Errorf("formatPageNumber(%d,%q) = %q, want %q", c.n, c.style, got, c.want)
		}
	}
}

// \pagestyle and \pagenumbering set the engine state.
func TestPagestyleAndNumbering(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if e.pageStyle != "empty" {
		t.Errorf("default pageStyle = %q, want empty", e.pageStyle)
	}
	if _, err := e.Run(`\pagestyle{plain}\pagenumbering{roman}`); err != nil {
		t.Fatal(err)
	}
	if e.pageStyle != "plain" {
		t.Errorf("pageStyle = %q, want plain", e.pageStyle)
	}
	if e.pageNumStyle != 'r' {
		t.Errorf("pageNumStyle = %q, want 'r'", e.pageNumStyle)
	}
	if _, err := e.Run(`\pagestyle{empty}`); err != nil {
		t.Fatal(err)
	}
	if e.pageStyle != "empty" {
		t.Errorf("pageStyle = %q, want empty", e.pageStyle)
	}
}

// With \pagestyle{plain}, a rendered page carries a centred foot number; with
// empty it does not.
func TestPageFooterRendered(t *testing.T) {
	render := func(src string) string {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		e.vsize = 200 * unity
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		pages := e.Pages()
		if len(pages) == 0 {
			t.Fatal("no pages")
		}
		var b strings.Builder
		collectChars(pages[0].list, &b)
		return b.String()
	}
	// spMock renders digits as charNodes; the footer number "1" should appear.
	withNum := render(`\pagestyle{plain}Hello.`)
	if !strings.Contains(withNum, "1") {
		t.Errorf("plain page should show foot number 1, got %q", withNum)
	}
	noNum := render(`Hello.`)
	if strings.Contains(noNum, "1") {
		t.Errorf("empty page should have no foot number, got %q", noNum)
	}
}

// \today expands to Options.Date (supplied because a wasm build has no clock).
func TestToday(t *testing.T) {
	doc, err := NewDocument(Options{Date: "5 May 2026"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.today != "5 May 2026" {
		t.Errorf("today = %q, want the supplied date", doc.today)
	}
	// \today is a non-expandable prim: it typesets the date in text (its real use),
	// though it is not gullet-expandable inside \message/\edef.
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.today = "May"
	if _, err := e.Run(`\today`); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "May") {
		t.Errorf("\\today should typeset the date; got %q", b.String())
	}
}
