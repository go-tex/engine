// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestXymatrixSourceTakesOnlyAWholeFormula(t *testing.T) {
	for _, c := range []struct{ src, opts, body string }{
		{`\xymatrix{A & B}`, "", "A & B"},
		{`  \xymatrix{A}  `, "", "A"},
		{`\xymatrix@C=1pc@R=2pc{A \\ B}`, "@C=1pc@R=2pc", `A \\ B`},
		{`\xymatrix{ {A} & {B} }`, "", " {A} & {B} "},
	} {
		opts, body, ok := xymatrixSource(c.src)
		if !ok || opts != c.opts || body != c.body {
			t.Errorf("xymatrixSource(%q) = (%q,%q,%v), want (%q,%q,true)", c.src, opts, body, ok, c.opts, c.body)
		}
	}
	// A diagram embedded in a larger formula is NOT taken: the surrounding
	// material would have to be laid out around it, and saying so plainly is
	// better than drawing half of it.
	for _, src := range []string{
		`x = \xymatrix{A & B}`,
		`\xymatrix{A & B} + 1`,
		`\frac{a}{b}`,
		`\xymatrix`,
		``,
	} {
		if _, _, ok := xymatrixSource(src); ok {
			t.Errorf("xymatrixSource(%q) was taken", src)
		}
	}
}

// Every number in the layout was read off tectonic. 2pc is the default and the
// separation is measured between cell BOXES, so these are the two that decide
// where everything lands.
func TestXySepDefaultsToTwoPica(t *testing.T) {
	for _, c := range []struct {
		opts string
		key  byte
		want float64
	}{
		{"", 'C', 24}, {"", 'R', 24},
		{"@C=1pc", 'C', 12}, {"@C=3pc", 'C', 36},
		{"@C=1pc@R=2pc", 'R', 24},
		{"@R=10pt", 'R', 10},
		{"@C=0pc", 'C', 0},
		{"@!0", 'C', 24}, // an option this does not read leaves the default
	} {
		if got := xySep(c.opts, c.key); got != c.want {
			t.Errorf("xySep(%q,%c) = %g, want %g", c.opts, c.key, got, c.want)
		}
	}
}

func TestParseXymatrixGrid(t *testing.T) {
	g := parseXymatrix(`A \ar[r] & B \\ C & D`)
	if len(g) != 2 || len(g[0]) != 2 || len(g[1]) != 2 {
		t.Fatalf("grid is %v", g)
	}
	if got := strings.TrimSpace(g[0][0].math); got != "A" {
		t.Errorf("the \\ar was left in the maths: %q", got)
	}
	if len(g[0][0].arrows) != 1 || g[0][0].arrows[0].dc != 1 || g[0][0].arrows[0].dr != 0 {
		t.Errorf("arrow = %+v, want one pointing right", g[0][0].arrows)
	}
	if len(g[1][1].arrows) != 0 || strings.TrimSpace(g[1][1].math) != "D" {
		t.Errorf("the last cell is %+v", g[1][1])
	}
}

// The forms the reported diagram uses, and the ones a paper reaches for next.
func TestParseXyArrowStylesAndLabels(t *testing.T) {
	g := parseXymatrix(`A \ar@{^(->}[r]\ar[d]_f & B \ar@{=}[d] \\ C \ar[r]^{g} & D`)
	a := g[0][0].arrows
	if len(a) != 2 {
		t.Fatalf("cell A has %d arrows, want 2", len(a))
	}
	if a[0].style != "^(->" || a[0].dc != 1 {
		t.Errorf("the hooked arrow is %+v", a[0])
	}
	if a[1].dr != 1 || len(a[1].labels) != 1 || a[1].labels[0].text != "f" || a[1].labels[0].side != '_' {
		t.Errorf("the labelled arrow is %+v", a[1])
	}
	if g[0][1].arrows[0].style != "=" {
		t.Errorf("the equality is %+v", g[0][1].arrows[0])
	}
	if l := g[1][0].arrows[0].labels; len(l) != 1 || l[0].text != "g" || l[0].side != '^' {
		t.Errorf("a braced label read as %+v", l)
	}
}

// A direction is a run of letters, so [rr] is two columns and [dr] is a diagonal.
func TestXyDirections(t *testing.T) {
	for src, want := range map[string][2]int{
		`A \ar[r] & B`:  {0, 1},
		`A \ar[l] & B`:  {0, -1},
		`A \ar[d] & B`:  {1, 0},
		`A \ar[u] & B`:  {-1, 0},
		`A \ar[rr] & B`: {0, 2},
		`A \ar[dr] & B`: {1, 1},
		`A \ar[ul] & B`: {-1, -1},
	} {
		a := parseXymatrix(src)[0][0].arrows
		if len(a) != 1 || a[0].dr != want[0] || a[0].dc != want[1] {
			t.Errorf("%s: %+v, want dr=%d dc=%d", src, a, want[0], want[1])
		}
	}
}

func TestReadXyStyle(t *testing.T) {
	for style, want := range map[string]struct {
		tail, head     int
		double, dashed bool
	}{
		"":     {0, 1, false, false},
		"->":   {0, 1, false, false},
		"-":    {0, 0, false, false},
		"=":    {0, 0, true, false},
		"=>":   {0, 1, true, false},
		"->>":  {0, 2, false, false},
		"^(->": {1, 1, false, false},
		"_(->": {-1, 1, false, false},
		".>":   {0, 1, false, true},
		"-->":  {0, 1, false, true},
	} {
		tail, head, double, dashed := readXyStyle(style)
		if tail != want.tail || head != want.head || double != want.double || dashed != want.dashed {
			t.Errorf("readXyStyle(%q) = (%d,%d,%v,%v), want %+v", style, tail, head, double, dashed, want)
		}
	}
}

// The whole thing, end to end: the reported diagram draws, nothing is dropped,
// and the maths layer is not asked to make sense of \xymatrix.
func TestTheReportedDiagramDraws(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	src := `\documentclass{article}\begin{document}` +
		`\[\xymatrix{` + "\n" +
		`A \ar@{^(->}[r]\ar[d]_f & B \ar@{=}[d] \\` + "\n" +
		`C \ar[r] & D}\]` +
		`\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := len(e.mathDropped); n != 0 {
		t.Errorf("the maths layer was asked and refused: %v", e.mathDropped)
	}
	svg := strings.Join(e.RenderPages(72), "")
	// Four objects and the lines between them: A, B, C, D and f are glyph paths,
	// the shafts and heads are filled paths of this file's making.
	if n := strings.Count(svg, "<path"); n < 12 {
		t.Errorf("%d paths on the page — the diagram did not draw:\n%s", n, svg)
	}
	for _, want := range []string{"<svg", `fill="black"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
}

func TestBoxExitLeavesAtTheEdge(t *testing.T) {
	p := placed{cx: 10, cy: 20, halfW: 4, halfH: 3}
	for _, c := range []struct{ dx, dy, wx, wy float64 }{
		{1, 0, 14, 20},  // due east: the vertical edge
		{-1, 0, 6, 20},  // due west
		{0, 1, 10, 23},  // due south: the horizontal edge
		{0, -1, 10, 17}, // due north
	} {
		x, y := boxExit(p, c.dx, c.dy)
		if x != c.wx || y != c.wy {
			t.Errorf("boxExit(%g,%g) = (%g,%g), want (%g,%g)", c.dx, c.dy, x, y, c.wx, c.wy)
		}
	}
	// A diagonal leaves through whichever edge it reaches first.
	if x, y := boxExit(p, 1, 1); x != 13 || y != 23 {
		t.Errorf("a diagonal left at (%g,%g), want (13,23)", x, y)
	}
}

// An <svg> clips to its own viewport, silently. Sizing the picture from the grid
// alone cut whatever reached past it: a label on the left of the leftmost
// column's arrow is placed at a NEGATIVE x — the reported \ar[d]_f landed at
// translate(-3.65, 20.12) — and half of the f was simply not there.
func TestALabelOutsideTheGridIsNotClipped(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	plain, ok := e.makeXymatrix("", `A \ar[d] & B \\ C & D`, "")
	if !ok {
		t.Fatal("the unlabelled diagram did not draw")
	}
	labelled, ok := e.makeXymatrix("", `A \ar[d]_f & B \\ C & D`, "")
	if !ok {
		t.Fatal("the labelled diagram did not draw")
	}
	if labelled.width <= plain.width {
		t.Errorf("the label added no width: %d against %d — it is being clipped",
			labelled.width, plain.width)
	}
	// And nothing is left outside the viewport: every coordinate the picture's own
	// wrapper does not cover would be cut.
	w, h := xyViewport(t, labelled.svg)
	dx, dy := xyWrapperShift(t, labelled.svg)
	if dx < 0 || dy < 0 {
		t.Errorf("the shift is (%g,%g) — it must move marks INTO the viewport", dx, dy)
	}
	if w <= 0 || h <= 0 {
		t.Errorf("the viewport is %gx%g", w, h)
	}
}

var xySizeRe = regexp.MustCompile(`<svg[^>]*width="([\d.]+)" height="([\d.]+)"`)
var xyShiftRe = regexp.MustCompile(`<svg[^>]*><g transform="translate\((-?[\d.]+),(-?[\d.]+)\)">`)

func xyViewport(t *testing.T, svg string) (float64, float64) {
	t.Helper()
	m := xySizeRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("no viewport in %.200s", svg)
	}
	return parseFloat(m[1]), parseFloat(m[2])
}

func xyWrapperShift(t *testing.T, svg string) (float64, float64) {
	t.Helper()
	m := xyShiftRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("no shift wrapper in %.200s", svg)
	}
	return parseFloat(m[1]), parseFloat(m[2])
}

// The canvas is what makes that possible: every primitive says where it went.
func TestXyCanvasRecordsWhatIsDrawn(t *testing.T) {
	var c xyCanvas
	if c.any {
		t.Error("an empty canvas claims to hold something")
	}
	c.at(10, 20)
	c.box(-5, 30, 15, 35)
	if c.minX != -5 || c.minY != 20 || c.maxX != 15 || c.maxY != 35 {
		t.Errorf("extent = (%g,%g)-(%g,%g), want (-5,20)-(15,35)", c.minX, c.minY, c.maxX, c.maxY)
	}
}

// An equality is TWO rules with white between them. Measured off tectonic at
// 600dpi across the middle of the reported diagram: 0.36 and 0.48bp thick with
// 1.56bp of white, so their centres are about 2pt apart. Drawn one rule-thickness
// apart the pair reads as a single thick line.
func TestAnEqualityIsTwoSeparatedRules(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	n, ok := e.makeXymatrix("", `A \ar@{=}[d] \\ C`, "")
	if !ok {
		t.Fatal("it did not draw")
	}
	xs := firstXOfEachPath(n.svg)
	if len(xs) < 2 {
		t.Fatalf("%d paths, want at least the two rules:\n%s", len(xs), n.svg)
	}
	sort.Float64s(xs)
	gap := xs[len(xs)-1] - xs[0]
	if gap < xyDoubleSep-0.2 || gap > xyDoubleSep+0.2 {
		t.Errorf("the two rules are %.2f apart, want %.2f", gap, xyDoubleSep)
	}
}

var xyPathStartRe = regexp.MustCompile(`<path d="M ([-\d.]+) `)

func firstXOfEachPath(svg string) []float64 {
	var out []float64
	for _, m := range xyPathStartRe.FindAllStringSubmatch(svg, -1) {
		out = append(out, parseFloat(m[1]))
	}
	return out
}

// A <place> is what XY-pic calls the thing between a label's ^_| and the label
// itself, and a bare - is one: \PATHanchor@i (xyarrow.tex) reads it as <>(.5),
// the middle of the connection. Read as the LABEL instead — which is what this
// used to do — \ar@{.>}[dr]|-{(x,y)} put a lone dash on the arrow and let the
// {(x,y)} fall through into the cell's own maths, where it printed beside the U.
func TestALabelPlaceIsConsumedNotTypeset(t *testing.T) {
	grid := parseXymatrix(`U \ar@{.>}[dr]|-{(x,y)} \\ & V`)
	c := grid[0][0]
	if c.math != "U" {
		t.Errorf("cell maths = %q, want %q — the label leaked into the cell", c.math, "U")
	}
	if len(c.arrows) != 1 {
		t.Fatalf("%d arrows, want 1", len(c.arrows))
	}
	a := c.arrows[0]
	if len(a.labels) != 1 {
		t.Fatalf("%d labels, want 1: %+v", len(a.labels), a.labels)
	}
	if got := a.labels[0]; got.text != "(x,y)" || got.side != '|' || got.pos != 0.5 {
		t.Errorf("label = %+v, want {text:(x,y) side:| pos:0.5}", got)
	}
}

// ^-{…} is the same place on an above-label, and it is the form the corpus
// actually uses: 2408.06962 writes \ar[r]^-{\pi^*} twenty-one times.
func TestAnAboveLabelTakesAPlaceToo(t *testing.T) {
	a, rest, ok := parseXyArrow(`[r]^-{\pi^*}&`)
	if !ok {
		t.Fatal("it did not parse")
	}
	if rest != "&" {
		t.Errorf("rest = %q, want %q", rest, "&")
	}
	if len(a.labels) != 1 || a.labels[0].text != `\pi^*` || a.labels[0].side != '^' {
		t.Errorf("labels = %+v, want one ^ label \\pi^*", a.labels)
	}
}

// A place can also name WHERE along the arrow the label goes.
func TestAPlaceCanBeAFraction(t *testing.T) {
	a, _, ok := parseXyArrow(`[r]^(0.3)x`)
	if !ok {
		t.Fatal("it did not parse")
	}
	if len(a.labels) != 1 || a.labels[0].pos != 0.3 {
		t.Errorf("labels = %+v, want one at 0.3", a.labels)
	}
	// Something that is not a number is the LABEL, not a place: \ar[r]^(a) puts
	// (a) beside the arrow.
	b, _, ok := parseXyArrow(`[r]^{(a)}`)
	if !ok {
		t.Fatal("it did not parse")
	}
	if len(b.labels) != 1 || b.labels[0].text != "(a)" || b.labels[0].pos != 0.5 {
		t.Errorf("labels = %+v, want (a) at the middle", b.labels)
	}
}

// One \ar can carry several @ groups: \ar@/^/@{.>}[r] both curves and dots.
func TestSeveralAtGroupsOnOneArrow(t *testing.T) {
	a, _, ok := parseXyArrow(`@/^/@{.>}[r]`)
	if !ok {
		t.Fatal("it did not parse")
	}
	if a.curve != "^" || a.style != ".>" {
		t.Errorf("curve = %q, style = %q; want ^ and .>", a.curve, a.style)
	}
}

// The curving distance comes from XY-pic, not from taste: \ar@/^/ is
// \ar@slashing{^}, whose control point is the midpoint plus TWICE the slide
// vector, and \vfromslide@i makes that vector .5pc when no distance is given. A
// quadratic Bézier passes half way to its control point, so the belly is .5pc.
func TestACurveSpecReadsItsDirectionAndDistance(t *testing.T) {
	for _, tc := range []struct {
		spec   string
		dir    float64
		amount float64
		ok     bool
	}{
		{"^", 1, xyCurveDefault, true},
		{"_", -1, xyCurveDefault, true},
		{"^1pc", 1, 12, true},
		{"_2pt", -1, 2, true},
		{"", 0, 0, false},
		{"`", 0, 0, false},
	} {
		dir, amount, ok := readXyCurve(tc.spec)
		if ok != tc.ok || (ok && (dir != tc.dir || amount != tc.amount)) {
			t.Errorf("readXyCurve(%q) = (%g, %g, %v), want (%g, %g, %v)",
				tc.spec, dir, amount, ok, tc.dir, tc.amount, tc.ok)
		}
	}
}

// And the drawn arrow actually leaves the straight line by that much, on the
// side ^ names: left of travel, which for a rightward arrow is UP the page.
//
// Measured against the line joining the two CENTRES, which is what XY-pic's
// control point is built from — not against the arrow's own endpoints, which sit
// on the boxes' edges and are already off that line by the time the curve gets
// there.
func TestACurvedArrowLeavesTheStraightLine(t *testing.T) {
	from := placed{cx: 0, cy: 0, halfW: 5, halfH: 5}
	to := placed{cx: 120, cy: 0, halfW: 5, halfH: 5}
	straight, ok := xyShaftFor(from, to, xyArrow{dc: 1})
	if !ok {
		t.Fatal("the straight one has no shaft")
	}
	if p, _, _ := straight.at(straight.length() / 2); p.y != 0 {
		t.Errorf("a straight arrow's middle is %.2fpt off the line", p.y)
	}
	for _, tc := range []struct{ curve, want string }{{"^", "up"}, {"_", "down"}} {
		sh, ok := xyShaftFor(from, to, xyArrow{dc: 1, curve: tc.curve})
		if !ok {
			t.Fatalf("@/%s/ has no shaft", tc.curve)
		}
		p, _, _ := sh.at(sh.length() / 2)
		off := p.y
		if tc.want == "up" {
			off = -off
		}
		if off < xyCurveDefault-0.2 || off > xyCurveDefault+0.2 {
			t.Errorf("@/%s/ bows %.2fpt (%s is positive), want %.2f %s",
				tc.curve, off, tc.want, xyCurveDefault, tc.want)
		}
	}
}

// A curve is drawn as a chain of short straight pieces, because a formula's SVG
// is a fill-only vocabulary in both drivers (see xymatrix.go).
func TestACurveIsDrawnAsManyPieces(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	straight, ok := e.makeXymatrix("", `A \ar[rr] & & B`, "")
	if !ok {
		t.Fatal("the straight one did not draw")
	}
	curved, ok := e.makeXymatrix("", `A \ar@/^/[rr] & & B`, "")
	if !ok {
		t.Fatal("the curved one did not draw")
	}
	if n := xyShaftPieces(straight.svg); n != 1 {
		t.Errorf("a straight shaft is %d pieces, want 1", n)
	}
	if n := xyShaftPieces(curved.svg); n < 8 {
		t.Errorf("a curved shaft is %d pieces, want a chain", n)
	}
}

// A label ON the line breaks it: XY-pic's | is \PATHlabelbreak@, which drops the
// label and then \Cbreak@@ (xy.tex) — the rule stops clear of the label's box
// instead of running under it.
func TestALabelOnTheLineBreaksIt(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	plain, ok := e.makeXymatrix("", `A \ar[rr] & & B`, "")
	if !ok {
		t.Fatal("the plain one did not draw")
	}
	broken, ok := e.makeXymatrix("", `A \ar[rr]|x & & B`, "")
	if !ok {
		t.Fatal("the broken one did not draw")
	}
	if got, want := xyShaftPieces(broken.svg), xyShaftPieces(plain.svg)+1; got != want {
		t.Errorf("the broken shaft is in %d pieces, want %d", got, want)
	}
}

// xyShaftPieces counts the filled quadrilaterals in a picture — the shaft's
// straight runs. A head is a triangle (three points) and is not counted.
func xyShaftPieces(svg string) int {
	n := 0
	for _, m := range regexp.MustCompile(`<path d="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		if strings.Count(m[1], "L") == 3 {
			n++
		}
	}
	return n
}

// A label is set in script style — XY-pic's \labelstyle is \scriptstyle
// (xyarrow.tex) against \objectstyle = \textstyle for the entries — so an f
// beside an arrow is smaller than the A it leaves.
func TestALabelIsSmallerThanAnEntry(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	label := e.makeMath(`\scriptstyle f`, false)
	entry := e.makeMath(`f`, false)
	if label.width == 0 || entry.width == 0 {
		t.Skip("no maths renderer in this build")
	}
	if label.width >= entry.width {
		t.Errorf("a label is %d wide and an entry %d — the label is not in script style",
			label.width, entry.width)
	}
}
