// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// This file emulates the physics package inside a formula. go-tex/math knows none
// of its commands, and an unknown command makes it refuse the WHOLE equation — so
// one \grad costs the formula it stands in. Three papers of the fidelity corpus
// lose 47 equations between them that way, and two of the three paginate four
// pages SHORT, which is what a lost formula's vertical space looks like.
//
// Only what those papers actually write is here. physics is a large package and a
// speculative surface costs more than it saves: what is missing will announce
// itself in the dropped-equation census, by name, the way these did.
//
// Everything is gated on the document having asked for the package: \div is the
// division sign ÷ in plain TeX and the divergence \nabla\cdot only under physics,
// so an ungated table would corrupt every other document's formulas.

// mathPhysicsArity is the number of MANDATORY braced arguments each emulated
// command takes. \pdv and \dv also accept an optional [order], read separately.
var mathPhysicsArity = map[string]int{
	"grad": 0, "div": 0, "curl": 0, "laplacian": 0,
	"vb": 1, "norm": 1,
	"pdv": 2, "dv": 2,
}

// physicsText composes one physics command as go-tex/math source. order is the
// optional [n] of a derivative ("" when absent).
func physicsText(name, order string, args []string) string {
	sup := ""
	if order != "" {
		sup = "^{" + order + "}"
	}
	switch name {
	case "grad":
		return `\nabla `
	case "div":
		return `\nabla\cdot `
	case "curl":
		return `\nabla\times `
	case "laplacian":
		return `\nabla^{2} `
	case "vb":
		return `\mathbf{` + args[0] + `}`
	case "norm":
		return `\left\lVert ` + args[0] + `\right\rVert `
	case "pdv":
		return `\frac{\partial` + sup + ` ` + args[0] + `}{\partial ` + args[1] + sup + `}`
	case "dv":
		return `\frac{d` + sup + ` ` + args[0] + `}{d ` + args[1] + sup + `}`
	}
	return ""
}

// resolvePhysics substitutes every occurrence of the physics command \name in a
// go-tex/math source string, and reports whether it changed anything. An occurrence
// whose arguments cannot be parsed is left verbatim, so a malformed source is never
// silently corrupted — go-tex/math then refuses it, as before.
func (e *Engine) resolvePhysics(src, name string) (string, bool) {
	if !e.pkgRequested["physics"] {
		return src, false
	}
	n, ok := mathPhysicsArity[name]
	if !ok {
		return src, false
	}
	needle := "\\" + name + " "
	var out strings.Builder
	changed := false
	for {
		i := strings.Index(src, needle)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := src[i+len(needle):]
		order := ""
		if name == "pdv" || name == "dv" {
			if a, r, ok := takeMathOptArg(rest); ok {
				order, rest = a, r
			}
		}
		args, consumed, ok := parseMathArgs(rest, n)
		if !ok {
			out.WriteString(needle) // leave it for go-tex/math to reject
			src = src[i+len(needle):]
			continue
		}
		out.WriteString(physicsText(name, order, args))
		src = rest[consumed:]
		changed = true
	}
	out.WriteString(src)
	return out.String(), changed
}
