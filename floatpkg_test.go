// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The float package's \newfloat declares a new float type that reuses the class
// \@float path, so \begin{<type>}…\end{<type>} typesets its body and \caption
// numbers it "Name N" via \fnum@<type>. Without it the \begin{<type>} environment
// is undefined and its whole body — often a tall block — is dropped; 14–15 of the
// 200 arXiv reference papers declare a custom float this way. \floatname sets the
// caption label after the type is declared.
func TestNewFloatTypeRendersBodyAndCaption(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{float}`+
		`\newfloat{program}{tbp}{lop}\floatname{program}{Program}`+
		`\begin{document}Avant.`+
		`\begin{program}Corpsduprogramme.\caption{Légende}\end{program}`+
		`Après.\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "Corpsduprogramme.") {
		t.Errorf("le corps du flottant est perdu: %q", got)
	}
	if !strings.Contains(got, "Program1:Légende") {
		t.Errorf("la légende doit être numérotée « Program 1: Légende »: %q", got)
	}
}

// A second \newfloat type numbers independently of the first, and \floatname may
// be omitted (the label defaults to the type name).
func TestNewFloatSecondTypeAndDefaultName(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{float}`+
		`\newfloat{listing}{tbp}{lol}`+
		`\begin{document}`+
		`\begin{listing}L.\caption{Un}\end{listing}`+
		`\begin{listing}L.\caption{Deux}\end{listing}`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "listing1:Un") || !strings.Contains(got, "listing2:Deux") {
		t.Errorf("les deux flottants doivent être numérotés 1 puis 2 avec le nom par défaut: %q", got)
	}
}
