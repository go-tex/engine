package engine

import "testing"

// A top-level paragraph of measured words breaks into multiple lines of \hsize,
// each packed to the line width, stacked with interline glue.
func TestParagraphBreaksIntoLines(t *testing.T) {
	e := New()
	e.SetFont(spMock{}) // every letter 5pt, space 3pt
	e.hsize = 40 * unity
	e.parindent = 0 // isolate line-wrapping from indentation (tested separately)
	e.baselineskip = 10 * unity
	// six 3-letter words: "www " ×6. Each word 15pt, space 3pt.
	src := `www www www www www www\par`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run: %v", err)
	}
	p := e.Page()
	if p == nil {
		t.Fatal("empty page")
	}
	// Count the line boxes in the main vertical list.
	nLines := 0
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			nLines++
			if b.width != 40*unity {
				t.Errorf("line width %d sp want %d (packed to hsize)", b.width, 40*unity)
			}
		}
	}
	if nLines < 2 {
		t.Fatalf("expected the paragraph to wrap into ≥2 lines, got %d", nLines)
	}
	// Page height must exceed a single line (interline glue + multiple lines).
	if p.height <= 7*unity {
		t.Errorf("page height %d sp too small for a multi-line paragraph", p.height)
	}
}

// \hsize is settable and reads back via \the.
func TestHsizeParam(t *testing.T) {
	e := New()
	got, _ := e.Run(`\hsize=100pt \message{\the\hsize}`)
	if trimNL(got) != "100.0pt" {
		t.Errorf("hsize got %q want 100.0pt", trimNL(got))
	}
}

// wordList builds a horizontal list of spMock glyphs and inter-word glue, the
// material breakSegment works on.
func wordList(e *Engine, s string) []node {
	var l []node
	for _, r := range s {
		if r == ' ' {
			l = append(l, glueNode{spec: e.curFont.spaceSP()})
			continue
		}
		w, h, d := e.curFont.charDimsSP(r)
		l = append(l, charNode{ch: r, width: w, height: h, depth: d})
	}
	return l
}

func countDiscs(list []node) int {
	n := 0
	for _, x := range list {
		if _, ok := x.(discNode); ok {
			n++
		}
	}
	return n
}

// TeX's first pass never looks at a hyphenation point (tex.web §16987): when the
// paragraph can be set within \pretolerance without hyphens, that is the list
// that gets used. Setting \pretolerance negative skips the pass, and the second
// one works on the hyphenated list.
func TestFirstPassSetsTheParagraphWithoutHyphens(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	hl := wordList(e, "hyphenation hyphenation hyphenation")

	// 113pt is exactly two words and the space between them: the first pass sets
	// that line with no stretch at all, well inside \pretolerance.
	list, lines, ok := e.breakSegment(hl, 113, Uniform(113))
	if !ok || len(lines) == 0 {
		t.Fatalf("first pass found nothing: ok=%v lines=%d", ok, len(lines))
	}
	if n := countDiscs(list); n != 0 {
		t.Errorf("the first pass set %d discretionaries; it must not hyphenate at all", n)
	}

	e.count[e.eq["pretolerance"].code] = -1 // \pretolerance<0: straight to the second pass
	list2, _, ok2 := e.breakSegment(hl, 113, Uniform(113))
	if !ok2 {
		t.Fatal("second pass found nothing")
	}
	if countDiscs(list2) == 0 {
		t.Error("with the first pass skipped, the list broken must be the hyphenated one")
	}
}

// A paragraph too tight for \pretolerance falls through to the second pass, which
// hyphenates to fit.
func TestSecondPassHyphenatesWhatTheFirstCannotSet(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	hl := wordList(e, "hyphenation hyphenation")
	list, lines, ok := e.breakSegment(hl, 70, Uniform(70)) // one word (55pt) alone stretches far too much
	if !ok || len(lines) == 0 {
		t.Fatalf("no solution: ok=%v lines=%d", ok, len(lines))
	}
	if countDiscs(list) == 0 {
		t.Fatal("expected the hyphenated list from the second pass")
	}
}

// \leftskip and \rightskip narrow the MEASURE the text is broken against; they
// are not decoration added to a line that was already broken. TeX accounts for
// them before it breaks (tex.web §827, background := left_skip + right_skip).
//
// Breaking against the whole \hsize and gluing them on afterwards makes every
// line overfull by their two widths, and the hpack that follows shrinks the
// inter-word glue to make it fit: the words come out touching and the paragraph
// is set far wider than it asked for. Measured before this: with
// \rightskip=100pt on a 343.73pt measure the text still reached x=463.94, 13.5pt
// short of the margin instead of 100. After: 377.87, which is the margin less
// 100 to a hundredth of a point.
func TestLineSkipsNarrowTheMeasureBeforeBreaking(t *testing.T) {
	const text = `A fairly long subsection title that has to wrap onto a second line to show where the right edge of the measure actually falls when it is set.`
	width := func(src string) float64 {
		t.Helper()
		e, err := compile([]byte(`\documentclass{article}`+"\n"+`\begin{document}`+"\n"+src+"\n"+`\end{document}`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		var widest float64
		for _, p := range e.Pages() {
			// The LINES, not the page: a page with a folio wraps its text block
			// (pageTextList, pagestructure_test.go). Taking p.list here measured
			// the block's own width and reported \rightskip=100pt as widening
			// the text by 199.56pt.
			for _, n := range pageTextList(p) {
				b, ok := n.(*boxNode)
				if !ok {
					continue
				}
				if w := lineInkWidth(b); w > widest {
					widest = w
				}
			}
		}
		return widest
	}
	plain := width(`\noindent ` + text)
	narrowed := width(`{\rightskip=100pt` + "\n" + `\noindent ` + text + `\par}`)
	if plain <= 0 {
		t.Fatal("the unconstrained paragraph drew nothing")
	}
	// 100pt of \rightskip must take 100pt off the text, give or take the word
	// that no longer fits on the line.
	if got := plain - narrowed; got < 80 || got > 120 {
		t.Errorf("\\rightskip=100pt narrowed the text by %.2fpt (from %.2f to %.2f), want about 100",
			got, plain, narrowed)
	}
	// \raggedright's \rightskip is 0pt plus 1fil: infinite stretch, NO width. It
	// stops the lines being justified to the margin; it does not move the margin.
	ragged := width(`{\rightskip=0pt plus 1fil` + "\n" + `\noindent ` + text + `\par}`)
	//
	// ⚠ Under a fil every line has badness 0, so every way of breaking ties on
	// demerits and tex.web §855 keeps the LATER break (linebreak v1.13.0): the
	// widest ragged line is no longer the one that fills the measure (320.60 of
	// 345.00 here). The narrowing this guards against is 100pt, so the tolerance is
	// 50, not the 20 a tie-breaking order that happened to fill lines allowed.
	if ragged < plain-50 {
		t.Errorf("a width-less \\rightskip narrowed the measure to %.2f from %.2f; ragged setting must not move the margin", ragged, plain)
	}

	// ⛔⛔ AND THE WIDTH IS TAKEN OFF ONCE, WHICH THE MARGIN CANNOT SEE.
	//
	// Everything above measures how far right the ink reaches, and #780 passed
	// all of it while counting \leftskip and \rightskip TWICE: the margin was
	// still right — the lines are justified to the narrowed measure either way —
	// but the breaker thought 100pt less would fit, so it packed less text and
	// stretched what it packed. Five lines where the reference sets three, each
	// one stretched to the same correct margin.
	//
	// So the question has to be asked of the TEXT, not the edge. tectonic 0.17.0
	// on this paragraph at \rightskip=100pt sets THREE lines, the first being
	// `A fairly long subsection title that has to wrap onto a` and ending at
	// x1=377.85; we end at 377.87 and split it identically.
	lines := func(src string) int {
		t.Helper()
		e, err := compile([]byte(`\documentclass{article}`+"\n"+`\begin{document}`+"\n"+src+"\n"+`\end{document}`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, p := range e.Pages() {
			for _, nd := range p.list {
				if b, ok := nd.(*boxNode); ok && b.kind == hbox && lineInkWidth(b) > 0 {
					n++
				}
			}
		}
		return n
	}
	// The page number is an hbox too, so the comparison is between the two
	// paragraphs rather than against an absolute count.
	plainLines := lines(`\noindent ` + text)
	narrowLines := lines(`{\rightskip=100pt` + "\n" + `\noindent ` + text + `\par}`)
	if narrowLines > plainLines+1 {
		t.Errorf("\\rightskip=100pt gave %d line(s) against %d unconstrained: more than one "+
			"extra line means the skip's width was taken off the measure TWICE — the "+
			"caller narrows lineWidth AND skipBackground() carried the width (#780, "+
			"against #583's witness, where the reference sets 3 and the double count "+
			"gave 5)", narrowLines, plainLines)
	}
}

// lineInkWidth is how far right the characters of one line reach, which is the
// quantity a reader compares against the margin — not the box's own width, which
// is \hsize whatever the glue inside it does.
func lineInkWidth(b *boxNode) float64 {
	var x, widest float64
	var walk func(nodes []node)
	walk = func(nodes []node) {
		for _, n := range nodes {
			switch v := n.(type) {
			case charNode:
				x += spToPt(v.width)
				widest = x
			case kernNode:
				x += spToPt(v.width)
			case glueNode:
				x += spToPt(b.setWidth(v.spec))
			case *boxNode:
				walk(v.list)
			}
		}
	}
	walk(b.list)
	return widest
}
