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
	"ifdim":   true,
	"ifmmode": true, "ifhmode": true, "ifvmode": true, "ifinner": true,
	"ifvoid": true, "ifhbox": true, "ifvbox": true,
	// xparse argument tests (see xparse.go) act in the gullet like conditionals.
	"IfNoValueTF": true, "IfNoValueT": true, "IfNoValueF": true,
	"IfValueTF": true, "IfValueT": true, "IfValueF": true,
	"IfBooleanTF": true, "IfBooleanT": true, "IfBooleanF": true,
	// \gotex@checkenv acts in the gullet exactly like \csname (into which \begin
	// feeds), so \begin keeps expanding cleanly wherever it did before.
	"gotex@checkenv": true,
	"gotex@endenv":   true,
}

func isExpandable(name string) bool { return expandableSet[name] }

func isIfPrim(name string) bool {
	if etexIfPrims[name] {
		return true
	}
	switch name {
	case "if", "ifnum", "ifx", "ifodd", "ifcase", "iftrue", "iffalse", "ifcat", "ifdim",
		"ifmmode", "ifhmode", "ifvmode", "ifinner", "ifvoid", "ifhbox", "ifvbox":
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
	e.prim("futurelet", func(e *Engine) { e.doFuturelet(false) })
	// \afterassignment saves one token to be inserted once the next assignment
	// has been carried out (see flushAfterAssignment).
	e.prim("afterassignment", func(e *Engine) {
		if t, ok := e.getNext(); ok {
			e.afterToken = &t
		}
	})
	e.prim("global", func(e *Engine) { e.doGlobal() })
	e.prim("chardef", func(e *Engine) { e.doChardef(false, false) })
	e.prim("mathchardef", func(e *Engine) { e.doChardef(false, true) })
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
	e.prim("begingroup", func(e *Engine) { e.beginGroupKind(semiSimpleGroup) })
	e.prim("endgroup", func(e *Engine) { e.closeSemiSimple() })
	// \bgroup and \egroup are \let to the group-opening/closing characters, so they
	// act as an implicit { and } (TeX §1063). Real classes rely on this, e.g.
	// amsart's \setbox\abstractbox=\vtop\bgroup … \egroup — without it the box never
	// opens and its material (the abstract, and everything after) leaks/vanishes.
	e.define("bgroup", &meaning{kind: mLetChar, ch: '{', cat: catBegin}, true)
	e.define("egroup", &meaning{kind: mLetChar, ch: '}', cat: catEnd}, true)
	e.prim("relax", func(e *Engine) {})
	// beamer hides what an overlay has not reached by wrapping it in pgf's
	// invisibility pair (beamerbaseoverlay.sty:316, \beamer@startcovered). The pair
	// lives in a pgfsys driver this engine does not load, so without these it was
	// undefined — skipped, in lenient mode — and every step of a \pause'd frame came
	// out carrying the WHOLE frame. Covered material keeps its metrics and draws no
	// ink, which is what beamer means by covered: the page must not move as the
	// steps arrive.
	e.prim("pgfsys@begininvisible", func(e *Engine) { e.coveringDepth++ })
	e.prim("pgfsys@endinvisible", func(e *Engine) {
		if e.coveringDepth > 0 {
			e.coveringDepth--
		}
	})
	// \nullfont selects the empty font (see nullfont.go); it is a font switch, so
	// it is scoped by the enclosing group like any other.
	e.eq["nullfont"] = &meaning{kind: mFont, font: nullFont{}, name: "nullfont"}
	e.prim("message", func(e *Engine) { e.doMessage() })
	e.prim("special", func(e *Engine) { e.doSpecial() })

	// expansion
	e.prim("expandafter", func(e *Engine) { e.doExpandafter() })
	e.prim("csname", func(e *Engine) { e.doCsname() })
	e.prim("endcsname", func(e *Engine) {})
	e.prim("noexpand", func(e *Engine) { e.doNoexpand() })
	e.prim("string", func(e *Engine) { e.doString() })
	e.prim("number", func(e *Engine) { e.pushString(strconv.Itoa(e.scanInt())) })
	e.prim("the", func(e *Engine) { e.doThe() })
	e.prim("romannumeral", func(e *Engine) { e.pushString(roman(e.scanInt())) })

	// e-TeX expressions (see etex.go). Executed on their own they contribute
	// their value, as \the would print it; the scanners below read them as
	// internal quantities.
	e.prim("numexpr", func(e *Engine) { e.pushString(strconv.Itoa(e.scanExpr(false))) })
	e.prim("dimexpr", func(e *Engine) { e.pushString(formatPt(e.scanExpr(true))) })

	// conditionals
	e.prim("ifnum", func(e *Engine) { e.doIf(e.scanCond(e.evalIfnum)) })
	e.prim("ifodd", func(e *Engine) { e.doIf(e.scanCond(func() bool { return e.scanInt()%2 != 0 })) })
	e.prim("ifx", func(e *Engine) { e.doIf(e.evalIfx()) })
	e.prim("if", func(e *Engine) { e.doIf(e.scanCond(e.evalIf)) })
	e.prim("iftrue", func(e *Engine) { e.doIf(true) })
	e.prim("iffalse", func(e *Engine) { e.doIf(false) })
	e.prim("ifcase", func(e *Engine) { e.doIfcase() })
	e.prim("else", func(e *Engine) {
		if e.insertRelax("else") {
			return
		}
		e.closeCond()
		e.skipToFi()
	})
	e.prim("fi", func(e *Engine) {
		if !e.insertRelax("fi") {
			e.closeCond()
		}
	})
	e.prim("or", func(e *Engine) {
		if e.insertRelax("or") {
			return
		}
		e.closeCond()
		e.skipToFi()
	})

	// e-TeX's conditionals and expansion primitives — see etexif.go.
	e.loadETeXConditionals()
	e.loadETeXExpansion()

	// \selectfont's engine-side primitive — see font.go.
	e.loadSelectFont()

	// The xparse / LaTeX3 document-command interface — see xparse.go.
	e.loadXparse()

	// TeX's named integer/dimension/glue parameters — see texparams.go.
	e.loadTeXParams()

	// The standard colour names, in the form a colour-reading package expects
	// (see colorbridge.go).
	e.publishNamedColors()

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
	protected := e.pendingProtected
	long := e.pendingLong
	e.pendingProtected, e.pendingLong = false, false
	if name != "" {
		e.define(name, &meaning{kind: mMacro, params: params, body: body, protected: protected, long: long}, global)
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
	if !ok {
		return
	}
	e.assignLetMeaning(name, rhs, global)
}

// assignLetMeaning gives the control sequence `name` the meaning carried by the
// token `rhs` — a copy of another control sequence's meaning, mUndef for an
// undefined one, or a "let character" for a character token. Shared by \let and
// \futurelet.
func (e *Engine) assignLetMeaning(name string, rhs tok, global bool) {
	if name == "" {
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

// doFuturelet implements \futurelet\cs<t1><t2>: it lets \cs take the meaning of
// t2 WITHOUT consuming either token — t1 and t2 are left in the input so the
// next thing processed is t1. This is the one-token lookahead that generic
// LaTeX scanners (\@ifnextchar, elsarticle's \elem@thanksref loop, …) rely on.
func (e *Engine) doFuturelet(global bool) {
	name := e.scanCSName()
	t1, ok1 := e.getNext()
	if !ok1 {
		return
	}
	t2, ok2 := e.getNext()
	if !ok2 {
		e.assignLetMeaning(name, t1, global)
		e.back(t1)
		return
	}
	e.assignLetMeaning(name, t2, global)
	e.push([]tok{t1, t2})
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
	if m == nil {
		return
	}
	// A \global-prefixed assignment through a register alias (\global\topskip42\p@,
	// \global\@tempcnta…). The engine's registers are otherwise unscoped, so the
	// global flag only matters for the \count/\dimen/\skip save stack; either way the
	// value must be consumed so it is not typeset.
	switch m.kind {
	case mCountRef:
		e.countRefAssign(m.code, true)
		return
	case mDimenRef:
		e.dimenRefAssign(m.code, true)
		return
	case mSkipRef:
		e.skipRefAssign(m.code, true)
		return
	case mToksRef:
		e.toksRefAssign(m.code)
		return
	}
	if m.kind != mPrim {
		e.back(t) // best-effort: run it locally (global scope not modelled here)
		return
	}
	switch m.name {
	case "def":
		e.doDef(true, false)
	case "edef":
		e.doDef(true, true)
	case "let":
		e.doLet(true)
	case "futurelet":
		e.doFuturelet(true)
	case "count":
		e.doCountAssign(true)
	case "dimen":
		e.doDimenAssign(true)
	case "advance":
		e.doAdvance(true)
	case "multiply":
		e.doMultiply(true)
	case "divide":
		e.doDivide(true)
	case "chardef":
		e.doChardef(true, false)
	case "mathchardef":
		e.doChardef(true, true)
	case "setbox":
		e.doSetbox(true)
	case "catcode":
		e.doCatcode(true)
	// \global in front of a LaTeX length command. In TeX a prefix applies to the
	// assignment it finds after EXPANDING what follows (tex.web §1211 takes the next
	// non-blank non-relax NON-CALL token), and \setlength is a macro there —
	// ltlength.dtx: \def\setlength#1#2{#1 #2\relax} — so \global reaches the register
	// assignment by itself. Here these are primitives, which getXToken does not
	// expand, so the prefix has to be handed to them.
	case "setlength":
		e.doSetlength(false, true)
	case "addtolength":
		e.doSetlength(true, true)
	case "settowidth":
		e.doSettodim('w', true)
	case "settoheight":
		e.doSettodim('h', true)
	case "settodepth":
		e.doSettodim('d', true)
	case "hsize", "textwidth", "vsize", "parindent", "baselineskip":
		// \global\hsize=… : the engine's own dimension parameters are scoped like
		// registers now, so the global flag has to reach the assignment or the
		// enclosing group would put the old value back.
		_, set, _ := e.engineDimenParam(t, true)
		e.scanEquals()
		if m.name == "baselineskip" {
			set(e.scanGlue().width)
		} else {
			set(e.scanDimen())
		}
	default:
		e.back(t) // \global\setbox…, \global\toks…, etc.: run the assignment locally
	}
}

// doChardef handles \chardef\name=<n> and, with math set, \mathchardef\name=<n>.
// Both make \name a constant that reads as the integer <n> wherever a number is
// wanted (\the, \count0=\name, \ifnum, …); they differ in MEANING, so \ifx
// separates a \chardef from a \mathchardef of the same value and \meaning prints
// \char"1F4 against \mathchar"1F4 — checked against real TeX. Packages reach for
// \mathchardef purely to get an integer constant that costs no \count register
// (etoolbox's roman-numeral table, LaTeX's \@M = 10000).
func (e *Engine) doChardef(global, math bool) {
	name := e.scanCSName()
	e.scanEquals()
	code := e.scanInt()
	if name != "" {
		e.define(name, &meaning{kind: mCharDef, code: code, mathChar: math}, global)
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
	// Allocation is GLOBAL: ltplain.dtx's \e@alloc ends with `\global#2#6\allocationnumber`,
	// so \newcount\x inside a group leaves \x a register afterwards.
	e.define(name, &meaning{kind: mCountRef, code: e.allocCnt}, true)
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
	e.define(name, &meaning{kind: mDimenRef, code: e.allocDim}, true) // global, see doNewcount
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
	e.define(name, &meaning{kind: mSkipRef, code: e.allocSkp}, true) // global, see doNewcount
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
		// The engine's own dimension parameters (\hsize and friends) are
		// primitives, not registers, so the register paths below never match them.
		if get, set, isParam := e.engineDimenParam(t, global); isParam {
			e.skipByKeyword()
			set(get() + e.scanDimen())
			return
		}
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

// engineDimenParam matches the dimension parameters the engine models as
// primitives of its own (\hsize and friends) rather than as registers, and gives
// read/write access to the value behind the name. Arithmetic on them —
// \advance\hsize, \divide\textwidth\p@ — has to work like arithmetic on any
// other dimension: a class computes its page geometry that way (LaTeX's size
// option files round every length with \divide…\multiply), and a parameter that
// silently ignores the operation ends up holding nonsense.
func (e *Engine) engineDimenParam(t tok, global bool) (get func() int, set func(int), ok bool) {
	if !t.cs_ {
		return nil, nil, false
	}
	m := e.eq[t.cs]
	if m == nil || m.kind != mPrim {
		return nil, nil, false
	}
	switch m.name {
	case "hsize":
		return func() int { return e.hsize }, func(v int) { e.setEngineDimen(saveHsize, &e.hsize, v, global) }, true
	case "textwidth":
		return func() int { return e.fullWidth() }, func(v int) { e.setTextWidth(v, global) }, true
	case "vsize":
		return func() int { return e.vsize }, func(v int) { e.setEngineDimen(saveVsize, &e.vsize, v, global) }, true
	case "parindent":
		return func() int { return e.parindent }, func(v int) { e.setEngineDimen(saveParindent, &e.parindent, v, global) }, true
	case "baselineskip":
		return func() int { return e.baselineskip }, func(v int) { e.setEngineDimen(saveBaselineskip, &e.baselineskip, v, global) }, true
	}
	return nil, nil, false
}

// The save-stack kinds for the engine's own dimension parameters. They are
// parameters rather than registers, so the register paths do not cover them and
// each needs its own kind (see setEngineDimen).
const (
	saveHsize        = 11
	saveVsize        = 12
	saveParindent    = 13
	saveBaselineskip = 14
)

// setEngineDimen assigns one of the engine's dimension parameters, recording the
// value it displaces so the group that made the assignment undoes it.
//
// TeX restores \hsize, \vsize, \parindent and \baselineskip at the end of a group
// exactly as it restores a register: only \global outlives the group. Assigning
// them straight to the field made every such assignment permanent, so a package
// that narrows the measure inside a box — \hbox to\@tempdima{\textwidth=\@tempdima
// …}, which is how beamer's own headline and footline templates are written —
// left that width in force for the rest of the document. A beamer talk came out
// 78pt wide for exactly this reason: the template ran once with a scratch
// dimension of zero and nothing put the real width back.
func (e *Engine) setEngineDimen(kind int, field *int, v int, global bool) {
	if global {
		e.forgetSaved(kind, 0, "")
	} else if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: kind, oldd: *field})
	}
	*field = v
}

// scaleEngineParam performs \multiply/\divide on such a parameter, reading the
// integer operand after the optional "by".
func (e *Engine) scaleEngineParam(t tok, divide, global bool) bool {
	get, set, ok := e.engineDimenParam(t, global)
	if !ok {
		return false
	}
	e.skipByKeyword()
	v := e.scanInt()
	switch {
	case divide && v != 0:
		set(get() / v)
	case !divide:
		set(get() * v)
	}
	return true
}

func (e *Engine) doMultiply(global bool) {
	if t, ok := e.getXToken(); ok {
		if e.scaleEngineParam(t, false, global) {
			return
		}
		e.back(t)
	}
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
		// A name \csname brings into existence means \relax, and that definition
		// is LOCAL like any other: a package asks whether a control sequence
		// exists by expanding \csname inside a group it then closes
		// (\begingroup…\endgroup around \ifx\csname directlua\endcsname\relax),
		// so that the question leaves no trace. Defining it globally would make
		// every such test answer "yes" from then on — and pgf would take its
		// LuaTeX branch on an engine that has no Lua.
		e.define(s, &meaning{kind: mPrim, name: "relax", prim: func(e *Engine) {}}, false)
	}
	e.back(csTok(s))
}

// doCheckEnv implements \gotex@checkenv{env}, the probe \begin runs just before it
// hands the name to \csname. \begin{env} expands to \csname env\endcsname, and an
// UNDEFINED env resolves to \relax there — a silent no-op that never reaches the
// undefined-command tally. This reads the same braced name (without expanding it,
// exactly as \csname builds it from #1's literal characters) and, when \env has no
// meaning yet, records it in undefinedEnvs so Diagnostics can surface the gap. It
// produces no tokens: the following \csname env\endcsname behaves exactly as before,
// so every DEFINED environment (equation, itemize, figure, \newenvironment's, …) is
// untouched. Runs in the gullet (it is in expandableSet), like \csname itself.
func (e *Engine) doCheckEnv() {
	name := e.readBraceName()
	if name == "" {
		return // no braced argument, or an empty one: nothing meaningful to probe
	}
	e.beginGroupKind(semiSimpleGroup) // \begin{env} … \end{env} is a group
	e.setCurrentEnv(name)
	if e.envUndefined(name) {
		if e.undefinedEnvs == nil {
			e.undefinedEnvs = map[string]int{}
		}
		e.undefinedEnvs[name]++
		e.undefinedEnvAsCode(name)
	}
}

// undefinedEnvAsCode rescues the body of an undefined environment that is plainly
// CODE, by setting it verbatim instead of executing it as prose.
//
// An undefined environment's body is typeset with the ordinary category codes,
// which is right for a prose wrapper and ruinous for a code block: a lone $ in
// "path = \"$write_dir/x\"" opens math mode, and the scan then runs past the
// environment's own \end and eats the rest of the DOCUMENT looking for the closing
// $. Measured on one arXiv paper (jlcode, a Julia listings environment the paper
// does not ship): the conclusions, the acknowledgements, the appendix bodies and
// all 71 bibliography entries vanished — half the paper, 15 pages instead of 27.
//
// The test is the imbalance itself: prose does not carry an odd number of math
// shifts, code does. When the body between here and \end{name} has one, it is read
// raw and set as a verbatim block, and the \end is consumed with it. Everything
// else keeps the behaviour it had.
//
// Only when the body is still in the FILE. Inside a captured body (a minipage, a
// float, a beamer column) the character cursor is already past it and reading there
// would copy the document that follows — the mistake #214 fixed. The tell is where
// the \end lives: a captured body carries its own \end in the pending token lists,
// while an ordinary \begin leaves only the tail of its own expansion there.
func pendingHoldsEnd(lists [][]tok) bool {
	for _, l := range lists {
		for _, t := range l {
			if t.cs_ && t.cs == "end" {
				return true
			}
		}
	}
	return false
}

func (e *Engine) undefinedEnvAsCode(name string) {
	if pendingHoldsEnd(e.lists) {
		return
	}
	end := `\end{` + name + `}`
	rest := string(e.base[e.bpos:])
	i := strings.Index(rest, end)
	if i < 0 || strings.Count(rest[:i], "$")%2 == 0 {
		return
	}
	content, line := e.readRawEnvBody(end) // consumes the body AND its \end
	e.renderVerbatimBlock(content, line, lstOptions{})
}

// setCurrentEnv records the environment being opened in \@currenvir, as LaTeX's own
// \begin does. A package reads it to tell which SYNTAX it was invoked with: beamer's
// \frame serves both \begin{frame}…\end{frame} and the command form \frame{…}, and
// picks between them with \ifx\@currenvir\beamer@frametext. Left undefined, that test
// always failed and every \begin{frame} took the command path — which then looked for
// a braced group that was not there and swallowed the frame's body.
//
// It is set HERE, from \begin's Go-side probe, rather than with a \def in \begin's
// body, because the engine's \begin is fully EXPANDABLE and must stay so: a \def in
// its expansion would print as literal text wherever \begin is only expanded.
//
// The body is built with the CURRENT catcodes so \ifx compares equal to the class's
// own \def\beamer@frametext{frame}, whose letters are category 11.
func (e *Engine) setCurrentEnv(name string) {
	body := make([]tok, 0, len(name))
	for _, r := range name {
		body = append(body, chTok(r, e.catcode[r]))
	}
	e.define("@currenvir", &meaning{kind: mMacro, body: body}, false)
}

// doEndEnv implements \gotex@endenv{env}, the probe \end runs after \end<env>: it
// closes the group \begin opened (ltmiscen.dtx ends \end with \endgroup). Like
// \gotex@checkenv it produces no tokens and runs in the gullet, so \end stays
// expandable.
func (e *Engine) doEndEnv() {
	if e.readBraceName() == "" {
		return
	}
	e.endEnvGroup()
}

// endEnvGroup closes the group \begin{env} opened, for an environment the engine
// implements in Go and which therefore swallows its own \end{env} instead of letting
// \end run (see ltmiscen.dtx: \begin ends with \begingroup, \end with \endgroup).
// Without it such an environment leaves a group open for the rest of the document.
//
// It closes whatever the innermost group is, as \endgroup does — including, through
// closeSemiSimple's "Missing } inserted" recovery, a box. beamer DEPENDS on that: its
// frame is \global\setbox\beamer@framebox=\vbox\bgroup …, and the \end that ends the
// frame meets that box. Guarding this to refuse a box, or to close only the group its
// own \begin opened, costs 140 pages over 200 talks — both were measured.
func (e *Engine) endEnvGroup() { e.closeSemiSimple() }

// envUndefined reports whether \name is not a real environment's opening control
// sequence. It is true when \name has no meaning at all AND when it is \relax:
// \csname coerces a missing \env to \relax the FIRST time \begin{env} runs (and
// that definition persists for the rest of the group), so a second \begin{env}
// would otherwise look "defined". A genuine environment's opening cs is always a
// macro or a primitive, never \relax, so counting the \relax case gives the true
// per-occurrence tally without ever flagging a defined environment.
func (e *Engine) envUndefined(name string) bool {
	m := e.eq[name]
	return m == nil || (m.kind == mPrim && m.name == "relax")
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
		e.pushString(e.escapeStr() + t.cs)
	} else {
		e.pushString(string(t.ch))
	}
}

// escapeStr is what TeX puts in front of a control-sequence NAME when it prints
// one — \string, \meaning, \message of a \the, an error report. It is the
// character \escapechar, NOT a hardwired backslash, and NOTHING AT ALL when
// \escapechar is outside 0..255 (TeX §63).
//
// The engine printed a backslash always, which is wrong wherever code sets
// \escapechar to build a string it will later COMPARE. beamer does exactly that:
//
//	\begingroup \escapechar=-1 \xdef\beamer@stopmode{\string\\mode} \endgroup
//
// It wants the six characters "\mode" to recognise that line when it reads the
// document verbatim; it got "\\mode" instead, so \beamer@processline — the reader
// that skips material outside the current mode — never recognised its stop line
// and LOOPED, eating the rest of the file and the whole document after it. That is
// why a real beamer talk rendered zero pages while the emulation rendered three.
func (e *Engine) escapeStr() string {
	c := e.escapechar()
	if c < 0 || c > 255 {
		return ""
	}
	return string(rune(c))
}

func (e *Engine) doThe() {
	t, ok := e.getXToken()
	if !ok {
		return
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil {
			switch {
			case m.kind == mPrim && isGroupQuery(m.name):
				v, _ := e.groupQuery(m.name)
				e.pushString(strconv.Itoa(v))
				return
			case m.kind == mPrim && m.name == "numexpr":
				e.pushString(strconv.Itoa(e.scanExpr(false)))
				return
			case m.kind == mPrim && m.name == "dimexpr":
				e.pushString(formatPt(e.scanExpr(true)))
				return
			case m.kind == mPrim && m.name == "lccode":
				e.pushString(strconv.Itoa(int(e.caseOf(rune(e.scanInt()), false))))
				return
			case m.kind == mPrim && m.name == "uccode":
				e.pushString(strconv.Itoa(int(e.caseOf(rune(e.scanInt()), true))))
				return
			case m.kind == mPrim && m.name == "catcode":
				// \the\catcode`\@ — a file that changes a character's category
				// saves the old value this way and restores it when it is done, so
				// reading a catcode matters as much as setting one.
				e.pushString(strconv.Itoa(int(e.catcode[rune(e.scanInt())])))
				return
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
			case m.kind == mPrim && m.name == "textwidth":
				e.pushString(formatPt(e.fullWidth()))
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
			case m.kind == mToksRef:
				e.push(e.theToks(e.toksValue(m.code)))
				return
			case m.kind == mPrim && m.name == "toks":
				e.push(e.theToks(e.toksValue(e.scanInt())))
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

// scanCond runs a conditional's operand scan with the "insert \relax" rule armed:
// while it is running, an \else/\fi that belongs to THIS conditional (rather than
// to one opened inside the scan) ends the scan instead of expanding. See
// insertRelax.
//
// It also records how many conditionals the scan LEFT OPEN. Expanding an operand
// can run a conditional of its own — \if u\pos where \pos is itself
// \if 3\num u\else l\fi, which is how pgfplots asks which side an axis label
// goes on — and that inner one's \fi is still in the input when this conditional
// comes to skip its own false branch. Mistaking it for one's own ends the skip at
// the wrong place: the outer conditional then executes the text that follows,
// which is the true branch it had just decided against.
func (e *Engine) scanCond(f func() bool) bool {
	mark := e.condOpen
	e.condMarks = append(e.condMarks, mark)
	defer func() { e.condMarks = e.condMarks[:len(e.condMarks)-1] }()
	r := f()
	// A scan can also CLOSE a conditional that was already open, by expanding
	// a \fi that belongs to it; nothing is pending then, and max keeps the count
	// from going negative without a branch no input can reach.
	e.condPending = max(0, e.condOpen-mark)
	return r
}

func (e *Engine) doIf(cond bool) {
	// Conditionals the operand scan left open: their \fi's come before this
	// one's, and the skip below has to let them past. Read once and cleared, so
	// a conditional that scans no operand (\iftrue) is unaffected.
	pending := e.condPending
	e.condPending = 0
	// \unless (e-TeX) reverses the sense of the conditional it prefixes.
	if e.negateNextIf > 0 {
		e.negateNextIf--
		cond = !cond
	}
	if cond {
		e.condOpen++ // its \else or \fi is still to come
		return       // execute the true branch; \else/\fi handle the rest
	}
	if e.skipToElseOrFiPast(pending) == "else" {
		e.condOpen++ // the \fi that closes the else branch is still to come
		return       // execute the else branch
	}
}

// skipToElseOrFiPast skips a false branch, letting the \else/\fi of `pending`
// conditionals opened by the operand scan go by first.
func (e *Engine) skipToElseOrFiPast(pending int) string {
	depth := 0
	for {
		t, ok := e.getNext()
		if !ok {
			return "fi"
		}
		if t.cs_ && t.cs == sentinel.cs {
			// The end of an isolated expansion (expandList). A conditional opened
			// inside it cannot be closed by material OUTSIDE it: TeX would read on to
			// the end of the file, but here that file is the caller's own pending
			// input, and skipping into it consumes the document. Put the sentinel
			// back so the expansion ends where it was told to.
			e.back(t)
			return "fi"
		}
		if !t.cs_ {
			continue
		}
		m := e.eq[t.cs]
		if m == nil || m.kind != mPrim {
			continue
		}
		switch {
		case isIfPrim(m.name):
			depth++
		case m.name == "fi":
			if depth > 0 {
				depth--
				continue
			}
			if pending > 0 {
				pending--
				if e.condOpen > 0 {
					e.condOpen--
				}
				continue
			}
			return "fi"
		case m.name == "else" && depth == 0:
			if pending > 0 {
				continue // it closes an inner conditional's true branch
			}
			return "else"
		}
	}
}

// closeCond records that one conditional's \else/\fi has been reached.
func (e *Engine) closeCond() {
	if e.condOpen > 0 {
		e.condOpen--
	}
}

func (e *Engine) doIfcase() {
	var n int
	e.scanCond(func() bool { n = e.scanInt(); return false })
	for n > 0 {
		r := e.skipToElseOrFiOrOr()
		if r == "fi" {
			return
		}
		if r == "else" {
			e.condOpen++
			return
		}
		n--
	}
	e.condOpen++ // the \or/\else/\fi that ends this case is still to come
}

// insertRelax implements TeX's "insert \relax" rule (tex.web §510). While a
// conditional is still SCANNING its operands, an \else / \fi / \or belongs to
// that conditional and not to the number being read, so TeX puts a \relax in
// front of it — the \relax ends the number scan, and the \else is read again
// afterwards, when the conditional has been evaluated.
//
// The idiom that needs this is the LaTeX kernel's own date comparison,
//
//	\ifnum<a><<b>\expandafter\@secondoftwo\else\expandafter\@firstoftwo\fi
//
// with nothing between the second number and \expandafter. Without the rule the
// number scan expands \expandafter, which expands \else, and the conditional
// unravels: \@ifl@t@r answered NOTHING for two equal dates, and every package
// that asks \IfFormatAtLeastTF took neither branch.
//
// It returns true when it fired, meaning the caller must not act on the token.
func (e *Engine) insertRelax(name string) bool {
	if len(e.condMarks) == 0 || e.condOpen != e.condMarks[len(e.condMarks)-1] {
		return false // no scan in progress, or this \else closes a NESTED conditional
	}
	e.push([]tok{csTok("relax"), csTok(name)})
	return true
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
	return e.ifxEqual(a, b)
}

// ifxEqual reports whether two tokens are \ifx-equal: same character+catcode for
// plain characters, or equal meanings for control sequences / active chars.
func (e *Engine) ifxEqual(a, b tok) bool {
	ma, mb := e.ifxMeaning(a), e.ifxMeaning(b)
	if ma == nil || mb == nil {
		// Only an undefined control sequence has no meaning at all, and they all
		// share the same one — "undefined" — so two of them are equal whatever
		// they are called. That is what makes \ifx\foo\undefined the standard way
		// to ask whether \foo exists, an idiom nearly every package is built on.
		return ma == nil && mb == nil
	}
	return meaningEq(ma, mb)
}

// ifxMeaning is what \ifx compares: the meaning of a token. A character token
// carries one too — its category and character — which is the same meaning a
// control sequence \let to that character has. So \ifx\next/ is true when \next
// was \let to a slash, the way every one-token-lookahead scanner tests what it
// peeked at (LaTeX's \@ifnextchar family, pgfkeys' path splitting, …). nil means
// undefined, which only a control sequence can be.
func (e *Engine) ifxMeaning(t tok) *meaning {
	if !t.cs_ {
		return &meaning{kind: mLetChar, ch: t.ch, cat: t.cat}
	}
	return e.meaningOf(t)
}

func (e *Engine) evalIf() bool {
	a, _ := e.getXToken()
	b, _ := e.getXToken()
	ca, _ := e.ifCharOf(a)
	cb, _ := e.ifCharOf(b)
	return ca == cb
}

// ifCharOf resolves the character a token stands for in \if and \ifcat (TeX
// §506). A control sequence \let to a CHARACTER behaves exactly as that
// character — that is the whole point of \futurelet, which \let's a lookahead
// control sequence to the token it peeked at and then asks \ifcat what it was.
// Every OTHER control sequence — a macro, a primitive, a \chardef'd constant, an
// undefined name — is character code 256, category 16, which is why two of them
// always compare equal to each other and never to a real character.
//
// The engine treated every control sequence that way, so \ifcat\next a was false
// for a \next \let to a letter. beamer decides whether a \mode<…> spec is a MODE
// NAME or an overlay range with exactly that test: it always answered "overlay",
// every \mode<presentation> concluded the mode did not apply, and the class
// silently commented out the rest of the document.
//
// The \chardef case is not an oversight: real TeX answers OTHER for
// \ifcat\chardefd a and DIFF for \if\chardefd A. Checked against it.
func (e *Engine) ifCharOf(t tok) (rune, cat) {
	if !t.cs_ {
		return t.ch, t.cat
	}
	if m := e.eq[t.cs]; m != nil && m.kind == mLetChar {
		return m.ch, m.cat
	}
	return 256, 16
}

func meaningEq(a, b *meaning) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case mPrim:
		return a.name == b.name
	case mCharDef:
		return a.code == b.code && a.mathChar == b.mathChar
	case mLetChar:
		return a.ch == b.ch && a.cat == b.cat
	case mMacro:
		// The prefixes are part of the meaning — tex.web keeps them in the macro's
		// own command code (§4209-4212: call / long_call) — so \protected\def\p{P}
		// and \def\q{P} are NOT \ifx-equal, and neither are \long\def\a{A} and
		// \def\b{A}. Measured against real TeX, which answers DIFF to both.
		return a.protected == b.protected && a.long == b.long &&
			sameToks(a.params, b.params) && sameToks(a.body, b.body)
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
		if t.cs_ && t.cs == sentinel.cs {
			// The end of an isolated expansion (expandList). A conditional opened
			// inside it cannot be closed by material OUTSIDE it: TeX would read on to
			// the end of the file, but here that file is the caller's own pending
			// input, and skipping into it consumes the document. Put the sentinel
			// back so the expansion ends where it was told to.
			e.back(t)
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
		if t.cs_ && t.cs == sentinel.cs {
			// The end of an isolated expansion (expandList). A conditional opened
			// inside it cannot be closed by material OUTSIDE it: TeX would read on to
			// the end of the file, but here that file is the caller's own pending
			// input, and skipping into it consumes the document. Put the sentinel
			// back so the expansion ends where it was told to.
			e.back(t)
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
		if t.cs_ && t.cs == sentinel.cs {
			// The end of an isolated expansion (expandList). A conditional opened
			// inside it cannot be closed by material OUTSIDE it: TeX would read on to
			// the end of the file, but here that file is the caller's own pending
			// input, and skipping into it consumes the document. Put the sentinel
			// back so the expansion ends where it was told to.
			e.back(t)
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
	raw := e.grabGroup()
	// \message/\typeout typeset a "moving argument": \protect must shield the
	// following control sequence from expansion (LaTeX does \let\protect\string
	// here). Without it, a self-referential warning such as ieeeconf's
	//   \def\@IEEEdestroythesectionargument#1{\typeout{… \protect\section …}}
	// re-expands \section — itself a macro that calls \@IEEEdestroythesectionargument
	// — and loops forever. Shielding it makes \protect\section print literally.
	savedProtect, hadProtect := e.eq["protect"]
	e.eq["protect"] = e.eq["string"]
	b := e.expandList(raw)
	if hadProtect {
		e.eq["protect"] = savedProtect
	} else {
		delete(e.eq, "protect")
	}
	if e.out.Len() > 0 {
		e.out.WriteByte(' ')
	}
	e.out.WriteString(e.toksToString(b))
}

// theToks delivers the value of \the<token variable>. Inside an \edef (or any
// other isolated expansion) those tokens are inserted but NOT expanded again —
// TeX's rule, and the whole reason \the\toks exists as a way to carry a token
// list through an expansion intact. Marking each token unexpandable-once is
// exactly that guarantee. In ordinary execution the tokens are read normally.
func (e *Engine) theToks(ts []tok) []tok {
	if e.expandDepth == 0 {
		return ts
	}
	out := make([]tok, len(ts))
	for i, t := range ts {
		t.noexp = true
		out[i] = t
	}
	return out
}

// sentinel marks the end of an isolated expansion (getXToken returns it
// literally since it has no meaning, so expandList can stop reliably regardless
// of what is already on the input stack).
var sentinel = tok{cs: "\x00end-expand", cs_: true}

// expandList fully expands a token list in isolation (for \edef / \message),
// stopping at the sentinel rather than by counting input lists.
func (e *Engine) expandList(ts []tok) []tok {
	// The list's own depth on the input stack. An unbalanced conditional inside ts
	// skips forward looking for its \fi and can swallow the sentinel with it; from
	// there the loop would read the CALLER's pending lists — the rest of the
	// document — and consume them into this expansion. Falling below the depth the
	// list was pushed at means exactly that, and stops it. Measured on a paper whose
	// formulas carry class internals: 78% of its glyphs left the page without this.
	base := len(e.lists)
	e.push(append(append([]tok(nil), ts...), sentinel))
	saved := e.noBase
	e.noBase = true
	e.expandDepth++
	var out []tok
	for {
		if len(e.lists) < base+1 {
			break // our own list is gone: stop rather than eat what follows it
		}
		t, ok := e.getXToken()
		if !ok || (t.cs_ && t.cs == sentinel.cs) {
			break
		}
		out = append(out, t)
	}
	e.expandDepth--
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
	e.prim("lccode", func(e *Engine) { e.doCharCode(e.lccode) })
	e.prim("uccode", func(e *Engine) { e.doCharCode(e.uccode) })
	e.prim("uppercase", func(e *Engine) { e.doCase(true) })
	e.prim("lowercase", func(e *Engine) { e.doCase(false) })
	e.prim("ifcat", func(e *Engine) { e.doIf(e.scanCond(e.evalIfcat)) })
	e.prim("meaning", func(e *Engine) { e.doMeaning() })
	expandableSet["meaning"] = true
	// \empty and \space are ordinary macros, defined as in plain TeX.
	e.eq["empty"] = &meaning{kind: mMacro}
	e.eq["space"] = &meaning{kind: mMacro, body: []tok{chTok(' ', catSpace)}}
	// The active ~ is TeX's interword tie: an UNBREAKABLE interword space, and one
	// of the most common tokens in real documents ("Section~\ref", "Figure~2",
	// "et~al.", "Theorem~1"). Plain TeX defines it \def~{\penalty\@M\ } — an
	// infinite (\@M = 10000) break penalty followed by a control space, i.e. an
	// interword glue that no line break may fall on. LaTeX \let~=\nobreakspace to
	// the same effect. Its meaning lives in the active-char slot that meaningOf
	// already consults, so getXToken expands it like any macro; a document may
	// still redefine ~ (scanCSName now names it). Without this the active ~ had no
	// meaning at all: it resolved to nothing and was dropped by the main-loop
	// dispatch, so "Section~1" typeset as "Section1" with the words jammed.
	// LaTeX declares \nobreakspace robust (\DeclareRobustCommand), so ~ is a
	// PROTECTED macro: it acts in the stomach but does NOT expand inside \message /
	// \edef / \write, where an unexpanded active ~ prints as the character "~"
	// (what real TeX shows for \message{~}). Without the flag the tie would expand
	// its \penalty… body into every moving argument that contains a ~.
	e.eq[activeName('~')] = &meaning{kind: mMacro, protected: true, body: []tok{
		csTok("penalty"),
		chTok('1', catOther), chTok('0', catOther), chTok('0', catOther),
		chTok('0', catOther), chTok('0', catOther),
		csTok(" "), // control space: the interword glue
	}}
	e.prim("par", func(e *Engine) { e.suppressParskip = false; e.endParagraph() })
	e.prim("halign", func(e *Engine) { e.doHalign() })
	e.prim("patterns", func(e *Engine) { e.doPatterns() })
	e.prim("documentclass", func(e *Engine) { e.doDocumentClass() })
	e.prim("usepackage", func(e *Engine) { e.doUsepackageLoad() })
	e.prim("LoadClass", func(e *Engine) { e.doLoadClass(false) })
	e.prim("LoadClassWithOptions", func(e *Engine) { e.doLoadClass(true) })
	e.prim("RequirePackageWithOptions", func(e *Engine) { e.doUsepackageLoad() })
	// LaTeX2e option processing (see packages.go), driving real .cls/.sty loading.
	e.prim("DeclareOption", func(e *Engine) { e.doDeclareOption() })
	e.prim("ProcessOptions", func(e *Engine) { e.doProcessOptions() })
	e.prim("ExecuteOptions", func(e *Engine) { e.doExecuteOptions() })
	e.prim("PassOptionsToPackage", func(e *Engine) { e.doPassOptionsTo() })
	e.prim("PassOptionsToClass", func(e *Engine) { e.doPassOptionsTo() })
	e.prim("IfFileExists", func(e *Engine) { e.doIfFileExists() })
	e.prim("InputIfFileExists", func(e *Engine) { e.doInputIfFileExists() })
	e.prim("@gotex@endload", func(e *Engine) { e.endLoad() })
	e.prim("gotexendinput", func(e *Engine) { e.endInput() })
	e.prim("@starttoc", func(e *Engine) { e.doStartTOC() }) // a real class's TOC command bridges to the engine's entry table
	// NFSS size-switch commands a class redefines \normalsize/\small/… to call.
	// The engine has no NFSS, but these MUST consume their arguments: a class body
	// like \renewcommand\normalsize{\@setfontsize\normalsize\@xpt\@xiipt …} would
	// otherwise leave \normalsize in the stream and recurse forever. Gobbling the
	// arguments makes the size switch a no-op (size is left unchanged) without loop.
	// The class's BASE point size (10/11/12pt) reaches the glyphs through the font
	// system instead — setPtsize rescales the bound text faces — not through this
	// token, so a size clo's \@setfontsize stays a pure, expansion-safe gobble.
	e.prim("@setfontsize", func(e *Engine) { e.grabUndelimited(); e.grabUndelimited(); e.grabUndelimited() })
	e.prim("@setsize", func(e *Engine) {
		e.grabUndelimited()
		e.grabUndelimited()
		e.grabUndelimited()
		e.grabUndelimited()
	})
	e.prim("[", func(e *Engine) { e.doDelimitedMath("]", true) })  // \[ … \] display math
	e.prim("(", func(e *Engine) { e.doDelimitedMath(")", false) }) // \( … \) inline math
	e.prim("]", func(e *Engine) {})                                // consumed by \[
	e.prim(")", func(e *Engine) {})                                // consumed by \(
	e.prim("gotex@checkenv", func(e *Engine) { e.doCheckEnv() })
	e.prim("gotex@endenv", func(e *Engine) { e.doEndEnv() })        // \begin's undefined-environment probe (see doCheckEnv)
	e.prim("@equationbody", func(e *Engine) { e.doEquationBody() }) // \begin{equation} body + number
	e.prim("equation*", func(e *Engine) { e.doEquationStar("equation*") })
	e.prim("endequation*", func(e *Engine) {})
	// \begin{displaymath} is the kernel's unnumbered display — the environment form of
	// \[ … \], identical to equation* minus the amsmath \tag. Without it the whole body
	// is skipped and every \frac/\sum/\left inside dropped as an unknown text command.
	e.prim("displaymath", func(e *Engine) { e.doEquationStar("displaymath") })
	e.prim("enddisplaymath", func(e *Engine) {})
	// amsmath multi-line displays: align/eqnarray/gather/multline and starred forms.
	e.prim("align", func(e *Engine) { e.doAlignEnv("align", true, alignPairs) })
	e.prim("align*", func(e *Engine) { e.doAlignEnv("align*", false, alignPairs) })
	e.prim("eqnarray", func(e *Engine) { e.doAlignEnv("eqnarray", true, eqnarrayCols) })
	e.prim("eqnarray*", func(e *Engine) { e.doAlignEnv("eqnarray*", false, eqnarrayCols) })
	e.prim("gather", func(e *Engine) { e.doAlignEnv("gather", true, nil) })
	e.prim("gather*", func(e *Engine) { e.doAlignEnv("gather*", false, nil) })
	e.prim("multline", func(e *Engine) { e.doMultline("multline", true) })
	e.prim("multline*", func(e *Engine) { e.doMultline("multline*", false) })
	// flalign(*) is amsmath's full-width align: the same &-separated column structure
	// as align, spread to fill \textwidth. We reuse align's column model (the content
	// and numbering are identical); without it the whole display is dropped.
	e.prim("flalign", func(e *Engine) { e.doAlignEnv("flalign", true, alignPairs) })
	e.prim("flalign*", func(e *Engine) { e.doAlignEnv("flalign*", false, alignPairs) })
	for _, n := range []string{"endalign", "endalign*", "endeqnarray", "endeqnarray*",
		"endgather", "endgather*", "endmultline", "endmultline*"} {
		e.prim(n, func(e *Engine) {})
	}
	e.prim("newcommand", func(e *Engine) { e.doNewcommand() })
	e.prim("renewcommand", func(e *Engine) { e.doNewcommand() })
	e.prim("providecommand", func(e *Engine) { e.doProvidecommand() })
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
	e.prim("onehalfspacing", func(e *Engine) { e.setLineStretch(onehalfStretch(e.ptsizeCode())) })
	e.prim("doublespacing", func(e *Engine) { e.setLineStretch(doubleStretch(e.ptsizeCode())) })
	e.prim("setstretch", func(e *Engine) { e.doSetstretch() })
	e.prim("linespread", func(e *Engine) { e.doSetstretch() })
	e.prim("spacing", func(e *Engine) { e.doSpacing() })
	e.prim("endspacing", func(e *Engine) { e.endSpacing() })
	// Applied once at \begin{document} (see kernelhelpers.go) to honor a native
	// \renewcommand{\baselinestretch}{f} the way \@setfontsize would.
	e.prim("gotex@applybaselinestretch", func(e *Engine) { e.applyBaselineStretch() })
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
	e.prim("pageref", func(e *Engine) { e.doPageref() })
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
	// put back, so no input is consumed. The chosen branch is halved for the same
	// reason \@ifnextchar's is (see halveParamHashes): this stands in for a
	// \@ifnextchar whose real definition stores the branch with \def first.
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
		e.push(halveParamHashes(chosen))
	})
	// halveParamHashes applies TeX's ## → # halving to a token list that is about to
	// be re-inserted as if it had been read as a macro BODY.
	//
	// LaTeX's \@ifnextchar does not insert the branch it picked: it stores it first,
	// ltdefns.dtx —
	//
	//	\long\def\@ifnextchar#1#2#3{\let\reserved@d=#1
	//	  \def\reserved@a{#2}\def\reserved@b{#3}\futurelet\@let@token\@ifnch}
	//
	// and \def\reserved@b{#3} SCANS the branch as a macro body, which halves ## a
	// second time (tex.web §479: a # followed by a # stores one #). Inserting the
	// branch verbatim, as this primitive did, skips that halving.
	//
	// It is not a nicety. keyval is built on it:
	//
	//	\def\define@key#1#2{\@ifnextchar[{\KV@def{#1}{#2}}{\long\@namedef{KV@#1@#2}####1}}
	//
	// The #### is halved once into \define@key's own body (two # tokens) and a second
	// time by \@ifnextchar, leaving the one # that \def then reads as the parameter
	// text #1. Without the second halving the generated macro's parameter text was
	// ##1, so it bound nothing: \setkeys{fam}{k=VAL} produced <<>> instead of <<VAL>>
	// and every key VALUE was lost. Checked against real LaTeX, which gives
	// \long macro:#1-><<#1>> where this engine gave macro:##1-><<#1>>.

	// \@ifnextchar<tok>{THEN}{ELSE}: the general LaTeX look-ahead, now that the
	// engine has real \ifx-based token comparison. The target token is read
	// unexpanded; the next non-space input token is peeked (never consumed) and,
	// when it is \ifx-equal to the target, THEN is chosen, else ELSE. This handles
	// targets that are control sequences — e.g. elsarticle/bmvc2k list scanners
	// use \@ifnextchar\<sentinel> to stop, which the bracket-only fallback could
	// never detect (it always took ELSE and looped forever).
	e.prim("@ifnextchar", func(e *Engine) {
		target, ok := e.getNext()
		if !ok {
			return
		}
		thenToks := e.grabUndelimited()
		elseToks := e.grabUndelimited()
		e.skipOptSpace()
		chosen := elseToks
		if t, tok := e.getNext(); tok {
			if e.ifxEqual(t, target) {
				chosen = thenToks
			}
			e.back(t)
		}
		e.push(halveParamHashes(chosen))
	})
	e.prim("verbatim", func(e *Engine) { e.doVerbatim() })
	e.prim("endverbatim", func(e *Engine) {}) // consumed literally by doVerbatim; defined for safety
	// Non-renderable picture environments (TikZ / PGF / tikz-cd): the whole body is
	// gobbled as raw source up to the matching \end{…}, so no \draw/\node/\path/
	// \foreach can leak into the text. Reached via \begin{…}=\csname …\endcsname.
	e.prim("tikzpicture", func(e *Engine) { e.doGobbleEnv("tikzpicture") })
	e.prim("endtikzpicture", func(e *Engine) {}) // consumed literally by doGobbleEnv; defined for safety
	e.prim("pgfpicture", func(e *Engine) { e.doGobbleEnv("pgfpicture") })
	e.prim("endpgfpicture", func(e *Engine) {})
	e.prim("tikzcd", func(e *Engine) { e.doGobbleEnv("tikzcd") })
	e.prim("endtikzcd", func(e *Engine) {})
	// comment package: \begin{comment}…\end{comment} is discarded ENTIRELY (no
	// placeholder — a comment is invisible). \excludecomment{name} makes `name`
	// a silently-gobbled environment too; \includecomment{name} makes its body
	// typeset (begin/end become no-ops so the content just flows). 1616 corpus
	// papers use \begin{comment}; typesetting the body instead of gobbling it
	// leaks stray \item/\\/unbalanced braces and can swallow the whole page.
	e.registerExcludedComment("comment")
	e.prim("excludecomment", func(e *Engine) {
		if n := e.grabEnvNameArg(); n != "" {
			e.registerExcludedComment(n)
		}
	})
	e.prim("includecomment", func(e *Engine) {
		if n := e.grabEnvNameArg(); n != "" {
			e.prim(n, func(e *Engine) {})
			e.prim("end"+n, func(e *Engine) {})
		}
	})
	e.prim("verb", func(e *Engine) { e.doVerb() })
	e.prim("Verb", func(e *Engine) { e.doVerbFancy() })          // fancyvrb: \Verb[opts]{text} or |text|
	e.prim("url", func(e *Engine) { e.doURL() })                 // hyperref: literal, clickable URL
	e.prim("Url", func(e *Engine) { e.doBigURL() })              // url.sty low-level \Url: typeset + close its \begingroup
	e.prim("href", func(e *Engine) { e.doHref() })               // hyperref: text clickable to a URL
	e.prim("nolinkurl", func(e *Engine) { e.doNolinkurl() })     // hyperref: literal URL, no link
	e.prim("hypertarget", func(e *Engine) { e.doHypertarget() }) // hyperref: named in-document destination
	e.prim("hyperlink", func(e *Engine) { e.doHyperlink() })     // hyperref: same-document link to a target
	e.prim("hypersetup", func(e *Engine) { e.doHypersetup() })   // hyperref: link-styling options (colorlinks, urlcolor, …)
	e.prim("hyperref", func(e *Engine) { e.doHyperref() })       // hyperref: internal link by \label, or 4-arg form
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
	e.prim("gotex@putbib", func(e *Engine) { e.doPutbib() })                 // bibunits: \input the current unit's bu<N>.bbl
	e.prim("tabular", func(e *Engine) { e.doTabular() })
	e.prim("tabular*", func(e *Engine) { e.doTabularStar() })
	e.prim("longtable", func(e *Engine) { e.doLongtable() })
	e.prim("endlongtable", func(e *Engine) {}) // consumed by doLongtable
	e.prim("endtabular*", func(e *Engine) {})  // consumed by doTabularStar
	e.prim("endtabular", func(e *Engine) {})   // consumed by doTabular; defined for safety
	e.prim("tabularx", func(e *Engine) { e.doTabularx() })
	e.prim("endtabularx", func(e *Engine) {}) // consumed by doTabularx; defined for safety
	e.prim("minipage", func(e *Engine) { e.doMinipage() })
	e.prim("endminipage", func(e *Engine) {})                           // consumed by doMinipage; defined for safety
	e.prim("subfigure", func(e *Engine) { e.doSubfigure("subfigure") }) // subcaption/subfig sub-panel
	e.prim("endsubfigure", func(e *Engine) {})                          // consumed by doSubfigure
	e.prim("subtable", func(e *Engine) { e.doSubfigure("subtable") })
	e.prim("endsubtable", func(e *Engine) {})
	// sidecap: SCfigure/SCtable (and the *-spanning forms) are ordinary figure/table
	// floats here — no side caption — with their [relwidth][pos] optionals consumed
	// (see sidecap.go).
	e.prim("SCfigure", func(e *Engine) { e.doSCfloat("figure") })
	e.prim("endSCfigure", func(e *Engine) { e.push([]tok{csTok("endfigure")}) })
	e.prim("SCfigure*", func(e *Engine) { e.doSCfloat("figure") })
	e.prim("endSCfigure*", func(e *Engine) { e.push([]tok{csTok("endfigure")}) })
	e.prim("SCtable", func(e *Engine) { e.doSCfloat("table") })
	e.prim("endSCtable", func(e *Engine) { e.push([]tok{csTok("endtable")}) })
	e.prim("SCtable*", func(e *Engine) { e.doSCfloat("table") })
	e.prim("endSCtable*", func(e *Engine) { e.push([]tok{csTok("endtable")}) })
	e.prim("overpic", func(e *Engine) { e.doOverpic("overpic") })   // graphic with a picture overlay
	e.prim("endoverpic", func(e *Engine) {})                        // consumed by doOverpic
	e.prim("overpic*", func(e *Engine) { e.doOverpic("overpic*") }) // starred: overlays a grid
	e.prim("endoverpic*", func(e *Engine) {})
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
	e.prim("import", func(e *Engine) { e.doImport(false) })    // import.sty
	e.prim("subimport", func(e *Engine) { e.doImport(true) })  // import.sty, path relative
	e.prim("inputfrom", func(e *Engine) { e.doImport(false) }) // \inputfrom = \import without \clearpage here
	e.prim("subinputfrom", func(e *Engine) { e.doImport(true) })
	e.prim("gotexendimport", func(e *Engine) { e.importPop() })
	e.prim("hsize", func(e *Engine) { e.scanEquals(); e.setEngineDimen(saveHsize, &e.hsize, e.scanDimen(), false) })
	// \textwidth is the width of the whole text block, NOT the column measure \hsize:
	// in two-column mode it spans both columns and the gutter, so a figure sized
	// width=0.48\textwidth fills nearly a column and width=\textwidth spans the page.
	// It reads e.fullWidth() and its assignment routes through setTextWidth (see
	// twocolumn.go), which keeps the old \let\textwidth\hsize behaviour before the
	// two-column measure is split. \columnwidth / \linewidth stay \let to \hsize.
	e.prim("textwidth", func(e *Engine) { e.scanEquals(); e.setTextWidth(e.scanDimen(), false) })
	e.prim("vsize", func(e *Engine) { e.scanEquals(); e.setEngineDimen(saveVsize, &e.vsize, e.scanDimen(), false) })
	e.prim("baselineskip", func(e *Engine) {
		e.scanEquals()
		e.setEngineDimen(saveBaselineskip, &e.baselineskip, e.scanGlue().width, false)
	})
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
	e.prim("parindent", func(e *Engine) { e.scanEquals(); e.setEngineDimen(saveParindent, &e.parindent, e.scanDimen(), false) })
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
	e.prim("@addtoreset", func(e *Engine) { e.doAddtoreset() })
	e.prim("@stpelt", func(e *Engine) { e.doStpelt() })
	e.prim("refstepcounter", func(e *Engine) { e.doRefstepcounter() })
	e.prim("@Roman", func(e *Engine) { e.pushString(strings.ToUpper(roman(e.scanInt()))) })
	expandableSet["@Roman"] = true
	// LaTeX length interface: \newlength allocates a skip register (a rubber
	// length) and aliases the given cs to it; \setlength/\addtolength assign or
	// advance it (or a \newdimen register / engine parameter); \settoX measure
	// content typeset as an hbox. See lengths.go.
	e.prim("newlength", func(e *Engine) { e.doNewlength() })
	e.prim("setlength", func(e *Engine) { e.doSetlength(false, false) })
	e.prim("addtolength", func(e *Engine) { e.doSetlength(true, false) })
	e.prim("settowidth", func(e *Engine) { e.doSettodim('w', false) })
	e.prim("settoheight", func(e *Engine) { e.doSettodim('h', false) })
	e.prim("settodepth", func(e *Engine) { e.doSettodim('d', false) })
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
	// minted package: Pygments-highlighted code (see minted.go). With no shell-escape
	// / Pygments available the faithful fallback is verbatim, exactly as minted's own
	// draft mode does — the \begin{minted}[opts]{lang} block and the \mintinline /
	// \mint inline forms all reuse the listings verbatim machinery.
	e.prim("minted", func(e *Engine) { e.doMinted() })
	e.prim("endminted", func(e *Engine) {})
	e.prim("mintinline", func(e *Engine) { e.doMintinline() })
	e.prim("mint", func(e *Engine) { e.doMintinline() })
	e.prim("inputminted", func(e *Engine) { e.doInputminted() })
	e.prim("newminted", func(e *Engine) { e.doNewminted() })
	e.prim("newmintinline", func(e *Engine) { e.doNewmintinline("inline") })
	e.prim("newmint", func(e *Engine) { e.doNewmintinline("") })
	// jss.cls (Journal of Statistical Software / Sweave / knitr): Code, CodeInput and
	// CodeOutput are fancyvrb verbatim environments; CodeChunk is a transparent
	// wrapper grouping an input/output pair (see jsscode.go).
	for _, n := range []string{"Code", "CodeInput", "CodeOutput"} {
		name := n
		e.prim(name, func(e *Engine) { e.doNamedVerbatimEnv(name) })
		e.prim("end"+name, func(e *Engine) {}) // consumed literally by readRawEnvBody
	}
	e.prim("CodeChunk", func(e *Engine) {})    // \newenvironment{CodeChunk}{}{}: no-op wrapper
	e.prim("endCodeChunk", func(e *Engine) {}) // body (CodeInput/CodeOutput) flows through
	// geometry package: page size and margins driving \hsize/\vsize and the render
	// margin (see geometry.go). \usepackage[..]{geometry} is intercepted in
	// doUsepackage; \geometry{..} re-applies at any point.
	e.prim("geometry", func(e *Engine) { e.doGeometry() })
	// \newgeometry{opts} re-applies geometry mid-document (the geometry package's
	// bracketing command). Apply it like \geometry — far more faithful than letting
	// it stay undefined, which strict-fails and, lenient, typesets the option list
	// as body text. \restoregeometry restores the preamble geometry; without a
	// saved-state stack it is a no-op (the applied change persists), which is right
	// for the common whole-document re-application. \savegeometry / \loadgeometry
	// gobble their name argument.
	e.prim("newgeometry", func(e *Engine) { e.doGeometry() })
	e.prim("restoregeometry", func(e *Engine) {})
	e.prim("savegeometry", func(e *Engine) { e.readBraceGroupString() })
	e.prim("loadgeometry", func(e *Engine) { e.readBraceGroupString() })
	// enumitem subset (see enumitem.go): \@enumitemopt{<kind>} reads the optional
	// [key=value] argument of a list environment and reconfigures the current list
	// group; \@enumitemrec records an enumerate's final counter value for [resume].
	e.prim("@enumitemopt", func(e *Engine) { e.doEnumitemOpt() })
	e.prim("@enumitemrec", func(e *Engine) { e.recordEnumitemResume() })
	e.loadStomach()
}

// loadStomach registers the box-building primitives. When invoked standalone (in
// the main vertical list, which this milestone does not yet assemble) box/glue/
// kern producers scan and discard their argument to keep the parser in sync; the
// real work happens when they are reached inside \setbox via scanBox/buildBoxList.
func (e *Engine) loadStomach() {
	e.prim("setbox", func(e *Engine) { e.doSetbox(false) })
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
	// \twocolumn[span] / \onecolumn switch the page column mode (twocolumn.go). Under the
	// two-column opt-in they start a fresh region; otherwise they keep the historical
	// no-op (gobble the optional [span] and do nothing), so the default corpus is untouched.
	e.prim("twocolumn", func(e *Engine) {
		if e.twoColLive || twoColumnOptIn() {
			e.startTwoColumn()
			return
		}
		e.scanOptBracketToks()
	})
	e.prim("onecolumn", func(e *Engine) {
		if e.twoColLive || twoColumnOptIn() {
			e.startOneColumn()
		}
	})
	// \gotex@revtexbodytwocol is the internal hook the revtex emulation's \maketitle
	// runs at its end in reprint / journal (two-column) mode: the frontmatter typeset
	// so far (title, authors, affiliations, abstract) stays a full-width one-column
	// region and the body below switches to two columns (see twocolumn.go, revtex.go).
	e.prim("gotex@revtexbodytwocol", func(e *Engine) {
		if e.revtexReprint {
			e.switchToTwoColumn(nil)
		}
	})
	// \gotex@dblfloat{figure|table} is the internal hook \begin{figure*}/\begin{table*}
	// runs (via \@dblfloat, classprims.go). Under the two-column opt-in it typesets the
	// float full-width as a spanning band across both columns; otherwise it sets the
	// unstarred one-column float (twocolumn.go, doDblFloat).
	e.prim("gotex@dblfloat", func(e *Engine) { e.doDblFloat(e.readBraceName()) })
	// \gotex@floatbegin{figure|table} is the internal hook the redefined \@float runs under
	// GOTEX_FLOATS (FloatPlacementSubstrate, floatplace.go): it captures the standard float
	// environment's body into a box and contributes a floatNode for real top/bottom/float-
	// page placement. It is registered unconditionally but reached only when the substrate
	// (loaded only under the flag) redefines \@float to call it.
	e.prim("gotex@floatbegin", func(e *Engine) { e.doFloatBegin() })
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
// halveParamHashes applies TeX's ## → # halving to a token list re-inserted as a
// macro body. See the note at \@ifnextchar.
func halveParamHashes(ts []tok) []tok {
	out := make([]tok, 0, len(ts))
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if !t.cs_ && t.cat == catParam && t.ch == '#' && i+1 < len(ts) {
			if n := ts[i+1]; !n.cs_ && n.cat == catParam && n.ch == '#' {
				out = append(out, tok{ch: '#', cat: catParam})
				i++
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// doIfstar implements \@ifstar{STAR}{PLAIN}. ltdefns.dtx builds it ON \@ifnextchar —
// \def\@ifstar#1{\@ifnextchar *{\@firstoftwo{#1}}} — so in real LaTeX the branch goes
// through \def\reserved@a{#2} and is halved there. This primitive picks the branch
// itself, so it has to halve for the same reason (see halveParamHashes).
func (e *Engine) doIfstar() {
	yes := e.grabUndelimited()
	no := e.grabUndelimited()
	if e.scanOptStar() {
		e.push(halveParamHashes(yes))
		return
	}
	e.push(halveParamHashes(no))
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
		g[i].ch = e.caseOf(x.ch, up)
	}
	e.push(g)
}

// caseOf maps a character through the \uccode/\lccode table, falling back to the
// ASCII letter case when the table has no entry (TeX's initial state, where only
// the letters have one). A package sets an entry to make \lowercase substitute
// one character for another — pgfmath builds its parser's catcode block that way
// (\lccode of ~ set to ", then \lowercase{… \let~ …}) — so the table has to be
// real, not just a letter-case rule.
func (e *Engine) caseOf(r rune, up bool) rune {
	table := e.lccode
	if up {
		table = e.uccode
	}
	if v, ok := table[r]; ok {
		if v == 0 {
			return r // 0 means "leave it alone", as in TeX
		}
		return rune(v)
	}
	if up && r >= 'a' && r <= 'z' {
		return r - 32
	}
	if !up && r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

// doCharCode implements the \lccode/\uccode assignment: a character code, an
// optional =, and the value.
func (e *Engine) doCharCode(table map[rune]int) {
	c := e.scanInt()
	e.scanEquals()
	v := e.scanInt()
	table[rune(c)] = v
}

func (e *Engine) evalIfcat() bool {
	a, _ := e.getXToken()
	b, _ := e.getXToken()
	_, ca := e.ifCharOf(a)
	_, cb := e.ifCharOf(b)
	return ca == cb
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
		// TeX names the prefix as part of the meaning: \protected macro:->…
		// TeX names the prefixes as part of the meaning, \protected before \long
		// whatever order they were written in (checked against real LaTeX:
		// \long\protected\def and \protected\long\def both print
		// "\protected\long macro:").
		pfx := ""
		if m.protected {
			pfx = e.escapeStr() + "protected"
		}
		if m.long {
			pfx += e.escapeStr() + "long"
		}
		if pfx != "" {
			pfx += " "
		}
		return pfx + "macro:" + e.toksToString(m.params) + "->" + e.toksToString(m.body)
	case mPrim:
		return e.escapeStr() + m.name
	case mCharDef:
		if m.mathChar {
			return e.escapeStr() + "mathchar\"" + itoaHex(m.code)
		}
		return e.escapeStr() + "char\"" + itoaHex(m.code)
	case mCountRef:
		return e.escapeStr() + "count" + strconv.Itoa(m.code)
	case mDimenRef:
		return e.escapeStr() + "dimen" + strconv.Itoa(m.code)
	case mSkipRef:
		return e.escapeStr() + "skip" + strconv.Itoa(m.code)
	case mToksRef:
		return e.escapeStr() + "toks" + strconv.Itoa(m.code)
	case mBoxRef:
		// \newbox allocates with \chardef, so its handle reads as a character
		// constant: \meaning of one is \char"33 under a real TeX, not \box51.
		return e.escapeStr() + "char\"" + itoaHex(m.code)
	case mFont:
		return "select font " + m.name
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
