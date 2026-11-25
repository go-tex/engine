// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements page styles and page numbering: \pagestyle{plain|empty}
// (and \thispagestyle, treated the same), \pagenumbering{arabic|roman|Roman|alph|
// Alph} which selects the page-number format, and \today (the document date, taken
// from Options.Date since a pure-Go wasm build has no ambient clock). When the page
// style is not "empty", the page builder draws a centred page number at the foot of
// every page (see assemblePage in footnote.go).
//
// LaTeX's article class defaults to the "plain" style (a bottom-centred number). The
// engine defaults to "empty" so existing single-pass renders are unchanged; a
// document opts in with \pagestyle{plain}. \thepage inside running text expands to
// the arabic page counter, which this single-pass engine cannot know in advance, so
// it is best-effort (1) — the foot number, computed per page at break time, is
// always correct. Both are honest, documented limitations.

import (
	"strconv"
	"strings"
)

// doPagestyle implements \pagestyle{name} and \thispagestyle{name}: only the number
// vs no-number distinction is modelled (plain/headings/myheadings all show the
// bottom-centred number; empty shows none).
// pageStyleNode carries \thispagestyle's override through page breaking. The page
// builder runs AFTER the document has been read, so at the moment \thispagestyle
// is called nothing knows which page it falls on; a node contributed to the
// vertical list lands on that page by construction, which is how TeX gets the same
// answer (its output routine reads \@specialstyle for the page it is shipping).
type pageStyleNode struct{ style string }

func (pageStyleNode) isNode() {}

// doThisPagestyle implements \thispagestyle{name}: the style applies to THIS page
// and the document's own \pagestyle resumes on the next.
//
// It used to take doPagestyle's path, which set the style permanently. Since
// \maketitle issues \thispagestyle{plain} (or {empty}) on nearly every paper, one
// title page silently turned the running head off for the whole document: on the
// 128 corpus papers with a reference, 23 draw a running head and this engine drew
// NONE. Beside tectonic on a three-page document with \thispagestyle{empty} on
// page 1, the reference heads pages 2 and 3 and this engine headed nothing.
//
// The style's \ps@name definitions are deliberately NOT run here: they set the
// header and footer FIELDS, which are global, so running \ps@empty for one page
// would clear them for the rest of the document. LaTeX avoids the same trap by
// running them inside a group in the output routine.
// The name is kept here for the same reason as in doPagestyle: flattening it made
// \thispagestyle{headings} ask for the plain path. \thispagestyle{plain} still
// records "plain" and still beats a class-wide running head — that is the one case
// the name "plain" is supposed to mean, and now the only one that claims it.
func (e *Engine) doThisPagestyle() {
	e.contribute(pageStyleNode{style: e.readBraceName()})
}

// ⛔⛔ THE STYLE'S OWN NAME IS KEPT. Every name but "empty" and "fancy" used to be
// recorded as "plain", and two sites then asked different questions about the same
// page:
//
//   - assemblePage routes on the NAME — `style == "fancy" || (style != "plain" &&
//     hasLatexHead())` — so a style flattened to "plain" took the plain path and
//     got a centred folio at the FOOT and no head;
//   - headBand routes on \@oddhead being DEFINED, so it reserved \headheight +
//     \headsep = 37pt for the head that path had already decided not to draw.
//
// \pagestyle{headings} is what book and report declare, and \ps@headings defines
// \@oddhead as `{\slshape\rightmark}\hfil\thepage` with an EMPTY \@oddfoot
// (latex.go:898). So the folio went to the band LaTeX leaves empty, the head band
// held nothing, and the page lost 37pt of text. Measured against tectonic on a
// witness of 26 sections: our last body line at 616.87 against 672.75, with no
// head where the reference has "1 SECTION NUMERO 1 1".
//
// ⛔ `style != "plain"` is NOT the defect and must stay: it is what makes
// \thispagestyle{plain} beat a class-wide running head, which
// TestThisPageStylePlainBeatsAClassWideRunningHead pins. The flattening was the
// defect — it made every style claim to be that exception.
//
// Nothing else compares this field to a closed set: "empty" is tested by name in
// two places (geometry.go, assemblePage) and "fancy" in one, and an unknown name
// now reaches the head path exactly when its \ps@name defined a head — which is
// the question that should have been asked all along.
func (e *Engine) doPagestyle() {
	name := e.readBraceName()
	e.pageStyle = name
	if name == "fancy" && e.headRule == 0 {
		// fancyhdr default: a rule under the header, none over the foot.
		e.headRule = defaultRule
	}
	// A named style RUNS the definitions stored under \ps@name:
	//
	//	\def\pagestyle#1{\@ifundefined{ps@#1}\undefinedpagestyle{\@nameuse{ps@#1}}}
	//	                                                        latex.ltx:13326-13329
	//
	// which is how a class's own \fancypagestyle{standardpagestyle}{…} reaches this
	// engine's header and footer fields. Without it the definitions had nowhere to
	// go: \fancypagestyle was undefined, so its arguments were left in the input and
	// EXECUTED where they landed, which set the fields by accident and printed the
	// style's name on the page (go-tex/engine#318).
	//
	// \thispagestyle takes the same path. LaTeX defers it through \@specialstyle to
	// the page being shipped; this engine keeps one live set of fields, so a style
	// asked for on one page applies from there — the simplification \thispagestyle
	// already had here.
	if m := e.eq["ps@"+name]; m != nil && m.kind == mMacro && len(m.body) > 0 {
		e.push(m.body)
	}
}

// doPagenumbering implements \pagenumbering{style}: select the page-number format.
// It also resets the page counter to 1, as latex.ltx:10818-10820 does
// (\global\c@page\@ne\gdef\thepage{…}); both ride the vertical list as a
// pageCounterSetNode, so the page that follows takes folio 1 in the new format.
func (e *Engine) doPagenumbering() {
	e.pageNumStyle = pageNumStyleOf(e.readBraceName())
	e.setCount(pageCounterReg, 1, true)
	e.contributePageCounterSet(pageCounterSetNode{value: 1, style: e.pageNumStyle})
}

// pageNumStyleOf maps a \pagenumbering argument to its format letter.
func pageNumStyleOf(name string) byte {
	switch name {
	case "roman":
		return 'r'
	case "Roman":
		return 'R'
	case "alph":
		return 'l'
	case "Alph":
		return 'L'
	}
	return 'a' // "arabic" and anything else
}

// formatPageNumber renders a 1-based page ordinal in the current page-number style.
func formatPageNumber(n int, style byte) string {
	if n < 1 {
		n = 1
	}
	switch style {
	case 'r':
		return roman(n)
	case 'R':
		return strings.ToUpper(roman(n))
	case 'l':
		return alphaLabel(n, 'a')
	case 'L':
		return alphaLabel(n, 'A')
	default:
		return strconv.Itoa(n)
	}
}

// alphaLabel renders n (>=1) as a, b, …, z, aa, ab, … in the alphabet based at
// base ('a' or 'A'), the LaTeX \alph/\Alph convention (bijective base-26).
func alphaLabel(n int, base byte) string {
	var b []byte
	for n > 0 {
		n--
		b = append([]byte{base + byte(n%26)}, b...)
		n /= 26
	}
	return string(b)
}

// pageOrdinal is the page number currently being assembled (1 when unknown), used
// by \thepage.
func (e *Engine) pageOrdinal() int {
	if e.curPageNum > 0 {
		return e.curPageNum
	}
	return 1
}

// pageFooter builds the foot of a page on the "plain" path: the page style's own
// \@oddfoot, set as \@outputpage sets it, \hb@xt@\textwidth{\@thefoot}
// (ltoutput.dtx).
//
// ⛔ IT WAS A FOLIO OF THE ENGINE'S OWN, in the body font, whatever the class
// said. amsart's \ps@plain is `\def\@oddfoot{\normalfont\scriptsize
// \hfil\thepage\hfil}` (amsart.cls:302-303), and the reference sets that folio at
// 7pt (LMRoman7) where this engine set it at 10pt. Our own kernel's \ps@plain had
// no \@oddfoot at all, which is how the stand-in went unnoticed: there was no
// field to read.
//
// ⚠ \ps@<style> RUNS ONLY FOR A \thispagestyle OVERRIDE, inside a group, as
// LaTeX's output routine runs \@specialstyle: its \def's are local and end with
// the page. Under the document's own style \pagestyle already ran it, and
// re-running it would undo a document's own later \def\@oddfoot.
//
// A style with no \ps@ definition at all keeps the engine's centred folio: there
// is no field to typeset, and a folio is what every standard style but "empty"
// and "headings" draws.
func (e *Engine) pageFooter(num int, style string) *boxNode {
	ps := e.eq["ps@"+style]
	if ps == nil || ps.kind != mMacro {
		numBox := e.textToHbox(formatPageNumber(num, e.pageNumStyle))
		return hpackSP([]node{e.hfil(), numBox, e.hfil()}, packTo, e.hsize)
	}
	// Always inside a group: the foot sets its own font and size (\scriptsize), and
	// typesetFieldToList restores only the current font.
	// ⛔ \@evenfoot on an even page of a twoside document (latex.ltx:15387-15393).
	foot := "@oddfoot"
	if m := e.pageSidedMacro("foot", num); m != nil && m == e.eq["@evenfoot"] {
		foot = "@evenfoot"
	}
	toks := []tok{chTok('{', catBegin), csTok(foot), chTok('}', catEnd)}
	if style != e.pageStyle {
		toks = []tok{chTok('{', catBegin), csTok("ps@" + style), csTok(foot), chTok('}', catEnd)}
	}
	list := e.typesetFieldToList(toks)
	if len(list) == 0 {
		return nil
	}
	return hpackSP(list, packTo, e.hsize)
}
