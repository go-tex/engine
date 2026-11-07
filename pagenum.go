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
// It also resets the page counter in LaTeX; here the foot number is a running
// ordinal, so only the format is recorded (documented simplification).
func (e *Engine) doPagenumbering() {
	switch e.readBraceName() {
	case "roman":
		e.pageNumStyle = 'r'
	case "Roman":
		e.pageNumStyle = 'R'
	case "alph":
		e.pageNumStyle = 'l'
	case "Alph":
		e.pageNumStyle = 'L'
	default: // "arabic" and anything else
		e.pageNumStyle = 'a'
	}
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

// pageFooter builds the centred page-number line placed at the foot of a page.
func (e *Engine) pageFooter(num int) *boxNode {
	numBox := e.textToHbox(formatPageNumber(num, e.pageNumStyle))
	return hpackSP([]node{e.hfil(), numBox, e.hfil()}, packTo, e.hsize)
}
