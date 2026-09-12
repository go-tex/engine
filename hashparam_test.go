package engine

import "testing"

// A bare # — one that is neither doubled nor followed by a parameter number — is
// legal input where it is an ARGUMENT rather than part of a definition. pgf's
// parser module reads the category of a token from the first two words of its
// \meaning and needs # itself to ask the question:
//
//	\edef\pgfparser@category@vi{\pgfparser@extractmeaning#}
//
// Our body scan consumed whatever followed the #, so the } was eaten as if it were
// a parameter number, the scan ran past the end of the group, and the rest of the
// FILE was absorbed into the definition — pgfmoduleparser.code.tex died on its
// line 528, taking \usetikzlibrary{svg.path} and every document that loads it.
//
// TeX reports "Illegal parameter number", BACKS THE TOKEN UP and keeps the # as a
// macro-parameter token (tex.web §479). The expected values below are tectonic's.
func TestBareHashDoesNotEatWhatFollows(t *testing.T) {
	cases := []struct{ src, want string }{
		// The # reaches the macro as its argument, and keeps its category.
		{`\def\take#1{[\meaning#1]}\message{\take#}`, "[macro parameter character #]"},
		// The same inside \edef: the } must survive the body scan.
		{`\def\take#1{[\meaning#1]}\edef\r{\take#}\message{\r}`, "[macro parameter character #]"},
		// And the definition stops at that }: what follows is NOT swallowed.
		{`\def\take#1{}\edef\r{\take#}\message{[after]}`, "[after]"},
		// ## is still halved, and #1 is still a parameter.
		{`\def\a#1{[#1]}\def\b{\a{x}}\message{\b}`, "[x]"},
		{`\def\a{\def\b##1{[##1]}}\a\message{\b{y}}`, "[y]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}

// \meaning names a character's category with TeX's own words (print_cmd_chr,
// tex.web §298). The four it knew were not a shortcut: pgf compares those words,
// so a # reported as "the character" made it equal to every ordinary character.
func TestMeaningNamesEveryCategory(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\def\m#1{\message{\meaning#1}}\m#`, "macro parameter character #"},
		{`\def\m#1{\message{\meaning#1}}\m$`, "math shift character $"},
		{`\def\m#1{\message{\meaning#1}}\m&`, "alignment tab character &"},
		{`\def\m#1{\message{\meaning#1}}\m^`, "superscript character ^"},
		{`\def\m#1{\message{\meaning#1}}\m_`, "subscript character _"},
		{`\def\m#1{\message{\meaning#1}}\m a`, "the letter a"},
		{`\def\m#1{\message{\meaning#1}}\m 1`, "the character 1"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}
