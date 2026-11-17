package engine

import "testing"

// ⛔⛔ A PAGE IS PACKED EXACTLY TO \vsize WHATEVER ITS STYLE, AND "empty" WAS PACKED
// NATURALLY. tex.web has no page-style branch in the packer — page styles are the
// output routine's business:
//
//	§19321  page_goal := vsize
//	§19564  best_size := page_goal
//	§19813  box(255) := vpackage(link(page_head), best_size, exactly, page_max_depth)
//
// A naturally packed page is exactly as tall as its material, so every vertical glue
// on it stretches by NOTHING. Under \pagestyle{empty} — which this engine takes as its
// DEFAULT — \vspace{\fill} put its word at y=143.91 against tectonic's 664.72.
//
// ⛔ THE HEIGHT IS ASSERTED, NOT THE PRESENCE OF A PAGE. A test that only checked that
// a page came out passes on both sides of this defect, which is how it survived.
//
// ⛔⛔ AND IT IS box255's HEIGHT, NOT THE SHIPPED PAGE'S. §19813 packs
// `box(255)` — the page CONTENT — and the running foot is not in it: \@outputpage
// puts `\@thefoot` UNDER `\box\@outputbox` (latex.ltx), so a page with a folio
// ships a box that is taller than \vsize by the foot band. This test asserted the
// height of whatever assemblePage returned, which was the same box only because
// the folio was being packed INSIDE the text block — the defect of #721, worth
// 19pt of lost stretch and a folio 30pt high. The invariant below is therefore
// stated of the text block, and the extra row says the foot is outside it.
func TestEveryPageStyleIsPackedToVsize(t *testing.T) {
	// Short material, so natural height and \vsize are far apart and the two answers
	// cannot be confused: one box of 10pt against a 300pt page.
	const body = `\hbox{\vrule width5pt height10pt}`

	for _, style := range []string{"empty", "plain"} {
		e := New()
		e.vsize = 300 * unity
		if _, err := e.Run(`\pagestyle{` + style + `}` + body); err != nil {
			t.Fatalf("%s: %v", style, err)
		}
		pages := e.Pages()
		if len(pages) != 1 {
			t.Fatalf("%s: %d page(s), attendu 1", style, len(pages))
		}
		block := pages[0]
		if style == "plain" {
			// The shipped box is [text block][glue \footskip][foot]. The block is
			// its first box, and the page MUST be taller than it — a page exactly
			// \vsize tall here means the folio went back inside the text block.
			i := firstBoxAt(pages[0].list)
			if i < 0 {
				t.Fatalf("%s: la page shippée ne contient aucune boîte", style)
			}
			block = pages[0].list[i].(*boxNode)
			if pages[0].height <= e.vsize {
				t.Errorf("\\pagestyle{%s}: la page shippée fait %d, pas plus que \\vsize = %d — "+
					"le pied doit être SOUS le bloc de texte, pas dedans (#721)",
					style, pages[0].height, e.vsize)
			}
		}
		if got := block.height; got != e.vsize {
			t.Errorf("\\pagestyle{%s}: hauteur du bloc de texte = %d, attendu \\vsize = %d "+
				"(tex.web §19813 empaquette box255 « exactly » à best_size)",
				style, got, e.vsize)
		}
	}
}

// ⛔ SANS \vsize IL N'Y A PAS DE BUT, et empaqueter à zéro en inventerait un. La hauteur
// naturelle est alors la seule réponse qui n'affirme rien — c'est le repli que la
// branche "plain" a toujours eu, et la correction ci-dessus le garde pour "empty".
func TestWithoutVsizeThePageKeepsItsNaturalHeight(t *testing.T) {
	e := New()
	e.vsize = 0
	if _, err := e.Run(`\pagestyle{empty}\hbox{\vrule width5pt height10pt}`); err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) != 1 {
		t.Fatalf("%d page(s), attendu 1", len(pages))
	}
	if pages[0].height == 0 {
		t.Error("hauteur nulle: sans \\vsize la page doit garder sa hauteur NATURELLE, " +
			"pas être empaquetée à zéro")
	}
}
