// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements amsthm-style theorem environments: \newtheorem and the
// theorem-like environments it generates (theorem/lemma/definition/…), plus a
// proof environment with a flush-right QED box.
//
// Design choice — \newtheorem is a Go primitive, not pure TeX macros.
// Classic latex.ltx implements \newtheorem as macros-generating-macros, and its
// two mutually-exclusive optional arguments sit in DIFFERENT positions —
// \newtheorem{env}[shared]{Heading}  (shared counter, bracket BEFORE the head)
// versus  \newtheorem{env}{Heading}[within]  (numbered within a parent counter,
// bracket AFTER the head). Distinguishing them classically needs \@ifnextchar /
// \futurelet, which this kernel does not provide. Parsing them in Go with the
// existing helpers (readBraceName, scanOptBracketToks) is unambiguous and small,
// and counter allocation reuses the same register machinery as \newcount. The
// *runtime* behaviour stays in TeX kernel macros: doNewtheorem only GENERATES,
// per environment, the counter-representation macro \the<env> and the \<env> /
// \end<env> environment macros; those macros step the counter, freeze the number
// into \@currentlabel (so \label/\ref resolve, exactly like equation.go), and
// hand the shared formatting to the fixed kernel macros \@begintheorem /
// \@endtheorem. Building the generated macros as token slices (not by pushing
// TeX text) keeps the @-bearing internal names catcode-immune, so \newtheorem
// works whether used in the preamble or the document body.

import (
	"strings"
	"unicode"
)

// doNewtheorem implements \newtheorem{env}{Heading}, its \newtheorem{env}[shared]
// {Heading} (share another environment's counter) and \newtheorem{env}{Heading}
// [within] (number within a parent counter, e.g. Theorem 1.1) forms.
func (e *Engine) doNewtheorem() {
	// \newtheorem*{env}{Heading} is amsthm's UNNUMBERED form. Unhandled, the star
	// stopped readBraceName dead: it read no name, the environment stayed undefined,
	// and the star and the heading were left in the stream to be TYPESET. One paper
	// opened on a page of its own carrying "*theoremTheorem *namedconjectureConjecture"
	// and then set its three theorem bodies as plain paragraphs.
	starred := false
	if t, ok := e.getNext(); ok {
		if !t.cs_ && t.ch == '*' {
			starred = true
		} else {
			e.back(t)
		}
	}
	env := e.readBraceName()
	sharedToks, hasShared := e.scanOptBracketToks() // [shared] BEFORE the heading
	// The heading is read as the TOKENS it is made of, never as a name: beamer
	// writes \newtheorem{theorem}{\translate{Theorem}} so the word comes from the
	// reader's language, and a paper writes \newtheorem{thm}{\bfseries Th\'eor\`eme}.
	// readBraceName drops every control sequence and keeps the braces around them
	// as characters, which is how every beamer talk came to be headed
	// "{Theorem} 2." instead of "Theorem 2." or "Théorème 2.".
	head := e.readBraceToks()
	withinToks, hasWithin := e.scanOptBracketToks() // [within] AFTER the heading
	if env == "" {
		return
	}

	// ⛔ \newtheorem{X}[X]{…} asks X to share ITS OWN counter. latex.ltx's \@othm tests
	// \@ifundefined{c@#2} and errors ("No theorem environment #2 defined") because that
	// counter does not exist yet, so real LaTeX numbers nothing and carries on. Here the
	// alias was built anyway: \the<env> got the body \the<shared>, which is \the<env>
	// again — a self-referential macro the engine then spun 400 times into the runaway
	// guard. Two corpus papers write it, one directly
	//
	//	\newtheorem{corollary}[corollary]{Corollary}            (2607.21390)
	//
	// and one through a wrapper of its own, \newtheorem{#1vArIAblE}[#1vArIAblE]{#3}
	// (2606.14675) — each stopping at 3 and 2 pages. Dropping the self-share leaves the
	// environment with a counter of its own, which is what the rest of this function does
	// when no [shared] is given at all.
	if hasShared && strings.TrimSpace(e.toksToString(sharedToks)) == env {
		hasShared, sharedToks = false, nil
	}

	e.recordTheoremCrefName(env, head)
	if starred {
		e.defineUnnumberedTheorem(env, head)
		return
	}

	// Choose the counter register that this environment steps.
	ctr := "c@" + env
	ctrCode := -1
	switch {
	case hasShared && strings.TrimSpace(e.toksToString(sharedToks)) != "":
		ctr = "c@" + strings.TrimSpace(e.toksToString(sharedToks))
	default:
		// Allocate a fresh \count register, exactly as \newcount would.
		if e.allocCnt < 256 {
			ctrCode = e.allocCnt
			e.define(ctr, &meaning{kind: mCountRef, code: ctrCode}, true)
			e.allocCnt++
		}
	}

	// \the<env>: the printed representation of the number.
	//
	// A SHARED counter delegates to the environment it shares with, and does not print the
	// raw register: latex.ltx:12728, \@othm, is
	//
	//	\global\@namedef{the#1}{\@nameuse{the#2}}
	//
	// so \newtheorem{example}[theorem]{Example} gives \theexample = \thetheorem, which
	// itself may be \thesection.\the\c@theorem. Printing \the\c@theorem instead dropped
	// the section: acmart declares its whole theorem set on the theorem counter
	// (acmart.cls:3042-3070), and corpus paper 2402.04392 came out "Example 1", "Example 4",
	// "Example 5" where its reference has "Example 3.1", "3.4", "3.5" — the right numbers
	// with the section lost off the front.
	shared := ""
	if hasShared {
		shared = strings.TrimSpace(e.toksToString(sharedToks))
	}
	var numBody []tok
	switch {
	case shared != "":
		numBody = []tok{csTok("the" + shared)}
	case hasWithin:
		within := strings.TrimSpace(e.toksToString(withinToks))
		// Nest on the parent's FORMATTED number (\the<within>, which itself chains
		// e.g. section.subsection), matching LaTeX's \newtheorem[within].
		numBody = []tok{csTok("the" + within), chTok('.', catOther), csTok("the"), csTok(ctr)}
	default:
		numBody = []tok{csTok("the"), csTok(ctr)}
	}
	e.define("the"+env, &meaning{kind: mMacro, body: numBody}, true)

	// A within-numbered environment resets on its parent counter's step.
	if hasWithin && !hasShared && ctrCode >= 0 {
		within := strings.TrimSpace(e.toksToString(withinToks))
		e.addToReset(env, within)
	}

	// \<env>: \begin{env}. Open a group (so \it reverts at \end), globally step the
	// counter (global to survive the group), freeze the number as \@currentlabel,
	// then hand {Heading}{number} to \@begintheorem (which reads the optional note).
	body := []tok{
		csTok("par"), csTok("medskip"), csTok("begingroup"),
	}
	body = append(body, thmTypeToks(env)...)
	body = append(body, []tok{
		csTok("global"), csTok("advance"), csTok(ctr), chTok(' ', catSpace),
		chTok('b', catLetter), chTok('y', catLetter), chTok('1', catOther), csTok("relax"),
		csTok("edef"), csTok("@currentlabel"), chTok('{', catBegin), csTok("the" + env), chTok('}', catEnd),
		csTok("@oparg"), chTok('{', catBegin),
		csTok("@begintheorem"), chTok('{', catBegin),
	}...)
	body = append(body, head...)
	body = append(body, chTok('}', catEnd), chTok('{', catBegin), csTok("the"+env), chTok('}', catEnd))
	// Close \@oparg's argument and hand it the default empty bracket group, so the
	// call reaches \@begintheorem WITH a [note] whether or not the document wrote
	// one. That is what amsthm does, and what a class redefining \@begintheorem
	// with a delimited [#3] parameter requires: called without a bracket, such a
	// macro reads forward through the document looking for one.
	body = append(body, chTok('}', catEnd), chTok('[', catOther), chTok(']', catOther))
	e.define(env, &meaning{kind: mMacro, body: body}, true)

	// \end<env>: the fixed closing macro (end paragraph, close group, vertical space).
	e.define("end"+env, &meaning{kind: mMacro, body: []tok{csTok("@endtheorem")}}, true)
}

// defineUnnumberedTheorem defines the \newtheorem* form: the same heading and the
// same grouping as a numbered theorem, with no counter, no \the<env>, and an empty
// number handed to \@begintheorem — which is what amsthm's starred form produces.
func (e *Engine) defineUnnumberedTheorem(env string, head []tok) {
	body := []tok{
		csTok("par"), csTok("medskip"), csTok("begingroup"),
	}
	body = append(body, thmTypeToks(env)...)
	body = append(body, []tok{
		csTok("@beginthmnonum"), chTok('{', catBegin),
	}...)
	body = append(body, head...)
	body = append(body, chTok('}', catEnd))
	e.define(env, &meaning{kind: mMacro, body: body}, true)
	e.define("end"+env, &meaning{kind: mMacro, body: []tok{csTok("@endtheorem")}}, true)
}

// addToReset arranges for counter `name` to be zeroed whenever the parent
// counter `within` is stepped. It appends \@stpelt{name} to the reset-list macro
// \cl@<within>, and hooks the corresponding sectioning macro (\@nsection /
// \@nsubsection) once so that it runs the reset list after stepping — mirroring
// LaTeX's \@addtoreset / \cl@… mechanism.
//
// It appends \@stpelt{name} rather than a bare "\global\count<code>=0" because
// a reset CASCADES. LaTeX's kernel resets a counter by stepping it out of −1:
//
//	\def\@stpelt#1{\global\csname c@#1\endcsname \m@ne\stepcounter{#1}}
//
// so zeroing a counter also runs THAT counter's own reset list. A flat
// assignment stops at the first level, and the difference shows: with section
// registered within chapter and subsection within section, a \chapter used to
// leave the subsection counter untouched, so \thesubsection after the second
// chapter read 2.0.2 where LaTeX gives 2.0.0.
func (e *Engine) addToReset(name, within string) {
	e.hookReset(within)
	clname := "cl@" + within
	var body []tok
	if m := e.eq[clname]; m != nil && m.kind == mMacro {
		body = append(body, m.body...)
	}
	body = append(body, csTok("@stpelt"), chTok('{', catBegin))
	body = append(body, stringToToks(name)...)
	body = append(body, chTok('}', catEnd))
	e.define(clname, &meaning{kind: mMacro, body: body}, false)
}

// hookReset ensures the sectioning macro that owns `within` runs \cl@<within>
// after it steps its counter, so registered sub-counters reset. It edits the
// macro's meaning in place (rather than the kernel source), and is idempotent:
// a second call for the same parent leaves the already-appended call alone.
func (e *Engine) hookReset(within string) {
	var secMacro string
	switch within {
	case "section":
		secMacro = "@nsection"
	case "subsection":
		secMacro = "@nsubsection"
	default:
		return // no reset support for other parents (e.g. absent \chapter)
	}
	clname := "cl@" + within
	if e.eq[clname] == nil {
		e.define(clname, &meaning{kind: mMacro}, false)
	}
	m := e.eq[secMacro]
	if m == nil || m.kind != mMacro {
		return
	}
	if n := len(m.body); n > 0 && m.body[n-1].cs_ && m.body[n-1].cs == clname {
		return // already hooked
	}
	nb := append(append([]tok{}, m.body...), csTok(clname))
	e.define(secMacro, &meaning{kind: mMacro, params: m.params, body: nb}, false)
}

// stringToToks converts a literal heading string into character tokens, with the
// natural category codes (letters, spaces, everything else "other").
func stringToToks(s string) []tok {
	var out []tok
	for _, r := range s {
		switch {
		case r == ' ':
			out = append(out, chTok(' ', catSpace))
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			out = append(out, chTok(r, catLetter))
		default:
			out = append(out, chTok(r, catOther))
		}
	}
	return out
}

// digitToks renders a non-negative integer as "other"-category digit tokens.
func digitToks(n int) []tok {
	if n == 0 {
		return []tok{chTok('0', catOther)}
	}
	var rev []rune
	for n > 0 {
		rev = append(rev, rune('0'+n%10))
		n /= 10
	}
	out := make([]tok, len(rev))
	for i := range rev {
		out[i] = chTok(rev[len(rev)-1-i], catOther)
	}
	return out
}

// thmTypeToks builds \def\gotex@thmtype{env}, which is how a theorem environment
// tells \@begintheorem its own name. \refstepcounter cannot: the counter a
// theorem steps is not the environment — \newtheorem{theo}[definition]{Theorem}
// steps `definition` — and cleveref keys its naming on the ENVIRONMENT.
func thmTypeToks(env string) []tok {
	t := []tok{csTok("def"), csTok("gotex@thmtype"), chTok('{', catBegin)}
	t = append(t, stringToToks(env)...)
	return append(t, chTok('}', catEnd))
}

// recordTheoremCrefName names a theorem environment for \cref, from the heading
// it was declared with — cleveref's own rule, which it applies by patching all
// three of LaTeX's \newtheorem internals (cleveref.sty:105-173):
//
//	cref@<env>@name  = \MakeLowercase <Heading>
//	Cref@<env>@name  = \MakeUppercase <Heading>
//
// and \MakeUppercase takes ONE token, so those fold the FIRST LETTER only:
// "Theorem" gives "theorem" and "Theorem", not "THEOREM". Checked against
// tectonic, which prints "theorem 1" / "Theorem 1" for \newtheorem{theorem} and
// "lemma 1" / "Lemma 1" for \newtheorem{lem}{Lemma} — the name follows the
// HEADING, not the environment's own spelling.
//
// The PLURAL is deliberately left empty, because cleveref leaves it empty too:
// tectonic renders \cref{c:a,c:b} on a \newtheorem{cor}{Corollary} as "?? 1?? 2".
// crefText prints the bare numbers for an unnamed plural instead, which is the
// one place here that does not follow the reference — reproducing "??" would be
// faithful to a rendering nobody wants.
func (e *Engine) recordTheoremCrefName(env string, head []tok) {
	name := strings.TrimSpace(e.toksToString(e.expandList(append([]tok(nil), head...))))
	if name == "" || env == "" {
		return
	}
	if e.crefThmNames == nil {
		e.crefThmNames = map[string]crefForm{}
	}
	e.crefThmNames[env] = crefForm{
		lower: foldFirst(name, false),
		upper: foldFirst(name, true),
	}
}

// foldFirst upper- or lowercases the first letter of s and leaves the rest, which
// is what \MakeUppercase/\MakeLowercase applied to an unbraced argument do.
func foldFirst(s string, upper bool) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	if upper {
		r[0] = unicode.ToUpper(r[0])
	} else {
		r[0] = unicode.ToLower(r[0])
	}
	return string(r)
}

// doDeclaretheorem implements thmtools' \declaretheorem[<keys>]{<names>}[<keys>], which
// is a key-value front end onto \newtheorem. Read from thm-kv.sty rather than guessed,
// because three of its details are not guessable:
//
//  1. {<names>} is a COMMA LIST — \declaretheorem{theorem,lemma} declares both
//     (thm-kv.sty:337, \@for\thmt@tmp:=#2\do).
//  2. the two counter keys map to the two DIFFERENT optional arguments of \newtheorem,
//     and getting them the wrong way round gives numbering that looks plausible and is
//     wrong. thm-kv.sty:362 emits exactly
//
//     \newtheorem {<env>} [<sibling>]? {<name>} [<parent>]?        numbered
//     \newtheorem*{<env>} {<name>}                                 not numbered
//
//     so sibling/numberlike/sharenumber is \newtheorem's SHARED counter (before the
//     heading) and parent/numberwithin/within is its WITHIN counter (after it).
//  3. the default heading is the environment name with its FIRST LETTER uppercased, not
//     the whole word: thm-kv.sty:354 is \thmt@setthmname{\thmt@modifycase #1} with NO
//     braces, and \thmt@modifycase defaults to \MakeUppercase (:42, \ExecuteOptions),
//     which takes a single token as an undelimited argument.
//
// Measured before it was written: 219 corpus papers use \declaretheorem, 1735 times.
// Skipped, the environments it declares arrive undefined — which costs the heading and
// its number, not the body (26 glyph paths with \newtheorem against 17 without, the nine
// being "Theorem 1."), so this is a fidelity gap and not content loss.
//
// The style, qed and hook keys are formatting that the engine has no model for; they are
// read and dropped rather than refused, so a declaration carrying them still numbers.
func (e *Engine) doDeclaretheorem() {
	pre, _ := e.scanOptBracketToks()
	names := strings.TrimSpace(e.toksToString(e.readBraceToks()))
	post, _ := e.scanOptBracketToks() // thmtools accepts keys on either side
	keys := strings.TrimSpace(e.toksToString(pre))
	if p := strings.TrimSpace(e.toksToString(post)); p != "" {
		if keys != "" {
			keys += ","
		}
		keys += p
	}
	heading, sibling, parent := "", "", ""
	numbered := true
	for _, kv := range splitTopLevelComma(keys) {
		k, v := splitKeyValue(kv)
		switch k {
		case "name", "title", "heading":
			heading = v
		case "sibling", "numberlike", "sharenumber":
			sibling = v
		case "parent", "numberwithin", "within":
			parent = v
		case "numbered":
			// "unless unique" numbers only when the environment occurs more than once,
			// which needs a whole-document count; numbering it is the closer of the two.
			numbered = v != "no" && v != "false"
		}
	}
	var b strings.Builder
	for _, name := range splitTopLevelComma(names) {
		if name == "" {
			continue
		}
		head := heading
		if head == "" {
			head = upperFirstRune(name)
		}
		if !numbered {
			b.WriteString(`\newtheorem*{` + name + `}{` + head + `}`)
			continue
		}
		b.WriteString(`\newtheorem{` + name + `}`)
		if sibling != "" {
			b.WriteString(`[` + sibling + `]`)
		}
		b.WriteString(`{` + head + `}`)
		if parent != "" {
			b.WriteString(`[` + parent + `]`)
		}
	}
	if s := b.String(); s != "" {
		e.push(tokenizeTeX(s))
	}
}

// splitTopLevelComma splits on commas that are not inside braces, so name={A, B} and
// \declaretheorem{a,b} both read correctly, and trims each field as keyval does.
func splitTopLevelComma(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out
}

// splitKeyValue splits "key=value" at the first top-level '=' and strips one layer of
// braces from the value, which is how keyval passes name={Some Heading}.
func splitKeyValue(s string) (string, string) {
	depth := 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '=':
			if depth == 0 {
				k := strings.TrimSpace(s[:i])
				v := strings.TrimSpace(s[i+1:])
				if len(v) >= 2 && v[0] == '{' && v[len(v)-1] == '}' {
					v = v[1 : len(v)-1]
				}
				return k, v
			}
		}
	}
	return strings.TrimSpace(s), ""
}

// upperFirstRune uppercases the first rune only — \MakeUppercase of an UNDELIMITED
// argument takes one token, which is what thmtools relies on for its default heading.
func upperFirstRune(s string) string {
	for i, r := range s {
		return strings.ToUpper(string(r)) + s[i+len(string(r)):]
	}
	return s
}
