package engine

import "testing"

// \escapechar is the character TeX puts in front of a control-sequence NAME when
// it prints one — \string, \meaning, and the error reports. It is 92 ("\") by
// default, any character a package sets it to, and NOTHING AT ALL when it is
// outside 0..255 (TeX §63). The engine printed a backslash always.
//
// It matters wherever code sets \escapechar to build a string it will later
// COMPARE. beamer does exactly that:
//
//	\begingroup \escapechar=-1 \xdef\beamer@stopmode{\string\\mode} \endgroup
//
// It wants the five characters "\mode" so that \beamer@processline — the reader
// that skips material outside the current mode, reading the document verbatim —
// recognises that line and stops. It got "\\mode" instead.
//
// Every expected value below is a real TeX's (tectonic).
func TestEscapecharPrefixesAControlSequenceName(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"default-is-a-backslash", `\message{[\string\mode]}`, `[\mode]`},
		{"negative-prints-nothing", `\begingroup\escapechar=-1\relax\message{[\string\mode]}\endgroup`, `[mode]`},
		{"any-character", "\\begingroup\\escapechar=`\\X\\relax\\message{[\\string\\mode]}\\endgroup", `[Xmode]`},
		{"a-character-token-is-untouched", `\message{[\string a]}`, `[a]`},
		{"meaning-uses-it-too", `\begingroup\escapechar=-1\relax\message{[\meaning\relax]}\endgroup`, `[relax]`},
		// beamer's own line, and the reason this matters.
		{"beamers-stop-line", `\begingroup\escapechar=-1\relax\xdef\a{\string\\mode}\endgroup\message{[\a]}`, `[\mode]`},
		// The value is restored with the group, like any parameter.
		{"restored-with-the-group", `\begingroup\escapechar=-1\relax\endgroup\message{[\string\mode]}`, `[\mode]`},
		{"the-value-is-readable", `\message{[\the\escapechar]}`, `[92]`},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}
