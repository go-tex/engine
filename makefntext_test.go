package engine

import (
	"strings"
	"testing"
)

// boxWidths collects every box width on the assembled pages, descending into boxes:
// \@makefntext's box sits inside the footnote insert, inside a line, inside the page,
// so a collector that only looked at the top level would find nothing and report the
// same "no box" for the class's layout and for its absence.
func boxWidths(ns []node) []int {
	var out []int
	for _, n := range ns {
		if v, ok := n.(*boxNode); ok && v != nil {
			out = append(out, v.width)
			out = append(out, boxWidths(v.list)...)
		}
	}
	return out
}

func pageBoxWidths(e *Engine) []int {
	var out []int
	for _, p := range e.Pages() {
		out = append(out, boxWidths([]node{p})...)
	}
	return out
}

// nearWidth answers whether some box is within tol scaled points of want. TeX's own
// arithmetic rounds, so an exact comparison would be a test of the rounding rather than
// of the layout.
func nearWidth(ws []int, want, tol int) bool {
	for _, w := range ws {
		if d := w - want; d > -tol && d < tol {
			return true
		}
	}
	return false
}

// The note's first line in a 10pt article: \footnotesize is 8pt, so 1.8em is 14.4pt.
const fnBoxSP = 943718 // 1.8 * 8 * unity; measured 943720
const fnBoxTol = 1000  // sp — 0.015pt

// ⛔⛔ THE LAYOUT OF A NOTE BELONGS TO THE CLASS. queueFootnoteText put the mark, a
// space and the text in a flat row built in Go, so no class could place the mark — and
// all three base classes right-align it in a 1.8em box:
//
//	\newcommand\@makefntext[1]{\parindent 1em\noindent\hb@xt@1.8em{\hss\@makefnmark}#1}
//
// (article.cls:609-612, report.cls:708, book.cls:714). Every note's text therefore began
// one box-width to the left of the reference's. Measured here: no box of that width
// existed anywhere on the page before, and one of 14.400pt does now.
func TestFootnoteTextIsSetByTheClass(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\begin{document}
Corps.\footnote{NOTE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !nearWidth(pageBoxWidths(e), fnBoxSP, fnBoxTol) {
		t.Errorf("aucune boîte de 1.8em (%d sp) : article.cls:612 en pose une autour de la marque", fnBoxSP)
	}
	// ⛔ ET LE TEXTE EST TOUJOURS LÀ. La note est passée à \@makefntext ENTRE ACCOLADES;
	// un argument mal lu ne se voit pas dans une largeur de boîte, il se voit par la
	// disparition du texte — la panne qu'un test de géométrie seul laisserait passer.
	if got := pageText(e); !strings.Contains(got, "NOTE") {
		t.Error("le texte de la note a disparu en passant par \\@makefntext")
	}
}

// ⛔ LE TÉMOIN NÉGATIF: une classe qui ne pose PAS de boîte n'en fait pas apparaître
// une. Sans lui, le test ci-dessus passerait aussi sur un moteur qui construirait la
// boîte de 1.8em en Go — c'est-à-dire exactement le défaut corrigé, sous un autre nom.
func TestFootnoteTextBoxComesFromTheMacroAndNotFromGo(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\makeatletter\renewcommand\@makefntext[1]{\@makefnmark #1}\makeatother
\begin{document}
Corps.\footnote{NOTE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if nearWidth(pageBoxWidths(e), fnBoxSP, fnBoxTol) {
		t.Error("une boîte de 1.8em apparaît alors que \\@makefntext n'en demande aucune")
	}
	if got := pageText(e); !strings.Contains(got, "NOTE") {
		t.Error("le texte de la note a disparu")
	}
}

// ⛔ ET LE DOCUMENT L'EMPORTE. Même forme que TestFootnoteRuleObeysTheDocument: une
// largeur qu'aucun défaut de ce moteur ne produit, pour que la mesure ne puisse pas
// être satisfaite par hasard.
func TestMakefntextObeysTheDocument(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\makeatletter\renewcommand\@makefntext[1]{\noindent\hb@xt@3in{\hss\@makefnmark}#1}\makeatother
\begin{document}
Corps.\footnote{NOTE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := 3 * 7227 * unity / 100 // 3in = 216.81pt
	if !nearWidth(pageBoxWidths(e), want, fnBoxTol) {
		t.Errorf("la boîte de 3in demandée par le document est absente: largeurs = %v", pageBoxWidths(e))
	}
}

// ⛔ LA MARQUE PASSE PAR \@thefnmark, comme dans la référence (latex.ltx:13179), et
// \@makefnmark est ce qui en fait une boîte (latex.ltx:13168). Un document qui redéfinit
// l'une ou l'autre doit être entendu; ici \@makefnmark encadre la marque, ce qu'aucune
// largeur ne montrerait.
func TestFootnoteMarkReachesTheNoteThroughMakefnmark(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\makeatletter\renewcommand\@makefnmark{[\@thefnmark]}\makeatother
\begin{document}
Corps.\footnote{NOTE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageText(e); !strings.Contains(got, "[1]") {
		t.Errorf("\\@makefnmark redéfini n'a pas reçu \\@thefnmark: texte = %q", got)
	}
}

// ⛔ amsart.cls:1303 écrit \def\@makefntext{\indent\@makefnmark} — SANS PARAMÈTRE. La
// note lui est donc passée ENTRE ACCOLADES: pour les trois classes de base ce groupe est
// l'argument, pour amsart c'est un groupe ordinaire, et les deux se lisent correctement.
// Une note passée sans accolades perdrait tout son texte sur amsart et rien ailleurs —
// une panne qui ne se voit que sur une classe qu'aucun autre test n'exerce.
func TestFootnoteTextSurvivesAMacroWithNoParameter(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\makeatletter\renewcommand\@makefntext{\@makefnmark}\makeatother
\begin{document}
Corps.\footnote{NOTE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageText(e); !strings.Contains(got, "NOTE") {
		t.Errorf("texte perdu avec un \\@makefntext sans paramètre: %q", got)
	}
}
