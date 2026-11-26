// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// NFSS's series and shape axes, with the kernel's own merge rules.
//
// A font switch is not a face. \bfseries changes only the SERIES and \itshape
// only the SHAPE (ltfssdcl.dtx: \bfseries is \fontseries\bfdefault\selectfont),
// and \selectfont installs the face the family declares for the pair. Since the
// 2020 kernel, \fontshape does not simply overwrite \f@shape either: it MERGES the
// request into the current shape through a rule table, which is why
// \scshape\itshape gives italic small capitals rather than italics.
//
// The algorithm and both tables below are tectonic's (LaTeX2e 2021-11-15),
// read out of the running kernel with \meaning and \detokenize, not recalled:
//
//	\merge@font@shape #1 -> \csname shape@\f@shape @#1\endcsname {#1}\@nil
//	\merge@font@shape@ #1#2#3\@nil ->
//	   no rule: \f@shape := the request
//	   else:    the rule's result if the family declares series/result,
//	            else its fallback if declared, else the request (with a warning)
//
// and \merge@font@series is the same over \series@<current>@<requested>, testing
// <requested>/\f@shape. \DeclareFontShapeChangeRule{#1}{#2}{#3}{#4} is
// \@namedef{shape@#1@#2}{{#3}{#4}}.
//
// \selectfont then sets the face for \f@series/\f@shape. A pair the family does
// not declare is \wrong@fontshape's: the default shape (n) in the same series,
// then the default series and shape (m/n) — and \f@shape KEEPS the requested
// shape, so {\bfseries\scshape\mdseries x} is small capitals again (measured:
// "bx/sc ... using bx/n", then m/sc).
//
// Measured against tectonic (testdata/parity/nfssrules.tex):
//
//	\bfseries\scshape          bx/sc  -> lmroman10-bold          (bx/sc undeclared)
//	\scshape\itshape           m/scsl -> lmromancaps10-oblique   (sc+it = scit, else scsl)
//	\slshape\scshape           m/scsl -> lmromancaps10-oblique
//	\bfseries\scshape\mdseries m/sc   -> lmromancaps10-regular
//	\scshape\itshape\upshape   m/sc   -> lmromancaps10-regular   (scit+up = sc)
//	\scshape\ulcshape          m/n    -> lmroman10-regular
//	\bfseries\scshape\itshape  bx/it  -> lmroman10-bolditalic    (neither scit nor scsl in bx)
//	\fontshape{it}\selectfont  m/it   -> lmroman10-italic
//	\itshape\rmfamily          m/it   -> lmroman10-italic        (a family switch keeps the shape)
//
// The old one-letter switches do not merge: latex.ltx declares \bf as
// \normalfont\bfseries (\DeclareOldFontCommand), so {\it\bf x} is upright bold.
//
// ⚠ Bounded, and stated rather than guessed at:
//   - ONE family's declarations, tulmr.fd's. The typewriter and sans faces have no
//     shape table here; a series or shape switch inside them sets the roman face,
//     as this engine always did.
//   - tulmr.fd declares three pairs this engine has no face for: m/ui (unslanted
//     italic) and the b series (lmromandemi). Each is set in the nearest face it
//     has (nfssStandIn); \f@series/\f@shape still read what LaTeX's do.
//   - NFSS's font warnings are not printed.

// Internal names for the faces only a series+shape pair reaches. Not \bfit:
// documents define that one themselves.
const (
	boldItalicName = "gotex@font@bx/it"
	boldSlantName  = "gotex@font@bx/sl"
	capsSlantName  = "gotex@font@m/scsl"
)

// nfssShapeRules is the kernel's \shape@<current>@<requested> table, verbatim.
var nfssShapeRules = map[string][2]string{
	"n@it": {"it", "sl"}, "n@sl": {"sl", "it"}, "n@up": {"n", ""}, "n@ulc": {"n", ""},
	"it@sl": {"sl", "it"}, "it@sc": {"scit", "scsl"}, "it@up": {"n", ""}, "it@ulc": {"it", ""},
	"sl@it": {"it", "sl"}, "sl@sc": {"scsl", "scit"}, "sl@up": {"n", ""}, "sl@ulc": {"sl", ""},
	"sc@it": {"scit", "scsl"}, "sc@sl": {"scsl", "scit"}, "sc@up": {"n", ""}, "sc@ulc": {"n", ""},
	"sc@sw":   {"scsw", "sw"},
	"scit@it": {"scit", ""}, "scit@sl": {"scsl", "scit"}, "scit@sc": {"scit", ""},
	"scit@up": {"sc", ""}, "scit@ulc": {"it", ""}, "scit@sw": {"scsw", "sc"},
	"scsl@it": {"scit", "scsl"}, "scsl@sl": {"scsl", ""}, "scsl@sc": {"scsl", ""},
	"scsl@up": {"sc", ""}, "scsl@ulc": {"sl", ""}, "scsl@sw": {"scsw", "sc"},
	"sw@sc": {"scsw", ""}, "sw@up": {"n", ""}, "sw@ulc": {"sw", ""},
	"scsw@it": {"scit", "scsw"}, "scsw@sl": {"scsl", ""}, "scsw@sc": {"scsw", ""},
	"scsw@up": {"sc", ""}, "scsw@ulc": {"sw", ""}, "scsw@sw": {"scsw", ""},
}

// nfssSeriesRules is the kernel's \series@<current>@<requested> table, verbatim.
var nfssSeriesRules = map[string][2]string{
	"m@b": {"b", "bx"}, "m@bx": {"bx", "b"}, "m@sb": {"sb", "b"}, "m@l": {"l", "m"}, "m@c": {"c", "m"},
	"b@bx": {"bx", "b"}, "b@sb": {"sb", "b"}, "b@c": {"bc", "b"},
	"bx@b": {"bx", ""}, "bx@sb": {"sbx", ""}, "bx@l": {"lx", ""}, "bx@c": {"bc", "bx"},
	"bx@eb": {"ebx", ""}, "bx@ub": {"ubx", ""}, "bx@el": {"elx", ""}, "bx@ul": {"ulx", ""},
	"sb@c": {"sbc", "bc"},
	"l@b":  {"b", "bx"}, "l@bx": {"bx", "b"}, "l@sb": {"sb", "b"}, "l@c": {"lc", "l"},
	"c@b": {"bc", ""}, "c@bx": {"bx", "b"}, "c@sb": {"sbc", ""}, "c@l": {"lc", ""},
	"c@eb": {"ebc", ""}, "c@ub": {"ubc", ""}, "c@el": {"elc", ""}, "c@ul": {"ulc", ""},
	"bc@b": {"bc", ""}, "bc@sb": {"sbc", ""}, "bc@l": {"lc", ""}, "bc@c": {"bc", ""},
	"bc@eb": {"ebc", ""}, "bc@ub": {"ubc", ""}, "bc@el": {"elc", ""}, "bc@ul": {"ulc", ""},
	"lc@b": {"bc", ""}, "lc@sb": {"sbc", ""}, "lc@l": {"lc", ""}, "lc@c": {"lc", ""},
	"lc@eb": {"ebc", ""}, "lc@ub": {"ubc", ""}, "lc@el": {"elc", ""}, "lc@ul": {"ulc", ""},
	"eb@c": {"ebc", ""}, "ub@c": {"ubc", ""}, "el@c": {"elc", ""}, "ul@c": {"ulc", ""},
}

// lmrDeclared is every series/shape pair tulmr.fd declares (\ifcsname
// TU/lmr/<series>/<shape>, read from tectonic). b/it is declared as a
// substitution (sub*lmr/b/sl), which \ifcsname counts as declared.
var lmrDeclared = map[string]bool{
	"m/n": true, "m/it": true, "m/sl": true, "m/sc": true, "m/scsl": true, "m/ui": true,
	"b/n": true, "b/it": true, "b/sl": true,
	"bx/n": true, "bx/it": true, "bx/sl": true,
}

// nfssFaces is which bound face holds each declared pair.
var nfssFaces = map[string]string{
	"m/n": "rm", "bx/n": "bf", "m/it": "it", "m/sl": "sl", "m/sc": "sc",
	"bx/it": boldItalicName, "bx/sl": boldSlantName, "m/scsl": capsSlantName,
}

// Internal names for the sans faces a font package binds beside \sf (the regular).
const (
	sansBoldName   = "gotex@font@sf/bx/n"
	sansItalicName = "gotex@font@sf/m/it"
	// sansBoldSlantName is bx/sl of the sans family (tulmss.fd: lmsans10-boldoblique).
	sansBoldSlantName = "gotex@font@sf/bx/sl"
)

// nfssSansFaces is which bound face holds each pair in the sans family. A pair
// with no entry, or no bound face, is set in the roman family.
var nfssSansFaces = map[string]string{
	"m/n": "sf", "bx/n": sansBoldName, "m/it": sansItalicName, "m/sl": sansItalicName,
	// tulmss.fd declares bx/sl; bx/it is a substitution of it, as m/it is of m/sl.
	"bx/sl": sansBoldSlantName, "bx/it": sansBoldSlantName,
}

// nfssStandIn is the face set for a declared pair this engine has no file for.
var nfssStandIn = map[string]string{
	"m/ui": "m/it",
	"b/n":  "bx/n", "b/it": "bx/sl", "b/sl": "bx/sl", // b/it is sub*lmr/b/sl
}

// faceKey identifies the face a font was bound as, at whatever size it is now
// set: a Latin Modern face by its optical table (lmroman8-bold and lmroman10-bold
// are one face), any other by its bytes. nil for a face this cannot identify.
func faceKey(f fontFace) *byte {
	o, ok := f.(*OpenTypeFont)
	if !ok || o == nil {
		return nil
	}
	if len(o.optical) > 0 && len(o.optical[0].data) > 0 {
		return &o.optical[0].data[0]
	}
	if len(o.data) > 0 {
		return &o.data[0]
	}
	return nil
}

// nfssGet reads \f@series or \f@shape; def when the kernel defined neither.
func (e *Engine) nfssGet(name, def string) string {
	m := e.eq[name]
	if m == nil || m.kind != mMacro {
		return def
	}
	return plainText(m.body)
}

// nfssSet assigns \f@series or \f@shape locally, so a group restores it.
func (e *Engine) nfssSet(name, v string) {
	e.define(name, &meaning{kind: mMacro, body: stringToToks(v)}, false)
}

func (e *Engine) nfssSeries() string { return e.nfssGet("f@series", "m") }
func (e *Engine) nfssShape() string  { return e.nfssGet("f@shape", "n") }

// mergeShape is \merge@font@shape@: the shape a \fontshape{req} leaves.
func mergeShape(series, cur, req string) string {
	// No rule for the pair is the zero entry, whose two shapes are empty: no pair
	// "<series>/" is declared, so both tests fail and the request stands, as it does
	// when the kernel has no \shape@<cur>@<req>.
	r := nfssShapeRules[cur+"@"+req]
	if lmrDeclared[series+"/"+r[0]] {
		return r[0]
	}
	if lmrDeclared[series+"/"+r[1]] {
		return r[1]
	}
	return req
}

// mergeSeries is \merge@font@series@: the series a \fontseries{req} leaves.
func mergeSeries(cur, req, shape string) string {
	// No rule for the pair is the zero entry, as in mergeShape: "/<shape>" is not a
	// declared pair, and the request stands.
	r := nfssSeriesRules[cur+"@"+req]
	if lmrDeclared[r[0]+"/"+shape] {
		return r[0]
	}
	if lmrDeclared[r[1]+"/"+shape] {
		return r[1]
	}
	return req
}

// nfssFace is the face \selectfont installs for series/shape: the declared pair,
// else \wrong@fontshape's substitutes, series/n then m/n. nil when none of them
// is bound (a host that named its own faces).
func (e *Engine) nfssFace(series, shape string) fontFace {
	try := []string{series + "/" + shape}
	if !lmrDeclared[try[0]] {
		try = append(try, series+"/n", "m/n")
	}
	sans := e.nfssGet("f@family", "rm") == "sf"
	for _, k := range try {
		if s, ok := nfssStandIn[k]; ok {
			k = s
		}
		if sans {
			// A sans face bound for this pair wins; with none bound the roman one
			// is set, as this engine always did.
			if m := e.eq[nfssSansFaces[k]]; m != nil && m.kind == mFont {
				return m.font
			}
		}
		if m := e.eq[nfssFaces[k]]; m != nil && m.kind == mFont {
			return m.font
		}
	}
	return nil
}

// nfssSwitch runs a font switch through NFSS: it updates \f@series/\f@shape as
// the kernel would and returns the face to set. false means "set the switch's
// own face", as before: a family switch to the typewriter or sans, a face this
// table cannot find, or a name that is not an NFSS switch at all.
func (e *Engine) nfssSwitch(name string) (fontFace, bool) {
	series, shape := e.nfssSeries(), e.nfssShape()
	switch name {
	case "bfseries":
		series = mergeSeries(series, "bx", shape)
	case "mdseries":
		series = mergeSeries(series, "m", shape)
	case "itshape":
		shape = mergeShape(series, shape, "it")
	case "slshape":
		shape = mergeShape(series, shape, "sl")
	case "scshape":
		shape = mergeShape(series, shape, "sc")
	case "upshape":
		shape = mergeShape(series, shape, "up")
	case "ulcshape":
		shape = mergeShape(series, shape, "ulc")
	case "rmfamily":
		// The family changes; series and shape stay: {\itshape\rmfamily x} is italic.
		e.nfssSet("f@family", "rm")
	case "normalfont", "rm":
		series, shape = "m", "n"
		e.nfssSet("f@family", "rm")
	case "bf":
		series, shape = "bx", "n"
		e.nfssSet("f@family", "rm")
	case "it":
		series, shape = "m", "it"
		e.nfssSet("f@family", "rm")
	case "sl":
		series, shape = "m", "sl"
		e.nfssSet("f@family", "rm")
	case "sc":
		series, shape = "m", "sc"
		e.nfssSet("f@family", "rm")
	case "sf", "sffamily":
		// \sffamily is a copy of \sf made when the engine was built (aliasFontSwitches),
		// so its own face is the one bound then: a package that binds another sans
		// (libertine's Biolinum) is reached through the CURRENT binding of \sf.
		e.nfssSet("f@family", "sf")
		if name == "sffamily" {
			// A family switch keeps series and shape, as \rmfamily does.
			if f := e.nfssFace(series, shape); f != nil {
				return f, true
			}
			return nil, false
		}
		e.nfssSet("f@series", "m")
		e.nfssSet("f@shape", "n")
		if m := e.eq["sf"]; m != nil && m.kind == mFont {
			return m.font, true
		}
		return nil, false
	case "tt", "ttfamily":
		e.nfssSet("f@family", "tt")
		if name == "tt" {
			e.nfssSet("f@series", "m")
			e.nfssSet("f@shape", "n")
		}
		return nil, false
	default:
		return nil, false
	}
	e.nfssSet("f@series", series)
	e.nfssSet("f@shape", shape)
	if f := e.nfssFace(series, shape); f != nil {
		return f, true
	}
	return nil, false
}

// loadNFSS installs the primitives the kernel layers define \fontseries,
// \fontshape and \selectfont through.
func (e *Engine) loadNFSS() {
	e.prim("gotex@fontseries", func(e *Engine) {
		req := e.expandToString(e.grabUndelimited())
		e.nfssSet("f@series", mergeSeries(e.nfssSeries(), req, e.nfssShape()))
	})
	e.prim("gotex@fontshape", func(e *Engine) {
		req := e.expandToString(e.grabUndelimited())
		e.nfssSet("f@shape", mergeShape(e.nfssSeries(), e.nfssShape(), req))
	})
	// \selectfont: the face for \f@series/\f@shape at the size in force. It used to
	// reselect the \normalsize roman face, which undid both the shape and the size
	// in force. A package that switched to \nullfont gets the roman face back as
	// before, at the base size, since \nullfont has none.
	//
	// The kernel layers run \gotex@pendingsize first: a \fontsize recorded since
	// the last \selectfont is applied there, and this face is then taken at it.
	e.prim("gotex@selectfont", func(e *Engine) {
		f := e.nfssFace(e.nfssSeries(), e.nfssShape())
		if f == nil {
			f = e.baseFont
		}
		if f == nil {
			return
		}
		e.selectFont(e.faceAtCurrentSize(f))
	})
}
