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

// The escaped specials must not consume the following space or digits into the
// \char number: "50\% and \$5" → "50% and $5" (each terminated by \relax).
func TestEscapedSpecialsSpaceDigit(t *testing.T) {
	e := New()
	e.LoadPlain()
	e.SetFont(spMock{})
	e.Run(`\setbox0=\hbox{50\% and \$5}`)
	var got string
	for _, n := range e.box[0].list {
		switch c := n.(type) {
		case charNode:
			got += string(c.ch)
		case glueNode:
			got += " "
		}
	}
	if got != "50% and $5" {
		t.Errorf("got %q want '50%% and $5'", got)
	}
}
