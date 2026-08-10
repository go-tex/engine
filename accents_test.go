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
