package engine

import "testing"

// A package that draws asks the colour package which model a colour is in, so it
// can pick the right device. This engine keeps colours in RGB rather than
// xcolor's model tables, so it answers rgb — which is both true here and what
// lets those packages load at all: TikZ's fadings library stops on the very
// first of these names otherwise, and with it the eight other libraries a real
// paper loads in the same breath.
func TestXcolorModelMachinery(t *testing.T) {
	cases := []struct{ src, want string }{
		// The model of a colour, as pgf asks for it before choosing its shading device.
		{`\makeatletter\XC@sdef\test{\XC@tgt@mod{natural}}\message{[\test]}`, "[rgb]"},
		{`\makeatletter\XC@sdef\test{\XC@tgt@mod{natural}}` +
			`\message{[\ifx\test\XC@mod@cmyk cmyk\else\ifx\test\XC@mod@gray gray\else rgb\fi\fi]}`, "[rgb]"},
		// A colour comes back as {model}{values}.
		{`\extractcolorspec{red}\spec\message{[\spec]}`, "[{rgb}{1,0,0}]"},
		{`\definecolor{mine}{RGB}{0,128,255}\extractcolorspec{mine}\spec\message{[\spec]}`,
			"[{rgb}{0,0.50196,1}]"},
		// Converting to a model yields the values themselves: RGB is all there is.
		{`\convertcolorspec{rgb}{1,0,0}{cmyk}\out\message{[\out]}`, "[1,0,0]"},
	}
	for _, c := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		out, err := e.Run(c.src)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := trimNL(out); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}

// \XC@sdef defines by expansion, as xcolor's does.
func TestXCsdef(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\makeatletter\def\src{valeur}\XC@sdef\cible{\src}\message{[\meaning\cible]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[macro:->valeur]" {
		t.Errorf("= %q, want the definition to be expanded", got)
	}
}

// A colour the document never defined has nothing to extract, and saying so must
// not derail the run.
func TestExtractColorSpecUnknown(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\extractcolorspec{pasunecouleur}\spec\message{[fini]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[fini]" {
		t.Errorf("= %q", got)
	}
}
