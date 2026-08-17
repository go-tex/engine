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

	// source-position tracking (see srcmap.go): where the mouth is reading, so
	// glyphs/boxes can be stamped with their origin line for errors + navigation.
	lineStarts []int // rune offset of each source line's start
	srcPos     int   // rune offset where the current token began
	curSrcLine int   // 1-based line of the current token (0 = unknown)
	curSrcCol  int   // 0-based column of the current token
	count      [256]int
	dimen      [256]int          // \dimen registers, in scaled points (1pt = 65536sp)
	skip       [256]glueSpec     // \skip (glue) registers
	box        [256]*boxNode     // \box registers (nil = void)
	mvl        []node            // main vertical list (top-level contributions)
	curFont    fontFace          // current font for measuring/rendering characters
	baseFont   fontFace          // the \normalsize font — glyph source + size reference for scaling
	baseFontPx int               // \normalsize size in px/pt (the 100% for \large/\small/…)
	curColor   uint32            // current text colour (0xRRGGBB; 0 = default black)
	colors     map[string]uint32 // \definecolor names → 0xRRGGBB (see color.go)
	mathR      mathRendererT     // lazily-built go-tex/math renderer (see math.go)

	// paragraph-builder state (horizontal mode at top level)
	inPar            bool   // a paragraph is being accumulated
	parList          []node // the current paragraph's horizontal list
	everypar         []tok  // \everypar hook, fired at each paragraph start
	inEverypar       bool   // guard: do not re-fire \everypar while it is running
	hsize            int    // line width for breaking (sp)
	vsize            int    // page height for the page builder (sp)
	baselineskip     int    // baseline-to-baseline glue (sp)
	baseBaselineskip int    // the single-spaced baseline skip, the 1.0 reference for setspace
	spacingSaved     []int  // \baselineskip stack for the setspace `spacing` environment
	lineskip         int    // minimum interline glue when baselineskip is too small (sp)
	parindent        int    // width of the indentation box at a paragraph's start (sp)
	prevDepth        int    // \prevdepth for interline glue (ignoreDepth = suppress)

	hyph          *hyphenator // loaded hyphenation patterns (nil = no hyphenation)
	hyphenpenalty int         // penalty at a discretionary hyphen
	leftskip      glueSpec    // glue at the left of every line
	rightskip     glueSpec    // glue at the right of every line (fil ⇒ ragged right)
	columnsep     int         // \columnsep: gap between multicols columns (sp)
	columnseprule int         // \columnseprule: rule thickness between columns (sp, 0 = none)
	geom          *geomState  // geometry package layout (nil until \usepackage[..]{geometry} or \geometry; see geometry.go)

	// save stack for grouping: each entry restores one eqtb/register/catcode.
	save   []saveItem
	groups []int // save-stack length at each group's start

	out    strings.Builder   // \message output
	labels map[string]string // \label → \@currentlabel text, resolved by \ref (two-pass)
	// typed cross-references (see typedrefs.go): recorded beside labels at \label
	// time and carried through the two-pass compile exactly like labels.
	refTypes map[string]string // \label → \@currentreftype, used by \autoref / \cref
	refNames map[string]string // \label → \@currentlabelname (title), used by \nameref
	err      error

	// BibTeX bibliography (see bibtex.go): which keys were \cite'd/\nocite'd (and
	// their first-citation order), whether \nocite{*} requested every entry, the
	// accepted \bibliographystyle name, and the first-author label per key used by
	// natbib's \citet — carried from the aux pass so a \citet before the
	// \bibliography resolves, exactly like labels.
	citedKeys map[string]bool
	citeOrder []string
	nociteAll bool
	bibStyle  string
	bibAuthor map[string]string

	// table of contents (see toc.go): entries recorded as \section/\subsection and
	// \caption run (via the \@tocentry prim), carried from the aux pass into the
	// render pass exactly like labels, and consumed by \tableofcontents /
	// \listoffigures / \listoftables — LaTeX's .toc/.lof/.lot two-pass mechanism.
	tocEntries []tocEntry // recorded during the current run
	tocSource  []tocEntry // entries carried from the aux pass, rendered by \tableofcontents

	// index (see index.go): \makeindex enables collection, \index records entries
	// (carried from the aux pass into the render pass exactly like tocEntries),
	// and \printindex typesets a sorted, grouped index — LaTeX's .idx two-pass.
	indexEnabled bool         // set by \makeindex; when false \index is a no-op
	indexEntries []indexEntry // recorded during the current run
	indexSource  []indexEntry // entries carried from the aux pass, rendered by \printindex

	// footnotes (see footnote.go): a counter, bodies awaiting attachment to the
	// vertical list, and a guard so a footnote's own paragraph build doesn't
	// recursively flush pending footnotes.
	footnoteCounter  int
	pendingFootnotes []*boxNode
	buildingFootnote bool
	noBase           bool // when true, getNext does not fall through to the base string
	negateNextIf     int  // pending \unless prefixes (e-TeX): reverse the next conditional
	allocCnt         int  // next free \count register handed out by \newcount
	allocDim         int  // next free \dimen register handed out by \newdimen
	allocSkp         int  // next free \skip register handed out by \newskip
	allocBox         int  // next free \box register handed out by \newsavebox

	// token registers (see toks.go): \toks<n> / \newtoks-allocated registers store
	// a token list each. A class's title/mark machinery (amsart's \andify, \toks@,
	// \@temptokena) reads and rewrites them via \the\toks@ and \toks@\expandafter{…}.
	// Stored globally (no group save-stack) — enough for the single-pass title
	// assembly they perform.
	toks      [][]tok
	allocToks int

	// page style (see pagenum.go): the page builder places a centred page number at
	// the foot of each page when pageStyle is not "empty". pageNumStyle formats it.
	pageStyle    string // "plain" ⇒ bottom-centred number; "empty" (default) ⇒ none
	pageNumStyle byte   // page-number format: 'a' arabic (default), 'r'/'R' roman, 'l'/'L' alph
	today        string // \today text (from Options.Date); empty ⇒ \today expands to nothing

	// page background colour (see color.go): \pagecolor fills the page; the drivers
	// paint it behind the content. hasPageColor distinguishes "unset" from black.
	pageColor    uint32
	hasPageColor bool

	// running headers/footers (see fancyhdr.go): with \pagestyle{fancy}, six fields
	// (header/footer × left/centre/right) are typeset at the top and foot of each
	// page. curPageNum is the ordinal being assembled, so \thepage in a field (or in
	// the plain foot number) reflects the real page.
	fancyHF    [6][]tok // 0 hl,1 hc,2 hr,3 fl,4 fc,5 fr
	headRule   int      // \headrulewidth (sp; a rule under the header)
	footRule   int      // \footrulewidth (sp; a rule over the footer)
	curPageNum int      // page ordinal during assembly (0 ⇒ unknown ⇒ 1)

	// enumitem (see enumitem.go): the last counter value each enumerate nesting
	// level reached, keyed by the kernel suffix (i, ii, …), so a [resume] list can
	// continue from it. Recorded by \@enumitemrec at every \end{enumerate}.
	enumitemLast map[string]int

	// lenient mode (see Options.Lenient): when set, an undefined control sequence
	// is skipped (with its likely argument block) instead of aborting, and its
	// name is tallied here for reporting. nil until the first skip.
	lenient   bool
	skippedCS map[string]int

	// class/package loading (see packages.go): the stack of files being \input as
	// classes/packages (each restores @'s catcode when done), the loaded registry,
	// options queued by \PassOptionsTo*, and a depth that makes loading tolerant of
	// commands the engine lacks (so a real .cls contributes what it can).
	loadStack      []loadFrame
	loadDepth      int
	loadedPackages map[string]bool
	passedOptions  map[string][]string
	lccode         map[rune]int // \lccode: what \lowercase maps a character to
	uccode         map[rune]int // \uccode: what \uppercase maps a character to
	afterGroup     [][]tok      // \aftergroup tokens, one list per open group
	inputNL        []int        // newlines of each \input file still being read (see endInput)
	loadedNL       int          // newlines in fully-loaded class/package files, subtracted from the document's source lines (see setSrcPos)

	// runaway guard: a bound on macro expansion so a pathological input (an
	// infinite \def loop, or a tolerantly-skipped arg-consuming command that
	// leaves a self-referential token behind) cannot hang. steps counts
	// expansions; runaway trips when steps or the input-stack depth exceed their
	// limits, stopping the loop (partial output in tolerant mode, an error in
	// strict mode).
	steps     int
	stepLimit int // absolute expansion ceiling (New sets maxExpandSteps; tests may lower it)
	runaway   bool
	// endlineReg is the \count register \endlinechar is bound to (see
	// loadTeXParams), cached because the mouth reads it at every line end.
	endlineReg int
	// condOpen counts the conditionals whose \else/\fi is still to come, and
	// condMarks records that count at the start of each conditional-operand scan.
	// Together they decide when TeX's "insert \relax" rule applies — see
	// insertRelax in primitives.go.
	condOpen  int
	condMarks []int

	// argRunaway: set when a delimited macro-argument scan reaches the end of the
	// file that began it without finding its delimiter (a TeX "Runaway argument").
	// matchParams stops grabbing and expandMacro abandons the call, pushing the
	// consumed tokens back so the rest of the file (and the document after it)
	// processes normally instead of being swallowed.
	argRunaway bool

	// tight-loop guard: TeX-style "no forward progress" detection. A pathological
	// loop (a self-referential macro, or a peeking idiom the kernel helpers
	// approximate imperfectly — e.g. amsart's \newtheorem…[section]) churns
	// expansion on the input stack while the mouth consumes NO new base input, so
	// e.bpos stays put. We count expansion steps taken with e.bpos unmoved and trip
	// the guard once that no-progress run exceeds tightLimit — catching such loops
	// in a fraction of a second, far below the coarse absolute ceiling. A legitimate
	// document (however large) keeps reading base input, which advances e.bpos and
	// resets the counter, so it never false-trips. e.bpos is monotonic within a Run
	// (Run resets it to 0; class/package loading splices file bodies into e.base at
	// e.bpos and scanning advances through them), which makes it a sound progress
	// signal even while a heavy .cls is loading.
	afterToken  *tok // token saved by \afterassignment, inserted after the next one
	expandDepth int  // >0 while an isolated expansion (\edef/\message) is running
	progBpos    int  // e.bpos at the last observed forward progress
	noProgSteps int  // expansion steps since e.bpos last advanced
	tightLimit  int  // no-progress ceiling (New sets tightLoopSteps; tests may adjust)
}

const (
	maxExpandSteps = 60_000_000 // absolute expansion ceiling; a large real document stays well under it
	maxInputDepth  = 200_000    // input-stack depth ceiling (catches immediate left-recursion)
	tightLoopSteps = 2_000_000  // no-progress ceiling: expansion steps with no new base input consumed
	maxArgToks     = 2_000_000  // single-argument ceiling: a runaway argument (TeX §338) is aborted here
)

// tolerant reports whether an unimplemented construct should be skipped rather
// than aborting: either the caller asked for lenient mode, or a class/package file
// is being loaded (a real .cls/.sty always loads best-effort).
func (e *Engine) tolerant() bool { return e.lenient || e.loadDepth > 0 }

type mkind uint8

const (
	mMacro mkind = iota
	mPrim
	mLetChar  // \let to a character token
	mCharDef  // \chardef
	mCountRef // \countdef / \newcount — an alias for a \count register (code = index)
	mDimenRef // \dimendef / \newdimen — an alias for a \dimen register (code = index)
	mSkipRef  // \skipdef / \newskip — an alias for a \skip register (code = index)
	mToksRef  // \toksdef / \newtoks — an alias for a \toks register (code = index)
	mBoxRef   // \newsavebox — an alias for a \box register (code = index)
	mFont     // a font-switching control sequence defined by \font
	mUndef
)

type meaning struct {
	kind   mkind
	params []tok // macro parameter text
	body   []tok // macro replacement text

	// optArg marks a LaTeX-style macro whose first parameter is optional: at call
	// time #1 is taken from a bracketed [..] argument if one is present, otherwise
	// from optDefault. The remaining params[1:] are grabbed as usual.
	optArg     bool
	optDefault []tok
	prim       func(e *Engine)
	name       string // primitive name (for \meaning/\string)
	ch         rune   // let-char / chardef code
	cat        cat
	code       int
	// mathChar marks a \mathchardef constant (as against \chardef): the two read as
	// the same integer but are different meanings to \ifx and \meaning.
	mathChar bool
	font     fontFace // mFont: the font this cs selects
}

type saveItem struct {
	kind    int // 0=eqtb 1=count 2=catcode 3=dimen 4=skip 5=font 6=leftskip 7=rightskip
	name    string
	old     *meaning
	idx     int
	oldi    int
	oldc    cat
	oldd    int
	oldg    glueSpec
	oldf    fontFace
	oldtoks []tok
}

// New builds an engine with TeX's default category codes and primitives loaded.
func New() *Engine {
	e := &Engine{eq: map[string]*meaning{}, allocCnt: 10, allocDim: 10, allocSkp: 10, allocBox: 10, allocToks: 10, toks: make([][]tok, 256)} // allocators start at 10
	e.lccode = map[rune]int{}
	e.uccode = map[rune]int{}
	e.hsize = ptToSP(6.5 * 7227.0 / 100.0) // plain TeX \hsize = 6.5in
	e.vsize = ptToSP(8.9 * 7227.0 / 100.0) // plain TeX \vsize = 8.9in
	e.baselineskip = 12 * unity            // 12pt
	e.baseBaselineskip = e.baselineskip    // setspace 1.0 ref
	e.lineskip = unity                     // 1pt
	e.parindent = 20 * unity               // plain TeX \parindent = 20pt
	e.columnsep = 10 * unity               // LaTeX \columnsep = 10pt (\columnseprule defaults to 0)
	e.hyphenpenalty = 50                   // plain TeX \hyphenpenalty
	e.prevDepth = ignoreDepth
	e.stepLimit = maxExpandSteps // absolute runaway-expansion ceiling (tests may lower it)
	e.tightLimit = tightLoopSteps
	e.pageStyle = "empty" // no page number until \pagestyle{plain}/\pagenumbering
	e.pageNumStyle = 'a'  // arabic page numbers by default
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
	// Carriage return is TeX's end-of-line character: TeX turns the end of every
	// input line into char 13 and gives it catcode 5. This engine reads a line
	// ending as \n instead (see normalizeEOL), but char 13 keeps its catcode so
	// that a package asking `\catcode`\^^M=12` — beamer does, to read a line
	// verbatim — sees the value TeX shows, and CHANGES it, rather than assigning
	// over a table entry that was already 12.
	e.catcode['\r'] = catEOL
	e.catcode['#'] = catParam
	e.catcode['^'] = catSup
	e.catcode['_'] = catSub
	e.catcode[' '] = catSpace
	e.catcode['\t'] = catSpace
	e.catcode['~'] = catActive
	e.catcode['%'] = catComment
	e.loadPrimitives()
	e.loadMore()
	e.loadClassPrims()
	e.loadToksPrims()
	e.loadAMSPrims()
	return e
}

// Run tokenizes src as the base input and processes it to completion, returning
// the accumulated \message output.
func (e *Engine) Run(src string) (string, error) {
	e.base = []rune(src)
	e.bpos = 0
	e.progBpos = 0    // fresh document: reset the no-progress guard
	e.noProgSteps = 0 // (e.bpos is monotonic within a Run, so this is a clean baseline)
	e.loadedNL = 0    // fresh document: no loaded-file lines discounted yet
	e.inputNL = nil
	e.afterGroup = nil
	e.buildLineStarts()
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
		start := e.bpos
		r, after, have := e.mouthChar(e.bpos)
		if !have {
			e.bpos = after // \endlinechar is out of range: this line end is nothing
			continue
		}
		c := e.catOf(r)
		switch c {
		case catEsc:
			e.setSrcPos(start)
			e.bpos = after
			return e.scanCS(), true
		case catComment:
			// A comment runs to the end of the line. In TeX the terminating
			// end-of-line is consumed by the comment — it produces NO interword
			// space — and the next line begins in state N, where leading spaces
			// are ignored. So `\def\x#1%⏎   {…}` defines an UNDELIMITED macro
			// (the `%` is exactly what suppresses the space that the line break
			// would otherwise leave in the parameter text), and `foo%⏎bar` joins
			// as `foobar`. A following blank line still breaks the paragraph.
			e.setSrcPos(start)
			for e.bpos < len(e.base) && e.base[e.bpos] != '\n' {
				e.bpos++
			}
			newlines := 0
			for e.bpos < len(e.base) {
				rr, nx, ok := e.mouthChar(e.bpos)
				if !ok {
					e.bpos = nx
					continue
				}
				cc := e.catOf(rr)
				if cc == catEOL {
					newlines++
					e.bpos = nx
					continue
				}
				if cc == catSpace {
					e.bpos = nx
					continue
				}
				break
			}
			if newlines >= 2 {
				return csTok("par"), true
			}
		case catSpace, catEOL:
			e.setSrcPos(start)
			// Collapse a run of spaces and line-endings into one token. A single
			// line-ending is interword space; two or more (a blank line) is a
			// paragraph break, which TeX turns into \par.
			newlines := 0
			if c == catEOL {
				newlines++
			}
			e.bpos = after
			for e.bpos < len(e.base) {
				rr, nx, ok := e.mouthChar(e.bpos)
				if !ok {
					e.bpos = nx
					continue
				}
				cc := e.catOf(rr)
				if cc != catSpace && cc != catEOL {
					break
				}
				if cc == catEOL {
					newlines++
				}
				e.bpos = nx
			}
			if newlines >= 2 {
				return csTok("par"), true
			}
			return chTok(' ', catSpace), true
		case catIgnore:
			e.bpos = after
		default:
			e.setSrcPos(start)
			e.bpos = after
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
	r, after, have := e.mouthChar(e.bpos)
	if !have {
		e.bpos = after
		return csTok("")
	}
	if e.catOf(r) != catLetter {
		e.bpos = after
		return csTok(string(r))
	}
	var name []rune
	for e.bpos < len(e.base) {
		rr, nx, ok := e.mouthChar(e.bpos)
		if !ok || e.catOf(rr) != catLetter {
			break
		}
		name = append(name, rr)
		e.bpos = nx
	}
	// a control word absorbs following spaces
	for e.bpos < len(e.base) {
		rr, nx, ok := e.mouthChar(e.bpos)
		if !ok {
			e.bpos = nx
			continue
		}
		cc := e.catOf(rr)
		if cc != catSpace && cc != catEOL {
			break
		}
		e.bpos = nx
	}
	return csTok(string(name))
}

// rawAt reads the character at index i, resolving TeX's ^^ notation (TeX §352),
// and returns it with the index just past what it consumed.
//
// A character whose catcode is 7 (superscript — "^" by default), doubled, escapes
// the character that follows: ^^ then TWO lowercase hex digits is that character
// code, otherwise the single following character is shifted by 64 — so ^^M is
// carriage return, ^^I is tab, ^^J is line feed and ^^7e is "~". This is how a
// package writes a control character it cannot type: beamer's
//
//	\catcode`\^^M=12
//
// asks for the code of carriage return, and without the notation the engine read
// a "^" and typeset the rest, putting a stray "M=12" on the first page of every
// beamer talk.
//
// The notation is resolved at the point a character is FETCHED, so it works
// everywhere: in ordinary text, in a control-sequence name, and in the argument
// of \catcode.
func (e *Engine) rawAt(i int) (rune, int) {
	if i >= len(e.base) {
		return 0, i + 1
	}
	r := e.base[i]
	if e.catOf(r) != catSup || i+2 >= len(e.base) || e.base[i+1] != r {
		return r, i + 1
	}
	if i+3 < len(e.base) {
		if h1, ok1 := lowerHexVal(e.base[i+2]); ok1 {
			if h2, ok2 := lowerHexVal(e.base[i+3]); ok2 {
				return rune(h1*16 + h2), i + 4
			}
		}
	}
	c := e.base[i+2]
	if c >= 128 {
		return r, i + 1 // not a character ^^ can shift; leave the "^" alone
	}
	if c < 64 {
		return c + 64, i + 3
	}
	return c - 64, i + 3
}

// endlinechar is the character TeX appends to every input line — 13 (^^M) unless
// a package changes it. A value outside 0..255 means "append nothing".
func (e *Engine) endlinechar() int {
	if e.endlineReg < 0 || e.endlineReg >= len(e.count) {
		return '\r'
	}
	return e.count[e.endlineReg]
}

// mouthChar returns the character the mouth sees at index i, the index just past
// it, and whether there is a character there at all.
//
// It resolves the two things TeX resolves at this level: the ^^ notation (rawAt),
// and the END OF LINE. TeX reads its input one LINE at a time and appends the
// character \endlinechar to each line; from then on that character acts on its
// CATCODE like any other. This engine keeps the whole source in one buffer with \n
// as the line separator, so the \n is presented as \endlinechar here.
//
// With the defaults — \endlinechar = 13 and \catcode13 = 5 — that is exactly the
// end-of-line token the mouth produced before. It matters when a package CHANGES
// them, and beamer does: it reads a line verbatim with \catcode`+"`"+`\^^M=12,
// \endlinechar=-1 and a macro delimited by ^^M. Against a hardwired end-of-line
// token that delimiter can never match, and the macro swallows the rest of the
// document — which is what happened to every line after \documentclass{beamer}.
func (e *Engine) mouthChar(i int) (rune, int, bool) {
	if i >= len(e.base) {
		return 0, i + 1, false
	}
	if e.base[i] == '\n' {
		c := e.endlinechar()
		if c < 0 || c > 255 {
			return 0, i + 1, false // \endlinechar out of range: the line just ends
		}
		return rune(c), i + 1, true
	}
	r, next := e.rawAt(i)
	return r, next, true
}

// lowerHexVal reports the value of a LOWERCASE hex digit, as TeX's ^^ notation
// requires: ^^4A is not hex (A is uppercase), it is "^^4" followed by "A".
func lowerHexVal(r rune) (int, bool) {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0'), true
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10, true
	}
	return 0, false
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
	if global {
		e.forgetSaved(0, 0, name)
	} else if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 0, name: name, old: e.eq[name]})
	}
	e.eq[name] = m
}

// forgetSaved drops the pending restores for one quantity, which is what makes an
// assignment *global*: the value must outlive every group that is open, so a
// local assignment made earlier in one of them must no longer be restored over
// it at the closing brace. Without this, {\x=1 \global\x=2 } would leave \x at
// its value from before the group — and the idiom that carries a result out of a
// group (pgf's \pgf@process does exactly {…\global\pgf@x=\pgf@x}) would return
// nothing.
func (e *Engine) forgetSaved(kind, idx int, name string) {
	if len(e.save) == 0 {
		return
	}
	out := e.save[:0]
	for _, it := range e.save {
		if it.kind == kind && ((kind == 0 && it.name == name) || (kind != 0 && it.idx == idx)) {
			continue
		}
		out = append(out, it)
	}
	e.save = out
}

func (e *Engine) setCount(i, v int, global bool) {
	if global {
		e.forgetSaved(1, i, "")
	} else if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 1, idx: i, oldi: e.count[i]})
	}
	e.count[i] = v
}

func (e *Engine) setDimen(i, v int, global bool) {
	if i < 0 || i >= 256 {
		return
	}
	if global {
		e.forgetSaved(3, i, "")
	} else if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 3, idx: i, oldd: e.dimen[i]})
	}
	e.dimen[i] = v
}

func (e *Engine) setSkip(i int, v glueSpec, global bool) {
	if i < 0 || i >= 256 {
		return
	}
	if global {
		e.forgetSaved(4, i, "")
	} else if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 4, idx: i, oldg: e.skip[i]})
	}
	e.skip[i] = v
}

// selectFont makes f the current font, saving the previous one for restoration
// at the end of the current group (so { \bf … } reverts afterwards).
func (e *Engine) selectFont(f fontFace) {
	if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 5, oldf: e.curFont})
	}
	e.curFont = f
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

func (e *Engine) beginGroup() {
	e.groups = append(e.groups, len(e.save))
	e.afterGroup = append(e.afterGroup, nil)
}

func (e *Engine) endGroup() {
	if len(e.groups) == 0 {
		return
	}
	mark := e.groups[len(e.groups)-1]
	e.groups = e.groups[:len(e.groups)-1]
	after := e.takeAfterGroup()
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
		case 5:
			e.curFont = s.oldf
		case 6:
			e.leftskip = s.oldg
		case 7:
			e.rightskip = s.oldg
		case 8:
			e.curColor = uint32(s.oldi)
		case 9:
			e.everypar = s.oldtoks
		}
	}
	// \aftergroup's tokens are put back once the group is closed and its values
	// restored, so they act outside it — which is what makes them useful: a
	// package builds something inside a box and arranges for the code that
	// finishes it to run after the box is complete (every TikZ node ends that
	// way). They are inserted in the order they were saved.
	for i := len(after) - 1; i >= 0; i-- {
		e.back(after[i])
	}
}

// afterGroupToken saves a token for insertion when the current group ends. With
// no group open TeX inserts it immediately, since there is nothing to wait for.
func (e *Engine) afterGroupToken(t tok) {
	if n := len(e.afterGroup); n > 0 {
		e.afterGroup[n-1] = append(e.afterGroup[n-1], t)
		return
	}
	e.back(t)
}

// takeAfterGroup pops the tokens saved for the group being closed.
func (e *Engine) takeAfterGroup() []tok {
	n := len(e.afterGroup)
	if n == 0 {
		return nil
	}
	ts := e.afterGroup[n-1]
	e.afterGroup = e.afterGroup[:n-1]
	return ts
}

// setEverypar assigns the \everypar hook, recording the old value on the save
// stack so a group (e.g. a list environment) restores it at \endgroup.
func (e *Engine) setEverypar(ts []tok) {
	if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 9, oldtoks: e.everypar})
	}
	e.everypar = ts
}

// ── expansion ───────────────────────────────────────────────────────────────

// getXToken returns the next token, expanding expandable control sequences
// (macros and expandable primitives) until a non-expandable token surfaces.
func (e *Engine) getXToken() (tok, bool) {
	for {
		if e.runaway {
			return tok{}, false
		}
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
			if e.stepOverrun() || len(e.lists) > maxInputDepth {
				e.tripRunaway()
				return tok{}, false
			}
			e.expandMacro(m)
		case mPrim:
			if isExpandable(m.name) {
				if e.stepOverrun() {
					e.tripRunaway()
					return tok{}, false
				}
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
	var args [][]tok
	if m.optArg && len(m.params) > 0 {
		// #1 is optional: read a bracketed [..] argument if present, else the
		// default. The rest of the parameters are grabbed normally.
		args = append(args, e.grabOptArg(m.optDefault))
		args = append(args, e.matchParams(m.params[1:])...)
	} else {
		args = e.matchParams(m.params)
	}
	if e.argRunaway {
		// A delimited argument ran away to a file-end sentinel: abandon this call.
		// grabDelimited has already reinserted the scanned tokens, so just drop the
		// macro (its body is not run) and let them process normally.
		e.argRunaway = false
		return
	}
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
	// A parameter text ending in "#{" (see scanDefText) puts a brace marker last:
	// the final argument runs up to the next opening brace, which stays in the
	// input.
	braceEnd := false
	if n := len(params); n > 0 && params[n-1].cat == catParam && !params[n-1].cs_ && params[n-1].ch == '{' {
		braceEnd = true
		params = params[:n-1]
	}
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
			switch {
			case len(delim) == 0 && braceEnd && j == len(params):
				args = append(args, e.grabUntilBrace())
			case len(delim) == 0:
				args = append(args, e.grabUndelimited())
			default:
				args = append(args, e.grabDelimited(delim))
			}
			if e.argRunaway {
				return args // a runaway arg abandons the call (expandMacro checks the flag)
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

// grabOptArg reads the optional first argument of a LaTeX macro: the content of a
// bracketed [..] group when one follows, otherwise the supplied default tokens.
func (e *Engine) grabOptArg(def []tok) []tok {
	if toks, ok := e.scanOptBracketToks(); ok {
		return toks
	}
	return def
}

// grabUntilBrace reads the argument of a "#{" parameter: everything up to the
// next opening brace, which is left in the input (TeX §399 — the brace delimits
// the argument and simultaneously opens the group that follows).
func (e *Engine) grabUntilBrace() []tok {
	var arg []tok
	for {
		t, ok := e.getNext()
		if !ok {
			return arg
		}
		if t.cat == catBegin && !t.cs_ {
			e.back(t)
			return arg
		}
		arg = append(arg, t)
		if e.argOverrun(arg) {
			return arg
		}
	}
}

// argOverrun reports a runaway argument (TeX §338): a macro argument grabbed
// while no base input is consumed, past the point where that can be legitimate.
// It charges each grabbed token to the same no-progress guard that catches a
// self-expanding macro (stepOverrun): while grabbing reads new base input the
// counter keeps resetting, so an ordinary — even very large — argument never
// trips, but an argument assembled entirely from re-pushed tokens does. A class
// whose kernel the engine models only in part supplies exactly such a case:
// revtex/aastex adds \rvtx@enddocument@patch (parameter text "#1#2\@checkend#3")
// to the enddocument hook, where it re-grabs and re-scans forever hunting for a
// \@checkend{document} the engine's simplified \enddocument never emits — the
// mouth already at end of file, so nothing advances while the loop churns. A hard
// maxArgToks ceiling backs the guard up for the case a single grab balloons. When
// either trips, tripRunaway unwinds the input so what was built still renders.
func (e *Engine) argOverrun(arg []tok) bool {
	if len(arg) > maxArgToks || e.stepOverrun() {
		e.tripRunaway()
		return true
	}
	return false
}

func (e *Engine) grabUndelimited() []tok {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok {
		return nil
	}
	// Never swallow a file-end sentinel as an argument (see grabDelimited): a macro
	// whose earlier delimited parameter ran away leaves the scan at the sentinel;
	// grabbing it here would drop the file's end and desynchronise the input.
	if isFileEndSentinel(t) {
		e.argRunaway = true
		e.back(t)
		return nil
	}
	if t.cat == catBegin && !t.cs_ {
		return e.grabGroup()
	}
	return []tok{t}
}

// isFileEndSentinel reports whether t is one of the control sequences the splicer
// appends at the end of a loaded file (\gotexendinput / \@endofpackagehook /
// \@endofclasshook, see io.go). These are hard "stop reading THIS file" markers.
func isFileEndSentinel(t tok) bool {
	if !t.cs_ {
		return false
	}
	switch t.cs {
	case "gotexendinput", "@endofpackagehook", "@endofclasshook", "@gotex@endload":
		return true
	}
	return false
}

func (e *Engine) grabDelimited(delim []tok) []tok {
	var arg []tok
	depth := 0
	for {
		if e.argOverrun(arg) {
			return arg
		}
		t, ok := e.getNext()
		if !ok {
			return arg
		}
		// A delimited argument must not run past the end of the file that began it:
		// if the closing delimiter is never found, scanning used to consume the
		// file-end sentinel and every token of the PARENT file after it, swallowing
		// the rest of the document. This bites real class code — amsart's unguarded
		// \expandafter\@tempa\[\@nil display-math patch, where \@tempa's #1$… scan
		// finds no $ because the engine's \[ is a primitive rather than a $$-macro.
		// TeX reports "Runaway argument" here; the engine abandons the macro call
		// (matchParams/expandMacro) and reinserts everything scanned so the file's
		// tail and the document after the sentinel process normally.
		if isFileEndSentinel(t) {
			e.argRunaway = true
			e.push(append(arg, t))
			return nil
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

// stripOuterBraces removes one level of braces when the whole argument is a single
// enclosing group — TeX strips the braces of a delimited macro argument that is
// wholly wrapped in one matching { } pair (so \@oparg{\@ynthm{thm}}[] delivers
// \@ynthm{thm}, not {\@ynthm{thm}}; the latter re-braces the token and derails a
// downstream delimited match — the amsart \newtheorem loop). It must NOT strip when
// the leading { closes before the end (e.g. {a}{b}), where the braces are not a
// single enclosing pair.
func (e *Engine) stripOuterBraces(arg []tok) []tok {
	if len(arg) < 2 || !(arg[0].cat == catBegin && !arg[0].cs_) || !(arg[len(arg)-1].cat == catEnd && !arg[len(arg)-1].cs_) {
		return arg
	}
	depth := 0
	for i, t := range arg {
		switch {
		case t.cat == catBegin && !t.cs_:
			depth++
		case t.cat == catEnd && !t.cs_:
			depth--
			if depth == 0 && i != len(arg)-1 {
				return arg // the first group closes before the end: not one enclosing pair
			}
		}
	}
	return arg[1 : len(arg)-1]
}

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
		if e.argOverrun(g) {
			return g
		}
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
		if !e.stepToken(t) {
			return
		}
	}
}

// implicitChar resolves an implicit character token: a control sequence \let to a
// character (mLetChar, e.g. \bgroup={ or \egroup=}). In the stomach and while
// scanning box material such a token acts as its character — opening or closing a
// group and so on — exactly as TeX treats implicit character tokens. Meaning-level
// operators (\ifx, \let, \string, \meaning) are unaffected: they read tokens
// without dispatching them through here. Returns the character token and true when
// t is such a control sequence.
func (e *Engine) implicitChar(t tok) (tok, bool) {
	if !t.cs_ {
		return t, false
	}
	if m := e.meaningOf(t); m != nil && m.kind == mLetChar {
		return tok{ch: m.ch, cat: m.cat}, true
	}
	return t, false
}

// stepToken processes one already-expanded token in the main horizontal/vertical
// loop: grouping, characters, interword space, math shift, or a control sequence.
// It returns false when a primitive asks the loop to stop. Shared by mainLoop and
// the sandboxed builds (e.g. a footnote body) that run material to completion.
func (e *Engine) stepToken(t tok) bool {
	if c, ok := e.implicitChar(t); ok {
		t = c // an implicit { / } / space etc. acts as its character in the stomach
	}
	if !t.cs_ {
		switch t.cat {
		case catBegin:
			e.beginGroup()
		case catEnd:
			e.endGroup()
		case catLetter, catOther, catParam:
			// catParam here is a stray literal '#' (a ## reduced by scanBody that was
			// not consumed as a parameter char by a nested \def); typeset it as text.
			e.startChar(t.ch) // begin/continue a paragraph in horizontal mode
		case catSpace:
			if e.inPar && e.curFont != nil {
				e.parList = append(e.parList, glueNode{spec: e.curFont.spaceSP()})
			}
		case catMath:
			e.doMath()
		}
		return true
	}
	return e.execCS(t)
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

// charNodeFor builds a measured character node for the current font, or reports
// ok=false when there is no font to measure with.
func (e *Engine) charNodeFor(ch rune) (charNode, bool) {
	if e.curFont == nil {
		return charNode{}, false
	}
	w, h, d := e.curFont.charDimsSP(ch)
	return charNode{ch: ch, width: w, height: h, depth: d, srcLine: e.curSrcLine}, true
}

// appendChar appends a measured character to a horizontal list. First it applies
// TeX's text ligatures and quote/dash forms (see ligature.go): a pair that forms a
// ligature folds into the trailing character, a lone ` or ' becomes a curly quote.
// Then rawAppendChar sets the resulting glyph with the font's inter-character kern.
func (e *Engine) appendChar(list []node, ch rune) []node {
	if prev, idx, ok := trailingChar(list); ok {
		if lig, ok := e.ligature(prev, ch); ok {
			return e.rawAppendChar(list[:idx], lig) // replace the trailing char with the ligature
		}
	}
	return e.rawAppendChar(list, e.singleForm(ch))
}

// rawAppendChar sets a single glyph, inserting the font's inter-character kern
// before it when the previous node is a character (TeX's font kern program).
func (e *Engine) rawAppendChar(list []node, ch rune) []node {
	if prev, ok := lastChar(list); ok {
		if k := e.curFont.kernSP(prev, ch); k != 0 {
			list = append(list, kernNode{width: k})
		}
	}
	w, h, d := e.curFont.charDimsSP(ch)
	return append(list, charNode{ch: ch, width: w, height: h, depth: d, srcLine: e.curSrcLine, size: e.curFont.sizePt(), color: e.curColor})
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
	// Fire \everypar at the very start of the paragraph, before any of its text —
	// TeX inserts the hook tokens as horizontal material. Running them to
	// completion here (into parList) keeps them ahead of the character that began
	// the paragraph. inPar is already set, and inEverypar guards against a \par or
	// paragraph-start inside the hook re-triggering it.
	if len(e.everypar) > 0 && !e.inEverypar {
		hook := e.everypar
		e.inEverypar = true
		e.execToks(hook)
		e.inEverypar = false
	}
}

// execToks runs a token list to completion in the current stomach mode, stopping
// at a private sentinel rather than falling through to the base input. It routes
// each token through stepToken, so the tokens act exactly as if they appeared in
// the source at this point (used to fire \everypar).
func (e *Engine) execToks(ts []tok) {
	e.push(append(append([]tok(nil), ts...), sentinel))
	for e.err == nil {
		t, ok := e.getXToken()
		if !ok || (t.cs_ && t.cs == sentinel.cs) {
			return
		}
		if !e.stepToken(t) {
			return
		}
	}
}

// execCS performs one control-sequence token (an assignment or non-expandable
// primitive). It returns false on a fatal error (undefined cs). Expandable
// tokens never reach here — getXToken has already expanded them. Both the main
// loop and box building route control sequences through this one dispatch.
func (e *Engine) execCS(t tok) bool {
	m := e.meaningOf(t)
	if m == nil {
		if e.tolerant() {
			e.skipUndefined(t.cs)
			return true
		}
		e.fail("Undefined control sequence \\" + t.cs)
		return false
	}
	if m.kind == mPrim && m.name == "afterassignment" {
		m.prim(e)
		return true
	}
	defer e.flushAfterAssignment(m)
	switch m.kind {
	case mCountRef:
		e.countRefAssign(m.code, false) // \n=<v>
	case mDimenRef:
		e.dimenRefAssign(m.code, false) // \d=<dimen>
	case mSkipRef:
		e.skipRefAssign(m.code, false) // \s=<glue>
	case mToksRef:
		e.toksRefAssign(m.code) // \t{<toks>} or \t\otherreg
	case mFont:
		e.selectFont(m.font) // \rm etc. selects the current font (group-scoped)
	case mPrim:
		if !isExpandable(m.name) {
			m.prim(e)
		}
	}
	return true
}

// skipUndefined handles an undefined control sequence in lenient mode: it tallies
// the name and swallows the command's likely argument block so a stray
// \somemacro[opt]{body} does not spill "opt"/"body" onto the page as text. The
// arity of an unknown macro is unknowable, so this is a heuristic: an optional
// star, then any run of immediately-following [optional] and {mandatory}
// arguments. A no-argument command (\somemacro Word) consumes nothing extra, so
// "Word" is typeset normally.
func (e *Engine) skipUndefined(name string) {
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	e.skippedCS[name]++
	// optional star form (\cmd*)
	if t, ok := e.getNext(); ok {
		if t.cs_ || t.ch != '*' {
			e.back(t)
		}
	}
	// any run of [optional] then {mandatory} arguments, in either order
	for {
		if _, ok := e.scanOptBracketToks(); ok {
			continue
		}
		e.skipOptSpace()
		t, ok := e.getNext()
		if !ok {
			return
		}
		if !t.cs_ && t.cat == catBegin {
			e.grabGroup()
			continue
		}
		e.back(t)
		return
	}
}

// SkippedCommands returns, for a lenient compile, how many times each undefined
// control sequence was skipped (empty when strict or when none were undefined).
// It lets a caller surface "these commands were dropped" after a preview compile.
func (e *Engine) SkippedCommands() map[string]int { return e.skippedCS }

// stepOverrun tallies one expansion step and reports whether the runaway guard
// should trip. Two independent conditions fire it: the absolute ceiling
// (stepLimit — a coarse backstop) and the tight-loop guard. The latter measures
// TeX-style forward progress by watching e.bpos, the mouth's position in the base
// input: every step taken with e.bpos unmoved bumps noProgSteps, and any advance
// of e.bpos resets it. A non-terminating expansion churns the input stack without
// reading new base input, so noProgSteps climbs to tightLimit in a fraction of a
// second; a legitimate document keeps consuming base input, which resets the
// counter long before it reaches tightLimit, so it never false-trips.
func (e *Engine) stepOverrun() bool {
	e.steps++
	if e.bpos > e.progBpos { // the mouth consumed new base input: real forward progress
		e.progBpos = e.bpos
		e.noProgSteps = 0
	} else {
		e.noProgSteps++
	}
	return e.steps > e.stepLimit || e.noProgSteps > e.tightLimit
}

// tripRunaway halts expansion when the step/depth guard fires: it discards the
// pending input so the loop unwinds, and (in strict mode only) records the error.
// In tolerant mode the partial document built so far is still rendered.
func (e *Engine) tripRunaway() {
	e.runaway = true
	e.lists = nil
	e.noBase = true
	if !e.tolerant() {
		e.fail("runaway expansion: aborted after too many macro expansions (possible infinite loop)")
	}
}

func (e *Engine) fail(msg string) {
	if e.err == nil {
		e.err = SourceError{Line: e.curSrcLine, Col: e.curSrcCol, Msg: msg}
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
				// A box-register handle from \newbox is a register *number*: TeX
				// allocates it with \chardef, so \box\mybox, \wd\mybox and
				// \setbox\mybox all read it as the integer it stands for.
				if m.kind == mBoxRef {
					return sign * m.code
				}
				if m.kind == mCountRef {
					return sign * e.count[m.code]
				}
				if m.kind == mPrim && m.name == "count" {
					return sign * e.count[e.scanInt()]
				}
				// TeX coerces an internal dimension (or glue) to an integer: its
				// value in scaled points. \number\pgf@x and \ifnum\wd0>0 both
				// rely on it, and a package that computes with lengths uses it
				// constantly.
				if e.isInternalDimen(t) && m.name != "dimexpr" {
					e.back(t)
					v, _ := e.scanDimenValue(false)
					return sign * v
				}
				if m.kind == mPrim && m.name == "catcode" {
					return sign * int(e.catcode[rune(e.scanInt())])
				}
				if m.kind == mPrim && m.name == "numexpr" {
					return sign * e.scanExpr(false)
				}
				if m.kind == mPrim && m.name == "dimexpr" {
					// A dimension used where an integer is wanted coerces to its
					// value in scaled points, as TeX's <internal dimen> does.
					return sign * e.scanExpr(true)
				}
			}
		}
		// TeX's alphabetic constant: `<character> or `<single-character control
		// sequence> is that character's code. It is how a source names a character
		// it cannot write as a number — \catcode`\%=14, \lccode`\a=`\A,
		// \chardef\bslash=`\\ — so a package that sets any catcode needs it.
		if t.is('`', catOther) {
			return sign * e.scanCharCode()
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

// assignmentPrims are the primitives that perform an assignment, after which a
// token saved by \afterassignment is inserted (TeX §1269). A register alias
// (\pgf@x=…) assigns too and is handled by kind, not by name.
var assignmentPrims = map[string]bool{
	"def": true, "gdef": true, "edef": true, "xdef": true, "let": true,
	"futurelet": true, "global": true, "chardef": true, "countdef": true,
	"dimendef": true, "skipdef": true, "toksdef": true, "newcount": true,
	"newdimen": true, "newskip": true, "newtoks": true, "newbox": true,
	"count": true, "dimen": true, "skip": true, "toks": true, "catcode": true,
	"advance": true, "multiply": true, "divide": true, "setbox": true,
	"font": true, "hsize": true, "vsize": true, "parindent": true,
	"baselineskip": true, "leftskip": true, "rightskip": true, "sfcode": true,
	"hskip": true, "vskip": true, "wd": true, "ht": true, "dp": true,
	"columnsep": true, "columnseprule": true,
}

// flushAfterAssignment inserts the token \afterassignment saved, once the
// assignment it was waiting for has been performed. TeX keeps exactly one such
// token, and it is inserted after the assignment, not before — which is what
// lets a macro see the value that was just assigned (pgf uses it to resume a
// scanner right after \let\next= has swallowed a token).
func (e *Engine) flushAfterAssignment(m *meaning) {
	if e.afterToken == nil {
		return
	}
	switch m.kind {
	case mCountRef, mDimenRef, mSkipRef, mToksRef, mFont:
	case mPrim:
		if !assignmentPrims[m.name] {
			return
		}
	default:
		return
	}
	t := *e.afterToken
	e.afterToken = nil
	e.back(t)
}

// scanCharCode reads the character after a ` : a character token gives its own
// code, and a control sequence gives the code of its single character (TeX reads
// this token unexpanded, so `\a is the letter a even when \a is a macro). A
// multi-letter control sequence is not a character constant and yields zero.
func (e *Engine) scanCharCode() int {
	t, ok := e.getNext()
	if !ok {
		return 0
	}
	code := 0
	if t.cs_ {
		r := []rune(t.cs)
		if len(r) != 1 {
			return 0
		}
		code = int(r[0])
	} else {
		code = int(t.ch)
	}
	e.skipOneOptSpace()
	return code
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
			// A skip register / LaTeX length used where a rigid <dimen> is wanted
			// coerces to its natural width component (TeX's glue→dimen coercion),
			// so \parbox{\len}, \framebox[\len] and \setlength{\x}{\len} all work.
			case m.kind == mSkipRef:
				return e.skip[m.code].width, 0
			case m.kind == mPrim && m.name == "skip":
				return e.skip[e.scanInt()].width, 0
			case m.kind == mPrim && m.name == "dimexpr":
				return e.scanExpr(true), 0
			case m.kind == mPrim && m.name == "numexpr":
				// An integer expression used as a dimension is a number of scaled
				// points, matching \dimexpr <number>sp.
				return e.scanExpr(false), 0
			case m.kind == mPrim && m.name == "dimen":
				return e.dimen[e.scanInt()], 0
			case m.kind == mPrim && m.name == "wd":
				return e.boxDim('w'), 0
			case m.kind == mPrim && m.name == "ht":
				return e.boxDim('h'), 0
			case m.kind == mPrim && m.name == "dp":
				return e.boxDim('d'), 0
			case m.kind == mPrim && m.name == "hsize":
				return e.hsize, 0
			case m.kind == mPrim && m.name == "vsize":
				return e.vsize, 0
			case m.kind == mPrim && m.name == "parindent":
				return e.parindent, 0
			}
			// An internal INTEGER here is the FACTOR of the dimension, not the
			// dimension itself: TeX's <dimen> is <factor><unit of measure>, and the
			// factor may be a count register or a \chardef'd constant, with the unit
			// following it — \dimen0=\count0\dimen1 is count0 times dimen1, and
			// \setlength\textheight{\@tempcnta\baselineskip} is how a class states
			// a height in lines.
			//
			// The unit must itself be a dimension for the factor to be read: TeX
			// also accepts a spelled-out one (\dimen0=\count0 pt is five points
			// when count0 is five), but taking that here disturbs a class file's
			// own parse in a way not yet understood, and every source that relies
			// on this writes the dimension form. When no dimension follows, the
			// input is put back untouched and the caller sees what it would have
			// seen before.
			if e.isInternalInteger(t) {
				e.back(t)
				mark := e.markInput()
				n := e.scanInt()
				if e.upcomingInternalDimen() {
					return e.applyUnit(n, 0), 0
				}
				e.restoreInput(mark)
				return 0, 0
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

// isInternalInteger reports whether a control sequence denotes an integer, so it
// can serve as the factor of a dimension.
func (e *Engine) isInternalInteger(t tok) bool {
	m := e.eq[t.cs]
	if m == nil {
		return false
	}
	switch m.kind {
	case mCountRef, mCharDef:
		return true
	case mPrim:
		switch m.name {
		case "count", "numexpr", "catcode", "lccode", "uccode":
			return true
		}
	}
	return false
}

// upcomingInternalDimen reports whether the input begins with a control sequence
// that denotes a dimension, without consuming anything. That is the unit a factor
// taken from an integer register is measured in — \setlength\textheight{\@tempcnta
// \baselineskip}, \pgfmath@ya=\c@pgfmath@counta\pgfmath@y — and restricting the
// factor form to it keeps every other reading of an integer untouched.
func (e *Engine) upcomingInternalDimen() bool {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return false
	}
	e.back(t)
	return t.cs_ && e.isInternalDimen(t)
}

// inputMark is a position in the input: where the mouth stands in the base text
// and what is stacked above it.
type inputMark struct {
	bpos  int
	lists [][]tok
}

// markInput records where the input stands, so a scanner can read ahead and then
// put back exactly what it read. Only the stack of pending lists is copied — the
// lists themselves are never written to, only sliced down as they are read.
func (e *Engine) markInput() inputMark {
	return inputMark{bpos: e.bpos, lists: append([][]tok(nil), e.lists...)}
}

// restoreInput returns the input to a recorded position.
func (e *Engine) restoreInput(m inputMark) {
	e.bpos = m.bpos
	e.lists = m.lists
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
	// Optional "true" prefix (TeX's magnification-independent units, e.g. .5truein).
	// The engine has no \mag, so a true<unit> length equals the plain <unit>; consume
	// the keyword and fall through to the ordinary unit scan.
	e.scanKeyword("true")
	a, ok := e.getXToken()
	if !ok {
		return intPart*unity + f // bare number ⇒ pt
	}
	if a.cs_ {
		// <factor><internal dimen>, e.g. 6\p@ = 6×1pt. The internal dimen provides
		// the unit; the factor (intPart.f) scales it. Common throughout real class
		// files (\@plus 1.5\p@, \leftmargini 2.5em stored as \p@ multiples).
		e.back(a)
		if v, isDimen := e.coerceInternalDimen(); isDimen {
			e.skipOneOptSpace()
			return int((int64(intPart)*int64(v)*unity + int64(f)*int64(v)) / unity)
		}
		e.back(a)
		return intPart*unity + f // a real cs that is not a dimen ⇒ bare number is pt
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
	case key == "em" || key == "ex":
		// Font-relative units: 1em ≈ the font size (quad), 1ex ≈ half of it.
		e.skipOneOptSpace()
		size := 10
		if e.curFont != nil {
			size = e.curFont.sizePt()
		}
		coeff := intPart*unity + f // the decimal coefficient × unity
		if key == "ex" {
			return coeff * size / 2
		}
		return coeff * size
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

// coerceInternalDimen reads a control sequence that denotes an internal dimension
// (a \newdimen register or dimen parameter, a \newskip register coerced to its
// width, or a \dimen/\skip/\wd/\ht/\dp primitive) and returns its value in sp. On a
// non-dimen (or non-cs) token it backs out and returns ok=false, so a caller can
// fall back. It is the unit half of TeX's <factor><internal dimen> (e.g. 6\p@).
func (e *Engine) coerceInternalDimen() (int, bool) {
	t, ok := e.getXToken()
	if !ok {
		return 0, false
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil {
			switch {
			case m.kind == mDimenRef:
				return e.dimen[m.code], true
			case m.kind == mSkipRef:
				return e.skip[m.code].width, true
			case m.kind == mPrim && m.name == "dimen":
				return e.dimen[e.scanInt()], true
			case m.kind == mPrim && m.name == "skip":
				return e.skip[e.scanInt()].width, true
			case m.kind == mPrim && m.name == "wd":
				return e.boxDim('w'), true
			case m.kind == mPrim && m.name == "ht":
				return e.boxDim('h'), true
			case m.kind == mPrim && m.name == "dp":
				return e.boxDim('d'), true
			case m.kind == mPrim && m.name == "hsize":
				return e.hsize, true
			case m.kind == mPrim && m.name == "vsize":
				return e.vsize, true
			case m.kind == mPrim && m.name == "parindent":
				return e.parindent, true
			case m.kind == mPrim && m.name == "baselineskip":
				return e.baselineskip, true
			case m.kind == mPrim && m.name == "leftskip":
				return e.leftskip.width, true
			case m.kind == mPrim && m.name == "rightskip":
				return e.rightskip.width, true
			case m.kind == mPrim && m.name == "dimexpr":
				return e.scanExpr(true), true
			}
		}
	}
	e.back(t)
	return 0, false
}

// scanKeyword tries to match the literal word (case-insensitively) after optional
// spaces, consuming it on success and backing out every token on failure.
func (e *Engine) scanKeyword(word string) bool {
	e.skipOptSpace()
	var buf []tok
	restore := func() bool {
		for k := len(buf) - 1; k >= 0; k-- {
			e.back(buf[k])
		}
		return false
	}
	// Match with expansion (as TeX does), so a keyword produced by a macro is seen:
	// LaTeX's \@plus expands to " plus" and \@minus to " minus". Skip a leading
	// space that such an expansion introduces before the first letter.
	leading := true
	for _, w := range word {
		t, ok := e.getXToken()
		for leading && ok && t.cat == catSpace && !t.cs_ {
			buf = append(buf, t)
			t, ok = e.getXToken()
		}
		leading = false
		if !ok {
			return restore()
		}
		buf = append(buf, t)
		if t.cs_ || lower(t.ch) != w {
			return restore()
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
			if !ok {
				continue
			}
			if n.cat == catBegin && !n.cs_ {
				// TeX's "#{": a parameter text ending in # is delimited by the
				// opening brace, which the macro does NOT consume — it is left in
				// the input, where it also opens the macro's own body. LaTeX's
				// \@yargd@f builds an n-argument definition this way, so without
				// it \newcommand's kernel path (and everything etoolbox defines on
				// top of it) silently defines nothing.
				params = append(params, tok{ch: '{', cat: catParam})
				body = e.scanBody()
				return
			}
			params = append(params, tok{ch: n.ch, cat: catParam})
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
				// ## → a single #, kept as a PARAMETER character (catParam), not catOther:
				// TeX's halving preserves the #'s parameter-ness so a nested definition
				// scanned from this body (amsart's \def\@andlistc##1{…##1…} inside
				// \newcommand\nxandlist) still sees #1 as a parameter. A stray # that
				// reaches the stomach is typeset as '#' by stepToken / the box builder.
				g = append(g, tok{ch: '#', cat: catParam}) // ## → #
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
