package engine

// TeX's marks: \mark as a node that travels with page breaking, and \topmark,
// \firstmark, \botmark reporting the marks that apply to the page being shipped.
//
// ⛔⛔ WHY THIS IS NOT A REFINEMENT OF THE RUNNING HEAD BUT THE ONLY ROUTE TO ONE.
// Before this the engine kept ONE live mark pair in \@themark and read the head
// out of it, which is right for the kernel's own \leftmark/\rightmark — and
// amsart.cls, which 27 of the 128 corpus papers with a reference use, reads
// neither (texmf/amsart.cls:325-326):
//
//	\def\leftmark{\expandafter\@firstoftwo\topmark{}{}}
//	\def\rightmark{\expandafter\@secondoftwo\botmark{}{}}
//
// It writes \@themark and never reads it back; its heads come from \topmark and
// \botmark. With no \mark at all those papers had no route to a head of any kind,
// which is what #426's own witness showed after the band (#726) and the pair
// (#728) were both right: the band drawn, the folio in it, and nothing else.
//
// ⛔ THE ARGUMENT IS EXPANDED AT MARK TIME, which is not a detail here either:
// amsart does \@temptokena{{#1}{#2}}\xdef\@themark{…}\mark{\the\@temptokena}, so
// an unexpanded \mark would store the three tokens "\the\@temptokena" and the
// head would read \the — tex.web scans it with scan_toks(false,true), expanding.

// markNode carries a \mark through page breaking, exactly as pageStyleNode
// carries \thispagestyle: at the moment \mark is read nothing knows which page it
// will land on, and a node contributed to the vertical list lands on that page by
// construction.
type markNode struct{ toks []tok }

func (markNode) isNode() {}

// doMark implements \mark{…}.
//
// ⛔⛔ A MARK IN THE PREAMBLE IS READ AND DROPPED, and the corpus is the only
// reason this line exists. ws-fnl.cls:804 carries a bare `\mark{{}{}}` at class
// level, outside the document body, and 2405.05720 uses that class: with the mark
// contributed there, the paper went from 9 pages to 10 with its ink IDENTICAL on
// both sides (23282 glyph units), against a reference of 9 — the one paper the
// engine had exactly right, lost. Σ 272 -> 273, exact 47 -> 46.
//
// The argument is still SCANNED, so the tokens are consumed exactly as they would
// be inside the body; only the node is not contributed. Nothing in the preamble
// is typeset here (see inBody), so a node landing on the main vertical list
// before the body starts is material on a page that does not exist yet.
func (e *Engine) doMark() {
	toks := e.expandList(e.readBraceToks())
	if !e.inBody {
		return
	}
	e.contribute(markNode{toks: toks})
}

// marks holds what \topmark, \firstmark and \botmark report for the page being
// shipped. A nil list is TeX's null: the primitive then expands to nothing
// (tex.web:7907-7910 inserts the mark text only `if cur_mark[cur_chr]<>null`).
type marks struct {
	top, first, bot []tok
	botSet          bool // bot has been set by some page, which is TeX's bot_mark<>null
}

// advance applies tex.web's per-page arithmetic to the marks found on one page,
// in the order the page holds them.
//
// The three lines of §1012 (tex.web:19718-19729), in the order TeX runs them:
//
//	if bot_mark<>null then begin top_mark:=bot_mark; first_mark:=null end
//	… walking the page: if first_mark=null then first_mark:=p;  bot_mark:=p  (:19782-19789)
//	if (top_mark<>null)and(first_mark=null) then first_mark:=top_mark
//
// Two consequences that a plainer reading of "first and last mark on the page"
// gets wrong, and both of them are what a running head depends on:
//
//   - bot_mark is NOT cleared for a page with no marks. It keeps the previous
//     page's last mark, so a head set once on page 1 stays on every page after
//     it — which is exactly what amsart's \markboth at \maketitle relies on.
//   - first_mark falls back to top_mark, never to empty, so a page that breaks in
//     the middle of a section still reports the mark in force at its top.
func (m *marks) advance(page [][]tok) {
	if m.botSet {
		m.top = m.bot
	}
	m.first = nil
	firstSet := false
	for _, toks := range page {
		if !firstSet {
			m.first, firstSet = toks, true
		}
		m.bot, m.botSet = toks, true
	}
	if !firstSet && m.botSet {
		m.first = m.top
	}
}

// takeMarks lifts every \mark off a page's vertical list, in order, and returns
// the list without them.
//
// They are removed rather than left for vpack to walk past, for the reason
// takePageStyle gives: a page's list is measured and shipped, and a marker that
// survives into either is one more thing to explain. A mark carries no dimension,
// so removing it cannot change the page's height — asserted by a test rather than
// assumed, since that is the whole claim this makes on the corpus.
func takeMarks(vlist []node) (found [][]tok, rest []node) {
	rest = make([]node, 0, len(vlist))
	for _, n := range vlist {
		if mk, ok := n.(markNode); ok {
			found = append(found, mk.toks)
			continue
		}
		rest = append(rest, n)
	}
	return found, rest
}

// publishMarks makes \topmark, \firstmark and \botmark report this page's values.
//
// They are defined as macros whose bodies are the mark text, which is how the
// engine already gives \jobname a value (api.go:127). TeX pushes the mark's token
// list onto the input stack instead (tex.web:7907-7910), and for every use a
// class makes of them the two are the same thing: amsart's
// \expandafter\@firstoftwo\topmark{}{} needs the pair to arrive as TOKENS to be
// split, and a macro body expands to exactly those tokens.
func (e *Engine) publishMarks() {
	for name, body := range map[string][]tok{
		"topmark":   e.marks.top,
		"firstmark": e.marks.first,
		"botmark":   e.marks.bot,
	} {
		e.define(name, &meaning{kind: mMacro, body: append([]tok(nil), body...)}, true)
	}
}
