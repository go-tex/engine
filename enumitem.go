// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strconv"
	"strings"
)

// This file implements a pragmatic subset of the enumitem package: the optional
// [key=value] argument accepted by \begin{itemize}, \begin{enumerate} and
// \begin{description}. The kernel list environments (see latex.go) call the Go
// primitive \@enumitemopt{<kind>} at the end of their body; that primitive reads
// the optional bracket (if any) and reconfigures the current list group. When no
// bracket follows, nothing is touched and the plain environment behaves exactly
// as before.
//
// Supported keys (unknown keys are accepted and ignored):
//
//   label=<spec>      Override the item marker. For enumerate, the counter
//                     commands \arabic* \alph* \Alph* \roman* \Roman* (the "*"
//                     stands for the list counter) are mapped to this level's
//                     \theenumN, so "label=\alph*)" yields "a)", "b)", … and
//                     "label=(\roman*)" yields "(i)", "(ii)", …. Surrounding
//                     literals (parentheses, dots, …) are kept verbatim. For
//                     itemize the spec is used verbatim as \labelitemN, so
//                     "label=--" or "label=$\star$" replaces the bullet.
//   start=<n>         Begin an enumerate at <n> (sets the counter to n-1 so the
//                     first \item is <n>). Ignored by itemize/description.
//   resume            Continue an enumerate from the last value reached by the
//                     previous list at the same nesting level, rather than
//                     resetting to 0. Ignored by itemize/description.
//   leftmargin=<dim>  Set the list indentation (\leftskip) to <dim> (absolute).
//   itemsep=<dim>     Vertical glue inserted before each item (inter-item space).
//   noitemsep         Zero inter-item glue.
//   nosep             Zero inter-item glue and cancel the list's top \smallskip.
//
// Documented limitations of this subset: leftmargin sets \leftskip absolutely
// (it does not stack on an enclosing list's indent); label on description is
// ignored (its terms come from \item[term]); start/resume are enumerate-only;
// nosep cancels the leading \smallskip but not the trailing one; and dimensions
// are whatever \leftskip / \vskip accept (pt, and the other units the dimen
// scanner supports).

// counterValue returns the value of a \countdef'd control sequence (given by its
// full name, e.g. "c@enumdepth"), or 0 when it is not a count register.
func (e *Engine) counterValue(name string) int {
	if m := e.eq[name]; m != nil && m.kind == mCountRef {
		return e.count[m.code]
	}
	return 0
}

// romanSuffix maps a 1-based list depth to the kernel's per-level suffix
// (i, ii, iii, iv), clamped to the range the kernel defines.
func romanSuffix(depth int) string {
	if depth < 1 {
		depth = 1
	}
	if depth > 4 {
		depth = 4
	}
	return roman(depth)
}

// intToks renders a possibly-negative integer as a token list (a leading "-"
// for negatives, then decimal digits).
func intToks(n int) []tok {
	if n < 0 {
		return append([]tok{chTok('-', catOther)}, digitToks(-n)...)
	}
	return digitToks(n)
}

// eiEntry is one parsed key[=value] pair from the option list.
type eiEntry struct {
	key    string
	val    []tok
	hasVal bool
}

// splitTopLevel splits a token list on an "other"-category separator rune, but
// only at brace depth 0 (so a comma inside {…} does not split).
func splitTopLevel(ts []tok, sep rune) [][]tok {
	var out [][]tok
	var cur []tok
	depth := 0
	for _, t := range ts {
		switch {
		case !t.cs_ && t.cat == catBegin:
			depth++
		case !t.cs_ && t.cat == catEnd && depth > 0:
			depth--
		case depth == 0 && !t.cs_ && t.cat == catOther && t.ch == sep:
			out = append(out, cur)
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	out = append(out, cur)
	return out
}

// parseEnumitemOpts turns the raw bracket tokens into ordered key/value entries.
func parseEnumitemOpts(e *Engine, ts []tok) []eiEntry {
	var entries []eiEntry
	for _, seg := range splitTopLevel(ts, ',') {
		seg = trimSpaceToks(seg)
		if len(seg) == 0 {
			continue
		}
		parts := splitTopLevel(seg, '=')
		keyToks := trimSpaceToks(parts[0])
		key := strings.TrimSpace(e.toksToString(keyToks))
		ent := eiEntry{key: key}
		if len(parts) > 1 {
			ent.hasVal = true
			// Re-join any '=' beyond the first into the value.
			var valToks []tok
			for i := 1; i < len(parts); i++ {
				if i > 1 {
					valToks = append(valToks, chTok('=', catOther))
				}
				valToks = append(valToks, parts[i]...)
			}
			ent.val = trimSpaceToks(valToks)
		}
		entries = append(entries, ent)
	}
	return entries
}

// enumLabelBody maps an enumerate label spec to a \theenumN body: the counter
// commands become this level's register, everything else is copied verbatim.
func enumLabelBody(val []tok, suffix string) []tok {
	reg := "c@enum" + suffix
	var body []tok
	for i := 0; i < len(val); i++ {
		t := val[i]
		if t.cs_ {
			var op string
			switch t.cs {
			case "arabic":
				op = "the"
			case "alph":
				op = "@alph"
			case "Alph":
				op = "@Alph"
			case "roman":
				op = "romannumeral"
			case "Roman":
				op = "@Roman"
			}
			if op != "" {
				body = append(body, csTok(op), csTok(reg))
				// Consume an immediately-following "*" (the counter placeholder).
				if i+1 < len(val) && !val[i+1].cs_ && val[i+1].ch == '*' {
					i++
				}
				continue
			}
		}
		body = append(body, t)
	}
	return body
}

// defToks builds "\def\<name>{<body>}".
func defToks(name string, body []tok) []tok {
	out := []tok{csTok("def"), csTok(name), chTok('{', catBegin)}
	out = append(out, body...)
	return append(out, chTok('}', catEnd))
}

// assignToks builds "\<reg>=<value>\relax".
func assignToks(reg string, value []tok) []tok {
	out := []tok{csTok(reg), chTok('=', catOther)}
	out = append(out, value...)
	return append(out, csTok("relax"))
}

// doEnumitemOpt implements \@enumitemopt{<kind>}: read the optional [key=value]
// list that follows the environment and reconfigure the current list group.
func (e *Engine) doEnumitemOpt() {
	kind := strings.TrimSpace(e.toksToString(e.grabUndelimited()))
	opts, ok := e.readEnumitemBracket()
	if !ok {
		return // plain \begin{env} with no option: leave everything untouched.
	}
	entries := parseEnumitemOpts(e, opts)
	if len(entries) == 0 {
		return
	}

	var suffix string
	switch kind {
	case "enumerate":
		suffix = romanSuffix(e.counterValue("c@enumdepth"))
	case "itemize":
		suffix = romanSuffix(e.counterValue("c@itemdepth"))
	}

	var out []tok
	// Spacing state, resolved after scanning every key so the last one wins.
	spacingSet := false
	nosep := false
	var sepToks []tok

	for _, ent := range entries {
		switch ent.key {
		case "label":
			if !ent.hasVal {
				break
			}
			switch kind {
			case "enumerate":
				out = append(out, defToks("theenum"+suffix, enumLabelBody(ent.val, suffix))...)
			case "itemize":
				out = append(out, defToks("labelitem"+suffix, append([]tok(nil), ent.val...))...)
			}
		case "start":
			if kind != "enumerate" || !ent.hasVal {
				break
			}
			if n, err := strconv.Atoi(strings.TrimSpace(e.toksToString(ent.val))); err == nil {
				out = append(out, assignToks("c@enum"+suffix, intToks(n-1))...)
			}
		case "resume":
			if kind != "enumerate" {
				break
			}
			saved := 0
			if e.enumitemLast != nil {
				saved = e.enumitemLast[suffix]
			}
			out = append(out, assignToks("c@enum"+suffix, intToks(saved))...)
		case "leftmargin":
			if ent.hasVal {
				out = append(out, assignToks("leftskip", append([]tok(nil), ent.val...))...)
			}
		case "itemsep":
			if ent.hasVal {
				spacingSet = true
				nosep = false
				sepToks = append([]tok(nil), ent.val...)
			}
		case "noitemsep":
			spacingSet = true
			nosep = false
			sepToks = stringToToks("0pt")
		case "nosep":
			spacingSet = true
			nosep = true
			sepToks = stringToToks("0pt")
		default:
			// Unknown key (widest, align, font, …): accepted and ignored.
		}
	}

	if spacingSet {
		if nosep {
			// nosep drops the list's top space: cancel the \addvspace\@topsepadd the
			// opener emitted (\topsep + \partopsep, its current value), then zero the
			// four registers enumitem.sty zeroes — \partopsep, \topsep, \itemsep and
			// \parsep (enumitem.sty l.725-729). \partopsep matters now that the list
			// openers add it, as latex.ltx's \@trivlist does.
			out = append(out, csTok("vskip"), chTok('-', catOther), csTok("@topsepadd"), csTok("relax"))
		}
		// Set the \itemsep register the list machinery's \@iteminterspace reads, rather
		// than wrapping \item (which would double the inter-item glue).
		out = append(out, assignToks("itemsep", sepToks)...)
		// \parsep goes with it: enumitem.sty zeroes BOTH for noitemsep (l.731-733) and
		// for nosep (l.725-729), and \@iteminterspace now adds \parsep as the list's
		// \parskip, so leaving it alone made noitemsep keep a paragraph skip enumitem
		// removes.
		out = append(out, assignToks("parsep", sepToks)...)
		if nosep {
			out = append(out, assignToks("partopsep", stringToToks("0pt"))...)
			out = append(out, assignToks("topsep", stringToToks("0pt"))...)
		}
	}

	e.push(out)
}

// itemSepWrapper builds the tokens that wrap the current \item so that each item
// is preceded by a vertical glue of the given size:
//
//	\let\@enitemsaved\item \def\item{\vskip <sep>\relax\@enitemsaved}
func (e *Engine) itemSepWrapper(sep []tok) []tok {
	out := []tok{csTok("let"), csTok("@enitemsaved"), csTok("item")}
	body := []tok{csTok("vskip")}
	body = append(body, sep...)
	body = append(body, csTok("relax"), csTok("@enitemsaved"))
	return append(out, defToks("item", body)...)
}

// readEnumitemBracket peeks (without expansion) for an optional [ … ] group and
// returns its raw tokens. When the next token is not a '[', it is pushed back and
// (nil,false) is returned so the caller leaves the input untouched.
func (e *Engine) readEnumitemBracket() ([]tok, bool) {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok {
		return nil, false
	}
	if t.cs_ || !(t.cat == catOther && t.ch == '[') {
		e.back(t)
		return nil, false
	}
	var toks []tok
	depth := 0
	for {
		u, ok := e.getNext()
		if !ok {
			return toks, true // unterminated: take what we have.
		}
		switch {
		case !u.cs_ && u.cat == catBegin:
			depth++
		case !u.cs_ && u.cat == catEnd && depth > 0:
			depth--
		case depth == 0 && !u.cs_ && u.cat == catOther && u.ch == ']':
			return toks, true
		}
		toks = append(toks, u)
	}
}

// recordEnumitemResume implements \@enumitemrec, called from \endenumerate before
// its \endgroup: it stores the counter value the just-finished list reached, so a
// later [resume] list at the same level can continue from it.
func (e *Engine) recordEnumitemResume() {
	if e.enumitemLast == nil {
		e.enumitemLast = map[string]int{}
	}
	suffix := romanSuffix(e.counterValue("c@enumdepth"))
	e.enumitemLast[suffix] = e.counterValue("c@enum" + suffix)
}
