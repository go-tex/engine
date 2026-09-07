package engine

import (
	"strings"
	"testing"
)

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

// xcolor's \definecolor carries TWO optional arguments:
//
//	\def\definecolor{\@testopt{\XC@definecolor}{}}
//	\def\XC@definecolor[#1]#2{\@testopt{\XC@definec@lor[#1]{#2}}\colornameprefix}
//	\def\XC@definec@lor[#1]#2[#3]#4#5{…}          xcolor.sty:473-476
//
// so it is \definecolor[type]{name}[prefix]{model}{spec}. Reading only the three
// mandatory arguments left the whole call in the input and every token of it was
// TYPESET: acmart declares its palette that way (acmart.cls:561-568), so eight lines
// of "[named]ACMBluecmyk1,0.1,0,0.1" came out on page 1 of every acmart paper.
func TestDefineColorOptionalArguments(t *testing.T) {
	const src = `\documentclass{article}\usepackage{xcolor}
\definecolor{Plain}{rgb}{1,0,0}
\definecolor[named]{Typed}{cmyk}{1,0.1,0,0.1}
\definecolor[named]{Both}[xc]{rgb}{0,1,0}
\begin{document}
A\textcolor{Plain}{r}\textcolor{Typed}{t}\textcolor{Both}{b}Z
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var b strings.Builder
	for _, p := range e.Pages() {
		b.WriteString(mvlText(p.list))
	}
	got := b.String()
	// The declarations set colours and leave NOTHING on the page.
	for _, junk := range []string{"named", "cmyk", "0.1", "xc"} {
		if strings.Contains(got, junk) {
			t.Errorf("a \\definecolor argument was typeset (%q): %q", junk, got)
		}
	}
	if !strings.Contains(got, "ArtbZ") {
		t.Errorf("the surrounding text is wrong: %q", got)
	}
	for _, name := range []string{"Plain", "Typed", "Both"} {
		if _, ok := e.colors[name]; !ok {
			t.Errorf("colour %q was not defined", name)
		}
	}
}
