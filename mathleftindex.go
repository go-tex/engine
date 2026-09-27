// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// leftindex.sty puts a symbol's indices on its LEFT: \leftindex^a_b X sets a and b
// before X rather than after. 27 equations on one arXiv paper of the reference corpus
// carried it, and one unknown command drops the whole equation.
//
// It is rewritten into source here rather than implemented in go-tex/math, following
// resolvePhysics: a PACKAGE's commands are the engine's business, and they are only
// rewritten when the document asked for the package. go-tex/math already renders left
// indices the ordinary TeX way, as an empty nucleus carrying the scripts —
// {}^{a}_{b}I measures 7.000 wide against I's 3.000 — so this needs no new layout.
//
// The package itself cannot be embedded: it is expl3 (\ProvidesExplPackage,
// \DeclareDocumentCommand, \cs_new_protected:Npn) and requires xparse and mathtools.
// Its signature is the reason a plain \newcommand cannot express it either:
//
//	leftindex.sty:12  \DeclareDocumentCommand\leftindex { o o E{^_}{{}{}} m }
//
//	o o          two optional bracket PHANTOMS, which only tune the kerning
//	E{^_}{{}{}}  xparse embellishments: ^ and _, EITHER ORDER, either omitted,
//	             defaulting to empty — so \leftindex X with no indices is legal
//	m            the symbol
//
// The phantoms are dropped. They compensate for a slanted symbol's overhang by
// measuring a stand-in glyph (\mathpalette over \vphantom), which is a kerning
// refinement and not content; keeping the indices is what the reader needs.
func (e *Engine) resolveLeftIndex(src, name string) (string, bool) {
	if !e.pkgRequested["leftindex"] || (name != "leftindex" && name != "manualleftindex") {
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
		if name == "manualleftindex" {
			// \manualleftindex{height phantom}{slant phantom}{sup}{sub} — the four-argument
			// form \leftindex itself calls (leftindex.sty:4). The two phantoms are dropped
			// for the same reason, and it does NOT consume the symbol: the caller writes it.
			args, consumed, ok := parseMathArgs(rest, 4)
			if !ok {
				out.WriteString(needle)
				src = rest
				continue
			}
			out.WriteString(leftIndexScripts(args[2], args[3]))
			src = rest[consumed:]
			changed = true
			continue
		}
		// Two optional phantoms, then the embellishments in either order.
		rest = skipMathOptArg(skipMathOptArg(rest))
		sup, sub := "", ""
		for {
			r := strings.TrimLeft(rest, " ")
			if len(r) == 0 || (r[0] != '^' && r[0] != '_') {
				break
			}
			args, consumed, ok := parseMathArgs(r[1:], 1)
			if !ok {
				break
			}
			if r[0] == '^' {
				sup = args[0]
			} else {
				sub = args[0]
			}
			rest = r[1+consumed:]
		}
		// The symbol is a required argument: without it the source is malformed and
		// left verbatim, so go-tex/math reports it rather than this silently eating
		// the command.
		args, consumed, ok := parseMathArgs(rest, 1)
		if !ok {
			out.WriteString(needle)
			src = rest
			continue
		}
		out.WriteString(leftIndexScripts(sup, sub))
		out.WriteString("{" + args[0] + "}")
		src = rest[consumed:]
		changed = true
	}
	if !changed {
		return src, false
	}
	out.WriteString(src)
	return out.String(), true
}

// leftIndexScripts writes the scripts as TeX's own left-index idiom: an EMPTY nucleus
// carrying them, so the next atom sets after them. Both empty yields nothing, which is
// what the package's {{}{}} default means.
func leftIndexScripts(sup, sub string) string {
	if sup == "" && sub == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("{}")
	if sup != "" {
		b.WriteString("^{" + sup + "}")
	}
	if sub != "" {
		b.WriteString("_{" + sub + "}")
	}
	return b.String()
}
