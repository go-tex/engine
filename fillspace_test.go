package engine

import "testing"

// \hspace and \vspace take a GLUE, not a dimen. The reference reads the argument
// into a SKIP register, which is what keeps the stretch and the shrink:
//
//	\newskip\sp@ce@skip					ltspace.dtx:1412
//	\def\@hspace#1{\setlength\sp@ce@skip{#1}\hskip\sp@ce@skip}	    l.1413
//	\def\@hspace#1{\hskip #1\relax}				    l.1423 (older)
//	\def\@vspace@calcify#1{\setlength\sp@ce@skip{#1}\vskip\sp@ce@skip}  l.450
//
// Both primitives read a rigid dimen here and dropped everything after the natural
// width. Silently: \hspace{\fill} became a 0pt space, and measured against tectonic
// on a line anchored at both ends, "gauche" and "droite" came out ADJOINING at
// x=188.50 where the reference pushes "droite" to the right margin at 477.48.
//
// ⛔ THE MUTATION THIS TEST EXISTS FOR is putting readBraceDimen back in either
// primitive, or in boxNodeFor. Each case below names a rubber component that a
// dimen scan cannot carry, so every one of the three routes fails on its own.
func TestHspaceKeepsTheRubber(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want glueSpec
	}{
		// \fill, the reference's own order-2 length (ltspace.dtx:1445-1446). This is
		// the case the witness measured.
		{`\hspace{\fill}`, `\setbox0=\hbox{a\hspace{\fill}b}`,
			glueSpec{stretch: unity, stretchOrder: 2}},
		// \stretch{n} multiplies it (l.1454). n=3 so a coefficient of 1 cannot pass.
		{`\hspace{\stretch{3}}`, `\setbox0=\hbox{a\hspace{\stretch{3}}b}`,
			glueSpec{stretch: 3 * unity, stretchOrder: 2}},
		// A FINITE rubber length: nothing infinite involved, so a scan that special
		// cased "fil" and still threw ordinary plus/minus away would be caught here.
		{`\hspace{4pt plus 2pt minus 1pt}`, `\setbox0=\hbox{a\hspace{4pt plus 2pt minus 1pt}b}`,
			glueSpec{width: 4 * unity, stretch: 2 * unity, shrink: unity}},
		// The STARRED form goes down a different branch (it emits a rule and a
		// penalty first), so it gets its own case rather than being assumed.
		{`\hspace*{\fill}`, `\setbox0=\hbox{a\hspace*{\fill}b}`,
			glueSpec{stretch: unity, stretchOrder: 2}},
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(c.src); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		g := findGlue(e.box[0])
		if g == nil {
			t.Errorf("%s: aucune glue produite", c.name)
			continue
		}
		if g.spec != c.want {
			t.Errorf("%s: glue = %+v, want %+v", c.name, g.spec, c.want)
		}
	}
	// ⛔ THE CONTROL. A rigid \hspace must stay rigid: if this came back with any
	// stretch, the cases above would be passing because the reader invented rubber,
	// not because it preserved it.
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{a\hspace{4pt}b}`); err != nil {
		t.Fatal(err)
	}
	if g := findGlue(e.box[0]); g == nil || g.spec != (glueSpec{width: 4 * unity}) {
		t.Errorf("témoin rigide: glue = %+v, want 4pt sans ressort", g)
	}
}

// ⛔⛔ THE PARAGRAPH ROUTE, which the box cases above DO NOT REACH.
//
// \hspace inside \hbox{…} is read by boxNodeFor; \hspace in running text is read by
// the primitive itself. They are two call sites with two separate scans, and the
// mutation was run on each: putting readBraceDimen back in boxNodeFor failed four
// cases above, and putting it back in the PRIMITIVE passed the entire suite —
// 12.7s of tests, nothing. The route the witness measured against tectonic was the
// one with no test on it at all.
//
// So this case exists for that mutation alone. A paragraph, the line it produces,
// and the glue inside that line.
func TestHspaceInRunningTextKeepsTheRubber(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`gauche\hspace{\fill}droite\par`); err != nil {
		t.Fatal(err)
	}
	line := firstLineBox(e.mvl)
	if line == nil {
		t.Fatal("aucune ligne produite")
	}
	// ⛔ "An infinite glue in the line" is NOT the test. Every line ends in
	// \parfillskip, which is 0pt plus 1fil — order ONE — so a search for
	// stretchOrder > 0 finds the PARAGRAPH'S OWN glue and never looks at \hspace.
	// Run against the defect it reported order 1 with a straight face. The subject
	// is order TWO, which is \fill (0pt plus 1FILL, ltspace.dtx:1446) and which
	// nothing else on this line produces.
	var found *glueNode
	for _, n := range line.list {
		if g, ok := n.(glueNode); ok && g.spec.stretchOrder == 2 {
			gg := g
			found = &gg
			break
		}
	}
	if found == nil {
		t.Fatalf("aucune glue d'ordre 2 dans la ligne: \\hspace{\\fill} a perdu son "+
			"ressort, les deux mots sont accolés (ligne = %d nœuds)", len(line.list))
	}
	if found.spec.stretch != unity {
		t.Errorf("glue de \\hspace{\\fill} = %+v, want 1fill", found.spec)
	}
}

// The same for \vspace, through \@vspace@calcify (ltspace.dtx:450, 1152-1183).
//
// ⛔ \vspace{\stretch{1}} did not merely lose its stretch: it ABORTED with
// "Extra }, or forgotten \endgroup", because the dimen scan stopped at "0pt" and met
// "plus" where it wanted the closing brace. \stretch ends in \relax by construction
// (l.1454) and the reference never meets a brace after it — \setlength\sp@ce@skip{#1}
// is a macro, so #1's braces are gone before the assignment scans. readBraceGlue has
// to absorb that \relax itself, which is the second half of this fix.
func TestVspaceKeepsTheRubberAndSurvivesStretch(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want glueSpec
	}{
		{`\vspace{\fill}`, `\setbox0=\vbox{\hbox{a}\vspace{\fill}\hbox{b}}`,
			glueSpec{stretch: unity, stretchOrder: 2}},
		{`\vspace{\stretch{2}}`, `\setbox0=\vbox{\hbox{a}\vspace{\stretch{2}}\hbox{b}}`,
			glueSpec{stretch: 2 * unity, stretchOrder: 2}},
		{`\vspace{5pt plus 3pt}`, `\setbox0=\vbox{\hbox{a}\vspace{5pt plus 3pt}\hbox{b}}`,
			glueSpec{width: 5 * unity, stretch: 3 * unity}},
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(c.src); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		g := findGlue(e.box[0])
		if g == nil {
			t.Errorf("%s: aucune glue produite", c.name)
			continue
		}
		if g.spec != c.want {
			t.Errorf("%s: glue = %+v, want %+v", c.name, g.spec, c.want)
		}
	}
}

// \enskip, \fill and \samepage were absent from this format while ltspace.dtx
// documents all three, and each is a DOCUMENT command rather than an internal:
//
//	\def\enskip{\hskip.5em\relax}		l.1501
//	\newskip\fill  \fill = 0pt plus 1fill	l.1445-1446
//	\DeclareRobustCommand\samepage{…}	l.314-322
//
// So strict mode aborted the compilation on each, and -lenient dropped them, which
// for \enskip and \fill glues the words on either side together — corrupt output
// rather than a missing feature.
//
// ⛔ The planted control is \hfill, which was already right (477.50 against the
// reference's 477.48 in the witness): a run where everything came back undefined
// would convict the test, not the format.
func TestDocumentSpacingCommandsAreDefined(t *testing.T) {
	for _, c := range []struct {
		name, src string
		wantErr   bool
	}{
		{`\enskip`, `\enskip`, false},
		{`\fill`, `\newskip\s\s=\fill`, false},
		{`\stretch`, `\newskip\s\s=\stretch{1}`, false},
		{`\samepage`, `\samepage`, false},
		// \samepage sets nine penalty registers, four of them LaTeX's own
		// (\@beginparpenalty, \@endparpenalty, \@itempenalty, \@secpenalty) plus
		// \@M. grep found those only inside COMMENTS here, which reads as "not
		// implemented"; setting each one compiled clean. This case pins that: if any
		// of them is ever lost, \samepage above fails for a reason that would
		// otherwise be invisible, and this line says which.
		{"les registres de \\samepage", `\@beginparpenalty\@M \@endparpenalty\@M ` +
			`\@itempenalty\@M \@secpenalty\@M \interfootnotelinepenalty\@M`, false},
		// THE CONTROL: a name nothing defines must still be an error, or this test
		// would pass against an engine that silently accepted everything.
		{"nom non défini", `\gotexNoSuchSpacingCommand`, true},
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		_, err := e.Run(c.src)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr = %v", c.name, err, c.wantErr)
		}
	}
}
