package engine

import "testing"

func TestPrecompose(t *testing.T) {
	cases := []struct {
		acc  string
		base rune
		want rune
	}{
		{"'", 'e', 'é'}, {"`", 'a', 'à'}, {"^", 'o', 'ô'}, {"\"", 'i', 'ï'},
		{"~", 'n', 'ñ'}, {"c", 'c', 'ç'}, {"c", 'C', 'Ç'}, {"v", 's', 'š'},
		{"=", 'o', 'ō'}, {"'", 'E', 'É'}, {"u", 'g', 'ğ'}, {"r", 'a', 'å'},
		{"'", 'x', 'x'}, // no precomposed form ⇒ base unchanged
	}
	for _, c := range cases {
		if got := precompose(c.acc, c.base); got != c.want {
			t.Errorf("precompose(%q,%q) = %q want %q", c.acc, c.base, got, c.want)
		}
	}
}

// Accents typeset the precomposed character into the paragraph.
func TestAccentTypesets(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.hsize = 500 * unity
	e.Run("\\'el\\`eve\\par") // élève
	var got string
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			for _, c := range b.list {
				if ch, ok := c.(charNode); ok {
					got += string(ch.ch)
				}
			}
		}
	}
	if got != "élève" {
		t.Errorf("accented paragraph = %q want élève", got)
	}
}

// A LaTeX accent takes a braced argument: \^{a} accents the a. The brace group
// must be consumed whole — its closing } must not leak into the surrounding list,
// where it would close an enclosing group and drop the rest of the document (the
// eptcs \publicationstatus "C\^{a}mpeanu" truncation). Cases: a single-char group
// with trailing text, a control-sequence base (\i), a lone braced accent, an
// empty group, a multi-character group whose tail is re-injected, and a group
// inside \begingroup whose } must not close the group.
func TestAccentBracedArgument(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`C\^{a}mpeanu\par`, "Câmpeanu"},
		{`na\"{\i}ve\par`, "naïve"},
		{`\^{\j}\par`, "ĵ"}, // \j (dotless) base inside the group
		{`\~{n}\par`, "ñ"},
		{`\^{}x\par`, "x"},                          // empty group: accents nothing, x survives
		{`\^{ab}\par`, "âb"},                        // multi-char group: a accented, b re-injected
		{`\^{a{b}c}\par`, "âbc"},                    // nested braces stay balanced in the group
		{`\^{\relax}Z\par`, "Z"},                    // unknown cs base: dropped, Z survives
		{`X\begingroup\'{e}\endgroup Y\par`, "XéY"}, // } must not close the \begingroup
		{`Q\^{ab`, "Qâb"},                           // unterminated group: consumed to EOF, no leak
		{`\^`, ""},                                  // accent with no argument at all (end of input)
	}
	for _, c := range cases {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		e.hsize = 500 * unity
		if _, err := e.Run(c.in); err != nil {
			t.Fatalf("Run(%q): %v", c.in, err)
		}
		if got := mvlText(e.mvl); got != c.want {
			t.Errorf("Run(%q) = %q want %q", c.in, got, c.want)
		}
	}
}
