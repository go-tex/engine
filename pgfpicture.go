// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// When pgf is emulated, a pgfpicture is gobbled: the engine has no pgf to draw it
// with. One shape is worth more than its footprint, because it is TEXT: beamer's
// numbered ball items,
//
//	\begin{pgfpicture}{-1ex}{-0.65ex}{1ex}{1ex}
//	  \usebeamercolor{item projected}
//	  {\pgftransformscale{1.75}\pgftext{\normalsize\pgfuseshading{bigsphere}}}
//	  {\pgftransformshift{\pgfpoint{0pt}{0.5pt}}
//	    \pgftext{\usebeamerfont*{item projected}\color{fg}\insertenumlabel}}
//	\end{pgfpicture}                       beamerbaseauxtemplates.sty:374-384
//
// lost every item number (go-tex/engine#884). A picture whose body is nothing but
// \pgftext, the origin moves and the colour and font settings around them is
// typeset instead, by \gotex@pgfoldpic (latex.go): each \pgftext's text centred on
// the origin, in the box the picture declares. Anything else in the body — a path,
// a node, an option to \pgftext — and the picture is gobbled as before.

// pgfTextCommand describes a command allowed in a typeset picture body: whether it
// takes a star and an optional [..], and how many braced arguments follow.
type pgfTextCommand struct {
	star, opt bool
	args      int
}

var pgfTextCommands = map[string]pgfTextCommand{
	"pgftext":           {args: 1},
	"pgftransformshift": {args: 1},
	"pgftransformscale": {args: 1},
	"usebeamercolor":    {star: true, opt: true, args: 1},
	"usebeamerfont":     {star: true, args: 1},
	"color":             {opt: true, args: 1},
	"relax":             {},
	"par":               {},
}

// typesetPgfTextPicture typesets a declared-size picture whose body is only text,
// and reports whether it did. body is what gobbleEnvBody recorded, up to and
// including the closing \end.
func (e *Engine) typesetPgfTextPicture(minx, miny, maxx, maxy int, body []tok) bool {
	clean, hasText, ok := cleanPgfTextBody(body, true)
	if !ok || !hasText {
		return false
	}
	out := []tok{csTok("gotex@pgfoldpic")}
	for _, d := range []int{minx, miny, maxx, maxy} {
		out = append(out, chTok('{', catBegin))
		out = append(out, tokenizeTeX(fmt.Sprintf("%dsp", d))...)
		out = append(out, chTok('}', catEnd))
	}
	out = append(out, chTok('{', catBegin))
	out = append(out, clean...)
	out = append(out, chTok('}', catEnd))
	e.push(out)
	return true
}

// cleanPgfTextBody checks a picture body against pgfTextCommands and returns it
// without the spaces between its commands: in a picture they are not text (pgf
// sets the picture's own material in \nullfont), and here they would be glue that
// moves every \pgftext after them. Arguments are kept as written. top is true for
// the body itself, which ends at its \end.
func cleanPgfTextBody(ts []tok, top bool) (out []tok, hasText, ok bool) {
	for i := 0; i < len(ts); {
		t := ts[i]
		switch {
		case !t.cs_ && t.cat == catSpace:
			i++
		case !t.cs_ && t.cat == catBegin:
			// An unclosed group leaves next at len(ts): the body then never
			// reaches its \end and is refused below. Groups inside one are
			// balanced, so nextGroup's ok needs no test of its own here.
			inner, next, _ := nextGroup(ts, i)
			sub, text, good := cleanPgfTextBody(inner, false)
			if !good {
				return nil, false, false
			}
			hasText = hasText || text
			out = append(out, chTok('{', catBegin))
			out = append(out, sub...)
			out = append(out, chTok('}', catEnd))
			i = next
		case t.cs_ && top && (t.cs == "end" || t.cs == "endpgfpicture"):
			return out, hasText, true
		case t.cs_:
			cmd, known := pgfTextCommands[t.cs]
			if !known {
				return nil, false, false
			}
			out = append(out, t)
			i++
			if cmd.star {
				if j := skipSpaces(ts, i); j < len(ts) && !ts[j].cs_ && ts[j].ch == '*' {
					out = append(out, ts[j])
					i = j + 1
				}
			}
			if j := skipSpaces(ts, i); j < len(ts) && !ts[j].cs_ && ts[j].ch == '[' {
				if !cmd.opt {
					return nil, false, false // \pgftext[left] and the like: not handled
				}
				k := j
				for k < len(ts) && (ts[k].cs_ || ts[k].ch != ']') {
					k++
				}
				if k == len(ts) {
					return nil, false, false
				}
				out = append(out, ts[j:k+1]...)
				i = k + 1
			}
			for a := 0; a < cmd.args; a++ {
				arg, next, found := nextGroup(ts, i)
				if !found {
					return nil, false, false
				}
				out = append(out, chTok('{', catBegin))
				out = append(out, arg...)
				out = append(out, chTok('}', catEnd))
				i = next
			}
			if t.cs == "pgftext" {
				hasText = true
			}
		default:
			return nil, false, false // a character: material pgf would not set
		}
	}
	if top {
		return nil, false, false // never reached the closing \end
	}
	return out, hasText, true
}

// shadeStop is one colour stop of a radial shading: its radius in sp and its
// colour expression, resolved at use.
type shadeStop struct {
	r     int
	color string
}

// doShadeDisk implements \gotex@shadedisk{<colour spec>}: the box \pgfuseshading
// places for a radial shading, with the shading drawn in it as one flat disk.
//
// The box is pgf's: a square of side twice the last stop's radius, depth 0
// (pgfcoreshade.code.tex:479-488), times the scale \pgftransformscale set. The
// drawing is an approximation, since the drivers have no gradients: past the
// next-to-last stop pgf's shading blends into its last colour, the surround
// (parent.bg for beamer's spheres), so the disk ends there. It takes the colour
// of the last stop inside that edge, which covers the widest ring of the ball:
// for bigsphere, bg!70!black from 0.33ex to 0.45ex, against the highlight's
// bg!15 at the centre.
func (e *Engine) doShadeDisk() {
	spec := e.grabUndelimited()
	stops := e.parseShadeStops(spec)
	scale := 1.0
	if m := e.eq["gotex@pgfscale"]; m != nil && m.kind == mMacro {
		if f, err := strconv.ParseFloat(strings.TrimSpace(e.toksToString(m.body)), 64); err == nil && f > 0 {
			scale = f
		}
	}
	// The last two stops, read without indexing: no stop at all leaves a zero
	// last stop, a side of 0, and nothing placed.
	var prev, last shadeStop
	for _, s := range stops {
		prev, last = last, s
	}
	side := int(float64(2*last.r) * scale)
	if side <= 0 {
		return
	}
	disk := last
	if len(stops) > 1 {
		disk = prev
	}
	radius := int(float64(disk.r) * scale)
	body := disk
	for _, s := range stops {
		if s.r < disk.r {
			body = s
		}
	}
	half := side / 2
	src := fmt.Sprintf(`\hbox to %dsp{\vrule width 0pt height %dsp depth 0pt\kern %dsp\raise %dsp\hbox{\color{%s}\gotex@circle %dsp 0pt 1\relax}\hss}`,
		side, side, half, half, body.color, 2*radius)
	e.push(tokenizeTeX(src))
}

// parseShadeStops reads a pgf colour specification, "color(<d>)=(<expr>); ...",
// with rgb(…)=(r,g,b) and gray(…)=(g) as pgf also allows (pgfcoreshade.code.tex,
// \pgf@parsefunc). Stops are returned by radius.
func (e *Engine) parseShadeStops(spec []tok) []shadeStop {
	text := e.toksToString(spec)
	var stops []shadeStop
	for _, part := range strings.Split(text, ";") {
		part = strings.TrimSpace(part)
		open := strings.IndexByte(part, '(')
		closeAt := strings.IndexByte(part, ')')
		eq := strings.Index(part, "=(")
		if open < 0 || closeAt < open || eq < closeAt {
			continue
		}
		model := strings.TrimSpace(part[:open])
		val := strings.TrimSuffix(strings.TrimSpace(part[eq+2:]), ")")
		r, ok := e.evalDimenToks(tokenizeTeX(part[open+1 : closeAt]))
		if !ok {
			continue
		}
		var color string
		switch model {
		case "color":
			color = strings.TrimSpace(val)
		case "rgb", "gray":
			c := parseColorSpec(model, val)
			color = fmt.Sprintf("%06X", c)
			if e.colors == nil {
				e.colors = map[string]uint32{}
			}
			// An anonymous colour is named by its own value, so \color can take it.
			color = "gotex@shade@" + color
			e.colors[color] = c
		default:
			continue
		}
		stops = append(stops, shadeStop{r: r, color: color})
	}
	sort.SliceStable(stops, func(i, j int) bool { return stops[i].r < stops[j].r })
	return stops
}
