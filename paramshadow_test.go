package engine

import (
	"sort"
	"strconv"
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
		t.Errorf("%d name(s) both a primitive and a listed parameter:\n  %s\n"+
			"The winner depends on LOAD ORDER (loadTeXParams at "+
			"primitives.go:209), so the primitive can be shadowed with no noise. "+
			"Remove the name from the list, or add it to allowedBoth with its reason.",
			len(bad), strings.Join(bad, "\n  "))
	}
}

// The companion to the test above, and the one that would have caught the four
// real cases. That one checks the Go-side accept-and-ignore LISTS; this checks the
// meaning a parameter actually ENDS UP with after everything has loaded, so it
// catches shadowing by any mechanism — including a \newdimen or \newskip in the
// TeX substrate, which is where all four came from:
//
//	\lastskip         a \newskip in amssubstrate.go, noted in its own comment
//	\prevdepth        a \newdimen in amssubstrate.go          (#438, #440)
//	\lineskiplimit    a \newdimen in amssubstrate.go
//	\lineskip         a \newskip in classkernel.go
//
// Each one compiled, each one read back what a package wrote, and each one left
// the engine's own field untouched — so the parameter looked alive and the rule it
// feeds was dead. \offinterlineskip was the visible cost of the last two: it sets
// \lineskip\z@ \lineskiplimit\maxdimen and still left 1pt between boxes.
//
// The list is explicit rather than derived: engineDimenParam's cases are a Go
// switch and cannot be enumerated, so a new parameter has to be added here. That
// is the point — the addition is the moment to check it is not shadowed.
func TestEngineParametersAreNotShadowed(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	for _, n := range []string{
		"hsize", "vsize", "parindent", "textwidth",
		"baselineskip", "lineskip", "lineskiplimit", "prevdepth",
	} {
		m := e.eq[n]
		if m == nil {
			t.Errorf(`\%s is not defined at all`, n)
			continue
		}
		if m.kind != mPrim {
			t.Errorf(`\%s has meaning kind=%d, not a primitive: a \newdimen/\newskip in the `+
				`substrate, or a texparams entry, SHADOWS it — so an assignment writes to a `+
				`register and the engine's own field never moves`, n, int(m.kind))
		}
	}
}

// The third of this family, and the one the first two leave a hole for.
//
// The two above ask whether a name ends up a PRIMITIVE: one that it must not,
// one that it must. Neither asks whether a name that is meant to be an ordinary
// accept-and-ignore register ends up the register of the RIGHT TYPE — and
// \hangafter did not:
//
//	texparams.go  "hangafter": 1      an INTEGER parameter (tex.web §236)
//	amssubstrate  \newdimen\hangafter a DIMENSION, and it loaded second
//
// Both tests above passed throughout. \hangafter was not a primitive, so the
// first had nothing to say; it was not in the second's list, because it is not
// a parameter the engine acts on. It compiled, it read back what a package
// wrote, and nothing was ever reported.
//
// ⛔ AND THE TYPE IS NOT A LABEL: IT IS HOW THE VALUE IS SCANNED. \hangafter=1
// against an integer reads the 1 and stops. Against a dimension it scans a
// <dimen>, which REQUIRES a unit, so it reads on into the following text
// looking for one — and TeX's units are ordinary letter pairs (in, pt, pc, bp,
// cm, mm, dd, cc, sp, ex, em). Measured before the fix:
//
//	\hangafter=1 integers follow   ->   BEFOREtegersfollow
//
// Two letters taken out of the middle of a word, with no diagnostic of any
// kind. A census over the 2935-paper corpus finds 109 papers that write
// \hangafter and none that lose a word to it, so what this closes is a latent
// trap and a wrong type — not a measured corpus deviation.
// See hangafter_probe_test.go.
//
// This derives its expectations from the three lists themselves, so a parameter
// added tomorrow is covered without anyone remembering this test exists. That
// is the difference between a gate and a note: the previous two occurrences
// (\prevdepth, then the four in the comment above) each ended in a note about
// load order, and the next one still got through.
func TestAListedParameterKeepsItsDeclaredType(t *testing.T) {
	e := New()
	e.LoadLaTeX()

	type want struct {
		kind mkind
		list string
	}
	expect := map[string]want{}
	for n := range texIntParams {
		expect[n] = want{mCountRef, "texIntParams"}
	}
	for _, n := range texDimenParams {
		expect[n] = want{mDimenRef, "texDimenParams"}
	}
	for _, n := range texGlueParams {
		expect[n] = want{mSkipRef, "texGlueParams"}
	}

	// A name that is legitimately something else, with the reason. A primitive
	// is already judged by the two tests above, so it is not re-reported here —
	// but anything else needs an argument written down.
	excused := map[string]string{}

	var bad []string
	for n, w := range expect {
		if _, ok := excused[n]; ok {
			continue
		}
		m := e.eq[n]
		if m == nil {
			bad = append(bad, `\`+n+" ("+w.list+") is not defined at all")
			continue
		}
		if m.kind == mPrim {
			continue // the other two tests own this case
		}
		if m.kind != w.kind {
			bad = append(bad, `\`+n+" ("+w.list+") has kind "+
				strconv.Itoa(int(m.kind))+", want "+strconv.Itoa(int(w.kind)))
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d listed parameter(s) ended up a register of the WRONG TYPE:\n  %s\n"+
			"A later \\newdimen/\\newskip/\\newcount of the same name rebinds it, and the "+
			"type decides how an assignment is SCANNED — a <dimen> scan eats a unit out of "+
			"the following text. Remove the duplicate declaration, or excuse the name here "+
			"with its reason.",
			len(bad), strings.Join(bad, "\n  "))
	}
}
