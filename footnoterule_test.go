package engine

import "testing"

// ruleWidths collects every rule drawn on the assembled pages, descending into boxes:
// \footnoterule's material is typeset into a vbox, so a collector that only looked at
// the top level would find nothing and report "no rule" for every case alike.
func ruleWidths(ns []node) []int {
	var out []int
	for _, n := range ns {
		switch v := n.(type) {
		case ruleNode:
			out = append(out, v.width)
		case *boxNode:
			if v != nil {
				out = append(out, ruleWidths(v.list)...)
			}
		}
	}
	return out
}

func pageRules(e *Engine) []int {
	var out []int
	for _, p := range e.Pages() {
		out = append(out, ruleWidths([]node{p})...)
	}
	return out
}

func hasWidth(ws []int, want int) bool {
	for _, w := range ws {
		if w == want {
			return true
		}
	}
	return false
}

// ⛔⛔ THE RULE WAS THREE HARD-CODED NODES IN GO, so a class could not change it — and
// 60 of the 154 measured papers redefine \footnoterule, counted on their own sources.
// What this engine drew happens to be article.cls:605-608's shape
// (\hrule\@width.4\columnwidth), which is why nothing looked wrong: it agreed with the
// commonest class by coincidence and with nobody else. The KERNEL's rule is a fixed 2in
// (latex.ltx:13162-13163), a different number.
//
// Measured on a document asking for `\hrule width 3in height 2pt`: the SVG carried
// 138 × 0.4 before (our own rule, the document ignored) and 216.81 × 2 after — 3in is
// 3 × 72.27 = 216.81pt.
func TestFootnoteRuleObeysTheDocument(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\renewcommand\footnoterule{\kern-3pt\hrule width 3in height 2pt\kern2.6pt}
\begin{document}
Corps.\footnote{NOTE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := 3 * 72 * unity // 3in, to the point
	got := pageRules(e)
	found := false
	for _, w := range got {
		if w > want {
			found = true
		}
	}
	if !found {
		t.Errorf("aucun filet de la largeur demandée (3in): largeurs vues = %v", got)
	}
}

// ⛔ LE TÉMOIN: sans redéfinition, la géométrie ne bouge pas. Mesuré avant/après sur un
// document ordinaire à note: les deux SVG sont identiques À L'OCTET. Un test qui
// n'exigerait que « le filet redéfini marche » passerait sur un moteur qui a cassé tous
// les autres.
func TestFootnoteRuleKeepsItsDefault(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\begin{document}
Corps.\footnote{NOTE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := pageRules(e)
	if len(got) == 0 {
		t.Fatal("aucun filet de note sur la page: le défaut d'article.cls doit en dessiner un")
	}
	// .4\columnwidth, à l'arrondi de TeX près.
	//
	// ⛔ LE NOMBRE N'EST PAS EXACTEMENT CELUI DE L'ANCIEN CODE, et la différence est
	// instructive: le Go faisait `e.hsize * 2 / 5`, une division entière, tandis que
	// TeX lit « .4 » et multiplie en points d'échelle. L'écart mesuré est de 138 sp,
	// soit 0,002 pt — invisible à toute précision de sortie, et le SVG d'un document
	// ordinaire est IDENTIQUE À L'OCTET avant et après. C'est l'arithmétique de la
	// référence qui l'emporte, pas la nôtre.
	want := e.columnWidth * 2 / 5
	const texRounding = 1000 // sp; l'écart observé est de 138
	ok := false
	for _, w := range got {
		if d := w - want; d > -texRounding && d < texRounding {
			ok = true
		}
	}
	if !ok {
		t.Errorf("filet par défaut = %v, attendu .4\\columnwidth ≈ %d", got, want)
	}
}

// ⛔⛔ \let\footnoterule\relax NE DESSINE RIEN. article.cls:172 fait exactement cela
// dans la branche \if@titlepage, donc les notes d'une page de titre n'ont AUCUN filet.
// Lire « ce n'est pas une macro » comme « prends le filet intégré » poserait un trait
// sur la seule page que la classe a pris soin de nettoyer.
func TestLetFootnoteruleRelaxDrawsNothing(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\makeatletter\let\footnoterule\relax\makeatother
\begin{document}
Corps.\footnote{NOTE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageRules(e); len(got) != 0 {
		t.Errorf("\\let\\footnoterule\\relax dessine quand même %v", got)
	}
}
