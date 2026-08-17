package engine

import "testing"

// \if and \ifcat compare TOKENS, and a control sequence \let to a CHARACTER
// behaves exactly as that character (TeX §506). That is the whole point of
// \futurelet: it \let's a lookahead control sequence to the token it peeked at,
// and then asks \ifcat what that token was.
//
// The engine treated EVERY control sequence as "not a character", so \ifcat\next a
// was false for a \next \let to a letter. beamer decides whether a \mode<…> spec
// is a MODE NAME or an overlay range with exactly that test: it always answered
// "overlay", so every \mode<presentation> concluded the mode did not apply and the
// class commented out the rest of the document — the whole body of every talk.
//
// The \chardef rows are not an oversight: a \chardef'd constant is NOT a character
// token, and real TeX answers OTHER and DIFF for it. Every expected value below is
// a real TeX's (tectonic), leading space included.
func TestIfAndIfcatResolveALetCharacter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"futurelet-letter", `\def\g#1{}\futurelet\next\g p\message{[\ifcat\next a LETTER\else OTHER\fi]}`, "[ LETTER]"},
		{"futurelet-same-char", `\def\g#1{}\futurelet\next\g p\message{[\if\next p SAMECHAR\else DIFF\fi]}`, "[ SAMECHAR]"},
		{"digit-is-not-a-letter", `\def\g#1{}\futurelet\nxt\g 1\message{[\ifcat\nxt a LETTER\else OTHER\fi]}`, "[OTHER]"},
		{"digit-matches-a-digit", `\def\g#1{}\futurelet\nxt\g 1\message{[\ifcat\nxt 1 OTHER12\else NO\fi]}`, "[ OTHER12]"},
		{"let-to-a-brace", `\let\lb={ \message{[\ifcat\lb\bgroup BEGIN\else NO\fi]}`, "[BEGIN]"},
		{"chardef-is-not-a-letter", `\chardef\cd=65 \message{[\ifcat\cd a LETTER\else OTHER\fi]}`, "[OTHER]"},
		{"chardef-is-not-its-character", `\chardef\cd=65 \message{[\if\cd A SAME\else DIFF\fi]}`, "[DIFF]"},
		{"two-control-sequences-share-a-category", `\let\ra\relax\message{[\ifcat\ra\relax BOTHCS\else NO\fi]}`, "[BOTHCS]"},
		{"two-control-sequences-share-a-code", `\let\ra\relax\message{[\if\ra\relax SAMECS\else DIFF\fi]}`, "[SAMECS]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// beamer's own test, reduced: peek at the first token of a spec and branch on
// whether it is a letter (a mode name) or not (an overlay range). The leading
// space in the expected value is the one after "a" in the \ifcat.
func TestIfcatDecidesAModeNameFromAnOverlayRange(t *testing.T) {
	cases := []struct{ spec, want string }{
		{"presentation", "[ NAME]"},
		{"beamer", "[ NAME]"},
		{"1-", "[RANGE]"},
		{"2-3", "[RANGE]"},
	}
	for _, c := range cases {
		src := `\def\c#1:{\message{[\ifcat\next a NAME\else RANGE\fi]}}` +
			`\def\s{` + c.spec + `}` +
			`\expandafter\futurelet\expandafter\next\expandafter\c\s:`
		if got := runExpr(t, src); got != c.want {
			t.Errorf("spec %q = %q, want %q", c.spec, got, c.want)
		}
	}
}
