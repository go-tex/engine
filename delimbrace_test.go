// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A delimited macro argument that is wholly enclosed in one matching { } pair has
// its outer braces stripped, exactly as TeX does (TeXbook §20: "if the argument
// found ... is enclosed within braces ... the outermost pair of braces is
// removed"). Getting this wrong re-braces the token and derails any downstream
// delimited match — the mechanism behind amsart's \newtheorem loop, where
// \@oparg{\@ynthm{thm}}[] must deliver \@ynthm{thm}, not {\@ynthm{thm}}.
func TestDelimitedArgStripsSingleEnclosingGroup(t *testing.T) {
	cases := []struct{ src, want string }{
		// a single enclosing group: braces stripped
		{`\def\q#1.{[#1]}\message{\q{abc}.}`, "[abc]"},
		// the group hides the '.' delimiter, and its braces are still stripped
		{`\def\q#1.{[#1]}\message{\q{a.b}.}`, "[a.b]"},
		// a bare (unbraced) argument is unaffected
		{`\def\q#1.{[#1]}\message{\q abc.}`, "[abc]"},
		// two adjacent groups are NOT one enclosing pair: braces kept
		{`\def\q#1.{[#1]}\message{\q{a}{b}.}`, "[{a}{b}]"},
		// a group followed by loose tokens is NOT wholly enclosed: braces kept
		{`\def\q#1.{[#1]}\message{\q{a}b.}`, "[{a}b]"},
		// nested single enclosing group: only the OUTERMOST pair is removed
		{`\def\q#1.{[#1]}\message{\q{{a}}.}`, "[{a}]"},
	}
	for _, c := range cases {
		if got := run(t, c.src); got != c.want {
			t.Errorf("src=%q\n got =%q\n want=%q", c.src, got, c.want)
		}
	}
}
