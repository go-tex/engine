// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// styleToks is the piece that failed first: stringToToks makes CHARACTER tokens
// only, so basicstyle=\scriptsize arrived as the text "\scriptsize" and was
// typeset — an extra line on the page and no size change. A style value needs
// control sequences.
func TestStyleToksMakesControlSequences(t *testing.T) {
	got := styleToks(`\ttfamily\scriptsize`)
	if len(got) != 2 {
		t.Fatalf("got %d tokens, want 2: %v", len(got), got)
	}
	for i, want := range []string{"ttfamily", "scriptsize"} {
		if !got[i].cs_ || got[i].cs != want {
			t.Errorf("token %d is %+v, want the control sequence %q", i, got[i], want)
		}
	}
	// A control word swallows the spaces after it, as TeX does, so a value written
	// with spaces is the same thing.
	if spaced := styleToks(`\ttfamily \scriptsize`); len(spaced) != 2 {
		t.Errorf("with spaces: got %d tokens, want 2: %v", len(spaced), spaced)
	}
	// A backslash before a non-letter takes exactly that character, and braces
	// stay braces: \color{red} must not become one long control word.
	if n := len(styleToks(`\color{red}`)); n != 6 {
		t.Errorf(`styleToks(\color{red}) made %d tokens, want 6 (\color { r e d })`, n)
	}
}

// ⛔ A later key wins over an earlier one, which is listings' rule, and a style is
// pulled in WHERE IT IS NAMED rather than at the end — otherwise a document that
// says \lstset{style=m, basicstyle=\tiny} would get m's size back.
func TestLstOptionsForLastKeyWins(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.lstStyles = map[string]string{"m": `basicstyle=\scriptsize,numbers=left`}
	e.lstDefaults = `style=m,basicstyle=\tiny`
	o := e.lstOptionsFor("")
	if o.basicStyle != `\tiny` {
		t.Errorf("basicStyle = %q, want the later \\tiny", o.basicStyle)
	}
	if !o.numbers {
		t.Error("numbers=left from the style was lost")
	}
	// The block's own option is later still.
	if o := e.lstOptionsFor(`basicstyle=\small`); o.basicStyle != `\small` {
		t.Errorf("the block's own basicstyle lost: %q", o.basicStyle)
	}
}
