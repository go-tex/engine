package engine

import (
	"strings"
	"testing"
)

// \footnote numbers its notes, drops a raised marker inline, and attaches each
// note to the vertical list so the page builder can place it at the foot.
func TestFootnote(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent A\footnote{First}B\footnote{Second}.`); err != nil {
		t.Fatal(err)
	}
	if e.footnoteCounter != 2 {
		t.Errorf("footnote counter = %d, want 2", e.footnoteCounter)
	}
	// Two footnote nodes reached the main vertical list.
	var notes []*boxNode
	for _, n := range e.mvl {
		if fn, ok := n.(footnoteNode); ok {
			notes = append(notes, fn.body)
		}
	}
	if len(notes) != 2 {
		t.Fatalf("footnote nodes on mvl = %d, want 2", len(notes))
	}
	// Bodies are numbered "N. text" (the space is glue, dropped by mvlText).
	if got := mvlText([]node{notes[0]}); got != "1.First" {
		t.Errorf("note 1 body = %q, want %q", got, "1.First")
	}
	if got := mvlText([]node{notes[1]}); got != "2.Second" {
		t.Errorf("note 2 body = %q, want %q", got, "2.Second")
	}
	// The inline marker is a raised (negative-shift) box on the first line.
	line, _ := e.mvl[0].(*boxNode)
	if line == nil {
		t.Fatal("no first line box")
	}
	raised := false
	for _, n := range line.list {
		if b, ok := n.(*boxNode); ok && b.shift < 0 {
			raised = true
		}
	}
	if !raised {
		t.Error("no raised footnote marker on the first line")
	}
}

// A footnoteNode reserves its OWN height, and nothing more. What the foot area
// costs the page beyond that — \skip\footins — is charged once for the page, not
// once per note (tex.web:19638).
func TestFootnoteReservesItsOwnHeightOnly(t *testing.T) {
	body := &boxNode{kind: vbox, height: 20 * unity, depth: 3 * unity}
	if got, want := vContribution(footnoteNode{body: body}), 20*unity+3*unity; got != want {
		t.Errorf("footnote vContribution = %d, want %d (its body, with no per-note allowance)", got, want)
	}
}

// \skip\footins is charged ONCE for a page however many notes land on it. Charging
// it per note reserved room nobody used: measured against tectonic on a page of
// four footnotes, our foot area ended 45pt above the bottom of the text block where
// the reference's ends flush with it.
func TestFootinsSkipIsChargedOncePerPage(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	note := func() footnoteNode {
		return footnoteNode{body: &boxNode{kind: vbox, height: 10 * unity}}
	}
	line := func() node { return &boxNode{kind: hbox, height: 10 * unity} }
	skip := e.footinsSkip()
	if skip <= 0 {
		t.Fatal("\\skip\\footins reads zero; the test would prove nothing")
	}
	// vsize large enough that neither list breaks: the break index is the length.
	e.vsize = 1000 * unity
	one := []node{line(), note()}
	two := []node{line(), note(), note()}
	if e.findPageBreak(one, 0) != len(one) || e.findPageBreak(two, 0) != len(two) {
		t.Fatal("the probe lists broke; vsize is too small for the test")
	}
	// Now squeeze: a vsize that fits one note's worth of foot area plus the lines.
	// With the skip charged per note the second list would break early; with it
	// charged once it does not.
	e.vsize = 10*unity + 10*unity + 10*unity + skip
	if got := e.findPageBreak(two, 0); got != len(two) {
		t.Errorf("page broke at %d of %d: \\skip\\footins looks charged per note, not per page", got, len(two))
	}
}

// The assembled page lifts footnote bodies out of the content flow and stacks
// them below a separator rule.
func TestFootnotePageAssembly(t *testing.T) {
	e := New()
	e.hsize = 300 * unity
	content := &boxNode{kind: hbox, width: 100 * unity, height: 8 * unity}
	note := &boxNode{kind: vbox, width: 120 * unity, height: 10 * unity}
	page := e.assemblePage([]node{content, footnoteNode{body: note}}, 1)
	// The content box and the note box both appear, with a separator rule between.
	var boxes, rules int
	for _, n := range page.list {
		switch n.(type) {
		case *boxNode:
			boxes++
		case ruleNode:
			rules++
		}
	}
	if boxes != 2 {
		t.Errorf("assembled page has %d boxes, want 2 (content + note)", boxes)
	}
	if rules != 1 {
		t.Errorf("assembled page has %d rules, want 1 (footnote separator)", rules)
	}
}

// \thanks a DEUX moitiés, et n'en réparer qu'une déplace la perte.
//
// Il était défini pour jeter son argument (latex.go, classprims.go), et le
// recensement des avaleurs le donnait sur 29 papiers. La sonde de contenu —
// chercher les mots de l'argument dans le PDF de la référence ET dans le nôtre —
// en comptait 5 vraies pertes.
//
// ⛔ Le rendre \footnote{#1} ne suffit pas: \thanks vit dans \author, que
// \maketitle compose dans un \centerline, et une note dans une boîte n'atteint
// pas la page. Mesuré: le mot restait absent. Il faut ACCUMULER à l'appel et
// ÉMETTRE après le bloc de titre, ce que fait \maketitle.
func TestThanksReachesThePage(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\title{Un titre}
\author{Une autrice\thanks{A fine university.}}
\begin{document}
\maketitle
Corps.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	// ⛔ Sur e.mvl on ne voit que la MARQUE: le texte d'une note vit dans la zone
	// de pied que le constructeur de pages assemble. Chercher sur les PAGES.
	if got := pageText(e); !strings.Contains(got, "university") {
		t.Errorf("le texte du \\thanks n'atteint pas la page: %.200q", got)
	}
}

// …et il ne doit pas rester collé d'un \maketitle au suivant: \@thanks est vidé
// après émission, sinon le second titre réimprime la note du premier.
func TestThanksIsEmittedOnce(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\title{Un}
\author{A\thanks{Mot unique ici}}
\begin{document}
\maketitle
\title{Deux}
\author{B}
\maketitle
Corps.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(pageText(e), "unique"); n != 1 {
		t.Errorf("le \\thanks est composé %d fois, une seule attendue", n)
	}
}

// pageText rend le texte des PAGES assemblées, zone de pied comprise.
func pageText(e *Engine) string {
	var b strings.Builder
	for _, p := range e.Pages() {
		collectChars([]node{p}, &b)
	}
	return b.String()
}

// Et la voie SANS classe, que les deux tests ci-dessus ne touchent pas: avec un
// \documentclass, c'est article.cls qui émet \@thanks et le vide (article.cls:187
// et :193), et classprims.go qui définit \thanks. Les définitions de latex.go ne
// servent qu'au document qui ne charge aucune classe — et trois mutations les ont
// SURVÉCUES avant que ce test existe, parce que rien n'empruntait cette voie.
func TestThanksWithoutAClass(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\title{Un titre}
\author{Une autrice\thanks{A fine university.}}
\maketitle
Corps.`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := pageText(e); !strings.Contains(got, "university") {
		t.Errorf("sans classe, le \\thanks n'atteint pas la page: %.200q", got)
	}
	// Et il n'est pas réémis par un second \maketitle.
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\hsize=300pt
\title{Un}\author{A\thanks{Mot unique ici}}\maketitle
\title{Deux}\author{B}\maketitle
Corps.`); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(pageText(e2), "unique"); n != 1 {
		t.Errorf("sans classe, le \\thanks est composé %d fois, une seule attendue", n)
	}
}

// Un \thanks VIDE ne produit rien. eptcs.cls écrit \thanks\relax, et en faire une
// note donne une marque et une ligne de pied que la référence n'a pas.
//
// ⛔ L'observable est la DIFFÉRENCE avec le même document sans \thanks, pas la
// présence d'un chiffre: le premier jet cherchait « 1 » et trouvait le FOLIO.
func TestEmptyThanksProducesNothing(t *testing.T) {
	page := func(author string) string {
		t.Helper()
		e, err := compile([]byte(`\documentclass{article}
\title{T}
\author{`+author+`}
\begin{document}
\maketitle
Corps.
\end{document}`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		return pageText(e)
	}
	plain := page("A")
	for _, arg := range []string{"{}", `\relax`} {
		if got := page("A\\thanks" + arg); got != plain {
			t.Errorf("\\thanks%s a changé la page: %.120q contre %.120q", arg, got, plain)
		}
	}
	// Contrôle positif, sans quoi le test passerait aussi si \thanks était
	// redevenu un avaleur.
	if got := page("A\\thanks{Mot temoin ici}"); !strings.Contains(got, "temoin") {
		t.Errorf("le contrôle positif ne passe pas: %.120q", got)
	}
}
