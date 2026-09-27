// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// Two front-matter environments whose HEADING was missing while their body was on the page.
//
// \begin{keyword} — elsarticle.cls:711-724. The real one collects the keywords into \keybox
// and the title block unboxes it after the abstract (:783, :823) behind an italic
// "Keywords:" label, and defines \sep as "\unskip, " INSIDE the environment. Undefined, the
// words were typeset where they stood with no label and \sep leaked between them. Eight
// corpus papers.
//
// \begin{acks} — acmart.cls:3144-3150, a \specialcomment whose begin emits
// \section*{\acksname}. Undefined, the acknowledgements had no heading at all. Six papers.
//
// Measured together against their parent: Sigma 330 -> 330, +165 glyphs, nothing lost —
// which is the fourteen labels and nothing else.
func TestFrontMatterHeadingsThatWereMissing(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      []string
		reject    string
	}{
		{
			name: "keyword labels its list and \\sep separates it",
			src: `\documentclass{elsarticle}\begin{document}\begin{frontmatter}` +
				`\title{TTT}\begin{abstract}AAA\end{abstract}` +
				`\begin{keyword}KWONE \sep KWTWO\end{keyword}\end{frontmatter}BODY\end{document}`,
			want:   []string{"Keywords:", "KWONE, KWTWO", "BODY"},
			reject: "KWONE KWTWO",
		},
		{
			name: "acks gets its heading",
			src: `\documentclass{acmart}\begin{document}\title{TTT}\maketitle BODY` +
				`\begin{acks}THANKS\end{acks}\end{document}`,
			want:   []string{"Acknowledgments", "THANKS"},
			reject: "",
		},
	} {
		e, err := compile([]byte(c.src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("%s: compile: %v", c.name, err)
		}
		// Markup stripped: the renderer splits a line into <tspan> elements, so a phrase is
		// not a contiguous string in the raw SVG.
		text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
		for _, w := range c.want {
			if !strings.Contains(text, w) {
				t.Errorf("%s: %q is not on the page", c.name, w)
			}
		}
		if c.reject != "" && strings.Contains(text, c.reject) {
			t.Errorf("%s: %q is on the page — \\sep did not separate", c.name, c.reject)
		}
	}
}
