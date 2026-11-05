package engine

import (
	"strings"
	"testing"
)

func showsOf(t *testing.T, body string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass[10pt]{article}
\begin{document}`+body+`
x
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return e.Diagnostics().Shows
}

// ⛔ \showthe ÉTAIT INDÉFINI — une primitive TeX, et un document qui l'écrit s'arrêtait
// sur « Undefined control sequence ». tex.web:23721-23726 développe la quantité comme
// \the et l'imprime derrière « > » sur sa propre ligne.
func TestShowTheePrintsWhatTheWouldGive(t *testing.T) {
	got := showsOf(t, `\showthe\textheight\showthe\topskip`)
	for _, want := range []string{"> 550.0pt.", "> 10.0pt."} {
		if !strings.Contains(got, want) {
			t.Errorf("sortie %q, attendu %q", got, want)
		}
	}
}

// ⛔⛔ L'OPÉRANDE PEUT FAIRE PLUSIEURS JETONS, et c'est ce qui interdit de l'implémenter
// en macro TeX: \showthe\ht0 en compte deux, donc un \def\showthe#1{…} n'en prendrait
// que le premier et laisserait le « 0 » se composer sur la page. L'opérande est lue par
// \the LUI-MÊME, qui connaît déjà toute cette grammaire — registres, fontes, catcodes.
func TestShowTheReadsAMultiTokenOperand(t *testing.T) {
	got := showsOf(t, `\setbox0=\hbox{x}\showthe\ht0`)
	if !strings.Contains(got, "> ") || !strings.Contains(got, "pt.") {
		t.Errorf("\\showthe\\ht0 n'a rien montré d'utilisable: %q", got)
	}
	// Et le « 0 » ne doit pas avoir fui sur la page: s'il avait été laissé dans
	// l'entrée, il serait composé comme du texte ordinaire.
	e, err := compile([]byte(`\documentclass[10pt]{article}
\begin{document}\setbox0=\hbox{x}\showthe\ht0
FIN
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if txt := pageText(e); strings.Contains(txt, "0FIN") || strings.Contains(txt, "0 FIN") {
		t.Errorf("le « 0 » de \\ht0 a fui dans le texte: %q", txt)
	}
}

// ⛔⛔ CE N'EST PAS DU \message, ET LA DISTINCTION EST LA RAISON D'ÊTRE DU CHAMP.
// \message est le document qui parle, et un appelant peut légitimement ne pas le
// vouloir; \showthe est un DIAGNOSTIC demandé nommément, que tex.web écrit au journal
// et au terminal quel que soit le mode d'interaction. Les confondre remettait \showthe
// derrière un drapeau de ce moteur, que nul vrai TeX n'exige.
func TestShowTheIsSeparateFromMessage(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt]{article}
\begin{document}\message{BAVARDAGE}\showthe\topskip
x
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	d := e.Diagnostics()
	if strings.Contains(d.Shows, "BAVARDAGE") {
		t.Error("un \\message s'est retrouvé dans Shows")
	}
	if strings.Contains(d.Messages, "> 10.0pt") {
		t.Error("un \\showthe s'est retrouvé dans Messages")
	}
	if !strings.Contains(d.Messages, "BAVARDAGE") || !strings.Contains(d.Shows, "> 10.0pt.") {
		t.Errorf("chacun doit aller dans son flux: Shows=%q Messages=%q", d.Shows, d.Messages)
	}
}
