// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// xargs' \newcommandx is \newcommand generalised: an optional argument at ANY position,
// not only the first.
//
//	\newcommandx{\cmd}[<n>][<pos>=<default>,…]{<body>}
//
// ⛔ Undefined, it was SKIPPED — and skipping releases the arguments, so the body's group
// was left to the surrounding text. Measured: 2603.18955 (lmcs) skips \newcommandx five
// times and ends with exactly FIVE groups still open, and the document renders as ONE page
// out of 139KB of source. The arithmetic is the diagnosis: one leaked group per skip.
//
// 14 uses over 3 papers of the 999-paper corpus, and every one has its optional positions
// as a LEADING run — [1=…] ten times, [1=x,2=z] twice, [1=x,2=z,3=…,4=…] twice — so
// translating to an xparse specification of O{default} and m is exact, not an
// approximation. The specification is built here rather than parsed from a string because
// the defaults are TOKENS: \gamz's are \lr and \xz.
func (e *Engine) doNewcommandx(mode xpMode) {
	e.peekStar() // \newcommandx* — the star only picks \long, which every macro is here
	name := e.scanCmdName()
	n := e.scanOptBracketInt()
	spec, _ := e.scanOptBracketToks()
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return
	}
	body := e.scanBody() // always consumed, even when we will not (re)define
	if name == "" {
		return
	}
	if mode == xpProvide && e.eq[name] != nil {
		return
	}
	defs := parseNewcommandxDefaults(spec)
	specs := make([]xpArg, 0, n)
	for i := 1; i <= n; i++ {
		if d, ok := defs[i]; ok {
			specs = append(specs, xpArg{kind: xpOptional, def: d})
			continue
		}
		specs = append(specs, xpArg{kind: xpMandatory})
	}
	e.eq[name] = &meaning{
		kind: mPrim,
		name: "gotex@doc@" + name,
		// The specification is exact — only O{default} and m come out of the loop above —
		// so the maths layer's string-level expansion may serve it, exactly as it does a
		// \NewDocumentCommand of the same shape (see expandXparseInMathSource).
		xpDoc:   true,
		xpSpecs: specs,
		body:    body,
		prim: func(e *Engine) {
			args := e.grabXparseArgs(specs)
			e.push(substituteParams(body, args))
		},
	}
}

// parseNewcommandxDefaults reads xargs' "<pos>=<default>,…" list into the default value
// for each optional position.
//
// The split is at brace level zero on both separators: a default may itself hold a comma
// or an equals sign inside a group, and \gamz{…}[1=x,2=z,3=\lr,4=\xz] shows the values are
// control sequences as often as characters. A malformed entry — no "=", or a position that
// is not a positive integer — is DISCARDED rather than guessed at, which leaves that
// argument mandatory: wrong arity is what cost #497 its 62 equations, and a mandatory
// argument at least consumes what the call site wrote.
func parseNewcommandxDefaults(spec []tok) map[int][]tok {
	out := map[int][]tok{}
	depth := 0
	item := []tok{}
	flush := func() {
		defer func() { item = nil }()
		eq := -1
		d := 0
		for i, t := range item {
			switch {
			case !t.cs_ && t.cat == catBegin:
				d++
			case !t.cs_ && t.cat == catEnd:
				d--
			case !t.cs_ && d == 0 && t.ch == '=' && eq < 0:
				eq = i
			}
		}
		if eq < 0 {
			return
		}
		pos, ok := toksToPosInt(item[:eq])
		if !ok {
			return
		}
		out[pos] = append([]tok{}, item[eq+1:]...)
	}
	for _, t := range spec {
		switch {
		case !t.cs_ && t.cat == catBegin:
			depth++
		case !t.cs_ && t.cat == catEnd:
			depth--
		case !t.cs_ && depth == 0 && t.ch == ',':
			flush()
			continue
		}
		item = append(item, t)
	}
	flush()
	return out
}

// toksToPosInt reads a positive integer written as plain digit tokens, ignoring spaces.
// Anything else is not a position, and saying so is what keeps a malformed entry from
// becoming an optional argument at position 0.
func toksToPosInt(ts []tok) (int, bool) {
	n, any := 0, false
	for _, t := range ts {
		if t.cs_ {
			return 0, false
		}
		if t.ch == ' ' || t.ch == '\t' {
			continue
		}
		if t.ch < '0' || t.ch > '9' {
			return 0, false
		}
		n = n*10 + int(t.ch-'0')
		any = true
	}
	return n, any && n > 0
}
