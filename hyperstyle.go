// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// This file implements hyperref's link-styling options — the ones that change what
// a reader sees. hyperref can colour a link's TEXT (colorlinks) instead of drawing
// a coloured border rectangle around it; the coloured-text style is the one this
// engine can render, since a colour flows to both drivers through each glyph (see
// color.go), while a link border is not modelled. So \hypersetup{colorlinks=true}
// makes \url, \href and \hyperlink paint their text in the matching hyperref
// colour (urlcolor for \url/\href, linkcolor for \hyperlink), and with colorlinks
// off — hyperref's own default — links keep the surrounding text colour.
//
// Options arrive two ways, both routed here: \hypersetup{key=value,…} at any point,
// and the package-option form \usepackage[key=value,…]{hyperref} (see packages.go).

// applyHypersetup parses a hyperref key=value option list and records the
// link-styling options the engine acts on. Only options that change visible output
// here are honoured; hyperref's many others (bookmarks, pdfborder, pageanchor, …)
// are read past harmlessly. A value is resolved through the colour model, so names,
// rgb/HTML mixes and xcolor "!" expressions all work.
//
//   - colorlinks (boolean): colour link text instead of a border. A bare
//     "colorlinks" means true; "colorlinks=false" turns it back off.
//   - linkcolor: colour of internal links (\hyperlink).
//   - urlcolor:  colour of \url and \href.
//   - allcolors: sets both link and url colours at once.
func (e *Engine) applyHypersetup(opts string) {
	for _, seg := range splitKVSegments(opts, ',') {
		key, val, has := cutKeyVal(seg)
		switch key {
		case "colorlinks", "colourlinks":
			e.hyperColorlinks = boolOpt(val, has)
		case "linkcolor", "linkcolour":
			e.hyperLinkColor = e.resolveColor(val)
		case "urlcolor", "urlcolour":
			e.hyperURLColor = e.resolveColor(val)
		case "allcolors", "allcolours":
			c := e.resolveColor(val)
			e.hyperLinkColor, e.hyperURLColor = c, c
		}
	}
}

// doHypersetup implements \hypersetup{options}: read the braced key=value list and
// apply it. (The package-option form goes straight to applyHypersetup.)
func (e *Engine) doHypersetup() { e.applyHypersetup(e.readBraceName()) }

// beginLinkColor switches the current colour to c for building a link's inner box
// when colorlinks is on, returning the colour to restore afterwards. With
// colorlinks off it changes nothing, so the link text keeps the surrounding colour.
func (e *Engine) beginLinkColor(c uint32) uint32 {
	save := e.curColor
	if e.hyperColorlinks {
		e.curColor = c
	}
	return save
}

// splitKVSegments splits s on sep at brace depth 0, so a value carrying its own
// braces or commas — allcolors={red!50!blue} — is not cut apart. Each piece is
// trimmed and empty pieces are dropped.
func splitKVSegments(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case sep:
			if depth == 0 {
				if seg := strings.TrimSpace(s[start:i]); seg != "" {
					out = append(out, seg)
				}
				start = i + 1
			}
		}
	}
	if seg := strings.TrimSpace(s[start:]); seg != "" {
		out = append(out, seg)
	}
	return out
}

// cutKeyVal splits one "key=value" segment at its first '=' at brace depth 0. has
// is false for a bare boolean key ("colorlinks") with no '='. The value has any
// single wrapping pair of braces stripped (urlcolor={blue}), so it reaches the
// colour model as a plain expression.
func cutKeyVal(seg string) (key, val string, has bool) {
	depth := 0
	for i := 0; i < len(seg); i++ {
		switch seg[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '=':
			if depth == 0 {
				return strings.TrimSpace(seg[:i]), unwrapBraces(strings.TrimSpace(seg[i+1:])), true
			}
		}
	}
	return strings.TrimSpace(seg), "", false
}

// unwrapBraces removes one wrapping {…} pair from a value, if it is wrapped.
func unwrapBraces(v string) string {
	if len(v) >= 2 && v[0] == '{' && v[len(v)-1] == '}' {
		return strings.TrimSpace(v[1 : len(v)-1])
	}
	return v
}

// boolOpt reads a hyperref boolean option: a bare key is true; an explicit value
// is true unless it names an off state.
func boolOpt(val string, has bool) bool {
	if !has {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "false", "off", "no", "0":
		return false
	}
	return true
}
