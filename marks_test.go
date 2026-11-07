package engine

import (
	"strings"
	"testing"
)

// \mark must reach the vertical list, because that is how it travels to the page
// it belongs to (see markNode).
func TestMarkContributesANode(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}\mark{MARQUE}X\end{document}`); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, n := range e.mvl {
		if _, ok := n.(markNode); ok {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("%d markNode on the vertical list, want 1", found)
	}
}

// ⛔⛔ A \mark IN THE PREAMBLE CONTRIBUTES NOTHING, and the corpus is why.
//
// ws-fnl.cls:804 carries a bare `\mark{{}{}}` at class level, outside the
// document body. With it contributed there, 2405.05720 — which uses that class —
// went from 9 pages to 10 with its ink IDENTICAL on both sides (23282 glyph
// units) against a reference of 9: the one paper the engine had exactly right,
// lost. Σ 272 → 273 and exact 47 → 46 for that alone.
//
// The argument is still scanned, so the tokens are consumed exactly as they would
// be in the body; only the node is withheld.
func TestAMarkInThePreambleContributesNothing(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// The mark sits between \documentclass and \begin{document}, where ws-fnl puts
	// it, and the body then sets one of its own.
	if _, err := e.Run(`\documentclass{article}\mark{{}{}}` +
		`\begin{document}\mark{DANS LE CORPS}X\end{document}`); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, n := range e.mvl {
		if _, ok := n.(markNode); ok {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("%d markNode on the vertical list, want 1 — the preamble's mark was kept", found)
	}
}

// The per-page arithmetic of tex.web:19718-19729. The two cases a plainer reading
// gets wrong are the ones a running head depends on: a page with NO marks keeps
// the previous page's bot (so a head set once stays), and first falls back to top
// (so a page broken mid-section still reports the mark in force at its top).
func TestMarkArithmetic(t *testing.T) {
	txt := func(s string) []tok { return stringToToks(s) }
	flat := func(ts []tok) string {
		var b strings.Builder
		for _, t := range ts {
			b.WriteString(t.String())
		}
		return b.String()
	}
	var m marks

	// Page 1 carries two marks: top is still empty, first is A, bot is B.
	m.advance([][]tok{txt("A"), txt("B")})
	if flat(m.top) != "" || flat(m.first) != "A" || flat(m.bot) != "B" {
		t.Fatalf("page 1: top=%q first=%q bot=%q", flat(m.top), flat(m.first), flat(m.bot))
	}
	// Page 2 carries none: all three become the previous bot.
	m.advance(nil)
	if flat(m.top) != "B" || flat(m.first) != "B" || flat(m.bot) != "B" {
		t.Fatalf("page 2: top=%q first=%q bot=%q", flat(m.top), flat(m.first), flat(m.bot))
	}
	// Page 3 carries one: top is the previous bot, first and bot are it.
	m.advance([][]tok{txt("C")})
	if flat(m.top) != "B" || flat(m.first) != "C" || flat(m.bot) != "C" {
		t.Fatalf("page 3: top=%q first=%q bot=%q", flat(m.top), flat(m.first), flat(m.bot))
	}
}
