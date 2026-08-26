package engine

import "testing"

// TeX's named parameters are real registers: assignable (with or without =),
// readable with \the, and arithmetic applies.
func TestNamedIntParams(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\tracinglostchars=1 \message{\the\tracinglostchars}`, "1"},
		{`\hbadness 500 \message{\the\hbadness}`, "500"},
		{`\message{\the\tolerance}`, "200"}, // plain TeX's initial value
		{`\message{\the\mag}`, "1000"},      //
		{`\message{\the\widowpenalty}`, "150"},
		{`\advance\tolerance by 100 \message{\the\tolerance}`, "300"},
		{`\multiply\clubpenalty by 2 \message{\the\clubpenalty}`, "300"},
		{`\message{\ifnum\hbadness=0 Y\else N\fi}`, "Y"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// wrapMsg puts a bare conditional inside \message so its branch is captured.
func wrapMsg(src string) string {
	if len(src) > 5 && src[:5] == `\ifnum`[:5] {
		return `\message{` + src + `}`
	}
	return src
}

// Dimension and glue parameters likewise.
func TestNamedDimenAndGlueParams(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\maxdepth=4pt \message{\the\maxdepth}`, "4.0pt"},
		{`\emergencystretch 1in \message{\the\emergencystretch}`, "72.26999pt"},
		{`\spaceskip=3pt plus 1pt \message{\the\spaceskip}`, "3.0pt plus 1.0pt"},
		{`\topskip=10pt \message{\the\topskip}`, "10.0pt"},
		{`\advance\maxdepth by 1pt \message{\the\maxdepth}`, "1.0pt"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// A parameter is scoped by grouping, so a package that sets one inside a group
// does not leak it — the reason it sets it there.
func TestNamedParamsAreScoped(t *testing.T) {
	got := runExpr(t, `\tolerance=200 {\tolerance=9999 \message{[\the\tolerance]}}\message{[\the\tolerance]}`)
	if got != "[9999] [200]" {
		t.Errorf("scoping = %q, want [9999] [200]", got)
	}
}

// A parameter the engine models itself keeps its own primitive: the register
// table must not shadow it.
func TestNamedParamsDoNotShadowEngineOnes(t *testing.T) {
	got := runExpr(t, `\hsize=123pt \message{\the\hsize}`)
	if got != "123.0pt" {
		t.Errorf("\\hsize = %q, want 123.0pt", got)
	}
	got = runExpr(t, `\parindent=7pt \message{\the\parindent}`)
	if got != "7.0pt" {
		t.Errorf("\\parindent = %q, want 7.0pt", got)
	}
}

// \nullfont is a font with no characters: text set in it takes no space and
// draws nothing, which is how a package measures without setting.
func TestNullfont(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{\nullfont ABC}`); err != nil {
		t.Fatal(err)
	}
	b := e.getBox(0)
	if b == nil {
		t.Fatal("box 0 is void")
	}
	if b.width != 0 || b.height != 0 || b.depth != 0 {
		t.Errorf("\\nullfont box measures %d/%d/%d, want 0/0/0", b.width, b.height, b.depth)
	}
	if svg := renderBoxSVG(b, 0, 0, nullFont{}, "white"); count(svg, "<path") != 0 {
		t.Errorf("\\nullfont drew glyphs: %s", svg)
	}
	// It is a font switch, so a group restores the previous font.
	e2 := New()
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\setbox0=\hbox{{\nullfont A}A}`); err != nil {
		t.Fatal(err)
	}
	plain := New()
	plain.SetFont(spMock{})
	plain.Run(`\setbox0=\hbox{A}`)
	if e2.getBox(0).width != plain.getBox(0).width {
		t.Errorf("the font switch escaped its group: %d vs %d",
			e2.getBox(0).width, plain.getBox(0).width)
	}
}

func count(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

// A parameter list stops when the register file is full, and never overwrites a
// name that already means something — the two guards that keep the table from
// disturbing the rest of the engine.
func TestNamedParamsRespectExistingNames(t *testing.T) {
	e := New()
	// \hsize is a primitive of its own; the table must have left it alone.
	if m := e.eq["hsize"]; m == nil || m.kind != mPrim {
		t.Errorf("\\hsize was shadowed by the parameter table: %+v", m)
	}
	// Running the loader again must change nothing: every name is taken now.
	before := len(e.eq)
	cnt, dim, skp := e.allocCnt, e.allocDim, e.allocSkp
	e.loadTeXParams()
	if len(e.eq) != before || e.allocCnt != cnt || e.allocDim != dim || e.allocSkp != skp {
		t.Errorf("a second pass allocated more registers: %d/%d/%d → %d/%d/%d",
			cnt, dim, skp, e.allocCnt, e.allocDim, e.allocSkp)
	}
}

// With the register file full the loader stops rather than running past its end.
func TestNamedParamsStopWhenRegistersRunOut(t *testing.T) {
	e := New()
	e.allocCnt, e.allocDim, e.allocSkp = 256, 256, 256
	for name := range e.eq { // start from a table with none of the names taken
		if texIntParams[name] != 0 || name == "spaceskip" {
			delete(e.eq, name)
		}
	}
	e.loadTeXParams() // must not panic or allocate past the end
	if e.allocCnt != 256 || e.allocDim != 256 || e.allocSkp != 256 {
		t.Errorf("allocated past the register file: %d/%d/%d", e.allocCnt, e.allocDim, e.allocSkp)
	}
}
