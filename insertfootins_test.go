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

// Only the footnote class is a footnote. Another insert class is accepted and dropped
// (its braces are consumed, nothing reaches the foot of the page), a brace-less
// \insert leaves the following token alone, and an \insert at the end of the input
// ends quietly.
func TestInsertOtherClassesAreDropped(t *testing.T) {
	run := func(src string) *Engine {
		t.Helper()
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		return e
	}
	notes := func(e *Engine) int {
		n := 0
		for _, nd := range e.mvl {
			if _, ok := nd.(footnoteNode); ok {
				n++
			}
		}
		return n
	}
	if e := run(`\hsize=300pt \insert100{Zzz}\par`); notes(e) != 0 {
		t.Errorf("an insert of class 100 became a footnote")
	}
	e := run(`\hsize=300pt \insert99 QQ\par`)
	var b strings.Builder
	collectChars(e.mvl, &b)
	collectChars(e.parList, &b)
	if !strings.Contains(b.String(), "QQ") {
		t.Errorf("a brace-less \\insert swallowed the next token: %q", b.String())
	}
	run(`\insert99`)
	if e := run(`\hsize=300pt \footnote{a\insert255{Inner}b}\par`); notes(e) != 1 {
		t.Errorf("an \\insert inside a footnote made its own note: %d notes, want 1", notes(e))
	}
}
