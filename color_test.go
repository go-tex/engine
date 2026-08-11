package engine

import "testing"

// charColor records the colour stamped on the first occurrence of each character.
func charColor(nodes []node) map[rune]uint32 {
	m := map[rune]uint32{}
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case charNode:
				if _, ok := m[c.ch]; !ok {
					m[c.ch] = c.color
				}
			case *boxNode:
				walk(c.list)
			case frameNode:
				walk(c.inner.list)
			}
		}
	}
	walk(nodes)
	return m
}

// \textcolor colours its argument's glyphs and reverts afterwards (group-scoped).
func TestTextColor(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\textcolor{red}{A}B`); err != nil {
		t.Fatal(err)
	}
	cols := charColor(e.mvl)
	if cols['A'] != 0xFF0000 {
		t.Errorf("A colour = %06X, want FF0000", cols['A'])
	}
	if cols['B'] != 0 {
		t.Errorf("B colour = %06X, want 0 (black, reverted)", cols['B'])
	}
}

// \definecolor names a colour that \textcolor then uses.
func TestDefineColor(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\definecolor{marine}{RGB}{20,60,140}\noindent\textcolor{marine}{X}`); err != nil {
		t.Fatal(err)
	}
	if got := charColor(e.mvl)['X']; got != 0x143C8C {
		t.Errorf("marine X colour = %06X, want 143C8C", got)
	}
}

// \colorbox makes a filled, borderless box; \fcolorbox adds a coloured frame.
func TestColorbox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\colorbox{yellow}{X}`); err != nil {
		t.Fatal(err)
	}
	fr, ok := firstFrame(e.mvl)
	if !ok {
		t.Fatal("no frameNode from \\colorbox")
	}
	if fr.bg != 0xFFFF00 || fr.rule != 0 {
		t.Errorf("colorbox frame: bg=%06X rule=%d, want bg=FFFF00 rule=0", fr.bg, fr.rule)
	}
}

// resolveColor knows the built-in names and user colours; unknown → black.
func TestResolveColor(t *testing.T) {
	e := New()
	if e.resolveColor("red") != 0xFF0000 || e.resolveColor("blue") != 0x0000FF {
		t.Error("built-in colour resolution wrong")
	}
	if e.resolveColor("nope") != 0 {
		t.Error("unknown colour should be black (0)")
	}
}

// parseColorSpec handles the rgb / RGB / gray / HTML models.
func TestParseColorSpec(t *testing.T) {
	cases := []struct {
		model, spec string
		want        uint32
	}{
		{"RGB", "255,0,0", 0xFF0000},
		{"rgb", "0,0,1", 0x0000FF},
		{"gray", "0.5", 0x808080},
		{"HTML", "1a2b3c", 0x1A2B3C},
	}
	for _, c := range cases {
		if got := parseColorSpec(c.model, c.spec); got != c.want {
			t.Errorf("parseColorSpec(%q,%q) = %06X, want %06X", c.model, c.spec, got, c.want)
		}
	}
}

func TestHexColor(t *testing.T) {
	if got := hexColor(0x1A2B3C); got != "#1a2b3c" {
		t.Errorf("hexColor = %q, want #1a2b3c", got)
	}
}
