package engine

import "testing"

// TeX reads its input one LINE at a time and appends the character \endlinechar
// to each line; from then on that character acts on its CATCODE like any other.
// This engine keeps the whole source in one buffer with \n as the line separator,
// so the mouth presents that \n as \endlinechar (see mouthChar).
//
// With the defaults — \endlinechar = 13 (^^M) and \catcode13 = 5 — that is the
// end-of-line token the mouth always produced. It matters when a package CHANGES
// them, and beamer does: it reads a line verbatim with \catcode`\^^M=12 and a
// macro DELIMITED by ^^M. Against a hardwired end-of-line token that delimiter can
// never match, and the macro runs away with the rest of the file.
//
// Every expected value below is a real TeX's (tectonic).
func TestEndlinecharIsTheEndOfLineCharacter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"defaults", "\\message{[\\the\\endlinechar][\\the\\catcode`\\^^M]}", "[13][5]"},
		// \endlinechar = -1 appends nothing: the two lines join with no space.
		{"suppressed", "\\begingroup\\endlinechar=-1\n\\gdef\\A{a\nb}\\endgroup\n\\message{[\\meaning\\A]}", "[macro:->ab]"},
		// Any other character can be the line ending, and it lands in the body.
		{"another-character", "\\begingroup\\endlinechar=`\\X\\relax\n\\gdef\\B{a\nb}%\n\\endgroup%\n\\message{[\\meaning\\B]}", "[macro:->aXb]"},
		// beamer's idiom: read a whole line as the argument of a ^^M-delimited macro.
		{"line-read-verbatim", "\\begingroup\\catcode`\\^^M=12\\relax%\n\\long\\gdef\\readline#1^^M{\\gdef\\C{#1}}%\n\\endgroup%\n\\begingroup\\catcode`\\^^M=12\\relax%\n\\readline hello there\n\\endgroup%\n\\message{[\\C]}", "[hello there]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s: %q\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// The ordinary end of line is unchanged: one line ending is interword space, a
// blank line is \par.
func TestEndlinecharKeepsOrdinaryLineEndings(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"line-end-is-a-space", "\\message{[a\nb]}", "[a b]"},
		{"blank-line-is-par", "\\def\\par{\\message{PAR}}\\message{[a]}\n\n\\message{[b]}", "[a] PAR [b]"},
		{"comment-eats-the-line-end", "\\message{[a%\nb]}", "[ab]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s: %q\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// The two guards that keep the mouth honest when it is asked for a character that
// is not there, or before \endlinechar has been bound to a register.
func TestMouthCharGuards(t *testing.T) {
	e := New()
	if r, next, ok := e.mouthChar(len(e.base) + 3); ok || r != 0 || next != len(e.base)+4 {
		t.Errorf("mouthChar past the end = (%q, %d, %v), want (0, %d, false)", r, next, ok, len(e.base)+4)
	}
	// An engine whose \endlinechar register is not bound falls back to TeX's value.
	bare := &Engine{endlineReg: -1}
	if got := bare.endlinechar(); got != '\r' {
		t.Errorf("unbound \\endlinechar = %d, want 13", got)
	}
	if got := e.endlinechar(); got != '\r' {
		t.Errorf("\\endlinechar = %d, want 13", got)
	}
}
