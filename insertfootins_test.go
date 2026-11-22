// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \insert<n>{...} (tex.web §1097-1100) attaches its material to an insert class. The
// engine dropped it, so amsart's \@adminfootnotes (texmf/amsart.cls:641-646, through
// \@footnotetext = \insert\footins{...}, :1304-1314) lost every first-page footnote:
// the date, the subject classes, the keywords and \thanks. Tectonic sets four notes on
//
//	\documentclass[10pt,reqno]{amsart} ... \date{\today} \keywords{Alpha, Beta}
//	\subjclass[2010]{65M50, 65M60} \thanks{Funding text.} \maketitle
//
// and the engine set none.
func TestInsertFootinsReachesTheFootnoteModel(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt,reqno]{amsart}`+
		`\begin{document}\title{Title}\author{A. Author}\thanks{Funding text.}`+
		`\date{\today}\begin{abstract}Abstract text.\end{abstract}`+
		`\keywords{Alpha, Beta}\subjclass[2010]{65M50, 65M60}\maketitle Body text.`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	n := 0
	for _, nd := range e.mvl {
		if fn, ok := nd.(footnoteNode); ok {
			n++
			all.WriteString(mvlText([]node{fn.body}))
			all.WriteString("|")
		}
	}
	if n != 4 {
		t.Errorf("footnote nodes = %d, want 4 (tectonic); text %q", n, all.String())
	}
	for _, want := range []string{"Funding", "Alpha", "65M50", "Key"} {
		if !strings.Contains(all.String(), want) {
			t.Errorf("first-page notes lack %q: %q", want, all.String())
		}
	}
}
