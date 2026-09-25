package engine

import (
	"sort"
	"strings"
	"testing"
)

// texparams.go states the invariant this asserts, in its own words:
//
//	A parameter the engine *does* act on (\parindent, \hsize, \baselineskip, …) is
//	a primitive of its own elsewhere and is deliberately absent from these lists.
//
// It was a comment, and it had already been broken. \prevdepth was made a real
// parameter bound to e.prevDepth (#438) while "prevdepth" stayed in
// texDimenParams, and loadTeXParams runs at primitives.go:209 — BEFORE the
// primitive is registered — so the primitive won by an accident of ordering
// rather than by design. Move that one call later and \prevdepth silently becomes
// a dead register again, which is the defect #438 fixed, re-armed and invisible.
//
// Asserting the SET rather than the one name is the point: a future parameter
// promoted to a primitive gets caught here without anyone remembering to add a
// case. A name legitimately in both would have to be listed in allowedBoth below,
// with the reason.
func TestNoParameterIsBothAPrimitiveAndAListedParam(t *testing.T) {
	// Names that may appear in both, with why. Empty: the invariant is absolute
	// today, and an addition here needs an argument, not a shrug.
	allowedBoth := map[string]string{}

	e := New()
	e.LoadLaTeX()

	listed := map[string]string{}
	for _, n := range texDimenParams {
		listed[n] = "texDimenParams"
	}
	for n := range texIntParams {
		listed[n] = "texIntParams"
	}
	for _, n := range texGlueParams {
		listed[n] = "texGlueParams"
	}

	var bad []string
	for n, where := range listed {
		if _, ok := allowedBoth[n]; ok {
			continue
		}
		m := e.eq[n]
		if m != nil && m.kind == mPrim {
			bad = append(bad, `\`+n+" ("+where+")")
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d nom(s) à la fois primitive et paramètre listé:\n  %s\n"+
			"Le vainqueur dépend de l'ORDRE de chargement (loadTeXParams à "+
			"primitives.go:209), donc la primitive peut être masquée sans bruit. "+
			"Retirer le nom de la liste, ou l'ajouter à allowedBoth avec sa raison.",
			len(bad), strings.Join(bad, "\n  "))
	}
}
