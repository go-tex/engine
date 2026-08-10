package engine

import "testing"

// Escaped specials (\% \& \$ \# \_ \{ \}) typeset the literal characters.
func TestEscapedSpecials(t *testing.T) {
	e := New()
	e.LoadPlain()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{\%\&\$\#\_\{\}}`); err != nil {
		t.Fatal(err)
	}
	var got string
	for _, n := range e.box[0].list {
		if c, ok := n.(charNode); ok {
			got += string(c.ch)
		}
	}
	if got != "%&$#_{}" {
		t.Errorf("escaped specials produced %q want %%&$#_{}", got)
	}
}
