// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"unicode"
)

// This file implements typed cross-references: hyperref's \autoref and \nameref,
// and cleveref's \cref / \Cref. Unlike \ref (which prints a bare number), these
// print the number together with the NAME of the thing it points to — "Section 1",
// "Equation (1)", "Figure 2", "Theorem 3".
//
// To type a reference, the engine records — beside the reference number itself
// (\@currentlabel, see crossref.go) — WHAT KIND of thing the last counter-stepping
// command produced (\@currentreftype) and, for \nameref, its title text
// (\@currentlabelname, the hyperref name). Those two macros are set alongside every
// \edef\@currentlabel by additive kernel redefinitions appended at the end of
// MiniLaTeXKernel (see latex.go); \label freezes all three into parallel maps
// (labels / refTypes / refNames) that the two-pass compile carries from the aux run
// into the render pass exactly as it carries labels (see api.go compile).
//
// \cref abbreviation table (singular / plural, lowercase for \cref, capitalised for
// \Cref):
//
//	type        \cref            \Cref            plural (\cref / \Cref)
//	----------  ---------------  ---------------  -----------------------
//	section     section N        Section N        sections / Sections
//	subsection  subsection N     Subsection N     subsections / Subsections
//	equation    eq. (N)          Eq. (N)          eqs. / Eqs.
//	figure      fig. N           Fig. N           figs. / Figs.
//	table       tab. N           Tab. N           tabs. / Tabs.
//	theorem     thm. N           Thm. N           thms. / Thms.
//	part        part N           Part N           parts / Parts
//	item        item N           Item N           items / Items
//
// \autoref uses the full names: "Section", "Subsection", "Equation" (parenthesised
// number), "Figure", "Table", "Theorem", "Part", "item".
//
// Multi-key support: \cref{a,b,c} prints the group name once (plural, taken from the
// FIRST key's type) followed by the numbers joined "1, 2 and 3" — e.g.
// "sections 1 and 2", "eqs. (1) and (2)". The keys are assumed homogeneous (same
// type); with mixed types the first key's group name is used for all. An unknown key
// yields "??", as with \ref.

// autorefNames maps a reference type to hyperref's \autoref display name.
var autorefNames = map[string]string{
	"section":    "Section",
	"subsection": "Subsection",
	"equation":   "Equation",
	"figure":     "Figure",
	"table":      "Table",
	"theorem":    "Theorem",
	"part":       "Part",
	"item":       "item",
}

// crefForm holds cleveref's names for one reference type.
type crefForm struct {
	lower  string // singular, lowercase (\cref)
	upper  string // singular, capitalised (\Cref)
	lowerP string // plural, lowercase (\cref of several keys)
	upperP string // plural, capitalised (\Cref of several keys)
	paren  bool   // wrap the number in parentheses (equations)
}

// crefForms is cleveref's own default naming, read from cleveref.sty's
// \crefname/\Crefname block (cleveref.sty:3962-4055). The lower pair is what
// \cref prints, the upper pair what \Cref prints, and they are NOT the same word
// abbreviated: cleveref abbreviates the LOWERCASE figure/equation only, and
// spells the capitalised forms out.
//
// The table used to carry "Fig."/"Tab."/"Eq." for BOTH, and nothing at all for a
// section, so \Cref{sec:…} printed a bare number. 15 of the 154 corpus papers use
// \cref/\Cref, 904 times: 2302.04180 writes \Cref 51 times and its reference has
// "Section N" twelve times where ours had none.
var crefForms = map[string]crefForm{
	"section":       {"section", "Section", "sections", "Sections", false},
	"subsection":    {"section", "Section", "sections", "Sections", false},
	"subsubsection": {"section", "Section", "sections", "Sections", false},
	"chapter":       {"chapter", "Chapter", "chapters", "Chapters", false},
	"part":          {"part", "Part", "parts", "Parts", false},
	"appendix":      {"appendix", "Appendix", "appendices", "Appendices", false},
	"equation":      {"eq.", "Equation", "eqs.", "Equations", true},
	"figure":        {"fig.", "Figure", "figs.", "Figures", false},
	"subfigure":     {"fig.", "Figure", "figs.", "Figures", false},
	"table":         {"table", "Table", "tables", "Tables", false},
	"subtable":      {"table", "Table", "tables", "Tables", false},
	"algorithm":     {"algorithm", "Algorithm", "algorithms", "Algorithms", false},
	"listing":       {"listing", "Listing", "listings", "Listings", false},
	"line":          {"line", "Line", "lines", "Lines", false},
	"page":          {"page", "Page", "pages", "Pages", false},
	"footnote":      {"footnote", "Footnote", "footnotes", "Footnotes", false},
	"theorem":       {"theorem", "Theorem", "theorems", "Theorems", false},
	"lemma":         {"lemma", "Lemma", "lemmas", "Lemmas", false},
	"corollary":     {"corollary", "Corollary", "corollaries", "Corollaries", false},
	"proposition":   {"proposition", "Proposition", "propositions", "Propositions", false},
	"definition":    {"definition", "Definition", "definitions", "Definitions", false},
	"result":        {"result", "Result", "results", "Results", false},
	"example":       {"example", "Example", "examples", "Examples", false},
	"remark":        {"remark", "Remark", "remarks", "Remarks", false},
	"note":          {"note", "Note", "notes", "Notes", false},
	"item":          {"item", "Item", "items", "Items", false},
	"enumi":         {"item", "Item", "items", "Items", false},
	"enumii":        {"item", "Item", "items", "Items", false},
}

// recordRefMeta freezes the current \@currentreftype and \@currentlabelname under
// key, beside the number \label already stored in e.labels. Called additively from
// doLabel (see crossref.go).
func (e *Engine) recordRefMeta(key string) {
	if e.refTypes == nil {
		e.refTypes = map[string]string{}
	}
	if e.refNames == nil {
		e.refNames = map[string]string{}
	}
	e.refTypes[key] = e.toksToString(e.expandList([]tok{csTok("@currentreftype")}))
	e.refNames[key] = e.toksToString(e.expandList([]tok{csTok("@currentlabelname")}))
}

// doAutoref implements hyperref's \autoref{key}: "<Type> <number>", with the type
// chosen from what the label points to. Equations parenthesise their number. An
// unknown key yields "??" (the link that real hyperref adds is not modelled).
func (e *Engine) doAutoref() {
	key := e.readBraceName()
	e.pushString(e.autorefText(key))
}

// autorefText returns the \autoref rendering for key.
func (e *Engine) autorefText(key string) string {
	num := e.refText(key)
	if num == "??" {
		return "??"
	}
	name, ok := autorefNames[e.refTypes[key]]
	if !ok {
		return num // known number but no recognised type: bare number
	}
	if e.refTypes[key] == "equation" {
		return name + " (" + num + ")"
	}
	return name + " " + num
}

// doNameref implements hyperref's \nameref{key}: the TITLE / caption text of the
// target (recorded as \@currentlabelname when the section/caption ran). An unknown
// key, or a target with no name (an equation, a list item), yields "??".
func (e *Engine) doNameref() {
	key := e.readBraceName()
	if v, ok := e.refNames[key]; ok && v != "" {
		e.pushString(v)
		return
	}
	e.pushString("??")
}

// doCref implements cleveref's \cref (capital=false) and \Cref (capital=true). It
// accepts a comma-separated key list; see crefText for the multi-key rendering.
func (e *Engine) doCref(capital bool) {
	keys := splitComma(e.readBraceName())
	// A \crefformat replaces the whole rendering, name included, so it is tried
	// first. Only the single-key case: several keys go through
	// \crefmultiformat in cleveref, which no corpus paper uses.
	if len(keys) == 1 {
		if body, ok := e.crefFormatted(keys[0], capital); ok {
			e.push(body)
			return
		}
	}
	e.pushString(e.crefText(keys, capital))
}

// crefText renders one or more keys in cleveref style. A single key gives
// "section 1" / "eq. (1)"; several give "sections 1 and 2".
func (e *Engine) crefText(keys []string, capital bool) string {
	switch len(keys) {
	case 0:
		return "??"
	case 1:
		return e.crefOne(keys[0], capital)
	}
	form, ok := e.crefFormFor(e.refTypes[keys[0]])
	nums := make([]string, len(keys))
	for i, k := range keys {
		nums[i] = e.crefNum(k, form, ok)
	}
	if !ok {
		return joinAnd(nums) // no recognised type: numbers only
	}
	name := form.lowerP
	if capital {
		name = form.upperP
	}
	if name == "" {
		return joinAnd(nums) // named type, unnamed case: numbers only
	}
	return name + " " + joinAnd(nums)
}

// crefOne renders a single-key \cref/\Cref.
func (e *Engine) crefOne(key string, capital bool) string {
	num := e.refText(key)
	form, ok := e.crefFormFor(e.refTypes[key])
	if !ok || num == "??" {
		return num // "??" for an unknown key; a bare number for an untyped one
	}
	name := form.lower
	if capital {
		name = form.upper
	}
	if name == "" {
		return num
	}
	if form.paren {
		return name + " (" + num + ")"
	}
	return name + " " + num
}

// crefNum returns key's number, parenthesised when the (known) type wants it.
func (e *Engine) crefNum(key string, form crefForm, known bool) string {
	num := e.refText(key)
	if known && form.paren {
		return "(" + num + ")"
	}
	return num
}

// joinAnd joins parts as "a", "a and b", or "a, b and c" (no Oxford comma), matching
// cleveref's default list conjunction.
func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	}
	out := ""
	for i := 0; i < len(parts)-1; i++ {
		if i > 0 {
			out += ", "
		}
		out += parts[i]
	}
	return out + " and " + parts[len(parts)-1]
}

// crefFormFor returns the naming to use for a reference type: a document's own
// \crefname/\Crefname first, cleveref's defaults otherwise. Four of the 154
// corpus papers name their own types, 35 times, and three of the four ask for
// the SHORT forms ("Sec.", "Fig.", "Tab.", "Eq.") where cleveref's default
// spells the capitalised name out — so without this the defaults table is not a
// fallback, it is an override.
func (e *Engine) crefFormFor(typ string) (crefForm, bool) {
	if f, ok := e.crefNames[typ]; ok {
		return f, true
	}
	// A \newtheorem heading names its own type, between the built-in defaults and
	// the document's \crefname: cleveref writes it to the @preamble macros, so it
	// overrides the defaults and loses to anything the document says, whichever
	// order the two are written in.
	if f, ok := e.crefThmNames[typ]; ok {
		return f, true
	}
	f, ok := crefForms[typ]
	if !ok {
		return f, false
	}
	// cleveref's two naming options, applied over its defaults. Only equation and
	// figure are abbreviated at all, so noabbrev touches those two; capitalise
	// replaces the whole lowercase set with the capitalised one, and the
	// abbreviations with it when abbrev is still on (cleveref.sty:3889-3946).
	if e.crefNoAbbrev {
		switch typ {
		case "equation":
			f.lower, f.lowerP = "equation", "equations"
		case "figure", "subfigure":
			f.lower, f.lowerP = "figure", "figures"
		}
	}
	if e.crefCapitalise {
		f.lower, f.lowerP = f.upper, f.upperP
		if !e.crefNoAbbrev {
			switch typ {
			case "equation":
				f.lower, f.lowerP = "Eq.", "Eqs."
			case "figure", "subfigure":
				f.lower, f.lowerP = "Fig.", "Figs."
			}
		}
	}
	return f, true
}

// doCrefname implements cleveref's \crefname{type}{singular}{plural} and its
// \Crefname counterpart. Each names ONE case: \crefname what \cref prints,
// \Crefname what \Cref prints.
//
// The cross-fill is cleveref's own, from \@crefname (cleveref.sty:1296-1320):
// naming one case also defines the OTHER one, but only when that one is still
// unset, by \MakeUppercase or \MakeLowercase of the given name. It is why
// \Crefname{figure}{Fig.}{Figs.} alone makes \cref print "fig." too. The
// capitalising direction really does uppercase the whole word in cleveref —
// \crefname{equation}{eq.}{eqs.} alone gives \Cref "EQ." — and that is kept
// here, because the reference these runs are compared against is real cleveref.
//
// The parentheses around an equation number are NOT part of the name: they come
// from \creflabelformat (cleveref.sty:7840), so paren survives a renaming.
func (e *Engine) doCrefname(capital bool) {
	typ := e.readBraceName()
	sing := e.readBraceText()
	plur := e.readBraceText()
	if typ == "" {
		return
	}
	if e.crefNames == nil {
		e.crefNames = map[string]crefForm{}
	}
	// An entry starts EMPTY, inheriting only the parenthesisation: cleveref's own
	// defaults live in @preamble macros that lose to anything the document set, so
	// a document's \Crefname cross-fills the lowercase even for a type that has a
	// default. The empty string is therefore "this document has not named this
	// case", and it is also what makes \cref print a bare number.
	f, named := e.crefNames[typ]
	if !named {
		f = crefForm{paren: crefForms[typ].paren}
	}
	if capital {
		f.upper, f.upperP = sing, plur
		if f.lower == "" {
			f.lower, f.lowerP = strings.ToLower(sing), strings.ToLower(plur)
		}
	} else {
		f.lower, f.lowerP = sing, plur
		if f.upper == "" {
			f.upper, f.upperP = strings.ToUpper(sing), strings.ToUpper(plur)
		}
	}
	e.crefNames[typ] = f
}

// crefFmt holds the token templates \crefformat / \Crefformat install for one
// reference type. cleveref's format takes three parameters — #1 the number, #2 a
// prefix and #3 a suffix, which are its hyperlink wrappers and are empty here —
// and it replaces the NAME as well as the layout, so a format wins over every
// naming tier (cleveref.sty:543-549 applies cref@<type>@format and never looks
// at the name).
type crefFmt struct {
	lower []tok // \cref
	upper []tok // \Cref
}

// doCrefformat implements \crefformat{type}{template} and \Crefformat. One corpus
// paper in 154 uses them, twice — and it is the whole of that paper's remaining
// deficit: 2406.01525 writes \crefformat{section}{#2\S#1#3}, so its reference
// prints "§3" where the engine printed "Section 3", 56 times. Reading the format
// is also why its page count went one over in #527.
//
// The template is read RAW: #1 must survive to substitution time, and a \S in it
// must not expand before it is typeset.
//
// The cross-fill is cleveref's (\@crefformat, cleveref.sty:1886-1910): installing
// one case installs the other when that one is unset. cleveref wraps the copy in
// \MakeUppercase or \MakeLowercase; this copies the tokens unchanged, which
// differs only for a template carrying LETTERS — and a template that spells a
// word is one a document writes for both cases anyway, as the corpus paper does.
func (e *Engine) doCrefformat(capital bool) {
	typ := e.readBraceName()
	tpl := e.readBraceToksRaw()
	if typ == "" || len(tpl) == 0 {
		return
	}
	if e.crefFormats == nil {
		e.crefFormats = map[string]crefFmt{}
	}
	f := e.crefFormats[typ]
	if capital {
		f.upper = tpl
		if f.lower == nil {
			f.lower = foldFirstLetter(tpl, false)
		}
	} else {
		f.lower = tpl
		if f.upper == nil {
			f.upper = foldFirstLetter(tpl, true)
		}
	}
	e.crefFormats[typ] = f
}

// crefTypeFallback gives the type whose naming and format a type inherits when
// it has none of its own. Only the two the oracle was asked about are here;
// subfigure and subtable are left alone rather than guessed at.
func crefTypeFallback(typ string) string {
	switch typ {
	case "subsection", "subsubsection":
		return "section"
	}
	return typ
}

// crefFormatted renders one key through a \crefformat template, substituting the
// number for #1 and nothing for the hyperlink wrappers #2 and #3. It reports
// false when the key's type has no format, which is every type in all but one
// corpus paper.
func (e *Engine) crefFormatted(key string, capital bool) ([]tok, bool) {
	typ := e.refTypes[key]
	f, ok := e.crefFormats[typ]
	if !ok {
		// A format falls back the same way the NAME does: a subsection with no
		// format of its own follows the section's, which is why cleveref's default
		// name for one is "section" too. Checked against tectonic both ways — a
		// \crefformat{section} renders \cref of a subsection as "§1.1", while a
		// \crefname{subsection}{subsec.} IS honoured, so the type really is
		// `subsection` and this is a fallback, not an alias.
		if a := crefTypeFallback(typ); a != typ {
			f, ok = e.crefFormats[a]
		}
	}
	if !ok {
		return nil, false
	}
	tpl := f.lower
	if capital {
		tpl = f.upper
	}
	num := e.refText(key)
	if len(tpl) == 0 || num == "??" {
		return nil, false
	}
	out := make([]tok, 0, len(tpl)+len(num))
	for i := 0; i < len(tpl); i++ {
		if n, w := crefParamAt(tpl, i); n > 0 {
			i += w - 1
			if n == 1 {
				out = append(out, stringToToks(num)...)
			}
			continue // #2 and #3 are cleveref's hyperlink wrappers: nothing here
		}
		out = append(out, tpl[i])
	}
	return out, true
}

// crefParamAt reports the parameter number at tpl[i] and how many tokens it
// spans, or 0 if there is none. A template read with grabGroup keeps "#1" as TWO
// tokens — a catParam '#' and the digit — where a \def body folds them into one,
// which is why the first version of this substituted nothing and printed
// "#2§#1#3" on the page.
func crefParamAt(tpl []tok, i int) (int, int) {
	t := tpl[i]
	if t.cs_ || t.cat != catParam {
		return 0, 0
	}
	if t.ch >= '1' && t.ch <= '9' { // a \def-style folded parameter token
		return int(t.ch - '0'), 1
	}
	if i+1 < len(tpl) {
		if d := tpl[i+1]; !d.cs_ && d.ch >= '1' && d.ch <= '9' {
			return int(d.ch - '0'), 2
		}
	}
	return 0, 0
}

// foldFirstLetter upper- or lowercases the first LETTER a template would typeset,
// leaving its parameters, control sequences and punctuation alone. It is how
// cleveref cross-fills a one-sided \crefformat: \@crefformat wraps the copy in
// \MakeUppercase or \MakeLowercase (cleveref.sty:1886-1910), applied to an
// UNBRACED argument, so only the first token shifts.
//
// Checked against tectonic: \crefformat{figure}{fig.~#2#1#3} alone makes \Cref
// print "Fig. 1" — not "FIG. 1", and not the default "Figure 1".
func foldFirstLetter(tpl []tok, upper bool) []tok {
	out := append([]tok(nil), tpl...)
	for i := 0; i < len(out); i++ {
		if n, w := crefParamAt(out, i); n > 0 {
			i += w - 1
			continue
		}
		t := out[i]
		if t.cs_ || !unicode.IsLetter(t.ch) {
			continue
		}
		if upper {
			out[i].ch = unicode.ToUpper(t.ch)
		} else {
			out[i].ch = unicode.ToLower(t.ch)
		}
		return out
	}
	return out
}
