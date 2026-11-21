// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// A series switch and a shape switch COMBINE, as NFSS's two axes do.
//
// ⛔ EACH FONT SWITCH WAS ONE FACE, and the last one won: \textbf{\textit{x}} set x
// in the medium italic, and {\itshape\bfseries x} in the upright bold, where
// LaTeX selects TU/lmr/bx/it — lmroman10-bolditalic — for both. \bfseries only
// changes the SERIES and \itshape only the SHAPE (ltfssdcl.dtx: \bfseries is
// \fontseries\bfdefault\selectfont, \itshape is \fontshape\itdefault\selectfont),
// so the face is the pair, looked up in the family's shape table.
//
// Only the NFSS names combine. The old one-letter switches do NOT: latex.ltx
// declares \bf as \normalfont\bfseries (\DeclareOldFontCommand), so {\it\bf x} is
// upright bold in LaTeX too, and \bf, \it, \sl keep their own face here.
//
// ⚠ A pair the family does not have falls back to the switch's own face, which is
// what this engine always did. NFSS would substitute along its own rules (to the
// family's default shape first), so {\bfseries\scshape x} — bx/sc does not exist
// in Latin Modern — is medium small caps here where LaTeX gives upright bold.
// Bounded and stated rather than guessed at.

// nfssRole is a face's place on the two axes: series 'm' or 'b', shape 'n'
// (upright), 'i' (italic), 's' (slanted) or 'c' (small caps).
type nfssRole struct{ series, shape byte }

// Internal names for the two faces only a series+shape pair reaches. Not \bfit:
// documents define that one themselves.
const (
	boldItalicName = "gotex@font@bx/it"
	boldSlantName  = "gotex@font@bx/sl"
)

// nfssFaces is the family's shape table: which bound face holds each pair.
var nfssFaces = []struct {
	name string
	role nfssRole
}{
	{"rm", nfssRole{'m', 'n'}},
	{"bf", nfssRole{'b', 'n'}},
	{"it", nfssRole{'m', 'i'}},
	{"sl", nfssRole{'m', 's'}},
	{"sc", nfssRole{'m', 'c'}},
	{boldItalicName, nfssRole{'b', 'i'}},
	{boldSlantName, nfssRole{'b', 's'}},
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

// nfssRoleOf is where the current face sits on the two axes, or false when it is
// none of the family's bound faces (the typewriter, a math font, a host's face
// bound under no role).
func (e *Engine) nfssRoleOf(f fontFace) (nfssRole, bool) {
	k := faceKey(f)
	if k == nil {
		return nfssRole{}, false
	}
	for _, s := range nfssFaces {
		if m := e.eq[s.name]; m != nil && m.kind == mFont && faceKey(m.font) == k {
			return s.role, true
		}
	}
	return nfssRole{}, false
}

// nfssSwitch is the face an NFSS series or shape switch selects from the current
// one: the other axis is kept. false means "not such a switch, or no such pair":
// the caller then selects the switch's own face, as before.
func (e *Engine) nfssSwitch(name string) (fontFace, bool) {
	var series, shape byte
	switch name {
	case "bfseries":
		series = 'b'
	case "mdseries":
		series = 'm'
	case "itshape":
		shape = 'i'
	case "slshape":
		shape = 's'
	case "scshape":
		shape = 'c'
	case "upshape":
		shape = 'n'
	default:
		return nil, false
	}
	// A current face outside the family (the typewriter, a math font) has the zero
	// role. A switch sets one axis only, so the other stays 0, and no entry of
	// nfssFaces has a 0 axis: the search below finds nothing and the caller
	// selects the switch's own face, as it does for a pair the family lacks.
	want, _ := e.nfssRoleOf(e.curFont)
	if series != 0 {
		want.series = series
	}
	if shape != 0 {
		want.shape = shape
	}
	for _, s := range nfssFaces {
		if s.role == want {
			if m := e.eq[s.name]; m != nil && m.kind == mFont {
				return m.font, true
			}
		}
	}
	return nil, false
}
