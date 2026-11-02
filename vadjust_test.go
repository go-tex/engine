package engine

import "testing"

// \vadjust{…}: vertical material written inside a paragraph, which comes out into the
// enclosing vertical list AFTER the line it rode on.
//
// tex.web §2994-3001 (an adjust_node occurs only in horizontal lists), §12900-12903
// (hpack removes it, so it contributes no width), §17413-17418 (append the line box
// FIRST, then the adjustment), §21195 (forbidden in vertical mode),
// §21199-21231 (internal vertical mode, then the node carries the LIST).
//
// Measured against tectonic on a three-line paragraph with \vadjust{\vskip 30pt} on
// line one — baselines, oracle then ours:
//
//	126.735  168.578  180.533      pas 41.843 puis 11.955
//	127.149  168.994  180.949      pas 41.845 puis 11.956
//
// and a control paragraph with no \vadjust at 11.955 / 11.956 throughout. The 30pt
// falls between lines ONE and TWO in both engines.
//
// ⛔ THE ORDER IS THE WHOLE QUESTION, which is why the assertion is on POSITIONS in
// the vertical list and not on the glue's presence. Emitting the adjustment before the
// line box would put a \vadjust's space ABOVE the line it was written on, and a test
// that only asked "is there a 30pt glue?" would pass either way.
func TestVadjustMaterialFollowsTheLineItRodeOn(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// Enough words that the paragraph is several lines, with the \vadjust on the first.
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`aaaa bbbb\vadjust{\vskip 30pt} cccc dddd eeee ffff gggg hhhh iiii jjjj ` +
		`kkkk llll mmmm nnnn oooo pppp qqqq rrrr ssss tttt uuuu vvvv wwww xxxx\par`); err != nil {
		t.Fatal(err)
	}
	// Find the 30pt glue and the first line box in the vertical list.
	glueAt, firstBoxAt, boxes := -1, -1, 0
	for i, n := range e.mvl {
		switch c := n.(type) {
		case glueNode:
			if c.spec.width == 30*unity && glueAt < 0 {
				glueAt = i
			}
		case *boxNode:
			if len(c.list) > 0 {
				boxes++
				if firstBoxAt < 0 {
					firstBoxAt = i
				}
			}
		}
	}
	if boxes < 2 {
		t.Fatalf("%d ligne(s): le \\vadjust doit être sur une ligne qui N'EST PAS la "+
			"dernière, sinon « après la ligne » et « après le paragraphe » sont le "+
			"même endroit et le test ne distingue rien", boxes)
	}
	if glueAt < 0 {
		t.Fatal("aucune glue de 30pt dans la liste verticale: le \\vadjust n'est pas sorti")
	}
	if glueAt < firstBoxAt {
		t.Errorf("la glue est en position %d et la première ligne en %d — la matière "+
			"du \\vadjust est AU-DESSUS de sa ligne", glueAt, firstBoxAt)
	}
}

// ⛔ The node contributes NO WIDTH to the line it rides in (§12900-12903: hpack removes
// it). A \vadjust carrying 30pt of vertical space must not make its line 30pt wider,
// nor shift the words after it.
//
// The control is the same text without the \vadjust: the first line's width must be
// identical.
func TestVadjustAddsNoWidthToItsLine(t *testing.T) {
	width := func(src string) int {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}` + src + `\par`); err != nil {
			t.Fatal(err)
		}
		for _, n := range e.mvl {
			if b, ok := n.(*boxNode); ok && b != nil && len(b.list) > 0 {
				return b.width
			}
		}
		t.Fatalf("%q: aucune ligne", src)
		return 0
	}
	with := width(`aaaa bbbb\vadjust{\vskip 30pt} cccc`)
	without := width(`aaaa bbbb cccc`)
	if with != without {
		t.Errorf("largeur avec \\vadjust = %d, sans = %d — le nœud doit être retiré "+
			"avant l'empaquetage", with, without)
	}
}

// ⛔⛔ \vadjust IS AN ERROR IN VERTICAL MODE, not a no-op: §21195 puts `vmode+vadjust`
// in TeX's `@<Forbidden…@>` list. Accepting it between paragraphs would quietly make
// \vadjust a synonym for "contribute", which is a different command with the same
// spelling — and a document relying on that would break on real TeX.
//
// The control is the same \vadjust inside a paragraph, which must compile.
func TestVadjustIsForbiddenInVerticalMode(t *testing.T) {
	for _, c := range []struct {
		name, src string
		wantErr   bool
	}{
		{"entre deux paragraphes", `AAA\par\vadjust{\vskip 10pt}\par BBB`, true},
		{"dans un paragraphe", `AAA\vadjust{\vskip 10pt} BBB`, false},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		_, err := e.Run(`\documentclass{article}\begin{document}` + c.src + `\par`)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr = %v", c.name, err, c.wantErr)
		}
	}
}

// ⛔⛔ AND THIS IS WHAT \pagebreak NEEDED IN HORIZONTAL MODE. ltspace.dtx:348-355:
//
//	\def\@no@pgbk #1[#2]{%
//	  \ifvmode \penalty #1\@getpen{#2}%
//	  \else \@bsphack \vadjust{\penalty #1\@getpen{#2}}\@esphack \fi}
//
// #667 shipped the vertical branch and left this one undone on purpose: written
// straight into the horizontal list the penalty would be a LINE-break penalty, the
// same number meaning a different thing in a different list. Measured on a short first
// paragraph followed by `\pagebreak` ON THE LINE: the reference makes TWO pages, we
// made ONE, and we now make TWO.
//
// The assertion is on which LIST the penalty reaches, because that is the defect: a
// line-break penalty of −10000 would also "work" on some documents, for the wrong
// reason.
func TestPagebreakInHorizontalModeReachesTheVerticalList(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`AAAA\pagebreak\par BBBB\par`); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range e.mvl {
		if p, ok := n.(penaltyNode); ok && p.penalty == -10000 {
			found = true
		}
	}
	if !found {
		t.Errorf("pénalité −10000 absente de la liste VERTICALE: %d nœuds, "+
			"le \\pagebreak n'a pas traversé", len(e.mvl))
	}
	// ⛔ THE CONTROL: the same paragraph without \pagebreak must not carry one.
	c := New()
	if err := c.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	c.SetFont(spMock{})
	if _, err := c.Run(`\documentclass{article}\begin{document}AAAA\par BBBB\par`); err != nil {
		t.Fatal(err)
	}
	for _, n := range c.mvl {
		if p, ok := n.(penaltyNode); ok && p.penalty == -10000 {
			t.Error("témoin sans \\pagebreak: une pénalité −10000 est quand même là")
		}
	}
}

// ⛔⛔ AND THIS IS WHAT \\* NEEDED ALL ALONG. The reference's \@gnewline is
//
//	\unskip \reserved@e{\reserved@f#1} \nobreak \hfil \break
//
// where \reserved@e is \vadjust and \reserved@f is \nobreak for the starred form
// (ltspace.dtx:407-412, 485-492, 517-522). So `\\*` forbids its page break with a
// \nobreak INSIDE the \vadjust: the penalty lands in the VERTICAL list after the line,
// where 10000 is a page-break refusal. Outside, in the horizontal list, the same
// number is a LINE-break refusal — one value meaning two things in two lists, which is
// why this could not be done before \vadjust existed.
//
// #662 shipped with that half named as undone; it is done here.
func TestStarredCrForbidsAPageBreakAfterItsLine(t *testing.T) {
	pens := func(src string) []int {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}` + src + `\par`); err != nil {
			t.Fatal(err)
		}
		var out []int
		for _, n := range e.mvl {
			if p, ok := n.(penaltyNode); ok {
				out = append(out, p.penalty)
			}
		}
		return out
	}
	hasForbid := func(ps []int) bool {
		for _, p := range ps {
			if p == 10000 {
				return true
			}
		}
		return false
	}
	if ps := pens(`AAA\\* BBB`); !hasForbid(ps) {
		t.Errorf("\\\\* : pénalité +10000 absente de la liste VERTICALE (%v) — rien "+
			"n'interdit la coupure de page après cette ligne", ps)
	}
	// ⛔ THE CONTROL: a plain \\ must NOT forbid it. Without this the assertion above
	// would pass against an engine that refuses page breaks after every line break.
	if ps := pens(`AAA\\ BBB`); hasForbid(ps) {
		t.Errorf("\\\\ nue: pénalité +10000 présente (%v) — une coupure de ligne "+
			"ordinaire n'interdit pas la coupure de page", ps)
	}
}
