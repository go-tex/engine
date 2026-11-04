package engine

import "testing"

func penaltiesOf(e *Engine) []int {
	var out []int
	for _, n := range e.mvl {
		if p, ok := n.(penaltyNode); ok {
			out = append(out, p.penalty)
		}
	}
	return out
}

func countPenalty(ps []int, want int) int {
	n := 0
	for _, p := range ps {
		if p == want {
			n++
		}
	}
	return n
}

// ⛔⛔ \addpenalty ÉTAIT UN BOUCHON VIDE, donc TOUT indice de coupure que le noyau et les
// classes donnent était jeté en silence. Les trois d'une liste (latex.ltx:11523, :11562,
// :11567) valent -51 et ENCOURAGENT une coupure à la frontière de liste.
//
// ⛔ ET DEUX CORRECTIFS ÉTAIENT NÉCESSAIRES, ce qui est le piège de ce défaut: les macros
// de liste de ce moteur sont les siennes, pas le \list du noyau, et n'appelaient pas
// \addpenalty. Avec le bouchon en place, ajouter les appels n'aurait rien changé; avec
// les appels manquants, réparer \addpenalty n'aurait rien changé non plus. Chacun seul
// paraît inerte — et une mesure « sans effet » aurait pu faire abandonner le bon.
func TestListContributesItsThreePenalties(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt]{article}
\begin{document}
\noindent Avant.
\begin{itemize}\item Un\item Deux\end{itemize}
\noindent Apres.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	ps := penaltiesOf(e)
	// \@beginparpenalty à l'ouverture, \@itempenalty entre les deux items,
	// \@endparpenalty à la fermeture: trois fois -51.
	if got := countPenalty(ps, -51); got != 3 {
		t.Errorf("pénalités de liste = %d fois -51, attendu 3 (début, entre items, fin): %v", got, ps)
	}
}

// ⛔ L'ARGUMENT EST UN NOMBRE, PAS UN GROUPE. \gotex@addpenalty lit un entier comme
// \penalty le fait; passé entre accolades il lisait 0 — SILENCIEUSEMENT, ce qui est la
// pire forme: la pénalité était bien posée, avec la mauvaise valeur, et rien ne le
// signalait.
func TestAddPenaltyReadsItsNumber(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt]{article}
\begin{document}
x\par\makeatletter\addpenalty{-77}\makeatother y
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := countPenalty(penaltiesOf(e), -77); got != 1 {
		t.Errorf("\\addpenalty{-77} n'a pas posé -77: %v", penaltiesOf(e))
	}
}

// ⛔⛔ LA PÉNALITÉ DOIT PRÉCÉDER LA COLLE. TeX ne coupe à une colle que si l'élément qui
// la précède n'est pas jetable; une pénalité posée APRÈS la colle offre une coupure là
// où le constructeur de page est déjà passé. C'est toute la raison de la danse de
// \vskip de la référence (latex.ltx:6549-6563), que ce moteur remplace par une insertion
// puisque sa liste verticale est adressable.
func TestAddPenaltyGoesBeforeTheTrailingGlue(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt]{article}
\begin{document}
x\par\vskip 12pt\makeatletter\addpenalty{-33}\makeatother
y
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	idx := -1
	for i, n := range e.mvl {
		if p, ok := n.(penaltyNode); ok && p.penalty == -33 {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("pénalité -33 absente: %v", e.mvl)
	}
	if idx+1 >= len(e.mvl) {
		t.Fatalf("la pénalité est en DERNIER: rien ne la suit, donc elle a été ajoutée après la colle")
	}
	if _, ok := e.mvl[idx+1].(glueNode); !ok {
		t.Errorf("l'élément après la pénalité est %T, attendu la colle qu'elle doit précéder", e.mvl[idx+1])
	}
}

// ⛔ ET LE TÉMOIN: en mode HORIZONTAL, \addpenalty ne fait rien. C'est le garde \ifvmode
// de la release 2015/01/01 (ltspace.dtx:1057-1060), celle que porte le latex.ltx du
// paquet tectonic — donc celle que notre oracle exécute. Le noyau courant (2024/11/01)
// fait \par en mode horizontal externe à la place; s'aligner sur l'oracle est ici
// délibéré.
func TestAddPenaltyDoesNothingInHorizontalMode(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt]{article}
\begin{document}
x\makeatletter\addpenalty{-44}\makeatother y
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := countPenalty(penaltiesOf(e), -44); got != 0 {
		t.Errorf("\\addpenalty a agi en mode horizontal: %v", penaltiesOf(e))
	}
}
