package engine

import "testing"

// \aftergroup holds a token back until the current group closes and inserts it
// then — after the group's values have been restored, so it acts outside the
// group. That is what makes it useful: a package builds something inside a box
// and arranges for the code that finishes it to run once the box is complete.
// Every TikZ node ends that way, so with \aftergroup doing nothing a node was
// never finished and the whole picture was lost with it.
//
// The expected outputs below are a real TeX's (tectonic), not a guess.
func TestAftergroup(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\def\a{\message{[apres]}}{\aftergroup\a\message{[dans]}}\message{[fin]}`,
			"[dans] [apres] [fin]"},
		// The token acts outside the group: the group's own values are restored first.
		{`\newcount\n\n=1 \def\a{\message{[n=\the\n]}}{\n=2 \aftergroup\a}`, "[n=1]"},
		// Several tokens are inserted in the order they were saved.
		{`\def\a{\message{[a]}}\def\b{\message{[b]}}{\aftergroup\a\aftergroup\b}`, "[a] [b]"},
		// An inner group fires first, and its token lands inside the outer one.
		{`\def\a{\message{[a]}}\def\b{\message{[b]}}{{\aftergroup\a}\message{[milieu]}\aftergroup\b}\message{[fin]}`,
			"[a] [milieu] [b] [fin]"},
		// With no group open there is nothing to wait for.
		{`\def\a{\message{[a]}}\aftergroup\a\message{[suite]}`, "[a] [suite]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}

// It works for a box group too — the case a drawing package depends on, where
// the box is built with \hbox{…} or \hbox\bgroup…\egroup and the code that
// closes the construction runs once the box is finished.
func TestAftergroupFromABox(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\def\a{\message{[apres]}}\setbox0=\hbox{\aftergroup\a\message{[dans]}}\message{[fin]}`,
			"[dans] [apres] [fin]"},
		{`\def\a{\message{[apres]}}\setbox0=\hbox\bgroup\aftergroup\a\message{[dans]}\egroup\message{[fin]}`,
			"[dans] [apres] [fin]"},
		{`\def\a{\message{[apres]}}\setbox0=\vbox{\aftergroup\a\message{[dans]}}\message{[fin]}`,
			"[dans] [apres] [fin]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
	// The box itself is unaffected: it still measures what it holds.
	e := New()
	if _, err := e.Run(`\def\a{}\setbox0=\hbox{\aftergroup\a\kern5pt}\message{[\the\wd0]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[5.0pt]" {
		t.Errorf("box width = %q, want [5.0pt]", got)
	}
}

// A truncated \aftergroup, and one whose group never closes, leave the run alone.
func TestAftergroupMalformed(t *testing.T) {
	for _, src := range []string{
		`\aftergroup`,
		`{\aftergroup\undefinedthing`,
		`{\aftergroup}`,
	} {
		e := New()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil && src != `{\aftergroup\undefinedthing` {
			t.Errorf("%s: %v", src, err)
		}
	}
}

// takeAfterGroup is safe when the group stack is empty — a document that closes
// more groups than it opened must not take the engine with it.
func TestTakeAfterGroupWithoutAGroup(t *testing.T) {
	e := New()
	if ts := e.takeAfterGroup(); ts != nil {
		t.Errorf("= %v, want nothing", ts)
	}
	if _, err := e.Run(`\endgroup\message{[ok]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[ok]" {
		t.Errorf("= %q", got)
	}
}
