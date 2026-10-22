// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \captionsetup* differs from \captionsetup only in SUPPRESSING A WARNING
// (caption3.sty:237-243 raises \ifcaptionsetup@star; caption.sty:319 is its one use,
// \ifcaptionsetup@star\else…warning\fi). Nothing it typesets changes — so the
// assertion is that the two forms put the SAME thing on the page.
//
// Unread, the star WAS the argument: scanBracketList met "*" instead of "[", the key
// list was never read, and the whole lot went on the page. paper.cls (PNAS) writes
// \captionsetup*{format=pnasformat} at :155 and page 1 of 2310.11517 read
// "*format=pnasformat" in its body text.
// ⚠ Read through captionHead, not pageOf: a float's content is DEFERRED and never
// reaches the main vertical list, so pageOf comes back empty for both forms — a
// comparison of nothing with nothing, which is how this test first "passed".
func TestCaptionsetupStarChangesNothingItSets(t *testing.T) {
	const tail = `\begin{figure}\caption{x}\end{figure}`
	plain := captionHead(t, `\hsize=300pt\captionsetup[figure]{name={Fig.}}`+tail)
	star := captionHead(t, `\hsize=300pt\captionsetup*[figure]{name={Fig.}}`+tail)
	if star != plain {
		t.Errorf("étoilée = %q, simple = %q", star, plain)
	}
	// And the setup still WORKS through the star, or the fix has merely made
	// \captionsetup* a no-op that happens to print nothing.
	if !strings.Contains(star, "Fig.") {
		t.Errorf("le renommage n'a pas pris effet: %q", star)
	}
}

// \DeclareCaptionFormat[*]{<name>}[<single-line>]{<code>} — caption3.sty:696-706.
// Caption formats are not modelled here, so the declaration is consumed; what matters
// is that its name and body stop reaching the page. All four spellings the grammar
// allows are exercised, because a dispatcher that reads three of them and leaks the
// fourth looks exactly like one that works.
func TestDeclareCaptionFormatPutsNothingOnThePage(t *testing.T) {
	for _, src := range []string{
		`\DeclareCaptionFormat{gotexfmt}{GOTEXBODY}`,
		`\DeclareCaptionFormat*{gotexfmt}{GOTEXBODY}`,
		`\DeclareCaptionFormat{gotexfmt}[GOTEXSINGLE]{GOTEXBODY}`,
		`\DeclareCaptionFormat*{gotexfmt}[GOTEXSINGLE]{GOTEXBODY}`,
	} {
		got := pageOf(t, src+`APRES`)
		if got != "APRES" {
			t.Errorf("%s a laissé %q sur la page, want \"APRES\"", src, got)
		}
	}
}
