// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

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

// crefForms is the \cref abbreviation table documented above.
var crefForms = map[string]crefForm{
	"section":    {"section", "Section", "sections", "Sections", false},
	"subsection": {"subsection", "Subsection", "subsections", "Subsections", false},
	"equation":   {"eq.", "Eq.", "eqs.", "Eqs.", true},
	"figure":     {"fig.", "Fig.", "figs.", "Figs.", false},
	"table":      {"tab.", "Tab.", "tabs.", "Tabs.", false},
	"theorem":    {"thm.", "Thm.", "thms.", "Thms.", false},
	"part":       {"part", "Part", "parts", "Parts", false},
	"item":       {"item", "Item", "items", "Items", false},
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
	form, ok := crefForms[e.refTypes[keys[0]]]
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
	return name + " " + joinAnd(nums)
}

// crefOne renders a single-key \cref/\Cref.
func (e *Engine) crefOne(key string, capital bool) string {
	num := e.refText(key)
	form, ok := crefForms[e.refTypes[key]]
	if !ok || num == "??" {
		return num // "??" for an unknown key; a bare number for an untyped one
	}
	name := form.lower
	if capital {
		name = form.upper
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
