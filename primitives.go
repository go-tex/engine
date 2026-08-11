// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strconv"
	"strings"
)

// formatPt renders a dimension in scaled points the way TeX's \the does, using
// print_scaled (§103): e.g. 196608 → "3.0pt", 4736287 → "72.26999pt".
func formatPt(sp int) string {
	var b strings.Builder
	if sp < 0 {
		b.WriteByte('-')
		sp = -sp
	}
	b.WriteString(strconv.Itoa(sp / unity)) // integer part
	b.WriteByte('.')
	s := 10*(sp%unity) + 5
	delta := 10
	for {
		if delta > unity {
			s += unity/2 - 50000 // round the last displayed digit
		}
		b.WriteByte(byte('0' + s/unity))
		s = 10 * (s % unity)
		delta *= 10
		if s <= delta {
			break
		}
	}
	b.WriteString("pt")
	return b.String()
}

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
	e.prim("countdef", func(e *Engine) { e.doCountdef() })
	e.prim("newcount", func(e *Engine) { e.doNewcount() })

	// registers & arithmetic
	e.prim("count", func(e *Engine) { e.doCountAssign(false) })
	e.prim("dimen", func(e *Engine) { e.doDimenAssign(false) })
	e.prim("dimendef", func(e *Engine) { e.doDimendef() })
	e.prim("newdimen", func(e *Engine) { e.doNewdimen() })
	e.prim("skip", func(e *Engine) { e.doSkipAssign(false) })
	e.prim("skipdef", func(e *Engine) { e.doSkipdef() })
	e.prim("newskip", func(e *Engine) { e.doNewskip() })
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

	// siunitx subset (\num, \si, \unit, \SI/\qty, \ang) — see siunitx.go.
	e.loadSIUnitx()
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

// doCountdef handles \countdef\name=<n>: \name becomes an alias for \count<n>.
func (e *Engine) doCountdef() {
	name := e.scanCSName()
	e.scanEquals()
	n := e.scanInt()
	if name != "" {
		e.define(name, &meaning{kind: mCountRef, code: n}, false)
	}
}

// doNewcount handles \newcount\name: allocate the next free \count register and
// \countdef \name to it (a primitive stand-in for plain.tex's allocator).
func (e *Engine) doNewcount() {
	name := e.scanCSName()
	if name == "" || e.allocCnt >= 256 {
		return
	}
	e.define(name, &meaning{kind: mCountRef, code: e.allocCnt}, false)
	e.allocCnt++
}

// countRefAssign handles an assignment to a \countdef'd register: \name=<v>.
func (e *Engine) countRefAssign(code int, global bool) {
	e.scanEquals()
	v := e.scanInt()
	if code >= 0 && code < 256 {
		e.setCount(code, v, global)
	}
}

// countIndex reads a count-register reference for \advance/\multiply/\the: either
// the \count primitive followed by a number, or a \countdef'd control sequence.
func (e *Engine) countIndex() (int, bool) {
	t, ok := e.getXToken()
	if !ok || !t.cs_ {
		if ok {
			e.back(t)
		}
		return 0, false
	}
	m := e.eq[t.cs]
	if m == nil {
		e.back(t)
		return 0, false
	}
	if m.kind == mCountRef {
		return m.code, true
	}
	if m.kind == mPrim && m.name == "count" {
		return e.scanInt(), true
	}
	e.back(t)
	return 0, false
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

// doDimenAssign handles \dimen<n>=<dimen>.
func (e *Engine) doDimenAssign(global bool) {
	i := e.scanInt()
	e.scanEquals()
	v := e.scanDimen()
	e.setDimen(i, v, global)
}

// doDimendef handles \dimendef\name=<n>.
func (e *Engine) doDimendef() {
	name := e.scanCSName()
	e.scanEquals()
	n := e.scanInt()
	if name != "" {
		e.define(name, &meaning{kind: mDimenRef, code: n}, false)
	}
}

// doNewdimen handles \newdimen\name (allocate the next free \dimen register).
func (e *Engine) doNewdimen() {
	name := e.scanCSName()
	if name == "" || e.allocDim >= 256 {
		return
	}
	e.define(name, &meaning{kind: mDimenRef, code: e.allocDim}, false)
	e.allocDim++
}

// dimenRefAssign handles an assignment to a \dimendef'd register: \name=<dimen>.
func (e *Engine) dimenRefAssign(code int, global bool) {
	e.scanEquals()
	e.setDimen(code, e.scanDimen(), global)
}

// dimenIndex reads a dimen-register reference: the \dimen primitive followed by a
// number, or a \dimendef'd control sequence. ok=false if the next token is neither.
func (e *Engine) dimenIndex() (int, bool) {
	t, ok := e.getXToken()
	if !ok {
		return 0, false
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil {
			if m.kind == mDimenRef {
				return m.code, true
			}
			if m.kind == mPrim && m.name == "dimen" {
				return e.scanInt(), true
			}
		}
	}
	e.back(t)
	return 0, false
}

// doSkipAssign handles \skip<n>=<glue>.
func (e *Engine) doSkipAssign(global bool) {
	i := e.scanInt()
	e.scanEquals()
	e.setSkip(i, e.scanGlue(), global)
}

// doSkipdef handles \skipdef\name=<n>.
func (e *Engine) doSkipdef() {
	name := e.scanCSName()
	e.scanEquals()
	n := e.scanInt()
	if name != "" {
		e.define(name, &meaning{kind: mSkipRef, code: n}, false)
	}
}

// doNewskip handles \newskip\name (allocate the next free \skip register).
func (e *Engine) doNewskip() {
	name := e.scanCSName()
	if name == "" || e.allocSkp >= 256 {
		return
	}
	e.define(name, &meaning{kind: mSkipRef, code: e.allocSkp}, false)
	e.allocSkp++
}

// skipRefAssign handles an assignment to a \skipdef'd register: \name=<glue>.
func (e *Engine) skipRefAssign(code int, global bool) {
	e.scanEquals()
	e.setSkip(code, e.scanGlue(), global)
}

// skipIndex reads a skip-register reference: the \skip primitive followed by a
// number, or a \skipdef'd control sequence. ok=false if the next token is neither.
func (e *Engine) skipIndex() (int, bool) {
	t, ok := e.getXToken()
	if !ok {
		return 0, false
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil {
			if m.kind == mSkipRef {
				return m.code, true
			}
			if m.kind == mPrim && m.name == "skip" {
				return e.scanInt(), true
			}
		}
	}
	e.back(t)
	return 0, false
}

func (e *Engine) doAdvance(global bool) {
	// \advance\leftskip / \advance\rightskip: the special per-line glue
	// parameters are primitives, not skip registers, so the register paths
	// below never match them. Handle them explicitly so nested list
	// environments can accumulate indentation with \advance\leftskip by24pt.
	if t, ok := e.getXToken(); ok {
		if t.cs_ {
			if m := e.eq[t.cs]; m != nil && m.kind == mPrim && (m.name == "leftskip" || m.name == "rightskip") {
				e.skipByKeyword()
				g := e.scanGlue()
				if m.name == "leftskip" {
					if !global && len(e.groups) > 0 {
						e.save = append(e.save, saveItem{kind: 6, oldg: e.leftskip})
					}
					e.leftskip = addGlue(e.leftskip, g)
				} else {
					if !global && len(e.groups) > 0 {
						e.save = append(e.save, saveItem{kind: 7, oldg: e.rightskip})
					}
					e.rightskip = addGlue(e.rightskip, g)
				}
				return
			}
		}
		e.back(t)
	}
	if i, ok := e.countIndex(); ok {
		e.skipByKeyword()
		v := e.scanInt()
		if i >= 0 && i < 256 {
			e.setCount(i, e.count[i]+v, global)
		}
		return
	}
	if i, ok := e.dimenIndex(); ok {
		e.skipByKeyword()
		v := e.scanDimen()
		e.setDimen(i, e.dimen[i]+v, global)
		return
	}
	if i, ok := e.skipIndex(); ok {
		e.skipByKeyword()
		g := e.scanGlue()
		e.setSkip(i, addGlue(e.skip[i], g), global)
	}
}

func (e *Engine) doMultiply(global bool) {
	if i, ok := e.countIndex(); ok {
		e.skipByKeyword()
		v := e.scanInt()
		if i >= 0 && i < 256 {
			e.setCount(i, e.count[i]*v, global)
		}
		return
	}
	if i, ok := e.dimenIndex(); ok {
		e.skipByKeyword()
		v := e.scanInt() // \multiply scales a dimen by an integer
		e.setDimen(i, e.dimen[i]*v, global)
		return
	}
	if i, ok := e.skipIndex(); ok {
		e.skipByKeyword()
		v := e.scanInt()
		e.setSkip(i, scaleGlue(e.skip[i], v), global)
	}
}

// addGlue adds two glues, combining like-order stretch/shrink (a higher order
// dominates a lower one, matching TeX's glue arithmetic).
func addGlue(a, b glueSpec) glueSpec {
	r := glueSpec{width: a.width + b.width}
	r.stretch, r.stretchOrder = addInf(a.stretch, a.stretchOrder, b.stretch, b.stretchOrder)
	r.shrink, r.shrinkOrder = addInf(a.shrink, a.shrinkOrder, b.shrink, b.shrinkOrder)
	return r
}

func addInf(av, ao, bv, bo int) (int, int) {
	switch {
	case ao == bo:
		return av + bv, ao
	case ao > bo:
		return av, ao
	default:
		return bv, bo
	}
}

// scaleGlue multiplies a glue's width and (finite and infinite) stretch/shrink by
// an integer.
func scaleGlue(g glueSpec, n int) glueSpec {
	return glueSpec{
		width: g.width * n, stretch: g.stretch * n, shrink: g.shrink * n,
		stretchOrder: g.stretchOrder, shrinkOrder: g.shrinkOrder,
	}
}

// formatGlue renders a glue the way TeX's \the does (print_glue / print_spec):
// the width in pt, then optional "plus"/"minus" components with fil orders.
func formatGlue(g glueSpec) string {
	var b strings.Builder
	b.WriteString(formatPt(g.width)) // width always carries its "pt" unit
	if g.stretch != 0 {
		b.WriteString(" plus " + glueComponent(g.stretch, g.stretchOrder))
	}
	if g.shrink != 0 {
		b.WriteString(" minus " + glueComponent(g.shrink, g.shrinkOrder))
	}
	return b.String()
}

// glueComponent renders one stretch/shrink component: finite in pt, or a fil order.
func glueComponent(v, order int) string {
	if order == 0 {
		return formatPt(v)
	}
	s := strings.TrimSuffix(formatPt(v), "pt")
	return s + "fil" + strings.Repeat("l", order-1)
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
			case m.kind == mCountRef:
				e.pushString(strconv.Itoa(e.count[m.code]))
				return
			case m.kind == mPrim && m.name == "dimen":
				e.pushString(formatPt(e.dimen[e.scanInt()]))
				return
			case m.kind == mDimenRef:
				e.pushString(formatPt(e.dimen[m.code]))
				return
			case m.kind == mPrim && m.name == "skip":
				e.pushString(formatGlue(e.skip[e.scanInt()]))
				return
			case m.kind == mPrim && m.name == "wd":
				e.pushString(formatPt(e.boxDim('w')))
				return
			case m.kind == mPrim && m.name == "ht":
				e.pushString(formatPt(e.boxDim('h')))
				return
			case m.kind == mPrim && m.name == "dp":
				e.pushString(formatPt(e.boxDim('d')))
				return
			case m.kind == mPrim && m.name == "hsize":
				e.pushString(formatPt(e.hsize))
				return
			case m.kind == mPrim && m.name == "vsize":
				e.pushString(formatPt(e.vsize))
				return
			case m.kind == mPrim && m.name == "parindent":
				e.pushString(formatPt(e.parindent))
				return
			case m.kind == mPrim && m.name == "baselineskip":
				e.pushString(formatPt(e.baselineskip))
				return
			case m.kind == mPrim && m.name == "leftskip":
				e.pushString(formatGlue(e.leftskip))
				return
			case m.kind == mPrim && m.name == "rightskip":
				e.pushString(formatGlue(e.rightskip))
				return
			case m.kind == mSkipRef:
				e.pushString(formatGlue(e.skip[m.code]))
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
	e.prim("par", func(e *Engine) { e.endParagraph() })
	e.prim("halign", func(e *Engine) { e.doHalign() })
	e.prim("patterns", func(e *Engine) { e.doPatterns() })
	e.prim("documentclass", func(e *Engine) { e.doGobbleOptAndGroup() })
	e.prim("usepackage", func(e *Engine) { e.doUsepackage() })
	e.prim("[", func(e *Engine) { e.doDelimitedMath("]", true) })   // \[ … \] display math
	e.prim("(", func(e *Engine) { e.doDelimitedMath(")", false) })  // \( … \) inline math
	e.prim("]", func(e *Engine) {})                                 // consumed by \[
	e.prim(")", func(e *Engine) {})                                 // consumed by \(
	e.prim("@equationbody", func(e *Engine) { e.doEquationBody() }) // \begin{equation} body + number
	// amsmath multi-line displays: align/eqnarray/gather/multline and starred forms.
	e.prim("align", func(e *Engine) { e.doAlignEnv("align", true, alignPairs) })
	e.prim("align*", func(e *Engine) { e.doAlignEnv("align*", false, alignPairs) })
	e.prim("eqnarray", func(e *Engine) { e.doAlignEnv("eqnarray", true, eqnarrayCols) })
	e.prim("eqnarray*", func(e *Engine) { e.doAlignEnv("eqnarray*", false, eqnarrayCols) })
	e.prim("gather", func(e *Engine) { e.doAlignEnv("gather", true, nil) })
	e.prim("gather*", func(e *Engine) { e.doAlignEnv("gather*", false, nil) })
	e.prim("multline", func(e *Engine) { e.doMultline("multline", true) })
	e.prim("multline*", func(e *Engine) { e.doMultline("multline*", false) })
	for _, n := range []string{"endalign", "endalign*", "endeqnarray", "endeqnarray*",
		"endgather", "endgather*", "endmultline", "endmultline*"} {
		e.prim(n, func(e *Engine) {})
	}
	e.prim("newcommand", func(e *Engine) { e.doNewcommand() })
	e.prim("renewcommand", func(e *Engine) { e.doNewcommand() })
	e.prim("providecommand", func(e *Engine) { e.doNewcommand() })
	e.prim("newenvironment", func(e *Engine) { e.doNewenvironment() })
	e.prim("renewenvironment", func(e *Engine) { e.doNewenvironment() })
	e.prim("rule", func(e *Engine) { e.place(e.doRuleNode()) })
	e.prim("parbox", func(e *Engine) { e.place(e.doParbox()) })
	e.prim("fbox", func(e *Engine) { e.place(e.doFbox()) })
	e.prim("framebox", func(e *Engine) { e.place(e.doFramebox()) })
	// Text decorations: a rule under (\underline), through (\sout) or over the text.
	e.prim("underline", func(e *Engine) { e.place(e.makeDeco('u')) })
	e.prim("sout", func(e *Engine) { e.place(e.makeDeco('s')) })
	e.prim("textoverline", func(e *Engine) { e.place(e.makeDeco('o')) })
	// Phantom and smash boxes: reserve or suppress a box's dimensions.
	e.prim("phantom", func(e *Engine) { e.place(e.makePhantom(phantomFull)) })
	e.prim("hphantom", func(e *Engine) { e.place(e.makePhantom(phantomH)) })
	e.prim("vphantom", func(e *Engine) { e.place(e.makePhantom(phantomV)) })
	e.prim("smash", func(e *Engine) { e.place(e.makeSmash()) })
	// Page style and numbering (see pagenum.go): a centred foot number per page.
	e.prim("pagestyle", func(e *Engine) { e.doPagestyle() })
	e.prim("thispagestyle", func(e *Engine) { e.doPagestyle() })
	e.prim("pagenumbering", func(e *Engine) { e.doPagenumbering() })
	e.prim("today", func(e *Engine) { e.pushString(e.today) })
	// \thepage in running text is best-effort (this single-pass engine cannot know
	// the page in advance); in a header/footer field it reflects the real page
	// (curPageNum set during assembly), and the per-page foot number is always correct.
	e.prim("thepage", func(e *Engine) { e.pushString(formatPageNumber(e.pageOrdinal(), e.pageNumStyle)) })
	// fancyhdr running headers/footers (see fancyhdr.go).
	e.prim("lhead", func(e *Engine) { e.setFancyField(fldHL) })
	e.prim("chead", func(e *Engine) { e.setFancyField(fldHC) })
	e.prim("rhead", func(e *Engine) { e.setFancyField(fldHR) })
	e.prim("lfoot", func(e *Engine) { e.setFancyField(fldFL) })
	e.prim("cfoot", func(e *Engine) { e.setFancyField(fldFC) })
	e.prim("rfoot", func(e *Engine) { e.setFancyField(fldFR) })
	e.prim("fancyhead", func(e *Engine) { e.doFancyPos(fldHL) })
	e.prim("fancyfoot", func(e *Engine) { e.doFancyPos(fldFL) })
	e.prim("fancyhf", func(e *Engine) { e.doFancyhf() })
	// setspace line spacing (see setspace.go).
	e.prim("singlespacing", func(e *Engine) { e.setLineStretch(1) })
	e.prim("onehalfspacing", func(e *Engine) { e.setLineStretch(1.5) })
	e.prim("doublespacing", func(e *Engine) { e.setLineStretch(2) })
	e.prim("setstretch", func(e *Engine) { e.doSetstretch() })
	e.prim("linespread", func(e *Engine) { e.doSetstretch() })
	e.prim("spacing", func(e *Engine) { e.doSpacing() })
	e.prim("endspacing", func(e *Engine) { e.endSpacing() })
	// graphicx box transformations: scale, mirror, resize and rotate the content,
	// which the SVG/PDF drivers realise with native affine transforms.
	e.prim("scalebox", func(e *Engine) { e.place(e.doScalebox()) })
	e.prim("reflectbox", func(e *Engine) { e.place(e.doReflectbox()) })
	e.prim("resizebox", func(e *Engine) { e.place(e.doResizebox()) })
	e.prim("rotatebox", func(e *Engine) { e.place(e.doRotatebox()) })
	e.prim("color", func(e *Engine) { e.doColor() })
	e.prim("definecolor", func(e *Engine) { e.doDefineColor() })
	e.prim("colorlet", func(e *Engine) { e.doColorlet() })
	e.prim("pagecolor", func(e *Engine) { e.doPagecolor() })
	e.prim("nopagecolor", func(e *Engine) { e.hasPageColor = false })
	e.prim("normalcolor", func(e *Engine) { e.doNormalcolor() })
	e.prim("colorbox", func(e *Engine) { e.place(e.doColorbox()) })
	e.prim("fcolorbox", func(e *Engine) { e.place(e.doFcolorbox()) })
	e.prim("label", func(e *Engine) { e.doLabel() })
	e.prim("ref", func(e *Engine) { e.doRef() })
	e.prim("pageref", func(e *Engine) { e.doRef() }) // page numbers not modelled; reuse the ref text
	e.prim("eqref", func(e *Engine) { e.doEqref() })
	e.prim("cite", func(e *Engine) { e.doCite() })
	e.prim("@tocentry", func(e *Engine) { e.doTOCEntry() })
	e.prim("tableofcontents", func(e *Engine) { e.doTableOfContents() })
	e.prim("listoffigures", func(e *Engine) { e.doListOfFigures() })
	e.prim("listoftables", func(e *Engine) { e.doListOfTables() })
	// index mechanism (see index.go): \makeindex enables collection, \index
	// records an entry (a no-op until \makeindex), \printindex typesets the sorted
	// index; carried through the two-pass beside the TOC entries.
	e.prim("makeindex", func(e *Engine) { e.doMakeIndex() })
	e.prim("index", func(e *Engine) { e.doIndex() })
	e.prim("printindex", func(e *Engine) { e.doPrintIndex() })
	e.prim("@discardopt", func(e *Engine) { e.scanOptBracketToks() }) // eat an optional [placement] (figure/table)
	// \@ifnextbracket{THEN}{ELSE}: a \@ifnextchar[ built without \futurelet (which
	// this kernel lacks). It grabs two brace groups, then peeks — without
	// expanding — at the next non-space token. When that token is a '[', the THEN
	// tokens are pushed back (a delimited macro there can then read the optional
	// [label]); otherwise the ELSE tokens are pushed. The peeked token is always
	// put back, so no input is consumed.
	e.prim("@ifnextbracket", func(e *Engine) {
		thenToks := e.grabUndelimited()
		elseToks := e.grabUndelimited()
		e.skipOptSpace()
		chosen := elseToks
		if t, ok := e.getNext(); ok {
			if !t.cs_ && t.ch == '[' {
				chosen = thenToks
			}
			e.back(t)
		}
		e.push(chosen)
	})
	e.prim("verbatim", func(e *Engine) { e.doVerbatim() })
	e.prim("endverbatim", func(e *Engine) {}) // consumed literally by doVerbatim; defined for safety
	e.prim("verb", func(e *Engine) { e.doVerb() })
	e.prim("url", func(e *Engine) { e.doURL() })                 // hyperref: literal, clickable URL
	e.prim("href", func(e *Engine) { e.doHref() })               // hyperref: text clickable to a URL
	e.prim("nolinkurl", func(e *Engine) { e.doNolinkurl() })     // hyperref: literal URL, no link
	e.prim("hypertarget", func(e *Engine) { e.doHypertarget() }) // hyperref: named in-document destination
	e.prim("hyperlink", func(e *Engine) { e.doHyperlink() })     // hyperref: same-document link to a target
	e.prim("footnote", func(e *Engine) { e.doFootnote() })
	e.prim("gotexsize", func(e *Engine) { e.doFontSize() }) // \gotexsize<permille>: scale the base font
	e.prim("includegraphics", func(e *Engine) { e.doIncludegraphics() })
	e.prim("graphicspath", func(e *Engine) { e.grabUndelimited() }) // {dir} search path — accepted, not modelled
	// BibTeX bibliography (see bibtex.go): \nocite records keys, \citep/\citet are
	// natbib's variants, \bibliographystyle is accepted, and \bibliography reads the
	// .bib file and emits a thebibliography list.
	e.prim("nocite", func(e *Engine) { e.doNocite() })                       // record keys (or * = all) for \bibliography
	e.prim("citep", func(e *Engine) { e.doCitep() })                         // natbib: "[n]"
	e.prim("citet", func(e *Engine) { e.doCitet() })                         // natbib: "Author [n]"
	e.prim("bibliographystyle", func(e *Engine) { e.doBibliographyStyle() }) // accepted; only plain modelled
	e.prim("bibliography", func(e *Engine) { e.doBibliography() })           // read .bib, emit thebibliography
	e.prim("tabular", func(e *Engine) { e.doTabular() })
	e.prim("endtabular", func(e *Engine) {}) // consumed by doTabular; defined for safety
	e.prim("tabularx", func(e *Engine) { e.doTabularx() })
	e.prim("endtabularx", func(e *Engine) {}) // consumed by doTabularx; defined for safety
	e.prim("minipage", func(e *Engine) { e.doMinipage() })
	e.prim("endminipage", func(e *Engine) {})                          // consumed by doMinipage; defined for safety
	e.prim("char", func(e *Engine) { e.startChar(rune(e.scanInt())) }) // typeset a glyph by code
	for _, acc := range []string{"'", "`", "^", "\"", "~", "=", ".", "u", "v", "H", "r", "c", "k", "b", "d"} {
		a := acc
		e.prim(a, func(e *Engine) { e.doAccent(a) }) // accent commands: \'e → é, \c c → ç, …
	}
	e.prim(" ", func(e *Engine) { // control space: an explicit interword space
		if e.curFont != nil {
			e.placeHGlue(e.curFont.spaceSP())
		}
	})
	e.prim("cr", func(e *Engine) {})   // recognised structurally by \halign
	e.prim("crcr", func(e *Engine) {}) //  "
	e.prim("font", func(e *Engine) { e.doFont() })
	e.prim("input", func(e *Engine) { e.doInput() })
	e.prim("hsize", func(e *Engine) { e.scanEquals(); e.hsize = e.scanDimen() })
	e.prim("vsize", func(e *Engine) { e.scanEquals(); e.vsize = e.scanDimen() })
	e.prim("baselineskip", func(e *Engine) { e.scanEquals(); e.baselineskip = e.scanGlue().width })
	e.prim("leftskip", func(e *Engine) {
		e.scanEquals()
		g := e.scanGlue()
		if len(e.groups) > 0 {
			e.save = append(e.save, saveItem{kind: 6, oldg: e.leftskip})
		}
		e.leftskip = g
	})
	e.prim("rightskip", func(e *Engine) {
		e.scanEquals()
		g := e.scanGlue()
		if len(e.groups) > 0 {
			e.save = append(e.save, saveItem{kind: 7, oldg: e.rightskip})
		}
		e.rightskip = g
	})
	e.prim("parindent", func(e *Engine) { e.scanEquals(); e.parindent = e.scanDimen() })
	e.prim("indent", func(e *Engine) {
		if !e.inPar {
			e.beginParagraph(true)
		} else {
			e.parList = append(e.parList, &boxNode{kind: hbox, width: e.parindent})
		}
	})
	e.prim("noindent", func(e *Engine) {
		if !e.inPar {
			e.beginParagraph(false)
		}
	})
	e.prim("newtheorem", func(e *Engine) { e.doNewtheorem() })    // amsthm \newtheorem{env}{Heading}[within]/[shared]
	e.prim("newtheorem*", func(e *Engine) { e.doNewtheorem() })   // starred form: same here (number is still generated)
	e.prim("theoremstyle", func(e *Engine) { e.readBraceName() }) // style selector accepted; only "plain" is modelled
	// LaTeX counter interface. Counters are \count registers aliased \c@<name>;
	// these mutate them (see counters.go). \@Roman is an expandable helper (an
	// uppercase \romannumeral) used by the \Roman formatting macro in latex.go.
	e.prim("newcounter", func(e *Engine) { e.doNewcounter() })
	e.prim("setcounter", func(e *Engine) { e.doSetcounter() })
	e.prim("addtocounter", func(e *Engine) { e.doAddtocounter() })
	e.prim("stepcounter", func(e *Engine) { e.doStepcounter() })
	e.prim("refstepcounter", func(e *Engine) { e.doRefstepcounter() })
	e.prim("@Roman", func(e *Engine) { e.pushString(strings.ToUpper(roman(e.scanInt()))) })
	expandableSet["@Roman"] = true
	// LaTeX length interface: \newlength allocates a skip register (a rubber
	// length) and aliases the given cs to it; \setlength/\addtolength assign or
	// advance it (or a \newdimen register / engine parameter); \settoX measure
	// content typeset as an hbox. See lengths.go.
	e.prim("newlength", func(e *Engine) { e.doNewlength() })
	e.prim("setlength", func(e *Engine) { e.doSetlength(false) })
	e.prim("addtolength", func(e *Engine) { e.doSetlength(true) })
	e.prim("settowidth", func(e *Engine) { e.doSettodim('w') })
	e.prim("settoheight", func(e *Engine) { e.doSettodim('h') })
	e.prim("settodepth", func(e *Engine) { e.doSettodim('d') })
	// Typed cross-references (see typedrefs.go): hyperref \autoref/\nameref and
	// cleveref \cref/\Cref, which print the reference type together with the number.
	e.prim("autoref", func(e *Engine) { e.doAutoref() })
	e.prim("nameref", func(e *Engine) { e.doNameref() })
	e.prim("cref", func(e *Engine) { e.doCref(false) })
	e.prim("Cref", func(e *Engine) { e.doCref(true) })
	// listings package: code blocks and inline verbatim (see listings.go). Reached
	// via \begin{lstlisting}/\end{lstlisting} (endlstlisting is consumed literally by
	// doLstlisting, defined here for safety) and \lstinline<delim>…<delim>.
	e.prim("lstlisting", func(e *Engine) { e.doLstlisting() })
	e.prim("endlstlisting", func(e *Engine) {})
	e.prim("lstinline", func(e *Engine) { e.doLstinline() })
	// geometry package: page size and margins driving \hsize/\vsize and the render
	// margin (see geometry.go). \usepackage[..]{geometry} is intercepted in
	// doUsepackage; \geometry{..} re-applies at any point.
	e.prim("geometry", func(e *Engine) { e.doGeometry() })
	e.loadStomach()
}

// loadStomach registers the box-building primitives. When invoked standalone (in
// the main vertical list, which this milestone does not yet assemble) box/glue/
// kern producers scan and discard their argument to keep the parser in sync; the
// real work happens when they are reached inside \setbox via scanBox/buildBoxList.
func (e *Engine) loadStomach() {
	e.prim("setbox", func(e *Engine) { e.doSetbox() })
	// At top level (vertical mode) box/rule/glue producers contribute to the main
	// vertical list; inside \setbox they are reached via scanBox/buildBoxList.
	e.prim("hbox", func(e *Engine) { e.place(e.makeBox(hbox)) })
	e.prim("vbox", func(e *Engine) { e.place(e.makeBox(vbox)) })
	e.prim("vtop", func(e *Engine) { e.place(e.makeVtop()) })
	e.prim("vsplit", func(e *Engine) { e.place(e.doVsplit()) })
	e.prim("box", func(e *Engine) {
		i := e.scanInt()
		e.place(e.getBox(i))
		e.setBox(i, nil) // \box empties the register
	})
	e.prim("copy", func(e *Engine) { e.place(cloneBox(e.getBox(e.scanInt()))) })
	e.prim("kern", func(e *Engine) { e.place(kernNode{width: e.scanDimen()}) })
	e.prim("vskip", func(e *Engine) { e.contribute(glueNode{spec: e.scanGlue()}) })
	e.prim("hskip", func(e *Engine) { e.placeHGlue(e.scanGlue()) })
	e.prim("hfil", func(e *Engine) { e.placeHGlue(glueSpec{stretch: unity, stretchOrder: 1}) })
	e.prim("hfill", func(e *Engine) { e.placeHGlue(glueSpec{stretch: unity, stretchOrder: 2}) })
	e.prim("hss", func(e *Engine) {
		e.placeHGlue(glueSpec{stretch: unity, stretchOrder: 1, shrink: unity, shrinkOrder: 1})
	})
	for _, f := range []string{"vfil", "vfill", "vss"} {
		e.prim(f, func(e *Engine) {})
	}
	// LaTeX spacing commands. \hspace{d}/\hspace*{d} put fixed horizontal glue of
	// width d on the line; \vspace{d}/\vspace*{d} put fixed vertical glue on the
	// page. \hrulefill and \dotfill are fill glue (order 2) rendered as a rule or a
	// row of dots. The star is accepted but not distinguished (both variants space
	// the same here). Each also has a boxNodeFor case so it works inside an \hbox.
	e.prim("hspace", func(e *Engine) {
		e.scanOptStar()
		e.placeHGlue(glueSpec{width: e.readBraceDimen()})
	})
	e.prim("vspace", func(e *Engine) {
		e.scanOptStar()
		e.contribute(glueNode{spec: glueSpec{width: e.readBraceDimen()}})
	})
	e.prim("hrulefill", func(e *Engine) {
		e.placeHGlueNode(glueNode{spec: fillGlue(), leader: leaderRule})
	})
	e.prim("dotfill", func(e *Engine) {
		e.placeHGlueNode(glueNode{spec: fillGlue(), leader: leaderDots})
	})
	e.prim("@ifstar", func(e *Engine) { e.doIfstar() })
	e.prim("hrule", func(e *Engine) { e.contribute(e.scanRule(true)) })
	e.prim("vrule", func(e *Engine) { e.place(e.scanRule(false)) })
	e.prim("penalty", func(e *Engine) { e.place(penaltyNode{penalty: e.scanInt()}) })
	// Box shifting: \raise/\lower move an hbox/vbox off the baseline (horizontal
	// mode); \moveleft/\moveright shift a box horizontally (vertical mode). TeX's
	// shift_amount is positive downward, so \raise and \moveleft negate it.
	e.prim("raise", func(e *Engine) { d := e.scanDimen(); e.shiftAndPlace(-d, false) })
	e.prim("lower", func(e *Engine) { d := e.scanDimen(); e.shiftAndPlace(d, false) })
	e.prim("moveleft", func(e *Engine) { d := e.scanDimen(); e.shiftAndPlace(-d, true) })
	e.prim("moveright", func(e *Engine) { d := e.scanDimen(); e.shiftAndPlace(d, true) })
	e.prim("wd", func(e *Engine) { e.boxDimAssign('w') })
	e.prim("ht", func(e *Engine) { e.boxDimAssign('h') })
	e.prim("dp", func(e *Engine) { e.boxDimAssign('d') })
	// multicols: the environment, its \end sentinel, and its two length parameters.
	e.prim("multicols", func(e *Engine) { e.doMulticols() })
	e.prim("endmulticols", func(e *Engine) {}) // consumed by doMulticols; defined for safety
	e.prim("columnsep", func(e *Engine) { e.scanEquals(); e.columnsep = e.scanDimen() })
	e.prim("columnseprule", func(e *Engine) { e.scanEquals(); e.columnseprule = e.scanDimen() })
	e.loadBoxCmds()
}

// shiftAndPlace reads a box, applies a shift amount, and places it: vertical=true
// (\moveleft/\moveright) contributes to the vertical list; vertical=false
// (\raise/\lower) places it inline/vertically per the current mode.
func (e *Engine) shiftAndPlace(shift int, vertical bool) {
	b := e.scanShiftedBox(shift)
	if b == nil {
		return
	}
	if vertical {
		e.contribute(b)
	} else {
		e.place(b)
	}
}

// place adds material that is legal in both modes: inside a paragraph
// (horizontal mode) it becomes an inline node on the current line; in vertical
// mode it is contributed to the main vertical list. A nil box is dropped.
func (e *Engine) place(n node) {
	if b, ok := n.(*boxNode); ok && b == nil {
		return
	}
	if e.inPar {
		e.parList = append(e.parList, n)
		return
	}
	e.contribute(n)
}

// placeHGlue adds horizontal glue: inside a paragraph it joins the line; in
// vertical mode it starts a paragraph (an \hskip in vertical mode begins one).
func (e *Engine) placeHGlue(g glueSpec) { e.placeHGlueNode(glueNode{spec: g}) }

// placeHGlueNode is placeHGlue for a fully-formed glue node (so a leader flag
// survives), sharing the "an \hskip in vertical mode begins a paragraph" rule.
func (e *Engine) placeHGlueNode(n glueNode) {
	if !e.inPar {
		e.beginParagraph(false)
	}
	e.parList = append(e.parList, n)
}

// scanOptStar consumes an optional '*' after a command (as in \hspace*, \section*)
// and reports whether one was present. A non-star token is backed out unexpanded.
func (e *Engine) scanOptStar() bool {
	e.skipOptSpace()
	if t, ok := e.getNext(); ok {
		if !t.cs_ && t.ch == '*' {
			return true
		}
		e.back(t)
	}
	return false
}

// doIfstar implements LaTeX's \@ifstar#1#2: it grabs the two branch arguments,
// then peeks the next token — if it is a '*' the star is swallowed and #1 is
// pushed for execution, otherwise #2 is. This is what makes \section* work.
func (e *Engine) doIfstar() {
	yes := e.grabUndelimited()
	no := e.grabUndelimited()
	if e.scanOptStar() {
		e.push(yes)
		return
	}
	e.push(no)
}

// contribute adds top-level vertical material to the main vertical list. A
// vertical command first ends any paragraph in progress; boxes route through
// appendToPage (interline glue), a rule resets \prevdepth, other nodes append.
func (e *Engine) contribute(n node) {
	e.endParagraph()
	switch c := n.(type) {
	case *boxNode:
		if c != nil {
			// Under an active alignment (\centering/\flushleft/\flushright set a fil
			// \leftskip/\rightskip), a contributed box is wrapped to \hsize with those
			// skips so it aligns — this is how a tabular inside center gets centred.
			if e.leftskip.stretchOrder > 0 || e.rightskip.stretchOrder > 0 {
				c = hpackSP([]node{glueNode{spec: e.leftskip}, c, glueNode{spec: e.rightskip}}, packTo, e.hsize)
			}
			e.appendToPage(c)
		}
	case ruleNode:
		e.mvl = append(e.mvl, c)
		e.prevDepth = ignoreDepth
	default:
		e.mvl = append(e.mvl, n)
	}
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
