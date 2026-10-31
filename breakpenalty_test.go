package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ THE FOUR BREAK COMMANDS WERE COMPLETE NO-OPS.
//
// They were \@ifnextbracket\@gobbleoptonly\relax, so `un deux\linebreak trois` came out
// as the single word "deuxtrois": with no break and no glue — the space after a control
// word is absorbed by the tokeniser, in the reference too — the two words landed
// adjacent. 18 of the 154 measured corpus papers use one of the four.
//
// The reference reduces all four to a penalty, and the SIGN is the whole difference
// (ltspace.dtx:294-306, 344-375, 527-531):
//
//	\pagebreak   → \@testopt{\@no@pgbk-}4  → \penalty -\@getpen{4} = -10000
//	\nopagebreak → \@testopt\@no@pgbk4     → \penalty +\@getpen{4} = +10000
//	\@getpen#1   → \ifcase: 0, \@lowpenalty, \@medpenalty, \@highpenalty, else \@M
//
// ⛔ THE MUTATION THIS EXISTS FOR is putting \@ifnextbracket\@gobbleoptonly\relax back
// on any of the four: with no penalty contributed, every row below fails.
//
// ⛔ The three penalty VALUES are read out of the engine first (\the\@lowpenalty and
// friends print 51, 151, 301 — the reference's defaults) rather than hard-coded from
// the reference, because a test that hard-codes them asserts latex.ltx's numbers
// against whatever this engine holds, and would pass or fail for the wrong reason.
func TestPageBreakContributesThePenaltyWithTheRightSign(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      int
	}{
		{`\pagebreak`, `\pagebreak`, -10000},
		{`\pagebreak[4]`, `\pagebreak[4]`, -10000},
		{`\pagebreak[1]`, `\pagebreak[1]`, -51},
		{`\pagebreak[2]`, `\pagebreak[2]`, -151},
		{`\pagebreak[3]`, `\pagebreak[3]`, -301},
		// ⛔ Degree 0 is "no opinion": \z@, and the SIGN cannot show on a zero. It is
		// here because it is the one argument that must NOT force a break — the
		// witness document checks the page count stays 2 with a \pagebreak[0] in it.
		{`\pagebreak[0]`, `\pagebreak[0]`, 0},
		{`\nopagebreak`, `\nopagebreak`, 10000},
		{`\nopagebreak[2]`, `\nopagebreak[2]`, 151},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		// A paragraph, then the command in VERTICAL mode — the branch this change
		// makes faithful. The horizontal branch needs \vadjust (go-tex/engine#665).
		if _, err := e.Run(`\documentclass{article}\begin{document}` +
			`A\par ` + c.src + ` B\par`); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !hasPenalty(e.mvl, c.want) {
			t.Errorf("%s: pénalité %d absente de la liste verticale; pénalités vues = %v",
				c.name, c.want, penaltiesIn(e.mvl))
		}
	}

	// ⛔ THE CONTROL. Without any of the four commands, none of those penalties may
	// appear — otherwise every row above could be passing on something the paragraph
	// machinery emits on its own.
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}A\par B\par`); err != nil {
		t.Fatal(err)
	}
	for _, p := range []int{-10000, 10000, -51, -151, -301, 151} {
		if hasPenalty(e.mvl, p) {
			t.Errorf("témoin sans commande de coupure: pénalité %d présente quand même "+
				"(%v) — les assertions ci-dessus ne prouveraient rien", p, penaltiesIn(e.mvl))
		}
	}
}

// \linebreak ends the line AND leaves the space. The channel is the number of line
// boxes plus the text of the first one, because the corruption and the missing break
// are two different failures and one assertion cannot see both:
//
//	1 ligne  → la coupure n'a pas eu lieu
//	"deuxtrois" → l'espace a été mangé
//
// ⛔ \lastskip is saved, \unskip removes the glue, the penalty goes in, and the glue is
// put BACK (ltspace.dtx:364-375). The order matters: a \penalty after the interword
// glue is not a legal breakpoint for it, so \linebreak would do nothing at all without
// the \unskip. Both halves are asserted here.
func TestLineBreakEndsTheLineAndKeepsTheSpace(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`un deux\linebreak trois quatre\par`); err != nil {
		t.Fatal(err)
	}
	lines := lineBoxesOf(e.mvl)
	if len(lines) < 2 {
		t.Fatalf("%d ligne(s), want au moins 2: \\linebreak n'a pas coupé", len(lines))
	}
	// "un deuxtrois" is the defect: no break, and nothing between deux and trois
	// because the control word absorbed the space. "un" alone would mean the break
	// fell before the glue instead of after it.
	if got := boxText(lines[0]); got != "un deux" {
		t.Errorf("première ligne = %q, want \"un deux\"", got)
	}
	if got := boxText(lines[1]); !strings.HasPrefix(got, "trois") {
		t.Errorf("deuxième ligne = %q, want commençant par trois", got)
	}
}

// ⛔⛔ A CONTROL WORD ABSORBS THE SPACE AFTER IT, AND THE REFERENCE DOES THE SAME.
//
// This test first wrote `cinq six\nolinebreak sept huit` and asserted "cinq six sept
// huit". It failed with the fix in place, which looked like the fix not working. It was
// the SOURCE that was wrong: TeX's tokeniser swallows the spaces following a control
// word, so there is no glue there to keep. Asked directly, tectonic joins "sixsept"
// too, and separates them only with the braces:
//
//	cinq six\nolinebreak sept huit     → cinq  sixsept  huit   (oracle AND ours)
//	cinq six\nolinebreak{} sept huit   → cinq  six  sept  huit (oracle AND ours)
//
// So both forms are here, and the first one asserts the JOIN. Asserting a space there
// would have been a test demanding a divergence from the reference — and it would have
// been "fixed" by making the engine wrong.
//
// ⛔ This also retracts a claim made while diagnosing: the "deuxtrois" that
// \@gobbleoptonly produced was NOT an eaten space. The space was absorbed by the
// control-word rule either way; the two words landed adjacent because no BREAK
// happened and nothing separated them.
func TestNoLineBreakFollowsTheControlWordSpaceRule(t *testing.T) {
	for _, c := range []struct {
		name, src, want string
	}{
		{"sans accolades: l'espace est absorbé, comme chez l'oracle",
			`cinq six\nolinebreak sept huit`, "cinq sixsept huit"},
		{"avec {}: l'absorption est arrêtée, comme chez l'oracle",
			`cinq six\nolinebreak{} sept huit`, "cinq six sept huit"},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}` +
			c.src + `\par`); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		lines := lineBoxesOf(e.mvl)
		if len(lines) != 1 {
			t.Errorf("%s: %d ligne(s), want 1: \\nolinebreak ne doit pas couper",
				c.name, len(lines))
			continue
		}
		if got := boxText(lines[0]); got != c.want {
			t.Errorf("%s: ligne = %q, want %q", c.name, got, c.want)
		}
	}
	// ⛔ And the penalty must be there, FORBIDDING: without this the two rows above
	// would pass against a \nolinebreak that does nothing at all, which is the defect
	// being fixed. +\@getpen{4} = +10000.
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`cinq six\nolinebreak{} sept huit\par`); err != nil {
		t.Fatal(err)
	}
	lines := lineBoxesOf(e.mvl)
	if len(lines) != 1 {
		t.Fatalf("%d ligne(s), want 1", len(lines))
	}
	if !hasPenalty(lines[0].list, 10000) {
		t.Errorf("pénalité +10000 absente de la ligne: \\nolinebreak n'interdit rien; "+
			"pénalités vues = %v", penaltiesIn(lines[0].list))
	}
}

func hasPenalty(list []node, want int) bool {
	for _, p := range penaltiesIn(list) {
		if p == want {
			return true
		}
	}
	return false
}

func penaltiesIn(list []node) []int {
	var out []int
	for _, n := range list {
		if p, ok := n.(penaltyNode); ok {
			out = append(out, p.penalty)
		}
	}
	return out
}

// lineBoxesOf returns the non-empty boxes of a vertical list: the lines.
func lineBoxesOf(mvl []node) []*boxNode {
	var out []*boxNode
	for _, n := range mvl {
		if b, ok := n.(*boxNode); ok && b != nil && len(b.list) > 0 {
			out = append(out, b)
		}
	}
	return out
}

// boxText renders a box's characters AND its non-empty glue, as a space.
//
// ⛔⛔ THE FIRST VERSION DROPPED THE GLUE, AND THE \nolinebreak TEST THEN PASSED UNDER
// THE MUTATION. Concatenating only characters, "cinq six" and "cinqsix" are the SAME
// STRING, so a test about which words are joined could not see anything at all. The
// only reason the \linebreak test caught the mutation was the line COUNT, a different
// channel. Whether two words are joined is a question about a GLUE NODE, so the glue
// has to be in the channel.
func boxText(b *boxNode) string {
	var sb strings.Builder
	var walk func([]node)
	walk = func(l []node) {
		for _, n := range l {
			switch c := n.(type) {
			case charNode:
				sb.WriteRune(c.ch)
			case glueNode:
				// Only real space. \leftskip/\rightskip of width 0 and the glue a
				// line is packed with would otherwise add spaces nothing wrote.
				if c.spec.width > 0 {
					sb.WriteByte(' ')
				}
			case *boxNode:
				if c != nil {
					walk(c.list)
				}
			}
		}
	}
	walk(b.list)
	return sb.String()
}
