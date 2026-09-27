// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \IEEEauthorblockN and \IEEEauthorblockA were undefined, so a conference paper's names and
// affiliations were skipped as commands and their arguments ran into ONE paragraph. The
// class defines them as pass-throughs outside conference mode (IEEEtran.cls:4676-4677) and
// through \@IEEEauthorhalign inside it; the halign's \crcr is what separates the blocks, so
// a pass-through alone reproduces the run-together. Each block is its own paragraph here.
//
// \IEEEoverridecommandlockouts restores commands conference mode locked out
// (IEEEtran.cls:6274-6285). Nothing is locked out in this emulation, but the command must
// exist: a paper that calls it in its preamble stopped on an undefined control sequence.
func TestIEEEtranAuthorBlocks(t *testing.T) {
	const src = `\documentclass[conference]{IEEEtran}` +
		`\IEEEoverridecommandlockouts` +
		`\begin{document}\title{THETITLE}` +
		`\author{\IEEEauthorblockN{ALICEUN}\IEEEauthorblockA{AFFILUN}` +
		`\IEEEauthorblockN{BOBDEUX}\IEEEauthorblockA{AFFILDEUX}}` +
		`\maketitle\section{Intro}BODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	d := e.Diagnostics()
	for _, cs := range []string{"IEEEauthorblockN", "IEEEauthorblockA", "IEEEoverridecommandlockouts"} {
		if n := d.Skipped[cs]; n != 0 {
			t.Errorf("\\%s is still undefined (%d skipped)", cs, n)
		}
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"ALICEUN", "AFFILUN", "BOBDEUX", "AFFILDEUX"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 160))
		}
	}
	// ⛔ The point of the \par: the name and the affiliation are on DIFFERENT LINES.
	//
	// This cannot be asserted on the rendered TEXT. stripSVGTags joins lines with a space,
	// so two separate lines read as "ALICEUN AFFILUN" exactly as one run would — the first
	// version of this check tested the extractor and failed on a page that was correct.
	// The rendered SVG puts each line in its own <tspan> with its own y; compare those.
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	yName, yAffil := tspanY(t, svg, "ALICEUN"), tspanY(t, svg, "AFFILUN")
	if yName == yAffil {
		t.Errorf("the name and the affiliation are on the same line (y=%s) — the \\par did "+
			"nothing, which is what happens inside \\centerline's hbox", yName)
	}
}

// tspanY returns the y of the <tspan> the token is drawn in. The renderer emits one tspan
// per line, so two tokens sharing a y are on the same line.
func tspanY(t *testing.T, svg, token string) string {
	t.Helper()
	i := strings.Index(svg, ">"+token)
	if i < 0 {
		t.Fatalf("%q is not in the rendered SVG", token)
	}
	open := strings.LastIndex(svg[:i], "<tspan ")
	if open < 0 {
		t.Fatalf("%q is not inside a tspan", token)
	}
	seg := svg[open:i]
	j := strings.Index(seg, `y="`)
	if j < 0 {
		t.Fatalf("the tspan holding %q carries no y: %q", token, seg)
	}
	seg = seg[j+3:]
	return seg[:strings.Index(seg, `"`)]
}

// The generic \maketitle (latex.go) set the title block with \centerline{\@author}, and an
// \hbox cannot hold a multi-block IEEEtran author: the vertical list flattens into one line
// that does not wrap, and a \par inside it does nothing — which is why defining
// \IEEEauthorblockN with one changed the corpus by exactly ZERO glyphs on its own. The block
// also has to SPAN the columns, since the class sets two from \documentclass.
//
// Measured on 2408.02112 before: the affiliation ran off the right edge of the page and the
// title interleaved with the introduction. After: title, then each name over its affiliation
// The generic \maketitle (latex.go) set the title block with \centerline{\@author}, and an
// \hbox cannot hold a multi-block IEEEtran author: the vertical list flattens into one line
// that does not wrap. A \par inside it does nothing — which is why defining \IEEEauthorblockN
// with one changed the corpus by exactly ZERO glyphs, byte-identical renders, on its own.
//
// And the block has to SPAN the columns. IEEEtran sets two from \documentclass, and
// applyTwoColumnMeasure defers halving the measure to the first BODY paragraph, so the title
// block was typeset at full width and then sliced by paginateTwoColList as if it were column
// material. On 2408.02112 that put the title through the introduction and ran the affiliation
// off the right edge of the page. \twocolumn[…] hands it over as the region's span instead.
//
// ⛔ The assertion is on colRegions, not on the page text. A first version checked only that
// the words were present, and it PASSED against the unfixed engine — the words are present
// either way, wrongly placed. A witness small enough to assert on positions is also small
// enough not to overflow anything, so there is nothing to see in it; the region's span is the
// thing that either exists or does not.
func TestIEEEtranTitleBlockIsVerticalAndSpans(t *testing.T) {
	src := func(opts string) string {
		return `\documentclass[` + opts + `]{IEEEtran}\begin{document}\title{THETITLE}` +
			`\author{\IEEEauthorblockN{ALICEUN}\IEEEauthorblockA{AFFILUN}` +
			`\IEEEauthorblockN{BOBDEUX}\IEEEauthorblockA{AFFILDEUX}}` +
			`\maketitle\section{Intro}BODY\end{document}`
	}

	e, err := compile([]byte(src("conference")), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(e.colRegions) == 0 {
		t.Fatal("no column region at all on a two-column IEEEtran document")
	}
	if e.colRegions[0].span == nil {
		t.Error("the title block is not the region's SPAN — it will be sliced into the " +
			"columns, which is what put the title through the introduction")
	} else if w := e.colRegions[0].span.width; w <= e.hsize {
		t.Errorf("the span is %v wide against a column measure of %v: it does not span", w, e.hsize)
	}

	// ⛔ [onecolumn] is the case that must not regress. \twocolumn[…] GOBBLES its span when
	// the engine is not in two-column mode, so a spanning \maketitle would drop the whole
	// title block; that form gets the non-spanning one, and every piece must survive.
	e1, err := compile([]byte(src("conference,onecolumn")), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("[onecolumn] compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e1.RenderPages(e1.renderMargin(0)), ""))
	for _, want := range []string{"THETITLE", "ALICEUN", "AFFILUN", "BOBDEUX", "AFFILDEUX", "BODY"} {
		if !strings.Contains(text, want) {
			t.Errorf("[onecolumn] %q is not on the page; it reads %q", want, firstN(text, 160))
		}
	}
}

// The author blocks differ BY MODE in the class, and both ways are visible on the page.
// Conference/peerreviewca/transmag set each block as a ROW of \@IEEEauthorhalign, ended by
// its own \crcr (IEEEtran.cls:4632, :4649); outside those modes the class defines plain
// pass-throughs (:4676-4677), because a journal paper writes the blocks INLINE with its own
// punctuation between them.
//
// ⛔ Getting it backwards is visible either way. With the row form everywhere, 2405.05734's
// "\IEEEauthorblockN{Daanish Mahajan}, \IEEEauthorblockN{Chirag Jain}, …" broke after each
// name and stranded every comma alone on a line, where the reference reads
// "Daanish Mahajan, Chirag Jain, Navin Kashyap". With the inline form everywhere,
// 2408.02112's one-block-per-line conference author ran together on a single line.
func TestIEEEtranAuthorBlocksFollowTheClassMode(t *testing.T) {
	src := func(opts string) string {
		return `\documentclass[` + opts + `]{IEEEtran}\begin{document}\title{THETITLE}` +
			`\author{\IEEEauthorblockN{ALICEUN}, \IEEEauthorblockN{BOBDEUX}}` +
			`\maketitle\section{Intro}BODY\end{document}`
	}
	yOf := func(opts string) (string, string) {
		e, err := compile([]byte(src(opts)), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("[%s] compile: %v", opts, err)
		}
		svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
		return tspanY(t, svg, "ALICEUN"), tspanY(t, svg, "BOBDEUX")
	}
	// journal: the two names share a line, so the comma between them stays with them.
	if a, b := yOf("journal"); a != b {
		t.Errorf("journal mode broke between the blocks (y=%s then y=%s): the comma the "+
			"document wrote between them is now alone on a line", a, b)
	}
	// conference: one block per line.
	if a, b := yOf("conference"); a == b {
		t.Errorf("conference mode set both blocks on one line (y=%s): the \\crcr that ends "+
			"each row of \\@IEEEauthorhalign is missing", a)
	}
}
