package engine

import "testing"

// ⛔⛔ \vfil, \vfill AND \vss CONTRIBUTED NOTHING, while their horizontal twins three
// lines above them in primitives.go were right. The asymmetry was the whole defect: a
// page could not be filled from below.
//
// tex.web §20557-20559 binds them and §20600-20601 with §3318-3320 say what to:
//
//	primitive("vfil",vskip,fil_code)    fil_glue  == 0pt plus 1fil
//	primitive("vfill",vskip,fill_code)  fill_glue == 0pt plus 1fill
//	primitive("vss",vskip,ss_code)      ss_glue   == 0pt plus 1fil minus 1fil
//
// ⛔ AND \vfill WAS WORSE THAN EMPTY: format.go shadowed the primitive with
// \def\vfill{\vskip0pt plus 1fil\relax} — order ONE where TeX gives order TWO. So it
// did produce glue, and the glue lost to anything of order 2 and merely SHARED with
// the page style's own \vfil. Measured on the default page style, the word pushed down:
//
//	\vskip 0pt plus 1fil    398.890      \vfill avant  398.865   <- l'ordre UN
//	\vskip 0pt plus 1fill   658.625      \vfill après  658.585
//
// The two explicit glues are the controls and did not move; only \vfill did.
//
// ⛔ THE ORDER IS ASSERTED, NOT JUST THE PRESENCE. A test that only checked "some
// infinite glue" would have passed on the broken version, which is exactly how this
// survived: there WAS glue, of the wrong order.
func TestVerticalFilGluesHaveTheReferencesOrders(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      glueSpec
	}{
		{`\vfil`, `\vfil`, glueSpec{stretch: unity, stretchOrder: 1}},
		{`\vfill`, `\vfill`, glueSpec{stretch: unity, stretchOrder: 2}},
		{`\vss`, `\vss`, glueSpec{stretch: unity, stretchOrder: 1, shrink: unity, shrinkOrder: 1}},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}A\par` +
			c.src + `\par B\par`); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		found := false
		for _, n := range e.mvl {
			if g, ok := n.(glueNode); ok && g.spec == c.want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: glue %+v absente de la liste verticale; glues vues = %v",
				c.name, c.want, vglues(e.mvl))
		}
	}
	// ⛔ THE CONTROL: without any of the three, none of those glues may appear — or
	// every row above could be passing on something the page machinery emits anyway.
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}A\par B\par`); err != nil {
		t.Fatal(err)
	}
	for _, g := range vglues(e.mvl) {
		if g.stretchOrder == 2 {
			t.Errorf("témoin sans \\vfill: une glue d'ordre 2 est quand même là (%+v)", g)
		}
	}
}

func vglues(list []node) []glueSpec {
	var out []glueSpec
	for _, n := range list {
		if g, ok := n.(glueNode); ok {
			out = append(out, g.spec)
		}
	}
	return out
}
