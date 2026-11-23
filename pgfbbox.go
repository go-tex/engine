// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// A pgfpicture that declares no bounding box still has one: pgf grows it from
// every point the path construction protocols (\pgf@protocolsizes,
// pgfcorepoints.code.tex), and \pgfsys@typesetpicturebox sets the picture as a box
// of exactly that size, depth 0, baseline at the lower border
// (pgfcorescopes.code.tex:366-382, pgfsys.code.tex:1596-1611). The line width is
// not part of it.
//
// When the picture's body is nothing but straight path construction between
// \pgfpoint corners — \pgfpathrectangle, \pgfpathmoveto, \pgfpathlineto — and the
// state settings and \pgfusepath that paint it, that box is known without drawing,
// and the gobbled picture keeps it. beamer's shadow theme is one: its headline
// ends with a 4pt-tall shadow rectangle under \pgfsetfading
// (beamerouterthemeshadow.sty:35-39), and the 96x60pt placeholder in its place made
// the headline 64.8pt tall where the reference measures 13.7pt (go-tex/engine#926).
//
// Anything else — a curve, a node, text, a transformation — and the size is not
// knowable this way: the picture keeps the placeholder, as before.

// pgfPathPoints names the path commands whose arguments are \pgfpoint corners, and
// how many: a rectangle is a corner and a size, the others one point each.
var pgfPathPoints = map[string]int{
	"pgfpathrectangle": 2,
	"pgfpathmoveto":    1,
	"pgfpathlineto":    1,
}

// pgfNeutral are the commands that close, paint or style the path without adding
// a point, and how many braced arguments each reads, from pgf's definitions:
// pgfcorepathconstruct.code.tex:204, pgfcorepathusage.code.tex:80,
// pgfcoretransparency.code.tex:22/36/130, pgfcoregraphicstate.code.tex:25/126/161/186.
// Exactly that many are skipped: a group after them is text, not an argument.
var pgfNeutral = map[string]int{
	"pgfpathclose":        0,
	"relax":               0,
	"pgfusepath":          1,
	"pgfsetcolor":         1,
	"pgfsetfillcolor":     1,
	"pgfsetstrokecolor":   1,
	"pgfsetlinewidth":     1,
	"pgfsetfillopacity":   1,
	"pgfsetstrokeopacity": 1,
	"pgfsetfading":        2,
}

// pgfBodyBBox computes the bounding box of a recorded pgfpicture body, in sp. ok is
// false when the body holds anything whose extent this cannot know, or no point at
// all.
func (e *Engine) pgfBodyBBox(body []tok) (minx, miny, maxx, maxy int, ok bool) {
	seen := false
	add := func(x, y int) {
		if !seen {
			minx, maxx, miny, maxy = x, x, y, y
			seen = true
			return
		}
		minx, maxx = min(minx, x), max(maxx, x)
		miny, maxy = min(miny, y), max(maxy, y)
	}
	for i := 0; i < len(body); {
		t := body[i]
		switch {
		case !t.cs_ && t.cat == catSpace:
			i++
		case t.cs_ && (t.cs == "end" || t.cs == "endpgfpicture"):
			// The closing \end{pgfpicture} that gobbleEnvBody consumed last.
			return minx, miny, maxx, maxy, seen
		case t.cs_ && isNeutral(t.cs):
			i++
			for k := 0; k < pgfNeutral[t.cs]; k++ {
				_, next, found := nextGroup(body, i)
				if !found {
					return 0, 0, 0, 0, false
				}
				i = next
			}
		case t.cs_ && pgfPathPoints[t.cs] > 0:
			n := pgfPathPoints[t.cs]
			i++
			var pts [2][2]int
			for k := 0; k < n; k++ {
				// A missing group is an empty argument, which pgfPoint refuses.
				arg, next, _ := nextGroup(body, i)
				x, y, good := e.pgfPoint(arg)
				if !good {
					return 0, 0, 0, 0, false
				}
				pts[k] = [2]int{x, y}
				i = next
			}
			add(pts[0][0], pts[0][1])
			if t.cs == "pgfpathrectangle" {
				// The second point is the rectangle's SIZE, relative to the corner.
				add(pts[0][0]+pts[1][0], pts[0][1]+pts[1][1])
			}
		default:
			return 0, 0, 0, 0, false
		}
	}
	return 0, 0, 0, 0, false // never reached the closing \end
}

// pgfPoint evaluates a {\pgfpoint{x}{y}} argument.
func (e *Engine) pgfPoint(arg []tok) (x, y int, ok bool) {
	i := skipSpaces(arg, 0)
	if i >= len(arg) || !arg[i].cs_ || arg[i].cs != "pgfpoint" {
		return 0, 0, false
	}
	// A missing x group leaves next on a token that opens no group, so the y group
	// is missing too, and refused below.
	xs, next, _ := nextGroup(arg, i+1)
	ys, next, found := nextGroup(arg, next)
	if !found || skipSpaces(arg, next) != len(arg) {
		return 0, 0, false
	}
	if x, ok = e.evalDimenToks(xs); !ok {
		return 0, 0, false
	}
	if y, ok = e.evalDimenToks(ys); !ok {
		return 0, 0, false
	}
	return x, y, true
}

// evalDimenToks scans toks as one dimension, through the engine's own scanner, so
// a register (\paperwidth) or a unit (ex) means what it means in the document. It
// fails when anything is left over after the dimension.
func (e *Engine) evalDimenToks(toks []tok) (int, bool) {
	if len(toks) == 0 {
		return 0, false
	}
	const marker = "gotex@pgfbbox@end"
	in := make([]tok, 0, len(toks)+3)
	in = append(in, chTok('{', catBegin))
	in = append(in, toks...)
	in = append(in, chTok('}', catEnd), csTok(marker))
	e.push(in)
	d := e.readBraceDimen()
	t, ok := e.getNext()
	if ok && t.cs_ && t.cs == marker {
		return d, true
	}
	// Something followed the dimension: drain to the marker and give up.
	for ok && !(t.cs_ && t.cs == marker) {
		t, ok = e.getNext()
	}
	return 0, false
}

func skipSpaces(ts []tok, i int) int {
	for i < len(ts) && !ts[i].cs_ && ts[i].cat == catSpace {
		i++
	}
	return i
}

// nextGroup returns the contents of the brace group starting at ts[i] (after
// spaces), and the index past its closing brace.
func nextGroup(ts []tok, i int) (body []tok, next int, ok bool) {
	i = skipSpaces(ts, i)
	if i >= len(ts) || ts[i].cs_ || ts[i].cat != catBegin {
		return nil, i, false
	}
	depth := 0
	for j := i; j < len(ts); j++ {
		if ts[j].cs_ {
			continue
		}
		switch ts[j].cat {
		case catBegin:
			depth++
		case catEnd:
			depth--
			if depth == 0 {
				return ts[i+1 : j], j + 1, true
			}
		}
	}
	return nil, len(ts), false
}

func isNeutral(cs string) bool {
	_, ok := pgfNeutral[cs]
	return ok
}
