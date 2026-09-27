// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// mathSpaceSynonym maps a maths spacing command the layer does NOT know to the one it
// does, at the same width. Every value is read from amsmath.sty:170-178, which states
// each in both units:
//
//	\DeclareRobustCommand\!{\tmspace-\thinmuskip{.1667em}}    −3mu  \let\negthinspace\!
//	\DeclareRobustCommand\:{\tmspace+\medmuskip{.2222em}}      4mu  \let\medspace\:
//	\renewcommand\;{\tmspace+\thickmuskip{.2777em}}            5mu  \let\thickspace\;
//	\let\thinspace\,                                           3mu
//
// .1667em is 3/18, .2222em is 4/18, .2777em is 5/18 — so go-tex/math's own table
// (3, 4, 5, −3) is these values, and the mapping is exact rather than approximate.
//
// \> is plain TeX's medium space, which LaTeX spells \: because \> is tabbing there.
// It is the one that matters: 2157 papers of the 22127 harvested write it, against 399
// for \thinspace and single digits for the rest.
//
// A REWRITE and not a \def, because \thinspace and \negthinspace are text-mode
// commands too (format.go:45-46 sets them as \hskip of the same width, which is right
// in text) and \, is \let to \thinspace there — redefining either would be circular.
// Confining the translation to the maths source leaves text mode alone.
//
// \negmedspace and \negthickspace are deliberately absent: go-tex/math's table has one
// negative width, −3mu, so −4mu and −5mu cannot be expressed. Mapping them to \! would
// be a silently wrong width, which is worse than the command being reported.
var mathSpaceSynonym = map[string]string{
	">":            `\:`,
	"thinspace":    `\,`,
	"negthinspace": `\!`,
	"medspace":     `\:`,
	"thickspace":   `\;`,
}

// resolveMathSpaceSynonym substitutes such a command in a go-tex/math source string.
//
// Not gated on a package: every one of these is plain TeX or the LaTeX kernel, not a
// package's own name, so a document cannot mean something else by them.
func (e *Engine) resolveMathSpaceSynonym(src, name string) (string, bool) {
	target, ok := mathSpaceSynonym[name]
	if !ok {
		return src, false
	}
	return replaceMathCS(src, name, target), true
}
