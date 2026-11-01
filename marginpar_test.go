package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ \marginpar's ARGUMENT USED TO LAND IN THE RUNNING TEXT.
//
// It was undefined, so strict mode aborted and -lenient skipped the command name and
// then set its argument as ordinary text. 52 of the 154 measured corpus papers use it.
// Measured against tectonic:
//
//	source                                oracle                  avant
//	AVANTUN\marginpar{NOTEUN}APRESUN      AVANTUNAPRESUN          AVANTUNNOTEUNAPRESUN
//	AVANTTROIS\marginpar[G]{D}APRESTROIS  AVANTTROISAPRESTROIS    AVANTTROIS[G]DAPRESTROIS
//
// The optional form is the one that shows what was happening: BOTH arguments reached
// the page, the first with its literal brackets. That is corrupted body text, not a
// missing marginal note — and the body is what the corpus measures.
//
// ⛔ THE NOTE IS STILL NOT PLACED. Putting a box in the margin from inside a paragraph
// needs \vadjust (go-tex/engine#665). What is asserted here is that the body text is
// EXACTLY the reference's, which is what dropping the note gives: the oracle sets
// AVANTUNAPRESUN too, with NOTEUN out in the margin at x=488.44.
//
// ⛔ THE MUTATION THIS EXISTS FOR is removing the three definitions: every row then
// either aborts (strict) or carries the argument into the text.
func TestMarginparDoesNotLeakItsArgumentIntoTheText(t *testing.T) {
	for _, c := range []struct {
		name, src, want string
	}{
		{"forme simple",
			`AVANTUN\marginpar{NOTEUN}APRESUN`, "AVANTUNAPRESUN"},
		{"forme optionnelle: les DEUX arguments doivent disparaître",
			`AVANTTROIS\marginpar[GAUCHE]{DROITE}APRESTROIS`, "AVANTTROISAPRESTROIS"},
		{"en mode vertical, entre deux paragraphes",
			`AVANTQ\par\marginpar{NOTEQ}\par APRESQ`, ""},
		// ⛔ THE CONTROL: the same body with no \marginpar at all. Without it every row
		// above would pass against an engine that dropped the whole paragraph.
		{"témoin sans marginpar", `AVANTUNAPRESUN`, "AVANTUNAPRESUN"},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		// ⛔ STRICT mode, no -lenient: an undefined \marginpar aborted here, so a run
		// that merely compiles is already part of the assertion.
		if _, err := e.Run(`\documentclass{article}\begin{document}` +
			c.src + `\par`); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		var got strings.Builder
		for _, b := range lineBoxesOf(e.mvl) {
			got.WriteString(boxText(b))
		}
		// The vertical-mode row expects no body text of its own beyond the
		// surrounding words, so it is checked by containment rather than equality.
		if c.want == "" {
			if strings.Contains(got.String(), "NOTEQ") {
				t.Errorf("%s: la note est dans le texte: %q", c.name, got.String())
			}
			if !strings.Contains(got.String(), "AVANTQ") ||
				!strings.Contains(got.String(), "APRESQ") {
				t.Errorf("%s: le corps a été perdu: %q", c.name, got.String())
			}
			continue
		}
		if g := strings.ReplaceAll(got.String(), " ", ""); g != c.want {
			t.Errorf("%s: corps = %q, want %q", c.name, g, c.want)
		}
	}
}
