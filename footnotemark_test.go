package engine

import (
	"strings"
	"testing"
)

// labelled says the footnote whose text is `word` carries `mark` in front of it. The
// two spellings cover whether the collector renders the separating glue as a space.
func labelled(text, mark, word string) bool {
	return strings.Contains(text, mark+word) || strings.Contains(text, mark+" "+word)
}

// ⛔⛔ THE MARK WAS strconv.Itoa(n) IN GO, SO \thefootnote WAS UNREACHABLE. article.cls
// redefines it inside \maketitle —
//
//	\renewcommand\thefootnote{\@fnsymbol\c@footnote}      (article.cls:205)
//
// — and resets the counter after the group:
//
//	\endgroup \setcounter{footnote}{0}%                   (article.cls:222-223)
//
// ⛔ THE RESET IS NOT THE GROUP. LaTeX's counter commands are \global, so \endgroup
// does not restore \c@footnote; the group scopes the REPRESENTATION and the explicit
// \setcounter scopes the COUNT. Reading the group as what renumbers the body gets the
// fix wrong, and I did read it that way before opening article.cls.
//
// Measured against tectonic on this very document: `∗` for the \thanks and `1` for the
// body note, where this engine gave `1` and `2`.
func TestFootnoteMarkComesFromThefootnote(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\title{Un titre}
\author{Une autrice\thanks{AFFILIATION}}
\begin{document}
\maketitle
Corps.\footnote{NOTEDUCORPS}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	txt := pageText(e)

	// ⛔ LE CAS QUI PORTE LE TEST: la note du bloc de titre n'est PAS un chiffre.
	if labelled(txt, "1", "AFFILIATION") {
		t.Error(`la note du \thanks est numérotée « 1 »: \maketitle doit passer ` +
			`\thefootnote à \@fnsymbol (article.cls:205), la référence donne « ∗ »`)
	}
	if !labelled(txt, "*", "AFFILIATION") {
		t.Errorf("la note du \\thanks ne porte pas le symbole attendu: %.300q", txt)
	}

	// La première note du corps est « 1 », comme la référence.
	//
	// ⚠⚠ CE QUE CETTE LIGNE NE PROUVE PAS. Elle passe AUSSI sans le
	// \setcounter{footnote}{0} que \maketitle pose d'après article.cls:223 — vérifié
	// par mutation, et c'est pourquoi l'assertion « la note est 2 » qui se trouvait
	// ici a été retirée: elle ne tombait sous aucune mutation, donc elle ne mesurait
	// rien. Sur ce moteur \c@footnote relit 0 après \maketitle de toute façon, alors
	// que la marque du \thanks est bâtie sur la valeur 1 — la marque et le compteur ne
	// racontent pas la même histoire, et je n'ai pas trouvé pourquoi. Un \footnote
	// dans un \begingroup…\endgroup ordinaire laisse bien le compteur avancé (mesuré,
	// d'accord avec tectonic), donc le pas global du moteur est juste; c'est le chemin
	// de \maketitle qui ne l'est pas. Question OUVERTE, pas résultat.
	if !labelled(txt, "1", "NOTEDUCORPS") {
		t.Errorf("la première note du corps n'est pas numérotée « 1 »: %.300q", txt)
	}

	// ⛔ ET AUCUN POINT. \@makefntext pose la marque puis le texte, sans ponctuation
	// (article.cls:609-612); ce moteur écrivait « 1. ».
	if strings.Contains(txt, "1.NOTEDUCORPS") || strings.Contains(txt, "1. NOTEDUCORPS") {
		t.Error(`la marque de pied est suivie d'un point que la référence n'écrit pas`)
	}
}

// ⛔ ET LE DOCUMENT DOIT POUVOIR LE REDÉFINIR. C'est l'autre moitié de ce que donne un
// vrai \c@footnote: \renewcommand\thefootnote{…} était silencieusement ignoré, sur 70
// des 154 papiers mesurés qui écrivent ce nom.
func TestDocumentCanRedefineThefootnote(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\begin{document}
\renewcommand\thefootnote{\fnsymbol{footnote}}
Corps.\footnote{REDEFINIE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	txt := pageText(e)
	if labelled(txt, "1", "REDEFINIE") {
		t.Errorf("\\renewcommand\\thefootnote est ignoré, la note reste en chiffres: %.300q", txt)
	}
	if !labelled(txt, "*", "REDEFINIE") {
		t.Errorf("la note redéfinie ne porte pas le symbole attendu: %.300q", txt)
	}
}

// ⛔ LE TÉMOIN: sans rien redéfinir, la numérotation reste arabe et consécutive. Un test
// qui n'exigerait que « pas un chiffre » passerait sur un moteur qui a perdu la marque.
func TestOrdinaryFootnotesAreStillNumbered(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\begin{document}
Un.\footnote{PREMIERE} Deux.\footnote{SECONDE}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	txt := pageText(e)
	if !labelled(txt, "1", "PREMIERE") {
		t.Errorf("la première note ordinaire n'est pas « 1 »: %.300q", txt)
	}
	if !labelled(txt, "2", "SECONDE") {
		t.Errorf("la seconde note ordinaire n'est pas « 2 »: %.300q", txt)
	}
}
