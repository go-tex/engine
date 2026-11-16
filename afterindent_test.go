// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bodyLineIndents compiles testdata/parity/<name>.tex and returns, in bp, how far in
// the first character of each body line (the heading excluded) starts.
func bodyLineIndents(t *testing.T, name string) []string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "parity", name+".tex"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	var walk func(ns []node, depth int)
	walk = func(ns []node, depth int) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox {
				x, hasChar := 0.0, false
				for _, c := range b.list {
					if ch, ok := c.(charNode); ok {
						_ = ch
						hasChar = true
						break
					}
					switch v := c.(type) {
					case *boxNode:
						x += spToPt(v.width)
					case kernNode:
						x += spToPt(v.width)
					case glueNode:
						x += spToPt(b.setWidth(v.spec))
					}
				}
				if hasChar {
					out = append(out, fmt.Sprintf("%.2f", x*72/72.27))
					continue // a line: do not look inside it
				}
			}
			walk(b.list, depth+1)
		}
	}
	for _, p := range e.Pages() {
		walk([]node{p}, 0)
	}
	return out
}

// \section's beforeskip is negative in the standard classes, which is LaTeX's flag for
// "do not indent the paragraph after it" (\@afterindentfalse, honoured by \@afterheading).
// tectonic 0.17.0 sets the first body line flush with the margin and the second
// indented by \parindent (gotex-linediff: x0 133.77 then 148.71, the margin being
// 133.77); the engine indented both. Under babel's french option the flag is turned
// off (french.ldf:1822, \let\@afterindentfalse\@afterindenttrue) and both are indented.
func TestParityParagraphAfterAHeadingIsNotIndented(t *testing.T) {
	for _, name := range []string{"afterindent", "afterindentfrench"} {
		ref, err := os.ReadFile(filepath.Join("testdata", "parity", name+".tectonic.indents.txt"))
		if err != nil {
			t.Fatal(err)
		}
		got := bodyLineIndents(t, name)
		// The heading is the first line; the page number, the last.
		if len(got) < 4 {
			t.Fatalf("%s: read %d lines: %v", name, len(got), got)
		}
		if g, w := strings.Join(got[1:3], " "), strings.TrimSpace(string(ref)); g != w {
			t.Errorf("%s: the two body lines start %s bp in, tectonic's start %s", name, g, w)
		}
	}
}
