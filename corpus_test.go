// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCorpus is the document-level regression net: every .tex in testdata/corpus
// (real LaTeX documents, one per feature area, extracted from the browser demo) is
// compiled through the full public pipeline — CompileToSVGPages and CompileToPDF —
// and checked for structural validity. It complements the low-level TestConformance
// (mouth/gullet \message ratchet) and the per-feature unit tests by exercising whole
// documents end to end, so an interaction bug between features fails the build. It
// uses only the built-in font and no filesystem, so it runs unchanged in CI on every
// platform.
func TestCorpus(t *testing.T) {
	files, err := filepath.Glob("testdata/corpus/*.tex")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 30 {
		t.Fatalf("corpus has only %d files; expected the full feature set", len(files))
	}
	for _, f := range files {
		f := f
		t.Run(strings.TrimSuffix(filepath.Base(f), ".tex"), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}

			// SVG pipeline: must produce at least one page of real, drawn material.
			pages, err := CompileToSVGPages(src, Options{})
			if err != nil {
				t.Fatalf("CompileToSVGPages: %v", err)
			}
			if len(pages) == 0 {
				t.Fatal("no SVG pages produced")
			}
			all := strings.Join(pages, "")
			if !strings.Contains(all, "<svg") {
				t.Errorf("output is not SVG: %.80s", all)
			}
			// A drawn document contains glyph paths, rules, images or links — an empty
			// (all-white) page would mean the document silently produced nothing.
			if !containsAny(all, "<path", "<rect", "<image", "<a ", "<g transform") {
				t.Errorf("SVG has no drawn content (glyphs/rules/images/links)")
			}
			// …and it draws at least as much as it did: see glyphFloor.
			name := strings.TrimSuffix(filepath.Base(f), ".tex")
			floor, ok := glyphFloor[name]
			if !ok {
				t.Errorf("no glyph floor recorded for %s: add one (see glyphFloor)", name)
			} else if got := strings.Count(all, "<path"); got < floor {
				t.Errorf("drew %d glyphs, floor is %d: content was lost", got, floor)
			}

			// PDF pipeline: must produce a valid, non-trivial PDF without crashing.
			var buf bytes.Buffer
			n, err := CompileToPDF(src, Options{}, &buf)
			if err != nil {
				t.Fatalf("CompileToPDF: %v", err)
			}
			if n < 1 {
				t.Errorf("PDF reports %d pages", n)
			}
			if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF")) {
				t.Errorf("output is not a PDF (no %%PDF header)")
			}
			if buf.Len() < 500 {
				t.Errorf("PDF is suspiciously small: %d bytes", buf.Len())
			}
		})
	}
}

// glyphFloor is the census guard: the fewest glyphs each corpus document may draw
// before the build fails. TestCorpus already asks whether a document drew ANYTHING;
// that passes a document which silently lost half its content, which is exactly the
// failure this engine keeps having — a body scanner that swallows to the wrong \end,
// a group that never closes, an environment skipped whole. The numbers are today's
// counts less a tenth: ordinary layout work (a line breaking differently, a float
// moving) never moves a document's glyph count by that much, while a swallowed
// environment or a lost \input moves it far more.
//
// A floor is a RATCHET, not a target. When a change legitimately draws fewer glyphs
// (a stub replaced by real output that sets less, say), raise or lower the entry
// deliberately and say why in the commit — do not silently retune the table.
var glyphFloor = map[string]int{
	"01-tabularx":                   148,
	"02-listes-enumitem":            105,
	"03-interligne-setspace":        331,
	"04-multi-colonnes":             268,
	"05-image-svg":                  71,
	"06-en-tetes-fancyhdr":          177,
	"07-matrices-aligne":            106,
	"08-unites-siunitx":             135,
	"09-equations-tag-sous-numeros": 99,
	"10-boites-latex":               124,
	"11-couleur-avancee":            132,
	"12-code-lstlisting":            107,
	"13-references-typees":          94,
	"14-liens-internes":             117,
	"15-transformations":            110,
	"16-booktabs":                   45,
	"17-sectionnement":              161,
	"18-multline-eqnarray":          167,
	"19-compteurs-longueurs":        59,
	"20-phantom-smash":              115,
	"21-align-gather":               128,
	"22-theoremes":                  174,
	"23-decorations":                95,
	"24-sommaire":                   301,
	"25-equations":                  100,
	"26-couleur":                    104,
	"27-encadres-parbox":            159,
	"28-liens":                      74,
	"29-listes":                     76,
	"30-tailles-notes":              76,
	"31-image":                      40,
	"32-typographie":                95,
	"33-note-code":                  104,
	"34-article-latex":              91,
	"35-tableau":                    40,
	"36-colonnes-p":                 110,
	"37-references":                 153,
	"38-mathematiques":              78,
	"39-francais":                   69,
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
