// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// A TWO-SIDED PAGE MODEL, against tectonic (go-tex/engine#875). latex.ltx:15387-15393
// (\@outputpage) picks head, foot and margin by \if@twoside and the parity of \count0:
// an even page takes \@evenhead, \@evenfoot and \evensidemargin.
//
// Measured in tectonic 0.15 / TL bundle v33 on testdata/parity/twoside-*.tex, bp
// converted to pt (x 72.27/72):
//
//	amsart p2 head     "2 T"   first word x0 126.67bp   (we drew "T 2", x0 292 on the paper)
//	amsart p3 head     "T 3"   first word x0 303.15bp   odd page keeps the odd head
//	book   p2 head     "2 CHAPTER 1. ALPHA"  x0 160.67bp
//	book   p2 body     "b"     x0 175.61bp   even margin: 53.79pt right of the odd page's
//	book   p3 body     "c"     x0 121.81bp   odd margin
//
// amsart's even and odd margins are equal (amsart.cls:1883 \evensidemargin
// \oddsidemargin), so only its head differs; book's differ by bk10.clo:147-182.
func TestTwoSidedPageModelMatchesTheReference(t *testing.T) {
	const bp = 72.27 / 72
	cases := []struct {
		file string
		page int
		word string
		x0bp float64
	}{
		{"twoside-amsart.tex", 2, "2", 126.67},
		{"twoside-amsart.tex", 3, "T", 303.15},
		{"twoside-book.tex", 2, "2", 160.67},
		{"twoside-book.tex", 2, "b", 175.61},
		{"twoside-book.tex", 3, "c", 121.81},
	}
	re := regexp.MustCompile(`<tspan[^>]*\sx="([0-9.]+)"[^>]*>([^<]*)</tspan>`)
	pagesOf := map[string][]string{}
	for _, c := range cases {
		pages, ok := pagesOf[c.file]
		if !ok {
			src, err := os.ReadFile(filepath.Join("testdata", "parity", c.file))
			if err != nil {
				t.Fatal(err)
			}
			if pages, err = CompileToSVGPages(src, Options{}); err != nil {
				t.Fatal(err)
			}
			pagesOf[c.file] = pages
		}
		if c.page > len(pages) {
			t.Fatalf("%s: %d pages, want at least %d", c.file, len(pages), c.page)
		}
		// The FIRST word of the page carrying the text: the head is set before the body,
		// so for a head word that is the line's first tspan, and the body words are found
		// by their text.
		found := false
		for _, m := range re.FindAllStringSubmatch(pages[c.page-1], -1) {
			if m[2] != c.word {
				continue
			}
			x, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				t.Fatal(err)
			}
			found = true
			if want := c.x0bp * bp; math.Abs(x-want) > 0.1 {
				t.Errorf("%s page %d: %q at x0 %.2fpt, tectonic's is %.2fpt", c.file, c.page, c.word, x, want)
			}
			break
		}
		if !found {
			t.Errorf("%s page %d: no word %q on the page", c.file, c.page, c.word)
		}
	}
}

// The even head's ORDER, which the position alone cannot tell from the odd head's
// mirror image: on an even page the folio comes BEFORE the running head's text, on an
// odd page after it.
func TestTwoSidedEvenHeadPutsTheFolioFirst(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "twoside-amsart.tex"))
	if err != nil {
		t.Fatal(err)
	}
	pages, err := CompileToSVGPages(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`<tspan[^>]*\sx="([0-9.]+)"[^>]*\sy="([0-9.]+)"[^>]*>([^<]*)</tspan>`)
	headWords := func(svg string) []string {
		var out []string
		var y0 string
		for _, m := range re.FindAllStringSubmatch(svg, -1) {
			if y0 == "" {
				y0 = m[2]
			}
			if m[2] != y0 {
				break
			}
			out = append(out, m[3])
		}
		return out
	}
	if got := headWords(pages[1]); len(got) != 2 || got[0] != "2" || got[1] != "T" {
		t.Errorf("page 2 head is %q, want [2 T]", got)
	}
	if got := headWords(pages[2]); len(got) != 2 || got[0] != "T" || got[1] != "3" {
		t.Errorf("page 3 head is %q, want [T 3]", got)
	}
}
