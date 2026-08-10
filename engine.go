// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package engine is the core of a pure-Go (CGO=0) TeX engine: a faithful
// re-implementation of TeX's mouth and gullet — category-code tokenization,
// the equivalents table (eqtb) with grouping/scoping, macro definition with
// delimited parameters, and the expansion machinery (\def, \edef, \let,
// \expandafter, \csname, \noexpand, \string, \the, \number, conditionals,
// integer registers). It is the foundation on which the real LaTeX kernel and
// packages will run, gated by TeX's own conformance suite (the TRIP test) — the
// path to functional parity with a TeX distribution, not a subset.
package engine

import (
	"fmt"
	"strings"
)

// ── category codes & tokens ─────────────────────────────────────────────────

type cat uint8

const (
	catEsc     cat = 0 // \
	catBegin   cat = 1 // {
	catEnd     cat = 2 // }
	catMath    cat = 3 // $
	catAlign   cat = 4 // &
	catEOL     cat = 5 // end of line
	catParam   cat = 6 // #
	catSup     cat = 7 // ^
	catSub     cat = 8 // _
	catIgnore  cat = 9
	catSpace   cat = 10 // space
	catLetter  cat = 11 // a-z A-Z
	catOther   cat = 12 // everything else
	catActive  cat = 13 // ~
	catComment cat = 14 // %
	catInvalid cat = 15
)

// tok is a TeX token: either a control sequence (cs set) or a character with a
// category code. A parameter marker (from a macro parameter text) uses catParam
// with ch = '1'..'9', or ch = '#' for a literal doubled ##.
type tok struct {
	cs    string
	ch    rune
	cat   cat
	cs_   bool // is a control sequence
	noexp bool // \noexpand'd: pass through getXToken unexpanded once
}

func csTok(name string) tok         { return tok{cs: name, cs_: true} }
func chTok(r rune, c cat) tok       { return tok{ch: r, cat: c} }
func (t tok) isCS() bool            { return t.cs_ }
func (t tok) is(r rune, c cat) bool { return !t.cs_ && t.ch == r && t.cat == c }

func (t tok) String() string {
	if t.cs_ {
		return "\\" + t.cs
	}
	return string(t.ch)
}

// ── the engine ──────────────────────────────────────────────────────────────

// Engine holds all TeX state: the input stack, the eqtb (control-sequence
// meanings), integer registers, category codes, the grouping save stack, and
// the \message output buffer.
type Engine struct {
	// input: a stack of token lists (top = most recent), plus the base string.
	lists [][]tok
	base  []rune
	bpos  int

	catcode [256]cat
	eq      map[string]*meaning
	count   [256]int
	dimen   [256]int      // \dimen registers, in scaled points (1pt = 65536sp)
	skip    [256]glueSpec // \skip (glue) registers
	box     [256]*boxNode // \box registers (nil = void)
	mvl     []node        // main vertical list (top-level contributions)
	curFont fontFace      // current font for measuring/rendering characters

	// paragraph-builder state (horizontal mode at top level)
	inPar        bool   // a paragraph is being accumulated
	parList      []node // the current paragraph's horizontal list
	hsize        int    // line width for breaking (sp)
	vsize        int    // page height for the page builder (sp)
	baselineskip int    // baseline-to-baseline glue (sp)
	lineskip     int    // minimum interline glue when baselineskip is too small (sp)
	parindent    int    // width of the indentation box at a paragraph's start (sp)
	prevDepth    int    // \prevdepth for interline glue (ignoreDepth = suppress)

	// save stack for grouping: each entry restores one eqtb/register/catcode.
	save   []saveItem
	groups []int // save-stack length at each group's start

	out      strings.Builder // \message output
	err      error
	noBase   bool // when true, getNext does not fall through to the base string
	allocCnt int  // next free \count register handed out by \newcount
	allocDim int  // next free \dimen register handed out by \newdimen
	allocSkp int  // next free \skip register handed out by \newskip
}

type mkind uint8

const (
	mMacro mkind = iota
	mPrim
	mLetChar  // \let to a character token
	mCharDef  // \chardef
	mCountRef // \countdef / \newcount — an alias for a \count register (code = index)
	mDimenRef // \dimendef / \newdimen — an alias for a \dimen register (code = index)
	mSkipRef  // \skipdef / \newskip — an alias for a \skip register (code = index)
	mFont     // a font-switching control sequence defined by \font
	mUndef
)

type meaning struct {
	kind   mkind
	params []tok // macro parameter text
	body   []tok // macro replacement text
	prim   func(e *Engine)
	name   string // primitive name (for \meaning/\string)
	ch     rune   // let-char / chardef code
	cat    cat
	code   int
	font   fontFace // mFont: the font this cs selects
}

type saveItem struct {
	kind int // 0=eqtb, 1=count, 2=catcode, 3=dimen, 4=skip
	name string
	old  *meaning
	idx  int
	oldi int
	oldc cat
	oldd int
	oldg glueSpec
}

// New builds an engine with TeX's default category codes and primitives loaded.
func New() *Engine {
	e := &Engine{eq: map[string]*meaning{}, allocCnt: 10, allocDim: 10, allocSkp: 10} // allocators start at 10
	e.hsize = ptToSP(6.5 * 7227.0 / 100.0)                                            // plain TeX \hsize = 6.5in
	e.vsize = ptToSP(8.9 * 7227.0 / 100.0)                                            // plain TeX \vsize = 8.9in
	e.baselineskip = 12 * unity                                                       // 12pt
	e.lineskip = unity                                                                // 1pt
	e.parindent = 20 * unity                                                          // plain TeX \parindent = 20pt
	e.prevDepth = ignoreDepth
	for i := range e.catcode {
		e.catcode[i] = catOther
	}
	for c := 'a'; c <= 'z'; c++ {
		e.catcode[c] = catLetter
	}
	for c := 'A'; c <= 'Z'; c++ {
		e.catcode[c] = catLetter
	}
	e.catcode['\\'] = catEsc
	e.catcode['{'] = catBegin
	e.catcode['}'] = catEnd
	e.catcode['$'] = catMath
	e.catcode['&'] = catAlign
	e.catcode['\n'] = catEOL
	e.catcode['#'] = catParam
	e.catcode['^'] = catSup
	e.catcode['_'] = catSub
	e.catcode[' '] = catSpace
	e.catcode['\t'] = catSpace
	e.catcode['~'] = catActive
	e.catcode['%'] = catComment
	e.loadPrimitives()
	e.loadMore()
	return e
}

// Run tokenizes src as the base input and processes it to completion, returning
// the accumulated \message output.
func (e *Engine) Run(src string) (string, error) {
	e.base = []rune(src)
	e.bpos = 0
	e.mainLoop()
	return e.out.String(), e.err
}

// ── input & tokenization ────────────────────────────────────────────────────

// push puts a token list on the input stack (read before the base input).
func (e *Engine) push(ts []tok) {
	if len(ts) == 0 {
		return
	}
	e.lists = append(e.lists, ts)
}

// getNext returns the next raw token (no expansion), or ok=false at end.
func (e *Engine) getNext() (tok, bool) {
	for len(e.lists) > 0 {
		top := e.lists[len(e.lists)-1]
		if len(top) == 0 {
			e.lists = e.lists[:len(e.lists)-1]
			continue
		}
		t := top[0]
		if rest := top[1:]; len(rest) == 0 {
			e.lists = e.lists[:len(e.lists)-1] // drop the drained list eagerly so
		} else { //                              len(e.lists) reflects real nesting
			e.lists[len(e.lists)-1] = rest
		}
		return t, true
	}
	if e.noBase {
		return tok{}, false
	}
	return e.scan()
}

// back pushes a single token back onto the input.
func (e *Engine) back(t tok) { e.lists = append(e.lists, []tok{t}) }

// scan reads the next token from the base string using current catcodes.
func (e *Engine) scan() (tok, bool) {
	for e.bpos < len(e.base) {
		r := e.base[e.bpos]
		c := e.catOf(r)
		switch c {
		case catEsc:
			e.bpos++
			return e.scanCS(), true
		case catComment:
			for e.bpos < len(e.base) && e.base[e.bpos] != '\n' {
				e.bpos++
			}
		case catSpace, catEOL:
			e.bpos++
			// collapse spaces; emit one space token
			for e.bpos < len(e.base) {
				cc := e.catOf(e.base[e.bpos])
				if cc != catSpace && cc != catEOL {
					break
				}
				e.bpos++
			}
			return chTok(' ', catSpace), true
		case catIgnore:
			e.bpos++
		default:
			e.bpos++
			return chTok(r, c), true
		}
	}
	return tok{}, false
}

func (e *Engine) catOf(r rune) cat {
	if r < 256 {
		return e.catcode[r]
	}
	return catOther
}

// scanCS reads a control-sequence name after an escape char.
func (e *Engine) scanCS() tok {
	if e.bpos >= len(e.base) {
		return csTok("")
	}
	r := e.base[e.bpos]
	if e.catOf(r) != catLetter {
		e.bpos++
		return csTok(string(r))
	}
	st := e.bpos
	for e.bpos < len(e.base) && e.catOf(e.base[e.bpos]) == catLetter {
		e.bpos++
	}
	name := string(e.base[st:e.bpos])
	// a control word absorbs following spaces
	for e.bpos < len(e.base) {
		cc := e.catOf(e.base[e.bpos])
		if cc != catSpace && cc != catEOL {
			break
		}
		e.bpos++
	}
	return csTok(name)
}

// ── meanings & grouping ─────────────────────────────────────────────────────

func (e *Engine) meaningOf(t tok) *meaning {
	if t.cs_ {
		return e.eq[t.cs]
	}
	if t.cat == catActive {
		return e.eq["~active~"+string(t.ch)]
	}
	return nil
}

func (e *Engine) define(name string, m *meaning, global bool) {
	if !global && len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 0, name: name, old: e.eq[name]})
	}
	e.eq[name] = m
}

func (e *Engine) setCount(i, v int, global bool) {
	if !global && len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 1, idx: i, oldi: e.count[i]})
	}
	e.count[i] = v
}

func (e *Engine) setDimen(i, v int, global bool) {
	if i < 0 || i >= 256 {
		return
	}
	if !global && len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 3, idx: i, oldd: e.dimen[i]})
	}
	e.dimen[i] = v
}

func (e *Engine) setSkip(i int, v glueSpec, global bool) {
	if i < 0 || i >= 256 {
		return
	}
	if !global && len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 4, idx: i, oldg: e.skip[i]})
	}
	e.skip[i] = v
}

func (e *Engine) setCat(r rune, c cat, global bool) {
	if r >= 256 {
		return
	}
	if !global && len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 2, idx: int(r), oldc: e.catcode[r]})
	}
	e.catcode[r] = c
}

func (e *Engine) beginGroup() { e.groups = append(e.groups, len(e.save)) }

func (e *Engine) endGroup() {
	if len(e.groups) == 0 {
		return
	}
	mark := e.groups[len(e.groups)-1]
	e.groups = e.groups[:len(e.groups)-1]
	for len(e.save) > mark {
		s := e.save[len(e.save)-1]
		e.save = e.save[:len(e.save)-1]
		switch s.kind {
		case 0:
			if s.old == nil {
				delete(e.eq, s.name)
			} else {
				e.eq[s.name] = s.old
			}
		case 1:
			e.count[s.idx] = s.oldi
		case 2:
			e.catcode[rune(s.idx)] = s.oldc
		case 3:
			e.dimen[s.idx] = s.oldd
		case 4:
			e.skip[s.idx] = s.oldg
		}
	}
}

// ── expansion ───────────────────────────────────────────────────────────────

// getXToken returns the next token, expanding expandable control sequences
// (macros and expandable primitives) until a non-expandable token surfaces.
func (e *Engine) getXToken() (tok, bool) {
	for {
		t, ok := e.getNext()
		if !ok {
			return tok{}, false
		}
		if t.noexp {
			t.noexp = false
			return t, true
		}
		m := e.meaningOf(t)
		if m == nil {
			return t, true
		}
		switch m.kind {
		case mMacro:
			e.expandMacro(m)
		case mPrim:
			if isExpandable(m.name) {
				m.prim(e)
			} else {
				return t, true
			}
		default:
			return t, true
		}
	}
}

// expandMacro matches the macro's parameters against the input and pushes the
// substituted body.
func (e *Engine) expandMacro(m *meaning) {
	args := e.matchParams(m.params)
	var body []tok
	for i := 0; i < len(m.body); i++ {
		b := m.body[i]
		if b.cat == catParam && !b.cs_ && b.ch >= '1' && b.ch <= '9' {
			n := int(b.ch - '1')
			if n < len(args) {
				body = append(body, args[n]...)
			}
			continue
		}
		body = append(body, b)
	}
	e.push(body)
}

// matchParams consumes the arguments for a parameter text, honoring literal
// delimiter tokens between parameters.
func (e *Engine) matchParams(params []tok) [][]tok {
	var args [][]tok
	i := 0
	for i < len(params) {
		p := params[i]
		if p.cat == catParam && !p.cs_ && p.ch >= '1' && p.ch <= '9' {
			// Determine the delimiter that terminates this argument.
			var delim []tok
			j := i + 1
			for j < len(params) && !(params[j].cat == catParam && !params[j].cs_) {
				delim = append(delim, params[j])
				j++
			}
			if len(delim) == 0 {
				args = append(args, e.grabUndelimited())
			} else {
				args = append(args, e.grabDelimited(delim))
			}
			i = j
			continue
		}
		// literal delimiter in the parameter text: must match input.
		e.matchLiteral(p)
		i++
	}
	return args
}

func (e *Engine) grabUndelimited() []tok {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok {
		return nil
	}
	if t.cat == catBegin && !t.cs_ {
		return e.grabGroup()
	}
	return []tok{t}
}

func (e *Engine) grabDelimited(delim []tok) []tok {
	var arg []tok
	depth := 0
	for {
		t, ok := e.getNext()
		if !ok {
			return arg
		}
		if depth == 0 && t.cat == catBegin && !t.cs_ {
			depth++
			arg = append(arg, t)
			continue
		}
		if depth > 0 {
			if t.cat == catBegin && !t.cs_ {
				depth++
			} else if t.cat == catEnd && !t.cs_ {
				depth--
			}
			arg = append(arg, t)
			continue
		}
		// try to match the delimiter starting here.
		if tokEq(t, delim[0]) {
			matched := []tok{t}
			ok2 := true
			for k := 1; k < len(delim); k++ {
				u, uk := e.getNext()
				if !uk || !tokEq(u, delim[k]) {
					if uk {
						e.back(u)
					}
					ok2 = false
					break
				}
				matched = append(matched, u)
			}
			if ok2 {
				return e.stripOuterBraces(arg)
			}
			// partial match: keep first token, back the rest
			for k := len(matched) - 1; k >= 1; k-- {
				e.back(matched[k])
			}
		}
		arg = append(arg, t)
	}
}

// stripOuterBraces removes one level of braces if the whole arg is a group.
func (e *Engine) stripOuterBraces(arg []tok) []tok { return arg }

func (e *Engine) grabGroup() []tok {
	depth := 1
	var g []tok
	for {
		t, ok := e.getNext()
		if !ok {
			return g
		}
		if t.cat == catBegin && !t.cs_ {
			depth++
		} else if t.cat == catEnd && !t.cs_ {
			depth--
			if depth == 0 {
				return g
			}
		}
		g = append(g, t)
	}
}

func (e *Engine) matchLiteral(p tok) {
	t, ok := e.getNext()
	if !ok {
		return
	}
	if !tokEq(t, p) {
		e.back(t) // best-effort: don't consume a non-match
	}
}

func (e *Engine) skipOptSpace() {
	for {
		t, ok := e.getNext()
		if !ok {
			return
		}
		if !(t.cat == catSpace && !t.cs_) {
			e.back(t)
			return
		}
	}
}

func tokEq(a, b tok) bool {
	if a.cs_ != b.cs_ {
		return false
	}
	if a.cs_ {
		return a.cs == b.cs
	}
	return a.ch == b.ch && a.cat == b.cat
}

// ── main loop ───────────────────────────────────────────────────────────────

// mainLoop drives execution: it fetches expanded tokens and performs the
// non-expandable ones (assignments, grouping, \message, …). Characters that are
// not consumed by a primitive are dropped (this core has no typesetting stomach
// yet — that is the next stage).
func (e *Engine) mainLoop() {
	for e.err == nil {
		t, ok := e.getXToken()
		if !ok {
			e.endParagraph() // flush a trailing paragraph at end of input
			return
		}
		if !t.cs_ {
			switch t.cat {
			case catBegin:
				e.beginGroup()
			case catEnd:
				e.endGroup()
			case catLetter, catOther:
				e.startChar(t.ch) // begin/continue a paragraph in horizontal mode
			case catSpace:
				if e.inPar && e.curFont != nil {
					e.parList = append(e.parList, glueNode{spec: e.curFont.spaceSP()})
				}
			}
			continue
		}
		if !e.execCS(t) {
			return
		}
	}
}

// startChar appends a measured character to the current paragraph, starting one
// (with indentation) if needed. Without a current font there is nothing to
// measure, so it is a no-op (the character is dropped, as in the pre-font core).
func (e *Engine) startChar(ch rune) {
	if e.curFont == nil {
		return
	}
	if !e.inPar {
		e.beginParagraph(true)
	}
	e.parList = e.appendChar(e.parList, ch)
}

// appendChar appends a measured character to a horizontal list, inserting the
// font's inter-character kern before it when the previous node is a character
// (TeX's font kern program). It is shared by paragraph building and box building.
func (e *Engine) appendChar(list []node, ch rune) []node {
	if prev, ok := lastChar(list); ok {
		if k := e.curFont.kernSP(prev, ch); k != 0 {
			list = append(list, kernNode{width: k})
		}
	}
	w, h, d := e.curFont.charDimsSP(ch)
	return append(list, charNode{ch: ch, width: w, height: h, depth: d})
}

// lastChar returns the rune of the trailing character node, if the list ends in
// one (so a kern can be inserted before the next character).
func lastChar(list []node) (rune, bool) {
	if len(list) > 0 {
		if c, ok := list[len(list)-1].(charNode); ok {
			return c.ch, true
		}
	}
	return 0, false
}

// beginParagraph starts a paragraph, optionally prefixing the \parindent box
// (an empty hbox of that width) as TeX does for an indented paragraph.
func (e *Engine) beginParagraph(indent bool) {
	e.inPar = true
	if indent {
		e.parList = append(e.parList, &boxNode{kind: hbox, width: e.parindent})
	}
}

// execCS performs one control-sequence token (an assignment or non-expandable
// primitive). It returns false on a fatal error (undefined cs). Expandable
// tokens never reach here — getXToken has already expanded them. Both the main
// loop and box building route control sequences through this one dispatch.
func (e *Engine) execCS(t tok) bool {
	m := e.meaningOf(t)
	if m == nil {
		e.fail("Undefined control sequence \\" + t.cs)
		return false
	}
	switch m.kind {
	case mCountRef:
		e.countRefAssign(m.code, false) // \n=<v>
	case mDimenRef:
		e.dimenRefAssign(m.code, false) // \d=<dimen>
	case mSkipRef:
		e.skipRefAssign(m.code, false) // \s=<glue>
	case mFont:
		e.curFont = m.font // \rm etc. selects the current font
	case mPrim:
		if !isExpandable(m.name) {
			m.prim(e)
		}
	}
	return true
}

func (e *Engine) fail(msg string) {
	if e.err == nil {
		e.err = fmt.Errorf("texengine: %s", msg)
	}
}

// ── scanning helpers used by primitives ─────────────────────────────────────

// scanInt scans an optional-signed integer (decimal, or \count register, or
// `\char, or a \chardef'd cs).
func (e *Engine) scanInt() int {
	e.skipOptSpace()
	sign := 1
	for {
		t, ok := e.getXToken()
		if !ok {
			return 0
		}
		if t.is('+', catOther) {
			continue
		}
		if t.is('-', catOther) {
			sign = -sign
			continue
		}
		if t.is(' ', catSpace) {
			continue
		}
		// \count register?
		if t.cs_ {
			if m := e.eq[t.cs]; m != nil {
				if m.kind == mCharDef {
					return sign * m.code
				}
				if m.kind == mCountRef {
					return sign * e.count[m.code]
				}
				if m.kind == mPrim && m.name == "count" {
					return sign * e.count[e.scanInt()]
				}
			}
		}
		if !t.cs_ && t.ch >= '0' && t.ch <= '9' {
			n := int(t.ch - '0')
			for {
				u, uk := e.getXToken()
				if uk && !u.cs_ && u.ch >= '0' && u.ch <= '9' {
					n = n*10 + int(u.ch-'0')
					continue
				}
				if uk && !(u.cat == catSpace) {
					e.back(u)
				}
				break
			}
			return sign * n
		}
		e.back(t)
		return 0
	}
}

// unitRatio maps a physical unit keyword to TeX's exact (num, denom) ratio to
// points (§458 set_conversion). pt/sp are handled specially in scanDimen.
var unitRatio = map[string][2]int{
	"in": {7227, 100},
	"pc": {12, 1},
	"cm": {7227, 254},
	"mm": {7227, 2540},
	"bp": {7227, 7200},
	"dd": {1238, 1157},
	"cc": {14856, 1157},
}

const unity = 65536 // scaled points per point

// scanSign consumes optional leading spaces and +/- signs, returning the net sign.
func (e *Engine) scanSign() int {
	sign := 1
	for {
		t, ok := e.getXToken()
		if !ok {
			return sign
		}
		if t.is('+', catOther) {
			continue
		}
		if t.is('-', catOther) {
			sign = -sign
			continue
		}
		if t.is(' ', catSpace) {
			continue
		}
		e.back(t)
		return sign
	}
}

// scanDimen scans an optional-signed dimension and returns scaled points. It
// accepts a decimal factor plus a unit (pt, pc, in, bp, cm, mm, dd, cc, sp), a
// \dimen register, or a \dimendef'd alias — using TeX's exact sp arithmetic.
func (e *Engine) scanDimen() int {
	e.skipOptSpace()
	sign := e.scanSign()
	v, _ := e.scanDimenValue(false)
	return sign * v
}

// scanDimenValue reads one (unsigned) dimension value, returning its scaled-point
// size and glue order (0 = finite pt; 1/2/3 = fil/fill/filll when inf is true).
func (e *Engine) scanDimenValue(inf bool) (int, int) {
	t, ok := e.getXToken()
	if !ok {
		return 0, 0
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil {
			switch {
			case m.kind == mDimenRef:
				return e.dimen[m.code], 0
			case m.kind == mPrim && m.name == "dimen":
				return e.dimen[e.scanInt()], 0
			case m.kind == mPrim && m.name == "wd":
				return e.boxDim('w'), 0
			case m.kind == mPrim && m.name == "ht":
				return e.boxDim('h'), 0
			case m.kind == mPrim && m.name == "dp":
				return e.boxDim('d'), 0
			}
		}
		e.back(t)
		return 0, 0
	}
	if (t.ch >= '0' && t.ch <= '9') || t.ch == '.' || t.ch == ',' {
		e.back(t)
		intPart, f := e.scanDecimalSP() // integer part, 16-bit fraction
		if inf {
			if order := e.scanFil(); order > 0 {
				e.skipOneOptSpace()
				return intPart*unity + f, order
			}
		}
		return e.applyUnit(intPart, f), 0
	}
	e.back(t)
	return 0, 0
}

// scanFil matches a "fil", "fill", or "filll" infinite-glue unit, returning its
// order (1/2/3) or 0 if the upcoming tokens are not such a keyword (backed out).
func (e *Engine) scanFil() int {
	e.skipOptSpace()
	var buf []tok
	backOut := func() int {
		for k := len(buf) - 1; k >= 0; k-- {
			e.back(buf[k])
		}
		return 0
	}
	for _, w := range []rune{'f', 'i', 'l'} {
		t, ok := e.getXToken()
		if !ok {
			return backOut()
		}
		buf = append(buf, t)
		if t.cs_ || lower(t.ch) != w {
			return backOut()
		}
	}
	order := 1
	for order < 3 {
		t, ok := e.getXToken()
		if ok && !t.cs_ && lower(t.ch) == 'l' {
			order++
			buf = append(buf, t)
			continue
		}
		if ok {
			e.back(t)
		}
		break
	}
	return order
}

// scanDecimalSP reads a non-negative decimal and returns its integer part and the
// fractional part as a 16-bit value (TeX round_decimals: 0.d1d2… × 65536 rounded).
func (e *Engine) scanDecimalSP() (int, int) {
	intPart := 0
	var digs []int
	seenDot := false
	for {
		t, ok := e.getXToken()
		if !ok {
			break
		}
		if !t.cs_ && (t.ch == '.' || t.ch == ',') && !seenDot {
			seenDot = true
			continue
		}
		if !t.cs_ && t.ch >= '0' && t.ch <= '9' {
			if seenDot {
				if len(digs) < 17 { // TeX keeps at most 17 fraction digits
					digs = append(digs, int(t.ch-'0'))
				}
			} else {
				intPart = intPart*10 + int(t.ch-'0')
			}
			continue
		}
		e.back(t)
		break
	}
	return intPart, roundDecimals(digs)
}

// roundDecimals converts fraction digits to a 16-bit value (TeX §102).
func roundDecimals(digs []int) int {
	a := 0
	for k := len(digs) - 1; k >= 0; k-- {
		a = (a + digs[k]*(2*unity)) / 10
	}
	return (a + 1) / 2
}

// applyUnit converts an integer part + 16-bit fraction to scaled points given the
// unit keyword that follows (TeX §453–458). Defaults to pt if none is recognised.
func (e *Engine) applyUnit(intPart, f int) int {
	e.skipOptSpace()
	a, ok := e.getXToken()
	if !ok || a.cs_ {
		if ok {
			e.back(a)
		}
		return intPart*unity + f // bare number ⇒ pt
	}
	b, ok := e.getXToken()
	if !ok || b.cs_ {
		if ok {
			e.back(b)
		}
		e.back(a)
		return intPart*unity + f
	}
	key := string([]rune{lower(a.ch), lower(b.ch)})
	switch {
	case key == "pt":
		e.skipOneOptSpace()
		return intPart*unity + f
	case key == "sp":
		e.skipOneOptSpace()
		return intPart // sp is already scaled; fraction is dropped
	default:
		if r, isUnit := unitRatio[key]; isUnit {
			e.skipOneOptSpace()
			num, den := r[0], r[1]
			q, rem := xnOverD(intPart, num, den)
			f = (num*f + unity*rem) / den
			return q*unity + f
		}
	}
	e.back(b)
	e.back(a)
	return intPart*unity + f
}

// xnOverD returns the quotient and remainder of x*n/d (TeX §107, for x ≥ 0 and
// values within int64 — sufficient for dimension scanning).
func xnOverD(x, n, d int) (int, int) {
	xn := int64(x) * int64(n)
	return int(xn / int64(d)), int(xn % int64(d))
}

// glueSpec is a TeX glue quantity: a natural width plus stretch and shrink, each
// with an order (0 = finite pt, 1/2/3 = fil/fill/filll). All sizes are in sp.
type glueSpec struct {
	width, stretch, shrink    int
	stretchOrder, shrinkOrder int
}

// scanGlue scans a glue: <dimen> [plus <dimen or fil>] [minus <dimen or fil>], or
// an internal glue register / \skipdef'd alias copied whole.
func (e *Engine) scanGlue() glueSpec {
	e.skipOptSpace()
	// An internal glue quantity (\skip register or \skipdef alias) is copied whole.
	if t, ok := e.getXToken(); ok {
		if t.cs_ {
			if m := e.eq[t.cs]; m != nil {
				switch {
				case m.kind == mSkipRef:
					return e.skip[m.code]
				case m.kind == mPrim && m.name == "skip":
					return e.skip[e.scanInt()]
				}
			}
		}
		e.back(t)
	}
	sign := e.scanSign()
	w, _ := e.scanDimenValue(false)
	// A \skip register or \skipdef'd alias used directly as a glue value.
	g := glueSpec{width: sign * w}
	if e.scanKeyword("plus") {
		s := e.scanSign()
		v, o := e.scanDimenValue(true)
		g.stretch, g.stretchOrder = s*v, o
	}
	if e.scanKeyword("minus") {
		s := e.scanSign()
		v, o := e.scanDimenValue(true)
		g.shrink, g.shrinkOrder = s*v, o
	}
	return g
}

// scanKeyword tries to match the literal word (case-insensitively) after optional
// spaces, consuming it on success and backing out every token on failure.
func (e *Engine) scanKeyword(word string) bool {
	e.skipOptSpace()
	var buf []tok
	for _, w := range word {
		t, ok := e.getNext()
		if !ok {
			for k := len(buf) - 1; k >= 0; k-- {
				e.back(buf[k])
			}
			return false
		}
		buf = append(buf, t)
		if t.cs_ || lower(t.ch) != w {
			for k := len(buf) - 1; k >= 0; k-- {
				e.back(buf[k])
			}
			return false
		}
	}
	return true
}

// skipOneOptSpace consumes a single optional trailing space (TeX eats one space
// after a unit keyword).
func (e *Engine) skipOneOptSpace() {
	t, ok := e.getNext()
	if ok && !(t.cat == catSpace && !t.cs_) {
		e.back(t)
	}
}

func lower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

// scanCSName reads the next token and returns the name to define (\def\foo → foo).
func (e *Engine) scanCSName() string {
	t, ok := e.getNext()
	if !ok || !t.cs_ {
		return ""
	}
	return t.cs
}

// scanEquals consumes an optional '='.
func (e *Engine) scanEquals() {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if ok && !t.is('=', catOther) {
		e.back(t)
	}
}

// scanDefText reads a macro's parameter text (up to the opening brace) and its
// body (the balanced group).
func (e *Engine) scanDefText() (params, body []tok) {
	for {
		t, ok := e.getNext()
		if !ok {
			return
		}
		if t.cat == catBegin && !t.cs_ {
			break
		}
		if t.cat == catParam && !t.cs_ {
			n, ok := e.getNext()
			if ok {
				params = append(params, tok{ch: n.ch, cat: catParam})
			}
			continue
		}
		params = append(params, t)
	}
	body = e.scanBody()
	return
}

// scanBody reads a macro replacement text (the balanced group), converting
// #<digit> into a parameter marker and ## into a literal #.
func (e *Engine) scanBody() []tok {
	depth := 1
	var g []tok
	for {
		t, ok := e.getNext()
		if !ok {
			return g
		}
		switch {
		case t.cat == catBegin && !t.cs_:
			depth++
		case t.cat == catEnd && !t.cs_:
			depth--
			if depth == 0 {
				return g
			}
		case t.cat == catParam && !t.cs_:
			n, ok := e.getNext()
			if !ok {
				return g
			}
			if n.cat == catParam && !n.cs_ {
				g = append(g, chTok('#', catOther)) // ## → #
			} else {
				g = append(g, tok{ch: n.ch, cat: catParam}) // #digit → parameter
			}
			continue
		}
		g = append(g, t)
	}
}

// toksToString renders a token list to a string (for \message, \the, …).
func (e *Engine) toksToString(ts []tok) string {
	var b strings.Builder
	for _, t := range ts {
		switch {
		case t.cs_:
			b.WriteString("\\" + t.cs)
			if isWord(t.cs) {
				b.WriteByte(' ')
			}
		case t.cat == catParam:
			b.WriteByte('#')
			b.WriteRune(t.ch)
		default:
			b.WriteRune(t.ch)
		}
	}
	return b.String()
}

func isWord(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return s != ""
}
