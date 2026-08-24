// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A \global assignment drops the pending restores for the quantity it sets, so
// that the value outlives every open group. The entries it drops sit anywhere in
// the save stack, including below the mark of a group that is still open — and a
// mark is an index into that stack. If the marks are not moved down with it, the
// enclosing group closes restoring too few of its own entries, and an inner value
// escapes the group that made it.
//
// This is not a corner case: pgf makes a \global assignment on nearly every step
// of building a path, inside the group \tikz@@command@path opens, and the local
// \tikz@path@do@at@end that \tikz \draw …; relies on was lost exactly this way.
func TestGlobalAssignmentKeepsGroupMarks(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{
			"une entrée globale sous la marque du groupe intérieur",
			`\begingroup\def\g{L}\def\h{L}\begingroup\let\h\empty\global\def\g{G}\endgroup\h\endgroup`,
			"L",
		},
		{
			"plusieurs entrées supprimées d'un coup",
			`\begingroup\def\g{L}\def\h{L}\begingroup\def\g{M}\let\h\empty\global\def\g{G}\endgroup\h\endgroup`,
			"L",
		},
		{
			"registre entier rendu global",
			`\begingroup\count8=1 \def\h{L}\begingroup\let\h\empty\global\count8=2 \endgroup\h\endgroup`,
			"L",
		},
		{
			"dimension rendue globale",
			`\begingroup\dimen8=1pt \def\h{L}\begingroup\let\h\empty\global\dimen8=2pt \endgroup\h\endgroup`,
			"L",
		},
		{
			"deux groupes intérieurs successifs",
			`\begingroup\def\g{L}\def\h{L}\begingroup\global\def\g{G}\endgroup\begingroup\let\h\empty\global\def\g{H}\endgroup\h\endgroup`,
			"L",
		},
		{
			"trois niveaux, le global au plus profond",
			`\begingroup\def\g{L}\def\h{A}\begingroup\def\h{B}\begingroup\let\h\empty\global\def\g{G}\endgroup\h\endgroup\endgroup`,
			"B",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.SetFont(spMock{})
			e.Run(`\def\h{D}\def\g{D}\def\empty{}\setbox0=\hbox{` + c.src + `}`)
			if got := boxChars(e.box[0]); got != c.want {
				t.Errorf("%s\n  obtenu %q, attendu %q", c.src, got, c.want)
			}
		})
	}
}

// The global value itself must still survive the groups, which is what
// forgetSaved exists for. Moving the marks must not weaken that.
func TestGlobalAssignmentStillEscapesGroups(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.Run(`\def\g{D}\setbox0=\hbox{\begingroup\def\g{L}\begingroup\global\def\g{G}\endgroup\endgroup\g}`)
	if got := boxChars(e.box[0]); got != "G" {
		t.Errorf("valeur globale après fermeture des groupes : obtenu %q, attendu \"G\"", got)
	}
}
