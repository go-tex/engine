// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// Text-font packages, and why asking for one is worth reporting.
//
// The engine sets everything in ONE built-in face. A document that loads a text-font
// package therefore gets a substitute, always — and until now it got it in silence:
// no skipped command (the package's own macros are defined, or the .sty loads and
// simply has no metrics behind it), no warning, nothing in Diagnostics.
//
// That silence is expensive. acmart does \RequirePackage{libertine} and sets its body
// in Linux Libertine; measured on a controlled two-column sigconf document — same
// text, same 9pt, same 506.295pt block, same 24pt gutter, same 11pt leading, and 57
// lines per column in BOTH engines — our characters came out 4.619bp wide against
// tectonic's 3.990: 15.8% wider, so 8.77 words to a line instead of 10.04, 1005 lines
// instead of 875, and two extra pages out of eight. Nothing in the geometry could have
// explained it, and three passes went into measuring the geometry first
// (go-tex/engine#310).
//
// The list is of package names whose whole purpose is to change the TEXT font. A
// maths-only package (amssymb, mathtools) is not here; neither is fontenc/inputenc,
// which change encoding rather than face. lmodern is not here either: it names the
// face the engine already sets.
var textFontPackages = map[string]string{
	"libertine":        "Linux Libertine",
	"libertinus":       "Libertinus",
	"libertinust1math": "Libertinus",
	"biolinum":         "Linux Biolinum",
	"newtxtext":        "TeX Gyre Termes (Times)",
	"newtxmath":        "TeX Gyre Termes (Times)",
	"newpxtext":        "TeX Gyre Pagella (Palatino)",
	"newpxmath":        "TeX Gyre Pagella (Palatino)",
	"times":            "Times",
	"mathptmx":         "Times",
	"txfonts":          "Times",
	"helvet":           "Helvetica",
	"courier":          "Courier",
	"charter":          "Bitstream Charter",
	"XCharter":         "XCharter",
	"mathpazo":         "Palatino",
	"palatino":         "Palatino",
	"pxfonts":          "Palatino",
	"fourier":          "Utopia",
	"utopia":           "Utopia",
	"kpfonts":          "Kp-Fonts",
	"bookman":          "Bookman",
	"newcent":          "New Century Schoolbook",
	"avant":            "Avant Garde",
	"garamondx":        "Garamond",
	"cochineal":        "Cochineal",
	"crimson":          "Crimson",
	"inter":            "Inter",
	"sourcesanspro":    "Source Sans Pro",
	"sourceserifpro":   "Source Serif Pro",
	"roboto":           "Roboto",
	"lato":             "Lato",
	"opensans":         "Open Sans",
	"merriweather":     "Merriweather",
	"eulervm":          "Euler",
	"concrete":         "Concrete",
	"ccfonts":          "Concrete",
	"arev":             "Arev Sans",
	"cmbright":         "CM Bright",
	"iwona":            "Iwona",
	"antpolt":          "Antykwa Poltawskiego",
	"tgtermes":         "TeX Gyre Termes",
	"tgpagella":        "TeX Gyre Pagella",
	"tgheros":          "TeX Gyre Heros",
	"tgschola":         "TeX Gyre Schola",
	"tgbonum":          "TeX Gyre Bonum",
	"tgadventor":       "TeX Gyre Adventor",
	"tgcursor":         "TeX Gyre Cursor",
	"anttor":           "Antykwa Torunska",
	"fontspec":         "a font named by \\setmainfont",
}

// noteFontSubstitution records that a document asked for a text face the engine does
// not have. Called for every requested package, so the tally is of REQUESTS: whether
// or not the .sty was found changes nothing, since the engine has no metrics for any
// of these faces either way.
func (e *Engine) noteFontSubstitution(pkg string) {
	face, ok := textFontPackages[pkg]
	if !ok {
		return
	}
	if e.fontSubst == nil {
		e.fontSubst = map[string]string{}
	}
	e.fontSubst[pkg] = face
}
