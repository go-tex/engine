// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \usepackage[french]{babel}: the captions. Witness testdata/parity/babelfrench.tex,
// compiled by tectonic 0.17.0 with 0 errors; the engine printed "Abstract",
// "Contents", "List of Figures" and "List of Tables" for it.
func TestParityBabelFrenchCaptions(t *testing.T) {
	_, got, want := parityPair(t, "babelfrench")
	if got != want {
		t.Errorf("page text differs from tectonic's\n got: %q\nwant: %q", got, want)
	}
}

// The high punctuation: the text of the witness is the same with the spaces dropped,
// and the spaces themselves are asserted below, where they can be measured.
func TestParityBabelFrenchPunctuationText(t *testing.T) {
	_, got, want := parityPair(t, "babelfrenchpunct")
	if got != want {
		t.Errorf("page text differs from tectonic's\n got: %q\nwant: %q", got, want)
	}
}

// parListOf runs a one-paragraph document and returns its nodes, flattened out of
// the line boxes (the paragraph fits on one line, so no glue is discarded at a break).
func parListOf(t *testing.T, opts, body string) []node {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\usepackage` + opts + `{babel}\begin{document}\noindent ` + body + `\par`); err != nil {
		t.Fatal(err)
	}
	var flat []node
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			if b, ok := n.(*boxNode); ok {
				walk(b.list)
				continue
			}
			flat = append(flat, n)
		}
	}
	walk(e.mvl)
	return flat
}

// markGap returns what sits between the character before the n-th occurrence of mark
// and the mark: the nodes, in order.
func markGap(list []node, mark rune) (gap []node, ok bool) {
	for i, n := range list {
		c, isChar := n.(charNode)
		if !isChar || c.ch != mark {
			continue
		}
		j := i
		for j > 0 {
			if _, isChar := list[j-1].(charNode); isChar {
				break
			}
			j--
		}
		return list[j:i], true
	}
	return nil, false
}

// What tectonic does, measured on its PDF with pdftotext -bbox (witness
// babelfrenchpunct.tex, 10pt): the gap before ":" is 3.318pt, EXACTLY the interword
// gap (3.318pt after "x."), and the gap before "!" and "?" is 1.644pt and 1.664pt,
// half of it. A space the author typed ("Slurm :" and "Slurm:" give the same gap)
// is replaced, not doubled. spMock's space is 3pt, so the glue must be 3pt and 1.5pt.
func TestFrenchHighPunctuationSpacing(t *testing.T) {
	const pt = unity
	for _, c := range []struct {
		name, body string
		mark       rune
		width      int
	}{
		{"colon, none typed", `Slurm: x`, ':', 3 * pt},
		{"colon, space typed", `Slurm : x`, ':', 3 * pt},
		{"exclamation", `Vraiment! x`, '!', 3 * pt / 2},
		{"question, space typed", `Ou non ? x`, '?', 3 * pt / 2},
		{"semicolon", `Oui; x`, ';', 3 * pt / 2},
	} {
		gap, ok := markGap(parListOf(t, `[french]`, c.body), c.mark)
		if !ok {
			t.Fatalf("%s: no %q in the paragraph", c.name, c.mark)
		}
		if len(gap) != 2 {
			t.Errorf("%s: %d nodes before the mark, want a penalty and one glue: %#v", c.name, len(gap), gap)
			continue
		}
		p, isPen := gap[0].(penaltyNode)
		g, isGlue := gap[1].(glueNode)
		if !isPen || p.penalty != 10000 || !isGlue {
			t.Errorf("%s: gap is %#v, want \\penalty10000 then glue", c.name, gap)
			continue
		}
		if g.spec.width != c.width {
			t.Errorf("%s: glue width %d sp, want %d", c.name, g.spec.width, c.width)
		}
		if c.mark != ':' && (g.spec.stretch != 0 || g.spec.shrink != 0) {
			t.Errorf("%s: the thin space must not stretch or shrink: %#v", c.name, g.spec)
		}
	}
}

// ⛔ THE CONTROLS. Every row above would pass if the paragraph always carried a
// penalty and a glue before a mark; these are the cases where french.ldf adds none.
func TestFrenchHighPunctuationControls(t *testing.T) {
	for _, c := range []struct {
		name, opts, body string
		mark             rune
		nodes            int // nodes between the previous character and the mark
	}{
		{"english babel", `[english]`, `Slurm: x`, ':', 0},
		{"no babel language", ``, `Slurm: x`, ':', 0},
		{"the last language is the main one", `[french,english]`, `Slurm: x`, ':', 0},
		{"after a bracket", `[french]`, `(a [!`, '!', 0},
		{"after another mark", `[french]`, `a?!`, '!', 0},
		{"typewriter", `[french]`, `\texttt{a: b}`, ':', 0},
	} {
		gap, ok := markGap(parListOf(t, c.opts, c.body), c.mark)
		if !ok {
			t.Errorf("%s: no %q in the paragraph", c.name, c.mark)
			continue
		}
		if len(gap) != c.nodes {
			t.Errorf("%s: %d nodes before the mark, want %d: %#v", c.name, len(gap), c.nodes, gap)
		}
	}
	// …and french is the main language when it comes last.
	if gap, ok := markGap(parListOf(t, `[english,french]`, `Slurm: x`), ':'); !ok || len(gap) != 2 {
		t.Errorf("[english,french]: gap %#v, want penalty and glue", gap)
	}
}
