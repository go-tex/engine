// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// mdframed's style registry STORES a key list and typesets nothing
// (mdframed.sty:564, :567, :386). Undefined, all three left their arguments on the
// page: researchpaper.sty (PNAS) writes
//
//	\mdfdefinestyle{pnassigstyle}{linewidth=0.7pt,backgroundcolor=pnasblueback,…}
//
// at :23, and page 1 of 2310.11517 carried that as a 797pt line on a 612pt sheet
// (go-tex/engine#629). 144 papers of the arXiv pool write one of them.
func TestMdframedStyleRegistryPutsNothingOnThePage(t *testing.T) {
	for _, src := range []string{
		`\mdfdefinestyle{gotexsty}{linewidth=0.7pt,backgroundcolor=gotexcolour}`,
		`\mdfapptodefinestyle{gotexsty}{innertopmargin=6pt}`,
		`\mdfsetup{linewidth=0.7pt,innerleftmargin=6pt}`,
	} {
		if got := pageOf(t, src+`APRES`); got != "APRES" {
			t.Errorf("%s a laissé %q sur la page, want \"APRES\"", src, got)
		}
	}
}

// ⛔ The environment's BODY is document content and must survive — the PNAS
// significance statement lives in one. Only the optional key list is consumed. A
// gobbler that ate the body would pass the leak test above and silently delete a
// paragraph, which is the worse defect of the two.
func TestMdframedEnvironmentKeepsItsBody(t *testing.T) {
	for _, src := range []string{
		`\begin{mdframed}[style=gotexsty]GOTEXCORPS\end{mdframed}`,
		`\begin{mdframed}GOTEXCORPS\end{mdframed}`,
	} {
		got := pageOf(t, src)
		if !strings.Contains(got, "GOTEXCORPS") {
			t.Errorf("%s a perdu son corps: %q", src, got)
		}
		if strings.Contains(got, "gotexsty") || strings.Contains(got, "style=") {
			t.Errorf("%s a laissé fuir ses clés: %q", src, got)
		}
	}
}
