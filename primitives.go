// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strconv"

// expandableSet lists the primitives that act in the gullet (expansion).
var expandableSet = map[string]bool{
	"expandafter": true, "csname": true, "noexpand": true, "string": true,
	"the": true, "number": true, "romannumeral": true, "jobname": true,
	"if": true, "ifnum": true, "ifx": true, "ifodd": true, "ifcase": true,
	"iftrue": true, "iffalse": true, "else": true, "fi": true, "or": true, "ifcat": true,
}

func isExpandable(name string) bool { return expandableSet[name] }

func isIfPrim(name string) bool {
	switch name {
	case "if", "ifnum", "ifx", "ifodd", "ifcase", "iftrue", "iffalse", "ifcat":
		return true
	}
	return false
}

func (e *Engine) prim(name string, fn func(*Engine)) {
	e.eq[name] = &meaning{kind: mPrim, name: name, prim: fn}
}

// loadPrimitives installs the built-in control sequences.
func (e *Engine) loadPrimitives() {
	// definitions
	e.prim("def", func(e *Engine) { e.doDef(false, false) })
	e.prim("gdef", func(e *Engine) { e.doDef(true, false) })
	e.prim("edef", func(e *Engine) { e.doDef(false, true) })
	e.prim("xdef", func(e *Engine) { e.doDef(true, true) })
	e.prim("let", func(e *Engine) { e.doLet(false) })
	e.prim("global", func(e *Engine) { e.doGlobal() })
	e.prim("chardef", func(e *Engine) { e.doChardef(false) })

	// registers & arithmetic
	e.prim("count", func(e *Engine) { e.doCountAssign(false) })
	e.prim("advance", func(e *Engine) { e.doAdvance(false) })
	e.prim("multiply", func(e *Engine) { e.doMultiply(false) })
	e.prim("catcode", func(e *Engine) { e.doCatcode(false) })

	// grouping & misc
	e.prim("begingroup", func(e *Engine) { e.beginGroup() })
	e.prim("endgroup", func(e *Engine) { e.endGroup() })
	e.prim("relax", func(e *Engine) {})
	e.prim("message", func(e *Engine) { e.doMessage() })

	// expansion
	e.prim("expandafter", func(e *Engine) { e.doExpandafter() })
	e.prim("csname", func(e *Engine) { e.doCsname() })
	e.prim("endcsname", func(e *Engine) {})
	e.prim("noexpand", func(e *Engine) { e.doNoexpand() })
	e.prim("string", func(e *Engine) { e.doString() })
	e.prim("number", func(e *Engine) { e.pushString(strconv.Itoa(e.scanInt())) })
	e.prim("the", func(e *Engine) { e.doThe() })
	e.prim("romannumeral", func(e *Engine) { e.pushString(roman(e.scanInt())) })

	// conditionals
	e.prim("ifnum", func(e *Engine) { e.doIf(e.evalIfnum()) })
	e.prim("ifodd", func(e *Engine) { e.doIf(e.scanInt()%2 != 0) })
	e.prim("ifx", func(e *Engine) { e.doIf(e.evalIfx()) })
	e.prim("if", func(e *Engine) { e.doIf(e.evalIf()) })
	e.prim("iftrue", func(e *Engine) { e.doIf(true) })
	e.prim("iffalse", func(e *Engine) { e.doIf(false) })
	e.prim("ifcase", func(e *Engine) { e.doIfcase() })
	e.prim("else", func(e *Engine) { e.skipToFi() })
	e.prim("fi", func(e *Engine) {})
	e.prim("or", func(e *Engine) { e.skipToFi() })
}

// ── definitions ─────────────────────────────────────────────────────────────

func (e *Engine) doDef(global, expandBody bool) {
	name := e.scanCSName()
	params, body := e.scanDefText()
	if expandBody {
		body = e.expandList(body)
	}
	if name != "" {
		e.define(name, &meaning{kind: mMacro, params: params, body: body}, global)
	}
}

func (e *Engine) doLet(global bool) {
	name := e.scanCSName()
	e.skipOptSpace()
	if t, ok := e.getNext(); ok && t.is('=', catOther) {
		e.skipOptSpace0()
	} else if ok {
		e.back(t)
	}
	rhs, ok := e.getNext()
	if !ok || name == "" {
		return
	}
	if rhs.cs_ {
		if m := e.eq[rhs.cs]; m != nil {
			cp := *m
			e.define(name, &cp, global)
			return
		}
		e.define(name, &meaning{kind: mUndef}, global)
		return
	}
	e.define(name, &meaning{kind: mLetChar, ch: rhs.ch, cat: rhs.cat}, global)
}

func (e *Engine) skipOptSpace0() {
	if t, ok := e.getNext(); ok && !(t.cat == catSpace && !t.cs_) {
		e.back(t)
	}
}

func (e *Engine) doGlobal() {
	t, ok := e.getXToken()
	if !ok || !t.cs_ {
		return
	}
	m := e.eq[t.cs]
	if m == nil || m.kind != mPrim {
		return
	}
	switch m.name {
	case "def":
		e.doDef(true, false)
	case "edef":
		e.doDef(true, true)
	case "let":
		e.doLet(true)
	case "count":
		e.doCountAssign(true)
	case "advance":
		e.doAdvance(true)
	case "multiply":
		e.doMultiply(true)
	case "chardef":
		e.doChardef(true)
	case "catcode":
		e.doCatcode(true)
	}
}

func (e *Engine) doChardef(global bool) {
	name := e.scanCSName()
	e.scanEquals()
	code := e.scanInt()
	if name != "" {
		e.define(name, &meaning{kind: mCharDef, code: code}, global)
	}
}

// ── registers ───────────────────────────────────────────────────────────────

func (e *Engine) doCountAssign(global bool) {
	i := e.scanInt()
	e.scanEquals()
	v := e.scanInt()
	if i >= 0 && i < 256 {
		e.setCount(i, v, global)
	}
}

func (e *Engine) doAdvance(global bool) {
	t, ok := e.getXToken()
	if !ok || !t.cs_ || e.eq[t.cs] == nil || e.eq[t.cs].name != "count" {
		return
	}
	i := e.scanInt()
	e.skipByKeyword()
	v := e.scanInt()
	if i >= 0 && i < 256 {
		e.setCount(i, e.count[i]+v, global)
	}
}

func (e *Engine) doMultiply(global bool) {
	t, ok := e.getXToken()
	if !ok || !t.cs_ || e.eq[t.cs] == nil || e.eq[t.cs].name != "count" {
		return
	}
	i := e.scanInt()
	e.skipByKeyword()
	v := e.scanInt()
	if i >= 0 && i < 256 {
		e.setCount(i, e.count[i]*v, global)
	}
}

// skipByKeyword consumes an optional "by".
func (e *Engine) skipByKeyword() {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if ok && (t.is('b', catLetter) || t.is('b', catOther)) {
		u, _ := e.getXToken()
		_ = u
		return
	}
	if ok {
		e.back(t)
	}
}

func (e *Engine) doCatcode(global bool) {
	r := e.scanInt()
	e.scanEquals()
	c := e.scanInt()
	e.setCat(rune(r), cat(c), global)
}

// ── expansion primitives ────────────────────────────────────────────────────

func (e *Engine) doExpandafter() {
	t1, ok := e.getNext()
	if !ok {
		return
	}
	t2, ok := e.getNext()
	if !ok {
		e.back(t1)
		return
	}
	e.expandOnce(t2)
	e.back(t1)
}

// expandOnce expands t by one level (macro or expandable primitive), else backs it.
func (e *Engine) expandOnce(t tok) {
	if t.noexp {
		e.back(t)
		return
	}
	m := e.meaningOf(t)
	if m != nil && m.kind == mMacro {
		e.expandMacro(m)
		return
	}
	if m != nil && m.kind == mPrim && isExpandable(m.name) {
		m.prim(e)
		return
	}
	e.back(t)
}

func (e *Engine) doCsname() {
	var name []rune
	for {
		t, ok := e.getXToken()
		if !ok {
			break
		}
		if t.cs_ {
			if m := e.eq[t.cs]; m != nil && m.kind == mPrim && m.name == "endcsname" {
				break
			}
			// non-endcsname cs inside \csname: treat its name's chars? TeX errors;
			// we append its string form.
			name = append(name, []rune(t.cs)...)
			continue
		}
		name = append(name, t.ch)
	}
	s := string(name)
	if e.eq[s] == nil {
		e.eq[s] = &meaning{kind: mPrim, name: "relax", prim: func(e *Engine) {}}
	}
	e.back(csTok(s))
}

func (e *Engine) doNoexpand() {
	t, ok := e.getNext()
	if !ok {
		return
	}
	t.noexp = true
	e.back(t)
}

func (e *Engine) doString() {
	t, ok := e.getNext()
	if !ok {
		return
	}
	if t.cs_ {
		e.pushString("\\" + t.cs)
	} else {
		e.pushString(string(t.ch))
	}
}

func (e *Engine) doThe() {
	t, ok := e.getXToken()
	if !ok {
		return
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil {
			switch {
			case m.kind == mPrim && m.name == "count":
				e.pushString(strconv.Itoa(e.count[e.scanInt()]))
				return
			case m.kind == mCharDef:
				e.pushString(strconv.Itoa(m.code))
				return
			}
		}
	}
	e.back(t)
}

// ── conditionals ────────────────────────────────────────────────────────────

func (e *Engine) doIf(cond bool) {
	if cond {
		return // execute the true branch; \else/\fi handle the rest
	}
	if e.skipToElseOrFi() == "else" {
		return // execute the else branch
	}
}

func (e *Engine) doIfcase() {
	n := e.scanInt()
	for n > 0 {
		r := e.skipToElseOrFiOrOr()
		if r == "fi" || r == "else" {
			return
		}
		n--
	}
	// fall through to the n-th case body
}

func (e *Engine) evalIfnum() bool {
	a := e.scanInt()
	e.skipOptSpace()
	rel, ok := e.getXToken()
	b := 0
	if ok {
		b = e.scanInt()
	}
	switch {
	case rel.is('<', catOther):
		return a < b
	case rel.is('>', catOther):
		return a > b
	case rel.is('=', catOther):
		return a == b
	}
	return false
}

func (e *Engine) evalIfx() bool {
	a, ok1 := e.getNext()
	b, ok2 := e.getNext()
	if !ok1 || !ok2 {
		return false
	}
	ma, mb := e.meaningOf(a), e.meaningOf(b)
	if ma == nil && mb == nil {
		return tokEq(a, b)
	}
	if ma == nil || mb == nil {
		return false
	}
	return meaningEq(ma, mb)
}

func (e *Engine) evalIf() bool {
	a, _ := e.getXToken()
	b, _ := e.getXToken()
	ca, cb := charOf(a), charOf(b)
	return ca == cb
}

func charOf(t tok) rune {
	if t.cs_ {
		return -1
	}
	return t.ch
}

func meaningEq(a, b *meaning) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case mPrim:
		return a.name == b.name
	case mCharDef:
		return a.code == b.code
	case mLetChar:
		return a.ch == b.ch && a.cat == b.cat
	case mMacro:
		return sameToks(a.params, b.params) && sameToks(a.body, b.body)
	}
	return true
}

func sameToks(a, b []tok) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !tokEq(a[i], b[i]) {
			return false
		}
	}
	return true
}

// skipToElseOrFi skips balanced tokens to the matching \else or \fi.
func (e *Engine) skipToElseOrFi() string {
	depth := 0
	for {
		t, ok := e.getNext()
		if !ok {
			return "fi"
		}
		if !t.cs_ {
			continue
		}
		m := e.eq[t.cs]
		if m == nil || m.kind != mPrim {
			continue
		}
		if isIfPrim(m.name) {
			depth++
		} else if m.name == "fi" {
			if depth == 0 {
				return "fi"
			}
			depth--
		} else if m.name == "else" && depth == 0 {
			return "else"
		}
	}
}

func (e *Engine) skipToElseOrFiOrOr() string {
	depth := 0
	for {
		t, ok := e.getNext()
		if !ok {
			return "fi"
		}
		if !t.cs_ {
			continue
		}
		m := e.eq[t.cs]
		if m == nil || m.kind != mPrim {
			continue
		}
		if isIfPrim(m.name) {
			depth++
		} else if m.name == "fi" {
			if depth == 0 {
				return "fi"
			}
			depth--
		} else if depth == 0 && (m.name == "else" || m.name == "or") {
			return m.name
		}
	}
}

func (e *Engine) skipToFi() {
	depth := 0
	for {
		t, ok := e.getNext()
		if !ok {
			return
		}
		if !t.cs_ {
			continue
		}
		m := e.eq[t.cs]
		if m == nil || m.kind != mPrim {
			continue
		}
		if isIfPrim(m.name) {
			depth++
		} else if m.name == "fi" {
			if depth == 0 {
				return
			}
			depth--
		}
	}
}

// ── \message and helpers ────────────────────────────────────────────────────

func (e *Engine) doMessage() {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		return
	}
	b := e.expandList(e.grabGroup())
	if e.out.Len() > 0 {
		e.out.WriteByte(' ')
	}
	e.out.WriteString(e.toksToString(b))
}

// sentinel marks the end of an isolated expansion (getXToken returns it
// literally since it has no meaning, so expandList can stop reliably regardless
// of what is already on the input stack).
var sentinel = tok{cs: "\x00end-expand", cs_: true}

// expandList fully expands a token list in isolation (for \edef / \message),
// stopping at the sentinel rather than by counting input lists.
func (e *Engine) expandList(ts []tok) []tok {
	e.push(append(append([]tok(nil), ts...), sentinel))
	saved := e.noBase
	e.noBase = true
	var out []tok
	for {
		t, ok := e.getXToken()
		if !ok || (t.cs_ && t.cs == sentinel.cs) {
			break
		}
		out = append(out, t)
	}
	e.noBase = saved
	return out
}

func (e *Engine) pushString(s string) {
	ts := make([]tok, 0, len(s))
	for _, r := range s {
		c := catOther
		if r == ' ' {
			c = catSpace
		}
		ts = append(ts, chTok(r, c))
	}
	e.push(ts)
}

func roman(n int) string {
	if n <= 0 {
		return ""
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var b []byte
	for i, v := range vals {
		for n >= v {
			b = append(b, syms[i]...)
			n -= v
		}
	}
	return string(b)
}

// ── further mouth primitives (chaining toward parity) ───────────────────────

func (e *Engine) loadMore() {
	e.prim("uppercase", func(e *Engine) { e.doCase(true) })
	e.prim("lowercase", func(e *Engine) { e.doCase(false) })
	e.prim("ifcat", func(e *Engine) { e.doIf(e.evalIfcat()) })
	e.prim("meaning", func(e *Engine) { e.doMeaning() })
	expandableSet["meaning"] = true
	// \empty and \space are ordinary macros, defined as in plain TeX.
	e.eq["empty"] = &meaning{kind: mMacro}
	e.eq["space"] = &meaning{kind: mMacro, body: []tok{chTok(' ', catSpace)}}
}

// doCase applies \uppercase/\lowercase to the next {group}: it re-cases letter
// tokens and re-inserts the list for further processing (as TeX does).
func (e *Engine) doCase(up bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		return
	}
	g := e.grabGroup()
	for i, x := range g {
		if x.cs_ {
			continue
		}
		r := x.ch
		if up && r >= 'a' && r <= 'z' {
			g[i].ch = r - 32
		} else if !up && r >= 'A' && r <= 'Z' {
			g[i].ch = r + 32
		}
	}
	e.push(g)
}

func (e *Engine) evalIfcat() bool {
	a, _ := e.getXToken()
	b, _ := e.getXToken()
	ca, cb := catClass(a), catClass(b)
	return ca == cb
}

func catClass(t tok) cat {
	if t.cs_ {
		return 16 // control sequences compare equal to each other
	}
	return t.cat
}

func (e *Engine) doMeaning() {
	t, ok := e.getNext()
	if !ok {
		return
	}
	e.pushString(e.meaningString(t))
}

func (e *Engine) meaningString(t tok) string {
	m := e.meaningOf(t)
	if m == nil {
		if t.cs_ {
			return "undefined"
		}
		return catName(t.cat) + " " + string(t.ch)
	}
	switch m.kind {
	case mMacro:
		return "macro:" + e.toksToString(m.params) + "->" + e.toksToString(m.body)
	case mPrim:
		return "\\" + m.name
	case mCharDef:
		return "\\char\"" + itoaHex(m.code)
	case mLetChar:
		return catName(m.cat) + " " + string(m.ch)
	}
	return "undefined"
}

func catName(c cat) string {
	switch c {
	case catBegin:
		return "begin-group character"
	case catEnd:
		return "end-group character"
	case catLetter:
		return "the letter"
	case catSpace:
		return "blank space"
	default:
		return "the character"
	}
}

func itoaHex(n int) string {
	const d = "0123456789ABCDEF"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{d[n&15]}, b...)
		n >>= 4
	}
	return string(b)
}
