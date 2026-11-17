// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements \footnote — a simplified form of TeX's insertion
// mechanism. \footnote{text} places a raised reference number where it is called
// and typesets a numbered note that migrates to the bottom of the page the marker
// falls on. The note travels as a footnoteNode on the main vertical list whose
// page-height contribution reserves room for the note (so the page breaks early
// enough); the page builder then lifts the notes out of the content flow and
// stacks them below a short rule at the foot of the page.
//
// Not modelled (future work, matching the page-builder's own TODOs): splitting a
// long note across pages, \footnotemark/\footnotetext separation, and per-note
// \footnotesize. A note is set at the body size.

import (
	"strconv"
	"strings"
)

// footnoteNode carries a rendered footnote body down the main vertical list. It
// contributes its height to the page (reserving space) but is not painted inline;
// the page builder collects it into the foot of the page (see assemblePage).
type footnoteNode struct{ body *boxNode }

func (footnoteNode) isNode() {}

// footinsSkip is \skip\footins: the space a page gives up to the foot area the
// first time a footnote lands on it. ONCE per page, not once per note —
// tex.web:19638 reduces page_goal by width(skip n) inside "Create a page insertion
// node", which runs only for the FIRST \insert n of a page; every later insert
// costs its own height and nothing more (l.19613).
//
// size1x.clo:203 states it per class size (9 / 10 / 10.8pt); setPtsize puts those
// in the register. A zero reading means nobody set it — the register is allocated
// by the class kernel and was never given a value — so the 10pt default stands in
// rather than being taken for a document that wants no space at all.
func (e *Engine) footinsSkip() int {
	if m := e.eq["footins"]; m != nil && m.kind == mCountRef && m.code >= 0 && m.code < len(e.count) {
		if n := e.count[m.code]; n >= 0 && n < len(e.skip) {
			if w := e.skip[n].width; w > 0 {
				return w
			}
		}
	}
	return 9 * unity // size10.clo:203
}

// setFootinsSkip stores v in \skip\footins, through the \footins count the way a
// class would write it.
func (e *Engine) setFootinsSkip(v int) {
	if m := e.eq["footins"]; m != nil && m.kind == mCountRef && m.code >= 0 && m.code < len(e.count) {
		if n := e.count[m.code]; n >= 0 && n < len(e.skip) {
			e.skip[n] = glueSpec{width: v, stretch: 4 * unity, shrink: 2 * unity}
		}
	}
}

// doFootnote implements \footnote{text}: step the counter, typeset the numbered
// note into a vbox held until the enclosing paragraph attaches it to the vertical
// list, and drop a raised reference number at the current point.
func (e *Engine) doFootnote() {
	text := e.grabUndelimited()
	n := e.stepFootnote()
	mark := e.footnoteMark(n)
	e.queueFootnoteText(mark, text)
	e.emitFootnoteMark(mark)
}

// footnoteCounterIndex is the count register \c@footnote lives in, or -1 when no
// format declared the counter.
func (e *Engine) footnoteCounterIndex() int {
	if m := e.eq["c@footnote"]; m != nil && m.kind == mCountRef {
		return m.code
	}
	return -1
}

// stepFootnote advances the footnote counter and returns its new value.
//
// ⛔⛔ THE COUNTER WAS A GO int ON THE ENGINE, AND THAT IS WHY TWO SEPARATE DEFECTS
// COULD NOT BE FIXED. \thefootnote could not be consulted, because there was no
// \c@footnote for it to read; and \setcounter{footnote}{0} could not reach it, because
// LaTeX's counter commands all go through \c@<name>. article.cls:223 resets the counter
// that way immediately after \maketitle, which is how the body's first note is 1 while
// the title's \thanks has already consumed a number.
//
// ⛔ The Go field survives as the fallback for an engine running with no format loaded
// — several tests drive the primitives directly — and is kept in step so nothing reads
// a stale value. It is a fallback, not a second source of truth: when \c@footnote
// exists it is the counter, and the field only mirrors it.
func (e *Engine) stepFootnote() int {
	if i := e.footnoteCounterIndex(); i >= 0 {
		e.stepCounter("footnote")
		e.footnoteCounter = e.count[i]
		return e.count[i]
	}
	e.footnoteCounter++
	return e.footnoteCounter
}

// footnoteMark is what \thefootnote produces for value n — the ARABIC NUMBER in the
// body, and the SYMBOL (*, †, ‡) inside \maketitle, where article.cls:205 redefines it
// as \@fnsymbol\c@footnote.
//
// ⛔ Measured against tectonic before this existed: a paper whose title block carries
// \thanks got `1` where the reference gives `∗`, and the body's first note `2` where
// the reference gives `1`. 70 of the 154 measured papers write \thefootnote.
//
// ⛔ The register is set and restored around the expansion rather than stepped, because
// \footnotemark[3] names a number without disturbing the counter — LaTeX does the same
// with \begingroup\c@footnote#1\relax.
func (e *Engine) footnoteMark(n int) string {
	i := e.footnoteCounterIndex()
	m := e.eq["thefootnote"]
	if i < 0 || m == nil || m.kind != mMacro {
		return strconv.Itoa(n)
	}
	old := e.count[i]
	e.count[i] = n
	s := strings.TrimSpace(e.expandToString([]tok{csTok("thefootnote")}))
	e.count[i] = old
	if s == "" {
		// ⛔ An empty expansion is not a mark. Falling back to the digits keeps a note
		// visible rather than silently losing its reference, which is the failure a
		// reader cannot see.
		return strconv.Itoa(n)
	}
	return s
}

// queueFootnoteText builds the numbered note and holds it until the enclosing paragraph
// attaches it to the vertical list. Split out of doFootnote so \footnotetext can reach it:
// the two halves of a footnote are separable in LaTeX and were not here.
func (e *Engine) queueFootnoteText(mark string, text []tok) {
	// Body = \footnotesize "N. " + text, set as a mini-paragraph to the body width.
	//
	// ⛔ IN BRACES, because typesetGroupToVbox saves the vertical-list state and the
	// current font and NOT the leading — a documented property other callers rely
	// on (subfigure.go) — while \footnotesize moves \baselineskip. So every
	// footnote left the BODY set at the note's leading, for the whole rest of the
	// document: measured in book at 12pt, \the\baselineskip read 14.5pt before the
	// note and 12.0pt after it, and with \baselinestretch{1.25} 18.125 -> 15.0.
	//
	// The braces are the engine's own grouping, which restores it: a plain
	// {\footnotesize …} written in a document always came back correctly, and that
	// is what said the SANDBOX was not the place to fix this. Scoping it here keeps
	// the other six callers — floats, minipage, multicols, parbox, subfigure,
	// two-column spans — exactly as they were; putting a group inside the sandbox
	// broke multicols, whose body crosses it with groups open.
	// ⛔ NO FULL STOP. This wrote "1. " where the reference writes the mark and a
	// space: \@makefntext is \hb@xt@1.8em{\hss\@makefnmark}#1 (article.cls:609-612),
	// and \@makefnmark carries no punctuation. Measured against tectonic on a paper
	// with a \thanks and a body note: `∗ Affiliation` and `1 Note ordinaire.` there,
	// `1. Affiliation` and `2. Note ordinaire.` here.
	e.pendingFootnotes = append(e.pendingFootnotes, e.typesetGroupToVbox(e.footnoteTextBody(mark, text)))
}

// footnoteTextBody assembles the note the way latex.ltx:13198 does — by CALLING the
// class's \@makefntext with the note as its argument — instead of laying the mark out
// in Go.
//
// ⛔⛔ THE LAYOUT OF A NOTE BELONGS TO THE CLASS, and this put the mark, a space and
// the text in a flat row, so no class could place it. The three base classes all write
//
//	\newcommand\@makefntext[1]{\parindent 1em\noindent\hb@xt@1.8em{\hss\@makefnmark}#1}
//
// (article.cls:609-612, report.cls:708, book.cls:714), which RIGHT-ALIGNS the mark in a
// 1.8em box, so every note's text began one box-width to the left of the reference's.
// amsart.cls:1303 writes a different one, \def\@makefntext{\indent\@makefnmark} — no
// parameter at all — which is why the note is handed over BRACED: that brace group is
// the argument for the three base classes and an ordinary group for amsart, and both
// read correctly.
//
// The mark reaches it through \@thefnmark, as in the reference (latex.ltx:13179), and
// \@makefnmark (latex.go) is what turns it into a box — so a document redefining either
// one is honoured. \@thefnmark is \def'd INSIDE the note's group rather than globally:
// the reference's \protected@xdef is global, but it rewrites it before every note, so
// the two cannot be told apart from the output, and the local form cannot leak a mark
// into whatever is typeset next.
//
// The precedent is footnoteRuleNodes below, and latexHead (fancyhdr.go:176): a macro the
// reference defines is EXECUTED, not imitated.
func (e *Engine) footnoteTextBody(mark string, text []tok) []tok {
	body := []tok{chTok('{', catBegin), csTok("footnotesize")}
	body = append(body, csTok("def"), csTok("@thefnmark"), chTok('{', catBegin))
	body = append(body, stringToToks(mark)...)
	body = append(body, chTok('}', catEnd))
	if m := e.eq["@makefntext"]; m != nil && m.kind == mMacro {
		body = append(body, csTok("@makefntext"), chTok('{', catBegin))
		body = append(body, text...)
		body = append(body, chTok('}', catEnd), chTok('}', catEnd))
		return body
	}
	// ⛔ No class, no \@makefntext: latex.ltx DEFINES none — it only calls it (:13198),
	// every class supplying its own. So the fallback is this engine's, not the
	// reference's, and it stays what the note looked like before: the mark, a space,
	// the text. It still goes through \@makefnmark, so the mark is a class's or a
	// document's even when the note's layout is ours.
	body = append(body, csTok("@makefnmark"), chTok(' ', catSpace))
	body = append(body, text...)
	body = append(body, chTok('}', catEnd))
	return body
}

// emitFootnoteMark drops the raised reference number at the current point.
func (e *Engine) emitFootnoteMark(mark string) {
	if e.curFont == nil {
		return
	}
	if !e.inPar {
		e.beginParagraph(true)
	}
	e.parList = append(e.parList, e.footnoteMarker(mark))
}

// doFootnoteMark implements \footnotemark and doFootnoteText implements \footnotetext: the
// two halves of a footnote, placed separately. latex.ltx:13202-13206 and :13219-13222 —
//
//	\def\footnotemark{\@ifnextchar[\@xfootnotemark
//	  {\stepcounter{footnote}\protected@xdef\@thefnmark{\thefootnote}\@footnotemark}}
//	\def\footnotetext{\@ifnextchar[\@xfootnotenext
//	  {\protected@xdef\@thefnmark{\thempfn}\@footnotetext}}
//
// ⛔ \footnotemark was undefined — 10 uses over 7 corpus papers, and a hard stop for three of
// them — while \footnotetext was DEFINED as a stub that swallowed the note whole
// (\def\footnotetext{\@ifnextbracket\@gobbleoptarg\@gobble}, latex.go). So the pair lost the
// text and reported nothing: the census counts what the engine does not have, and a stub is
// something it has. Ten corpus papers use one or both. gobblers.py in go-tex/measure is the
// inventory that question produced.
//
// ⛔ Note which one steps the counter. \footnotemark does; \footnotetext does NOT, because it
// pairs with a mark that already stepped. Getting that backwards numbers every later note
// one too high. The bracket forms set the number explicitly and, in LaTeX, inside a group —
// so they do not disturb the counter either.
func (e *Engine) doFootnoteMark() {
	n, explicit := e.scanOptFootnoteNumber()
	if !explicit {
		n = e.stepFootnote()
	}
	e.emitFootnoteMark(e.footnoteMark(n))
}

func (e *Engine) doFootnoteText() {
	n, explicit := e.scanOptFootnoteNumber()
	if !explicit {
		if i := e.footnoteCounterIndex(); i >= 0 {
			n = e.count[i]
		} else {
			n = e.footnoteCounter
		}
	}
	e.queueFootnoteText(e.footnoteMark(n), e.grabUndelimited())
}

// scanOptFootnoteNumber reads the optional [n] both commands take. A bracket whose content
// is not a number is consumed and ignored rather than guessed at.
func (e *Engine) scanOptFootnoteNumber() (int, bool) {
	toks, ok := e.scanOptBracketToks()
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(e.toksToString(toks)))
	if err != nil {
		return 0, false
	}
	return n, true
}

// footnoteMarker builds the raised reference number placed inline at the \footnote
// call — the digits packed into an hbox shifted up by ~0.4em.
func (e *Engine) footnoteMarker(mark string) node {
	var mk []node
	for _, r := range mark {
		w, h, d := e.curFont.charDimsSP(r)
		mk = append(mk, charNode{ch: r, width: w, height: h, depth: d, srcLine: e.curSrcLine, size: e.curFont.sizePt(), face: e.curFont})
	}
	box := hpackSP(mk, packNatural, 0)
	box.shift = -ptToSP(e.curFont.sizePt() * 2 / 5) // negative shift raises (superscript)
	return box
}

// numberToks turns an integer into a run of digit character tokens.
func numberToks(n int) []tok {
	var out []tok
	for _, r := range strconv.Itoa(n) {
		out = append(out, chTok(r, catOther))
	}
	return out
}

// footnoteRuleNodes is \footnoterule, EXECUTED rather than imitated.
//
// ⛔⛔ THE RULE WAS THREE HARD-CODED NODES IN GO, so a class could not change it — and
// 60 of the 154 measured papers redefine \footnoterule, counted on their own sources.
// The shape we drew happens to match article.cls:605-608,
//
//	\renewcommand\footnoterule{\kern-3\p@ \hrule\@width.4\columnwidth \kern2.6\p@}
//
// which is why nothing looked wrong: we agreed with the commonest class by coincidence
// and with nobody else. The kernel's own rule is a FIXED 2in (latex.ltx:13162-13163),
// so even the default was only right for article and its relatives.
//
// ⛔ \let\footnoterule\relax DRAWS NOTHING. article.cls:172 does exactly that for the
// \if@titlepage branch, so a title page's notes carry no rule at all. A name that
// exists but is not a macro must therefore produce an EMPTY list, not the fallback —
// reading "not a macro" as "use the built-in rule" would put a rule on the one page the
// class took trouble to clear.
//
// ⭐ The precedent for executing a macro at page-assembly time is latexHead
// (fancyhdr.go:176), which reads \@oddhead's body and typesets it the same way.
func (e *Engine) footnoteRuleNodes() []node {
	if m := e.eq["footnoterule"]; m != nil {
		if m.kind != mMacro {
			return nil // \let\footnoterule\relax
		}
		if b := e.typesetGroupToVbox(m.body); b != nil {
			return []node{b}
		}
		return nil
	}
	// No format loaded: the engine's own primitives are being driven directly. The
	// built-in shape is article.cls's, which is what this drew before.
	return []node{
		kernNode{width: -ptToSP(3)},
		ruleNode{width: e.hsize * 2 / 5, height: defaultRule},
		kernNode{width: ptToSP(2.6)},
	}
}

// typesetGroupToVbox runs a token list through the main loop in isolation and
// returns the material it produced as a vbox — used to set a footnote body. The
// engine's horizontal/vertical state is saved and restored, and buildingFootnote
// guards endParagraph from flushing pending notes into this sandbox.
func (e *Engine) typesetGroupToVbox(toks []tok) *boxNode {
	savedMvl, savedPar, savedIn, savedPD := e.mvl, e.parList, e.inPar, e.prevDepth
	savedBuilding, savedFont := e.buildingFootnote, e.curFont
	e.mvl, e.parList, e.inPar, e.prevDepth = nil, nil, false, ignoreDepth
	e.buildingFootnote = true

	e.push(append(append([]tok{}, toks...), csTok("par"), sentinel))
	for e.err == nil {
		t, ok := e.getXToken()
		if !ok || (t.cs_ && t.cs == sentinel.cs) {
			break
		}
		if !e.stepToken(t) {
			break
		}
	}
	e.endParagraph()
	body := vpackSP(e.mvl, packNatural, 0)

	e.mvl, e.parList, e.inPar, e.prevDepth = savedMvl, savedPar, savedIn, savedPD
	e.buildingFootnote, e.curFont = savedBuilding, savedFont
	return body
}

// flushFootnotes moves any notes accumulated during the just-finished paragraph
// onto the main vertical list (as footnoteNodes, right after the paragraph that
// referenced them). No-op inside a footnote's own build.
func (e *Engine) flushFootnotes() {
	if e.buildingFootnote || len(e.pendingFootnotes) == 0 {
		return
	}
	for _, b := range e.pendingFootnotes {
		e.mvl = append(e.mvl, footnoteNode{body: b})
	}
	e.pendingFootnotes = e.pendingFootnotes[:0]
}

// assemblePage packs a page's slice of the vertical list, lifting footnoteNodes
// out of the content flow and stacking their bodies below a separator rule at the
// foot of the page. pageNum is this page's 1-based ordinal; when the page style is
// not "empty" a centred page number is placed at the very foot (see pagenum.go).
func (e *Engine) assemblePage(page []node, pageNum int) *boxNode {
	return e.assemblePageTopskip(page, pageNum, true)
}

// assemblePageTopskip is assemblePage with one switch: whether THIS function is the
// one that puts \topskip on.
//
// ⛔⛔ A PAGE THAT OPENS WITH A TOP-FLOAT BLOCK HAS ALREADY RECEIVED IT, under the
// block, and must not receive it again above the float. The comment below used to
// say that assembling in one place was enough — "both float placers assemble here,
// so the glue is put on once" — and that is true of the CALL and false of the
// ARITHMETIC: \topskip's width is reduced by the height of the page's first box
// (tex.web:19513), and on a float page that box is the float. A 60pt float swallows
// the whole 11pt, so the first line of text under it was placed by whatever glue the
// main vertical list happened to carry.
//
// LaTeX cannot make this mistake, because it attaches the float block AFTER the page
// box exists: \@cflt builds \vbox{<floats> \vskip-\floatsep \topfigrule
// \vskip\textfloatsep \unvbox\@outputbox} (ltoutput.dtx:3597-3604), and \unvbox
// SPLICES the page box in with the \topskip glue TeX already put at its head. So the
// first text baseline under a top float is set by \topskip exactly as it is on a page
// with no float at all.
//
// ⚠ MEASURED, at three values of \topskip, against tectonic, with a no-float control
// that must move with the reference (article at 11pt, a [t] float, first text
// baseline below it):
//
//	\topskip        0pt     11pt (defaut)   30pt
//	reference    243.05        246.20     265.13     moves: -3.15, +18.93
//	before       248.72        248.72     248.72     moves: 0.00 — \topskip is lost
//	control (no float), both engines                 moves: -3.15, +18.93
//
// The head of the page's vertical list says the same thing without a PDF
// (GOTEX_FLTRACE=1, floatplace.go's traceVlistHead): glue 20.00000pt for
// \textfloatsep, then glue 5.75981pt where the reference has 3.15981pt. Those two
// differ by 2.60000pt = \baselineskip - \topskip = 13.6 - 11.0, which is the whole
// of the residual #356 calls "a float is ~2.4pt short".
func (e *Engine) assemblePageTopskip(page []node, pageNum int, addTopskip bool) *boxNode {
	var content []node
	var notes []*boxNode
	for _, n := range page {
		if fn, ok := n.(footnoteNode); ok {
			notes = append(notes, fn.body)
			continue
		}
		content = append(content, n)
	}
	// \topskip above the page's first box (tex.web §1001, see topskip.go). Every page
	// passes through here — the single-list pager, the two-column pager and both float
	// placers — and findPageBreak charges its height through the same function.
	//
	// ⛔ But NOT every page's first box is the box \topskip is about: see
	// assemblePageTopskip's header. A page whose text sits under a top-float block
	// arrives with the glue already in place and addTopskip false.
	vlist := content
	if addTopskip {
		vlist = e.withTopskip(content)
	}
	if len(notes) > 0 {
		// \skip\footins above the rule, then \footnoterule — which costs NOTHING
		// net: \kern-3pt, a .4pt rule, \kern2.6pt (latex.ltx:13162-13163). The
		// gaps here were a flat 10pt and 4pt, 5.4pt more than the reference puts
		// there.
		vlist = append(vlist, glueNode{spec: glueSpec{width: e.footinsSkip()}})
		vlist = append(vlist, e.footnoteRuleNodes()...)
		for i, b := range notes {
			if i > 0 {
				vlist = append(vlist, glueNode{spec: glueSpec{width: 3 * unity}})
			}
			vlist = append(vlist, b)
		}
	}
	// \@textbottom closes the text block, after the footnotes, exactly where
	// \@make@normalcolbox puts it (ltoutput.dtx:2128-2146).
	if g, ok := e.textBottom(); ok {
		vlist = append(vlist, g)
	}
	// ⛔ THE MARKS ARE PUBLISHED BEFORE ANYTHING DRAWS A HEAD, and this is the one
	// place every page passes through, so they are published exactly once per page.
	// A head expands \topmark / \botmark while this page is assembled (amsart reads
	// its heads out of them, amsart.cls:325-326), so values set after the head is
	// built would be one page late — the classic form of this defect, and the
	// reason tex.web updates them at the TOP of fire_up (:19718-19723) before the
	// output routine runs at all.
	marksOnPage, vlist := takeMarks(vlist)
	e.marks.advance(marksOnPage)
	e.publishMarks()
	// \thispagestyle's override rides the vertical list and applies to the page it
	// landed on, then goes with it (see pageStyleNode).
	style := e.pageStyle
	if s, rest, found := takePageStyle(vlist); found {
		style, vlist = s, rest
	}
	// ⛔⛔ EVERY PAGE IS PACKED EXACTLY TO \vsize, WHATEVER THE PAGE STYLE. TeX has no
	// page-style branch here at all — styles are the output routine's business, not the
	// packer's — and the chain is three lines of tex.web:
	//
	//	§19321  page_goal := vsize
	//	§19564  best_size := page_goal
	//	§19813  box(255) := vpackage(link(page_head), best_size, exactly, page_max_depth)
	//
	// "empty" packed NATURALLY, so a page of this style was exactly as tall as its
	// material and every vertical glue on it stretched by nothing. \vspace{\fill} under
	// \pagestyle{empty} put its word at y=143.91 where tectonic puts it at 664.72 — a
	// 520pt error, on the style this engine takes as its DEFAULT (engine.go).
	//
	// ⛔ §19811-19814 sets vbadness := inf_bad and vfuzz := max_dimen around that call
	// "to inhibit error messages": a page that cannot fill itself is packed short in
	// silence, it is not a warning. vpackSP must stay as quiet here as it is for "plain".
	if style == "empty" {
		if e.vsize > 0 {
			return vpackSP(vlist, packTo, e.vsize)
		}
		// No \vsize is not "pack to zero": with no page goal there is nothing to pack
		// to, and natural height is the only answer that does not invent one. The same
		// fallback guards the "plain" branch below.
		return vpackSP(vlist, packNatural, 0)
	}
	e.curPageNum = pageNum // so \thepage in a header/footer field is this page

	// "fancy" always assembles head and foot; a LaTeX page style that declared
	// \@oddhead takes the same path, since the only difference is where the fields
	// came from (see latexHead). \pagestyle{headings} lands here.
	//
	// ⛔ The \@oddhead test has to be read TOGETHER with the style in force, not
	// instead of it. book.cls and report.cls say \pagestyle{headings} in their
	// preamble, so \@oddhead is defined for the whole document — and a page asking
	// for "plain" came down this path anyway, which put its folio in the running
	// head at the TOP where the class wants it centred at the FOOT. That is not a
	// rare page: every chapter opening in every book and report is a
	// \thispagestyle{plain} page, and so is the first page of its contents list.
	// Measured against tectonic on an 11pt book: the folio centred at y=694.40
	// there, flush left at y=102.89 here.
	if style == "fancy" || (style != "plain" && e.hasLatexHead()) {
		return e.assembleFancyPage(vlist)
	}
	// "plain": the BODY ALONE is packed to \vsize, and the folio goes below it.
	//
	// ⛔⛔ IT USED TO BE PACKED TOGETHER WITH A \vfil IN FRONT OF THE FOLIO. That fil
	// is first-order infinite, so it took ALL of the page's stretch: the body's own
	// glue — section skips, \parskip, display skips — stretched by nothing, and the
	// text block was set at its natural height on a page that should have filled.
	// The folio then landed at the bottom of the TEXT area instead of \footskip
	// below it. See #721, which measured both halves.
	//
	// The kernel keeps the two apart. \@makecol packs the text block alone
	// (latex.ltx:15314-15320, `\vbox to\@colht`), and \@outputpage places the foot
	// under it with
	//
	//	\box\@outputbox \baselineskip\footskip \hb@xt@\textwidth{\@thefoot}
	//
	// — an ordinary interline glue whose \baselineskip is \footskip, which is why
	// the folio's baseline sits \footskip from the text block's and not at a fixed
	// distance from the last line of text.
	//
	// ⚠ MEASURED against tectonic, article at 10pt, 12 sections breaking NATURALLY
	// under \flushbottom (#721's own witness, whose warning about \newpage is why
	// the break is natural). Page 1:
	//
	//	                     last body baseline      folio
	//	tectonic                     672.75         702.64
	//	before                       652.80         672.74
	//	after                        672.74         702.62
	//
	// The "empty" branch above was already right — it packs the body straight to
	// \vsize — and that is what this now does, with the foot moved out of the box.
	if e.vsize > 0 {
		body := vpackSP(vlist, packTo, e.vsize)
		return e.withFootAtFootskip(body, pageNum)
	}
	return e.withFootAtFootskip(vpackSP(vlist, packNatural, 0), pageNum)
}

// textBottom is \@textbottom: the glue LaTeX closes the text block with, INSIDE
// the \vbox to\@colht that \@make@normalcolbox packs (ltoutput.dtx:2128-2146):
//
//	\setbox\@outputbox \vbox to\@colht {%
//	    \@texttop  <content>  \vskip -\@outputbox@depth  \@textbottom }
//
// ⛔ IT IS THE SWITCH BETWEEN \raggedbottom AND \flushbottom, and it is the whole
// of it. \raggedbottom defines it as `\vskip \z@ \@plus.0001fil`, \flushbottom
// lets it \relax (ltpage.dtx:376-386), and the kernel's own default is \relax
// (ltoutput.dtx:3013-3014). A fil inside the block takes all the slack, so the
// block's own glue stretches by nothing and the page keeps its natural height;
// without it the block stretches to \@colht. That is the entire difference
// between the two modes, and this engine had both as \def...{} — measured
// against the reference, they differ by 36.9pt on the last line of the same page
// and by 0 here (#721).
//
// ⚠ IT TESTS THE NAME'S MEANING, NOT ITS BODY. A macro with a body means ragged;
// \relax or undefined means flush. ltoutput.dtx:2558 asks exactly this question
// the same way — `\ifx\@textbottom\relax` — so the test is the kernel's own. What
// it does not do is OBEY a \@textbottom someone defined differently: makeidx
// redefines it (makeindx.dtx:270) and would get this fil rather than its own
// tokens. Honest limit, not a hidden one.
func (e *Engine) textBottom() (glueNode, bool) {
	m := e.eq["@textbottom"]
	if m == nil || m.kind != mMacro || len(m.body) == 0 {
		return glueNode{}, false
	}
	// .0001fil, not a \vfil: two first-order fils share the slack half and half,
	// and ltpage.dtx:372-373 picks this value so a \newpage's own 1fil wins.
	return glueNode{spec: glueSpec{stretch: ptToSP(0.0001), stretchOrder: 1}}, true
}

// withFootAtFootskip puts the running foot under a packed text block, at the
// distance \@outputpage puts it: an interline glue whose \baselineskip is
// \footskip (latex.ltx, `\box\@outputbox \baselineskip\footskip
// \hb@xt@\textwidth{\@thefoot}`).
//
// ⛔ THE RULE IS §679's, NOT A SUBTRACTION. Borrowing interlineGlue rather than
// writing `footskip - depth - height` here is what keeps \lineskip and
// \lineskiplimit in play: a text block whose depth is large enough leaves less
// than \lineskiplimit and the foot must then sit \lineskip below it, exactly as
// any two boxes do. Writing the subtraction inline would be a fifth copy of §679
// with the threshold left out — the defect interlineGlue's own header records.
func (e *Engine) withFootAtFootskip(body *boxNode, pageNum int) *boxNode {
	foot := e.pageFooter(pageNum)
	if foot == nil {
		return body
	}
	body.textBlock = true // this box is \@outputbox; the page around it is not
	page := []node{body}
	if g, ok := e.footskipGlue(body.depth, foot.height); ok {
		page = append(page, g)
	}
	page = append(page, foot)
	return vpackSP(page, packNatural, 0)
}

// footskipGlue is the glue between the text block and the running foot:
// \@outputpage sets \baselineskip to \footskip and then puts the foot box down,
// so the foot's baseline sits \footskip from the block's.
//
// ⛔ ONE IMPLEMENTATION, because there are two callers — the plain page and the
// running-head page — and they had drifted into two different ideas of where the
// foot goes. The assignment is local, as it is in \@outputpage's box: it does not
// change the document's leading.
func (e *Engine) footskipGlue(blockDepth, footHeight int) (glueNode, bool) {
	saved := e.baselineskip
	e.baselineskip = e.classDimen("footskip", ptToSP(30))
	g, ok := e.interlineGlue(blockDepth, footHeight)
	e.baselineskip = saved
	return g, ok
}

// assembleFancyPage builds a \pagestyle{fancy} page: the running header (with its
// rule) above the content, and the running footer (with its rule) at the foot, the
// content stretched to \vsize between them. body is the content+footnotes vlist.
func (e *Engine) assembleFancyPage(body []node) *boxNode {
	var top []node
	if h := e.fancyHeader(); h != nil {
		top = append(top, h)
		if e.headRule > 0 {
			top = append(top,
				glueNode{spec: glueSpec{width: 2 * unity}},
				ruleNode{width: e.hsize, height: e.headRule},
			)
		}
		// \headsep separates the head from the text block (25pt in the standard
		// classes), not a fixed 6pt. classes.dtx, "Page Layout": the text starts at
		// 1in + \voffset + \topmargin + \headheight + \headsep, so the band above it
		// is exactly those two lengths — which is also why renderVMargin lifts the
		// page by them when a head is drawn (geometry.go). Measured against the
		// reference on 2405.18549 page 3, our head sat 72pt from the top with 10pt
		// under it where the reference has 37pt and 31pt.
		top = append(top, glueNode{spec: glueSpec{width: e.classDimen("headsep", 25*unity)}})
	}
	// ⛔⛔ THE SAME DEFECT THE plain PATH HAD, and on the path that carries nearly
	// every page of a real document: head, body, a \vfil and the foot were packed
	// into ONE box. A first-order fil takes all of the page's stretch, so the
	// body's own glue stretched by nothing and the foot sat wherever the fil left
	// it. #721's first comment measured it on \pagestyle{headings}: 616.87 against
	// tectonic's 672.75, a page holding 55.9pt less than it should — close to five
	// lines at that pitch, against 19pt on the plain path.
	//
	// \@outputpage keeps the three apart (latex.ltx):
	//
	//	\hb@xt@\textwidth{\@thehead} \vskip \headsep
	//	\box\@outputbox
	//	\baselineskip\footskip \hb@xt@\textwidth{\@thefoot}
	//
	// so the text block alone is packed to \@colht and the head and foot are
	// placed around it. \vsize is the TEXT block: the head band sits above it and
	// the page is lifted by exactly that much (renderVMargin/headBand), which is
	// why the block is packed to \vsize and the page is left at its natural height
	// rather than packed to \vsize + the band.
	var block *boxNode
	if e.vsize > 0 {
		block = vpackSP(body, packTo, e.vsize)
	} else {
		block = vpackSP(body, packNatural, 0)
	}
	block.textBlock = true
	page := append(top, block)
	if f := e.fancyFooter(); f != nil {
		foot := f
		if e.footRule > 0 {
			// The rule travels WITH the foot, as it does inside fancyhdr's
			// \@thefoot: \footskip is the distance to the whole foot assembly, so
			// leaving the rule outside it would measure to the text line and put
			// the rule above the block's own bottom.
			foot = vpackSP([]node{
				ruleNode{width: e.hsize, height: e.footRule},
				glueNode{spec: glueSpec{width: 2 * unity}},
				f,
			}, packNatural, 0)
		}
		if g, ok := e.footskipGlue(block.depth, foot.height); ok {
			page = append(page, g)
		}
		page = append(page, foot)
	}
	return vpackSP(page, packNatural, 0)
}

// NOTE on \footnotesep: LaTeX puts a \rule\z@\footnotesep at the HEAD of every
// note (latex.ltx:13199). It is a strut — it sets a minimum height for the note's
// own box — not glue between notes, so adding it here as a gap is wrong: tried,
// and it pushed the pitch from 10.41 to 11.06 against a reference of 9.63. What
// was left of that 0.78 was the note being set on the BODY leading rather than
// \footnotesize's 9.5pt.
//
// That held change has landed: \@setfontsize now applies the leading for every
// size, not only \normalsize. Measured on a note long enough to wrap, the pitch
// between two of its lines goes 12.0 -> 9.5 against a reference of 9.5, so the
// residual this note described is gone and \footnotesep stays out of here.

// takePageStyle lifts a \thispagestyle override out of a page's vertical list and
// returns the list without it. The node carries no dimension, but it is removed
// rather than left for vpack to walk past: a page's list is measured and shipped,
// and a marker that survives into either is one more thing to explain.
func takePageStyle(vlist []node) (style string, rest []node, found bool) {
	for i, n := range vlist {
		if ps, ok := n.(pageStyleNode); ok {
			out := make([]node, 0, len(vlist)-1)
			out = append(out, vlist[:i]...)
			out = append(out, vlist[i+1:]...)
			return ps.style, out, true
		}
	}
	return "", vlist, false
}
