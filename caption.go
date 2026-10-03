// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// This file implements the one option of the caption package that changes what a
// caption SAYS rather than how it looks: \captionsetup[<type>]{name=<text>},
// which renames the label a caption is headed with.
//
// acmart does exactly this, at acmart.cls:934:
//
//	\captionsetup[figure]{name={Fig.}}
//
// so every figure in an ACM paper is headed "Fig. 1" and not "Figure 1". Fifteen
// of the 154 corpus papers are acmart, and eleven papers carry a
// \captionsetup{…name=…} of their own or through a bundled class. Until now
// \captionsetup gobbled its arguments whole: 2406.01525's reference prints
// "Fig." 15 times where we printed "Figure", which is how this was found — it
// was the largest row left in a cleveref census that had nothing to do with
// cleveref.
//
// Only `name` is read. The rest of the package — label formats, separators,
// fonts, justification, margins — is styling, and gobbling it is still the right
// answer until something measures otherwise.

// doCaptionsetup implements \captionsetup[<type>]{<options>} and the no-type
// form. It reads `name=<text>` and rebinds \<type>name, which is what the
// kernel's caption head goes through (\fnum@figure is \figurename~\thefigure, see
// latex.go) — so renaming it reaches the caption without touching the caption
// code itself.
//
// With no [<type>], the caption package uses \@captype, which is UNDEFINED
// outside a float: real caption raises "Undefined control sequence" on a
// preamble \captionsetup{name={Thing}}, checked against tectonic. Here that case
// is ignored rather than made an error, since the engine has nothing to gain
// from reproducing it.
func (e *Engine) doCaptionsetup() {
	types := e.scanBracketList()
	opts := e.readBraceToksRaw()
	if len(opts) == 0 {
		return
	}
	if len(types) == 0 {
		if t := strings.TrimSpace(e.toksToString(e.expandList([]tok{csTok("@captype")}))); t != "" {
			types = []string{t}
		}
	}
	if len(types) == 0 {
		return
	}
	for _, seg := range splitTopLevelCommas(opts) {
		key, val, ok := splitKeyValueTokens(seg)
		if !ok || strings.TrimSpace(key) != "name" {
			continue
		}
		body := stripOuterBraceToks(trimSpaceToks(val))
		for _, typ := range types {
			typ = strings.TrimSpace(typ)
			if typ == "" {
				continue
			}
			e.define(typ+"name", &meaning{kind: mMacro, body: append([]tok(nil), body...)}, true)
		}
	}
}

// stripOuterBraceToks removes ONE matched pair of outer braces, so that
// name={Fig.} and name=Fig. give the same text. It leaves an inner group alone:
// name={\textbf{Fig.}} must keep the braces its own macro needs.
func stripOuterBraceToks(ts []tok) []tok {
	if len(ts) < 2 || ts[0].cs_ || ts[0].cat != catBegin || ts[len(ts)-1].cs_ || ts[len(ts)-1].cat != catEnd {
		return ts
	}
	depth := 0
	for i, t := range ts {
		if t.cs_ {
			continue
		}
		switch t.cat {
		case catBegin:
			depth++
		case catEnd:
			depth--
			if depth == 0 && i != len(ts)-1 {
				return ts // the opening brace closes early: not one outer group
			}
		}
	}
	return ts[1 : len(ts)-1]
}
