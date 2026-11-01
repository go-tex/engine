package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ `\\[20pt]` TYPESET THE FIVE CHARACTERS "[20pt]" AND `\\*` TYPESET A "*".
//
// The optional argument and the star were never scanned, so document markup reached
// the page as content — on 38 and 9 of the 154 measured corpus papers respectively
// (7918 and 1381 in the archive). Measured against tectonic on the witness
// `AAA\\ BBB\\[20pt] CCC\\* DDD`, baselines:
//
//	        oracle    nous
//	AAA     126.735   127.149
//	BBB     138.690   139.105
//	CCC     170.570   170.986
//	DDD     182.526   182.942
//
// a constant 0.41pt apart, with the BBB→CCC step at 31.88 on BOTH sides — one
// \baselineskip (11.96) plus the 20pt. And no "[20pt]" and no "*" anywhere.
//
// ⛔ THE MUTATION THIS EXISTS FOR is dropping either scan from \gotex@cr: the
// characters come back into the line, which the text assertions catch, and the space
// disappears, which the line count and the glue assertion catch. They are separate
// failures and neither assertion sees both.
func TestCrScansItsStarAndItsOptionalLength(t *testing.T) {
	for _, c := range []struct {
		name, src, wantText string
		wantLines           int
	}{
		// The planted control: a bare \\ worked before and must still work.
		{"nue", `AAA\\ BBB`, "AAABBB", 2},
		{"longueur optionnelle", `AAA\\[20pt] BBB`, "AAABBB", 2},
		{"étoile", `AAA\\* BBB`, "AAABBB", 2},
		{"étoile ET longueur", `AAA\\*[20pt] BBB`, "AAABBB", 2},
		// ⛔ A '[' that is NOT an optional argument must survive untouched. LaTeX
		// resolves the ambiguity in favour of the argument, so this is the form where
		// the bracket is separated from the \\ by text — the common prose case.
		{"crochet du TEXTE, pas un argument", `AAA\\ {}[1] BBB`, "AAA[1]BBB", 2},
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
		if len(lines) != c.wantLines {
			t.Errorf("%s: %d ligne(s), want %d", c.name, len(lines), c.wantLines)
			continue
		}
		var got strings.Builder
		for _, b := range lines {
			got.WriteString(boxText(b))
		}
		if g := strings.ReplaceAll(got.String(), " ", ""); g != c.wantText {
			t.Errorf("%s: texte = %q, want %q — un \"[20pt]\" ou un \"*\" ici est du "+
				"balisage composé comme du contenu", c.name, g, c.wantText)
		}
	}
}

// The optional length must actually reach the vertical list, as glue AFTER the line
// the break ends. The reference puts it through \vadjust (ltspace.dtx:485-492), which
// this engine does not have (#665); it rides on the forced penalty instead.
//
// ⛔ THE CONTROL IS THE SAME DOCUMENT WITHOUT THE ARGUMENT. Asserting only that a
// 20pt glue exists would pass against an engine that emits one for every \\.
func TestCrOptionalLengthReachesTheVerticalList(t *testing.T) {
	vskips := func(src string) []int {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}` +
			src + `\par`); err != nil {
			t.Fatal(err)
		}
		var out []int
		for _, n := range e.mvl {
			if g, ok := n.(glueNode); ok && g.spec.width == 20*unity {
				out = append(out, g.spec.width)
			}
		}
		return out
	}
	if n := len(vskips(`AAA\\[20pt] BBB`)); n != 1 {
		t.Errorf("%d glue(s) de 20pt dans la liste verticale, want 1", n)
	}
	if n := len(vskips(`AAA\\ BBB`)); n != 0 {
		t.Errorf("%d glue(s) de 20pt sans argument, want 0 — la glue ne doit pas "+
			"venir du \\\\ lui-même", n)
	}
}

// ⛔⛔ AND THE REASON \\ IS A Go PRIMITIVE: a formula containing one must still render.
//
// Written as \def\\{\@ifstar…}, the maths layer received `\@ifstar` and both of its
// branches and refused every formula with a matrix row separator in it. The capture
// writes a control sequence VERBATIM and never consults its meaning — measured four
// ways, including a primitive that calls fail and is never run — so the scanning
// belongs in Go.
//
// The channel is the formula's BOX, not its text: the engine puts a formula's SOURCE
// in the text layer on purpose, so a text comparison reads our source rather than
// glyph positions. Two stacked rows are TALLER and NARROWER than one row of two.
func TestCrInsideAFormulaStillStacksTheMatrix(t *testing.T) {
	box := func(src string) (w, h, d int) {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}` + src + `\par`); err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		var found *mathNode
		var walk func([]node)
		walk = func(l []node) {
			for _, n := range l {
				switch c := n.(type) {
				case mathNode:
					if found == nil {
						m := c
						found = &m
					}
				case *boxNode:
					if c != nil {
						walk(c.list)
					}
				}
			}
		}
		walk(e.mvl)
		if found == nil {
			t.Fatalf("%s: aucun nœud math — la formule a été refusée", src)
		}
		return found.width, found.height, found.depth
	}
	w2, h2, d2 := box(`$\begin{bmatrix}a\\b\end{bmatrix}$`)
	w1, h1, d1 := box(`$\begin{bmatrix}a&b\end{bmatrix}$`)
	if h2+d2 <= h1+d1 {
		t.Errorf("deux lignes empilées: hauteur %d, une ligne: %d — le séparateur "+
			"n'a pas atteint la couche math", h2+d2, h1+d1)
	}
	if w2 >= w1 {
		t.Errorf("deux lignes empilées: largeur %d, une ligne: %d — deux entrées "+
			"empilées occupent UNE colonne", w2, w1)
	}
}
