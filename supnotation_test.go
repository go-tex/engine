package engine

import "testing"

// TeX's ^^ notation (TeX §352) is how a source file writes a character it cannot
// type. A superscript character, doubled, escapes what follows: ^^ then two
// LOWERCASE hex digits is that character code, otherwise the single character
// after it is shifted by 64 — ^^M is carriage return, ^^I is tab, ^^7e is "~".
//
// The engine did not resolve it at all, so beamer's
//
//	\catcode`\^^M=12
//
// read a "^" and typeset what was left: every beamer talk opened with a stray
// "M=12" before its first slide.
//
// Every expected value below is a real TeX's (tectonic).
func TestSuperscriptNotation(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// Two lowercase hex digits: the character with that code.
		{"hex", `\message{[^^41]}`, "[A]"},
		{"hex-active", `\message{[^^7e]}`, "[~]"},
		// Not two hex digits: the single next character, shifted by 64. "4" is 52,
		// so ^^4 is 116 = "t", and the "x" that follows is left alone.
		{"shift", `\message{[^^4x]}`, "[tx]"},
		// It works inside a control-sequence name: \^^41 IS \A.
		{"in-cs-name", `\def\^^41{OK}\message{[\A]}`, "[OK]"},
		// …and in the argument of a backtick character constant.
		{"char-constant", "\\message{[\\number`\\^^41]}", "[65]"},
		// The catcodes TeX starts with, read through the notation.
		{"catcode-cr", "\\message{[\\the\\catcode`\\^^M]}", "[5]"},
		{"catcode-tab", "\\message{[\\the\\catcode`\\^^I]}", "[10]"},
		// And a package can change one, which is the whole point of the beamer line.
		{"catcode-set", "\\catcode`\\^^M=12\\message{[\\the\\catcode`\\^^M]}", "[12]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// A lone "^" is still a superscript, and a doubled "^" that cannot escape anything
// (nothing follows) is left alone rather than eating the end of the file.
func TestSuperscriptNotationLeavesOrdinaryCaretsAlone(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"single-caret-is-not-an-escape", `\message{[\ifnum\catcode` + "`" + `\^=7 SUP\else OTHER\fi]}`, "[SUP]"},
		// A caret whose catcode is no longer 7 escapes nothing.
		{"catcode-12-caret", `\catcode` + "`" + `\^=12 \message{[^^41]}`, "[^^41]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}
