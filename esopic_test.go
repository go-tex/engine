package engine

import (
	"strings"
	"testing"
)

// eso-pic draws its argument on every page at shipout. This engine paginates after
// reading and has no shipout hook, so the picture is not drawn; what the test pins is
// that the argument is DROPPED instead of being typeset into the body, where it was
// extra text and, in the measured paper (2402.04711), an extra page. Measured against
// tectonic: the picture text is on the page there, on a single page.
func TestEsoPicArgumentIsNotTypesetInTheBody(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\usepackage{eso-pic}
\AddToShipoutPictureBG*{\put(100,100){BGPRE}}
\begin{document}
\AddToShipoutPictureBG*{\AtPageLowerLeft{\put(10,10){BGSTAR}}}
\AddToShipoutPictureBG{\put(10,10){BGPLAIN}}
\AddToShipoutPictureFG{\AtPageCenter{\makebox(0,0){FGMARK}}}
\AddToShipoutPicture{\put(100,100){PICTEXT}}
BODYTEXT
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	txt := pageText(e)
	for _, w := range []string{"BGPRE", "BGSTAR", "BGPLAIN", "FGMARK", "PICTEXT"} {
		if strings.Contains(txt, w) {
			t.Errorf("eso-pic argument %s typeset into the body: %q", w, txt)
		}
	}
	if !strings.Contains(txt, "BODYTEXT") {
		t.Errorf("body text lost: %q", txt)
	}
	if n := len(e.Pages()); n != 1 {
		t.Errorf("%d pages, want 1", n)
	}
}
