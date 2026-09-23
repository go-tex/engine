package engine

import (
	"strings"
	"testing"
)

// Three of the 200 corpus papers REDEFINE the proof environment — to add an
// optional title, or a diamond QED — by writing amsthm's own internals. Without
// \pushQED/\popQED/\proofname the head is lost, which is content, not decoration:
//
//	reference   AVANT Proof. Le corps de la preuve. APRES
//	before      AVANT .     Le corps de la preuve. APRES
const redefinedProof = `\usepackage{amsthm}\makeatletter
\renewenvironment{proof}[1][]{%
  \par\pushQED{\qed}\normalfont\trivlist
  \item[\hskip\labelsep\bfseries \proofname
    \if\relax\detokenize{#1}\relax\else\space #1\fi \@addpunct{.}]%
  \ignorespaces}{%
  \popQED\endtrivlist\@endpefalse}
\makeatother`

func TestRedefinedProofKeepsItsHead(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}`+redefinedProof+
		`\begin{document}AVANT\begin{proof}Le corps.\end{proof}APRES\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "Proof.") {
		t.Errorf("page = %q, want it to carry the proof head %q", got, "Proof.")
	}
	if n := len(e.skippedCS); n != 0 {
		t.Errorf("%d command(s) still undefined: %v", n, e.skippedCS)
	}
}

// The stock proof environment already worked; this pins that the amsthm
// internals did not break it.
func TestStockProofStillHasItsHead(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsthm}\begin{document}`+
		`\begin{proof}Corps.\end{proof}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "Proof.") {
		t.Errorf("page = %q, want it to carry %q", got, "Proof.")
	}
}
