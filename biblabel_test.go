package engine

import (
	"strings"
	"testing"
)

const bibDoc = `\begin{document}
Texte~\cite{k}.
\begin{thebibliography}{9}
\bibitem{k} Une reference.
\end{thebibliography}
\end{document}`

func bibText(t *testing.T, preamble string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}`+preamble+bibDoc), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return pageText(e)
}

// ⛔⛔ LES CROCHETS DU MARQUEUR ÉTAIENT ÉCRITS DANS \@bibitemopt, donc aucune classe ne
// pouvait les changer. latex.ltx:13324 en fait une macro à part, \def\@biblabel#1{[#1]},
// et article.cls:568 l'appelle pour composer l'étiquette de la liste.
//
// 2961 papiers du fonds arXiv et 19 des 154 mesurés embarquent une CLASSE qui la
// redéfinit. Ce moteur EXÉCUTE ces classes: la définition arrivait donc bien dans
// l'eqtb, et était ignorée au seul endroit qui comptait. Mesuré avant: un document qui
// écrit \def\@biblabel#1{<<#1>>} rendait À L'OCTET comme un document qui ne l'écrit pas.
func TestBiblabelObeysTheDocument(t *testing.T) {
	got := bibText(t, `\makeatletter\def\@biblabel#1{<<#1>>}\makeatother`)
	if !strings.Contains(got, "<<1>>") {
		t.Errorf("\\@biblabel redéfini n'a pas servi à composer le marqueur: %q", got)
	}
}

// ⛔⛔ ET \@cite DE MÊME, pour l'appel de citation: \def\@cite#1#2{[{#1\if@tempswa , #2\fi}]}
// (latex.ltx:13322). 1450 papiers, 12 des 154, embarquent une classe qui le redéfinit.
func TestCiteWrapperObeysTheDocument(t *testing.T) {
	got := bibText(t, `\makeatletter\def\@cite#1#2{(((#1#2)))}\makeatother`)
	if !strings.Contains(got, "(((1)))") {
		t.Errorf("\\@cite redéfini n'a pas enveloppé la citation: %q", got)
	}
}

// ⛔ LE TÉMOIN: sans redéfinition, RIEN NE BOUGE. Les 154 papiers du corpus portent tous
// une bibliographie, donc un chemin qui aurait déplacé le défaut aurait tout déplacé.
// Un test qui n'exigerait que « la redéfinition marche » passerait sur un moteur qui a
// cassé le cas ordinaire.
func TestCitationDefaultsAreUnchanged(t *testing.T) {
	got := bibText(t, ``)
	for _, want := range []string{"[1]", "Unereference"} {
		if !strings.Contains(strings.ReplaceAll(got, " ", ""), strings.ReplaceAll(want, " ", "")) {
			t.Errorf("le rendu par défaut a changé: %q ne contient pas %q", got, want)
		}
	}
}

// ⛔⛔ \if@tempswa EST POSÉ ICI, pas hérité. La référence le met à vrai quand \cite a lu
// un crochet et à faux sinon (latex.ltx:13283-13284), et un \@cite redéfini le relit pour
// décider s'il imprime la note. Le laisser tranquille ferait dépendre le sort de la note
// de ce qui a touché l'interrupteur en dernier — un défaut qui ne se voit QUE sur un
// document qui cite deux fois, avec et sans note, dans cet ordre.
func TestCiteSetsTheSwitchItsOwnMacroReads(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\makeatletter\def\@cite#1#2{[#1\if@tempswa /NOTE:#2\fi]}\makeatother
\begin{document}
A~\cite[p. 5]{k} B~\cite{k}.
\begin{thebibliography}{9}
\bibitem{k} Une reference.
\end{thebibliography}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.ReplaceAll(pageText(e), " ", "")
	if !strings.Contains(got, "/NOTE:p.5") {
		t.Errorf("la citation AVEC note n'a pas vu \\if@tempswa vrai: %q", got)
	}
	// La seconde citation n'a pas de note: l'interrupteur doit être retombé à faux.
	if strings.Count(got, "/NOTE:") != 1 {
		t.Errorf("\\if@tempswa n'a pas été remis à faux pour la citation sans note: %q", got)
	}
}
