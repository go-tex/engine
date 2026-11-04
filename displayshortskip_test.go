package engine

import "testing"

// displaySkips returns the glue widths immediately above and below the display box,
// skipping the interline glue that sits between the above-skip and the box.
func displaySkips(t *testing.T, e *Engine) (above, below int) {
	t.Helper()
	dispIdx := -1
	for i, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			for _, c := range b.list {
				if _, ok := c.(mathNode); ok {
					dispIdx = i
				}
			}
		}
	}
	if dispIdx < 2 || dispIdx+1 >= len(e.mvl) {
		t.Fatalf("pas de display encadré de colle (idx %d sur %d)", dispIdx, len(e.mvl))
	}
	a, ok := e.mvl[dispIdx-2].(glueNode)
	if !ok {
		t.Fatalf("au-dessus du display: %T, attendu une colle", e.mvl[dispIdx-2])
	}
	b, ok := e.mvl[dispIdx+1].(glueNode)
	if !ok {
		t.Fatalf("sous le display: %T, attendu une colle", e.mvl[dispIdx+1])
	}
	return a.spec.width, b.spec.width
}

func shortSkipEngine(t *testing.T, body string) *Engine {
	t.Helper()
	e := New()
	e.SetFont(spMock{})
	e.hsize = 200 * unity
	src := `\newskip\abovedisplayskip \abovedisplayskip=10pt` +
		`\newskip\belowdisplayskip \belowdisplayskip=7pt` +
		`\newskip\abovedisplayshortskip \abovedisplayshortskip=1pt` +
		`\newskip\belowdisplayshortskip \belowdisplayshortskip=3pt` + body
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return e
}

// ⛔⛔ LA PAIRE COURTE N'ÉTAIT JAMAIS CHOISIE, et c'est 13,95pt de trop sur CHAQUE
// display. tex.web:22578-22585:
//
//	if (d+s <= pre_display_size) or l then   {not enough clearance}
//	  begin g1:=above_display_skip_code; g2:=below_display_skip_code; end
//	else
//	  begin g1:=above_display_short_skip_code; g2:=below_display_short_skip_code; end
//
// Mesuré ligne de base à ligne de base sur un display entre deux lignes de texte,
// avant correction: nous 22,00 au-dessus et 22,00 en dessous, tectonic 11,96 et 17,93.
// Les deux colles courtes (0pt et 6pt à 10pt) expliquent l'écart en entier.
func TestShortDisplaySkipsWhenTheLineClearsTheDisplay(t *testing.T) {
	e := shortSkipEngine(t, `before $$x$$ after\par`)
	above, below := displaySkips(t, e)
	if above != 1*unity || below != 3*unity {
		t.Errorf("colles = %.2fpt / %.2fpt, attendu 1pt / 3pt (la paire COURTE): "+
			"« before » finit bien à gauche d'un display centré",
			float64(above)/unity, float64(below)/unity)
	}
}

// ⛔ ET LE TÉMOIN DANS L'AUTRE SENS, sans lequel le test ci-dessus passerait aussi sur
// un moteur qui aurait simplement remplacé une paire par l'autre. Un display LARGE
// commence plus à gauche que la ligne précédente ne finit: pas de dégagement, paire
// LONGUE.
func TestLongDisplaySkipsWhenTheDisplayReachesBackUnderTheLine(t *testing.T) {
	e := shortSkipEngine(t, `before $$`+wideDisplay+`$$ after\par`)
	above, below := displaySkips(t, e)
	if above != 10*unity || below != 7*unity {
		t.Errorf("colles = %.2fpt / %.2fpt, attendu 10pt / 7pt (la paire LONGUE)",
			float64(above)/unity, float64(below)/unity)
	}
}

// ⛔ \predisplaysize PORTE UN TERME DE DEUX CADRATINS, et il n'est pas décoratif:
// tex.web:21782 part de `v:=shift_amount(just_box)+2*quad(cur_font)`. Sans lui une
// ligne serait jugée plus courte qu'elle n'est et la paire courte serait choisie trop
// souvent — l'erreur dans le sens qui RESSERRE la page.
func TestPreDisplaySizeCountsTwoQuads(t *testing.T) {
	line := &boxNode{width: 200 * unity, list: []node{
		charNode{width: 5 * unity}, charNode{width: 5 * unity},
	}}
	const quad = 10 * unity
	got := preDisplaySize(line, 0, quad)
	if want := 10*unity + 2*quad; got != want {
		t.Errorf("preDisplaySize = %.2fpt, attendu %.2fpt (10pt de texte + 2 cadratins)",
			float64(got)/unity, float64(want)/unity)
	}
}

// ⛔⛔ LA COLLE FINALE NE COMPTE PAS. Une ligne se termine par \parfillskip, `0pt plus
// 1fil`, et le texte ne « s'étend » pas jusque-là: tex.web arrête la somme au dernier
// nœud VISIBLE. La compter rendrait chaque ligne pleine largeur et choisirait la paire
// longue partout — c'est-à-dire le défaut d'origine, sous un autre nom.
//
// ⛔ Et seule une étirabilité INFINIE empoisonne la somme. La colle inter-mots ordinaire
// s'étire finiment et doit passer: sinon toute ligne justifiée rendrait \maxdimen.
// Contrôle par la référence: tectonic rapporte \predisplaysize à 48.37pt et 93.77pt sur
// deux lignes ordinaires — des valeurs FINIES, là où une somme empoisonnée afficherait
// 16383.99998pt.
func TestPreDisplaySizeStopsAtTheLastVisibleNode(t *testing.T) {
	const quad = 10 * unity
	withFil := &boxNode{width: 200 * unity, list: []node{
		charNode{width: 5 * unity},
		glueNode{spec: glueSpec{width: 2 * unity, stretch: unity}}, // inter-mots, fini
		charNode{width: 5 * unity},
		glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}, // \parfillskip
	}}
	got := preDisplaySize(withFil, 0, quad)
	if want := 12*unity + 2*quad; got != want {
		t.Errorf("preDisplaySize = %.2fpt, attendu %.2fpt: la colle finale en fil ne "+
			"doit pas compter, et la colle inter-mots finie doit compter",
			float64(got)/unity, float64(want)/unity)
	}
}
