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

import "strings"

// doNewtheorem implements \newtheorem{env}{Heading}, its \newtheorem{env}[shared]
// {Heading} (share another environment's counter) and \newtheorem{env}{Heading}
// [within] (number within a parent counter, e.g. Theorem 1.1) forms.
func (e *Engine) doNewtheorem() {
	env := e.readBraceName()
	sharedToks, hasShared := e.scanOptBracketToks() // [shared] BEFORE the heading
	heading := e.readBraceName()                    // {Heading}
	withinToks, hasWithin := e.scanOptBracketToks() // [within] AFTER the heading
	if env == "" {
		return
	}

	// Choose the counter register that this environment steps.
	ctr := "c@" + env
	ctrCode := -1
	switch {
	case hasShared:
		if shared := strings.TrimSpace(e.toksToString(sharedToks)); shared != "" {
			ctr = "c@" + shared
		}
	default:
		// Allocate a fresh \count register, exactly as \newcount would.
		if e.allocCnt < 256 {
			ctrCode = e.allocCnt
			e.define(ctr, &meaning{kind: mCountRef, code: ctrCode}, false)
			e.allocCnt++
		}
	}

	// \the<env>: the printed representation of the number.
	var numBody []tok
	if hasWithin {
		within := strings.TrimSpace(e.toksToString(withinToks))
		// Nest on the parent's FORMATTED number (\the<within>, which itself chains
		// e.g. section.subsection), matching LaTeX's \newtheorem[within].
		numBody = []tok{csTok("the" + within), chTok('.', catOther), csTok("the"), csTok(ctr)}
	} else {
		numBody = []tok{csTok("the"), csTok(ctr)}
	}
	e.define("the"+env, &meaning{kind: mMacro, body: numBody}, false)

	// A within-numbered environment resets on its parent counter's step.
	if hasWithin && !hasShared && ctrCode >= 0 {
		within := strings.TrimSpace(e.toksToString(withinToks))
		e.addToReset(ctrCode, within)
	}

	// \<env>: \begin{env}. Open a group (so \it reverts at \end), globally step the
	// counter (global to survive the group), freeze the number as \@currentlabel,
	// then hand {Heading}{number} to \@begintheorem (which reads the optional note).
	head := stringToToks(heading)
	body := []tok{
		csTok("par"), csTok("medskip"), csTok("begingroup"),
		csTok("global"), csTok("advance"), csTok(ctr), chTok(' ', catSpace),
		chTok('b', catLetter), chTok('y', catLetter), chTok('1', catOther), csTok("relax"),
		csTok("edef"), csTok("@currentlabel"), chTok('{', catBegin), csTok("the" + env), chTok('}', catEnd),
		csTok("@begintheorem"), chTok('{', catBegin),
	}
	body = append(body, head...)
	body = append(body, chTok('}', catEnd), chTok('{', catBegin), csTok("the"+env), chTok('}', catEnd))
	e.define(env, &meaning{kind: mMacro, body: body}, false)

	// \end<env>: the fixed closing macro (end paragraph, close group, vertical space).
	e.define("end"+env, &meaning{kind: mMacro, body: []tok{csTok("@endtheorem")}}, false)
}

// addToReset arranges for the count register ctrCode to be zeroed whenever the
// parent counter `within` is stepped. It appends "\global\count<code>=0" to the
// reset-list macro \cl@<within>, and hooks the corresponding sectioning macro
// (\@nsection / \@nsubsection) once so that it runs the reset list after stepping
// — mirroring LaTeX's \@addtoreset / \cl@… mechanism.
func (e *Engine) addToReset(ctrCode int, within string) {
	e.hookReset(within)
	clname := "cl@" + within
	var body []tok
	if m := e.eq[clname]; m != nil && m.kind == mMacro {
		body = append(body, m.body...)
	}
	body = append(body, csTok("global"), csTok("count"))
	body = append(body, digitToks(ctrCode)...)
	body = append(body, chTok('=', catOther), chTok('0', catOther), csTok("relax"))
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
