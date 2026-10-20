// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

// -align answers a question a page count cannot: WHERE the two documents fall out
// of step.
//
// "One page too long" is the same number whether the extra page appeared on page 2
// or on page 30, and those are opposite defects — the first is a cost paid once at
// the start, the second one accumulated down the document. go-tex/engine#622 spent
// a long time on per-line and per-page quantities on the strength of a mean that
// cannot tell them apart; measured, the corpus bias has no length dependence at all
// (R² = 0.004), which says the cost is paid once and sends the search to where a
// document BEGINS. This is the instrument for finding that page.
//
// For each page of the reference it takes a KEY — a short run of ordinary words
// from near the top of the page — and finds the page of ours that carries it,
// searching forward only, so a running head repeated on every page cannot drag the
// alignment backwards.
//
// ⛔ The key is words, not a line. The two engines break lines differently by
// construction, so a reference line's text does not appear verbatim in ours; what
// does survive is the sequence of words. Hyphenation differs too, so a word broken
// across a line in one and not the other breaks the key — which is why a key that
// is NOT FOUND is reported as not found, never as page 0 or as an offset of zero.
//
// The control is in the tests and it is the obvious one: a document aligned against
// ITSELF must come out at offset 0 on every page. An alignment that cannot do that
// cannot be trusted to say anything about two different documents.
type alignRow struct {
	RefPage  int
	OurPage  int // 0 when the key was not found anywhere at or after the previous match
	Key      string
	RefLines int
	OurLines int
}

// keyWords is how many words a page key holds. Six is long enough to be unique in
// a paper and short enough to survive one hyphenation difference inside the page.
const keyWords = 6

// skipWords drops the first words of a page before taking the key: the top of a
// page may carry the tail of a word hyphenated across the page break, which belongs
// to the previous page and not to this one.
const skipWords = 2

// ⛔ A RUNNING HEAD is not content, and dropping a fixed number of words will not
// get past it. On 2311.08855 the reference prints "Real Analysis Using …" at the top
// of every odd page and this engine prints no head at all (\pagestyle{empty} by
// default), so nine of eighteen keys were built out of the head and could not be
// found in a document that never carries it.
//
// The fix is not a list of journal names: it is that a running head REPEATS. Any
// word on more than this share of the reference's pages is treated as furniture and
// kept out of every key. A word genuinely used on most pages of a paper is too
// common to key on anyway, so nothing is lost by the same rule.
const furnitureShare = 0.5

// furnitureOf returns the words that appear on more than furnitureShare of the
// pages: the running head, the journal line, the authors' names in a head. It needs
// at least a few pages to mean anything — on two pages "more than half" is "on both"
// and would strip ordinary repetition — so below that it returns nothing.
func furnitureOf(pages []page) map[string]bool {
	if len(pages) < 4 {
		return nil
	}
	df := map[string]int{}
	for _, p := range pages {
		seen := map[string]bool{}
		for _, w := range p.words {
			if !seen[w] {
				seen[w] = true
				df[w]++
			}
		}
	}
	limit := float64(len(pages)) * furnitureShare
	out := map[string]bool{}
	for w, n := range df {
		if float64(n) > limit {
			out[w] = true
		}
	}
	return out
}

func align(refPath, ourPath string, w io.Writer) error {
	ref, err := pagesOf(refPath)
	if err != nil {
		return fmt.Errorf("%s: %w", refPath, err)
	}
	our, err := pagesOf(ourPath)
	if err != nil {
		return fmt.Errorf("%s: %w", ourPath, err)
	}
	rows := alignPages(ref, our)

	var sb strings.Builder
	fmt.Fprintf(&sb, "### %s: %d pages   %s: %d pages\n", refPath, len(ref), ourPath, len(our))
	fmt.Fprintf(&sb, "%6s %6s %7s %7s %7s  %s\n", "réf", "nous", "décal.", "lig.réf⚠", "lig.ns⚠", "clé")
	firstSlip, found := 0, 0
	for _, r := range rows {
		off, offs := 0, "—"
		switch {
		case r.OurPage == 0:
			offs = "introuv."
		default:
			found++
			off = r.OurPage - r.RefPage
			offs = fmt.Sprintf("%+d", off)
			if off != 0 && firstSlip == 0 {
				firstSlip = r.RefPage
			}
		}
		ours := "—"
		if r.OurPage > 0 {
			ours = fmt.Sprintf("%d", r.OurPage)
		}
		fmt.Fprintf(&sb, "%6d %6s %7s %7d %7d  %s\n",
			r.RefPage, ours, offs, r.RefLines, r.OurLines, trunc(r.Key, 56))
	}
	switch {
	case len(rows) == 0:
		sb.WriteString("aucune page de référence ne porte assez de mots pour une clé\n")
	case found == 0:
		// ⛔ "In phase" is what the first version printed here, because firstSlip was
		// still 0 — and it was 0 because NOTHING had been found. A measurement that
		// found nothing must not report the same thing as one that found agreement.
		sb.WriteString("⛔ AUCUNE clé retrouvée: les deux documents n'ont rien en commun que cet " +
			"instrument sache lire, et ce n'est PAS un alignement réussi\n")
	case found*2 < len(rows):
		fmt.Fprintf(&sb, "⚠ %d clé(s) sur %d seulement retrouvées: l'alignement porte sur moins de la "+
			"moitié des pages et ne décrit pas le document\n", found, len(rows))
	case firstSlip == 0:
		sb.WriteString("les deux documents restent en phase sur toutes les pages appariées\n")
	default:
		fmt.Fprintf(&sb, "décrochage à la page %d de la référence sur %d — c'est LÀ que la page de trop apparaît\n",
			firstSlip, len(ref))
	}
	// ⛔ The two line columns are NOT comparable between engines, and saying so is
	// part of the report. On 2311.08855 the reference reads 51 to 86 lines a page
	// against 28 to 42 here, in ratios from 1.07 to 2.60 — no typography does that.
	// It is how each writer lays out its text runs: one baseline can arrive as
	// several runs at minutely different y, and groupLines then counts several
	// lines. Within ONE document the column is a real signal; across two it is not.
	sb.WriteString("⚠ lig.réf et lig.ns ne sont PAS comparables entre moteurs " +
		"(regroupement des segments de texte, rapports de 1,07 à 2,60 sur un même papier): " +
		"à lire dans UN document, pas entre deux\n")
	_, err = io.WriteString(w, sb.String())
	return err
}

// alignPages walks the reference's pages in order and locates each one's key in
// ours, never searching backwards past a page already matched.
func alignPages(ref, our []page) []alignRow {
	furniture := furnitureOf(ref)
	// ⛔ The haystack is filtered by the SAME rule as the key. Dropping the running
	// head from the key alone would leave it between the key's words in the text, and
	// the key could never match — the vocabulary mistake this file already made once,
	// one level down, with the short words.
	ourText := make([]string, len(our))
	for j, p := range our {
		ourText[j] = joinWithout(p.words, furniture)
	}
	var rows []alignRow
	from := 1
	for i, p := range ref {
		key := keyOf(p.words, furniture)
		if key == "" {
			continue
		}
		row := alignRow{RefPage: i + 1, Key: key, RefLines: p.lines}
		for j := from - 1; j < len(our); j++ {
			if strings.Contains(ourText[j], key) {
				row.OurPage = j + 1
				row.OurLines = our[j].lines
				from = j + 1
				break
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// keyOf builds a page's key from its words. A word is kept only if it is ordinary
// letters: a number is a folio or an equation tag and repeats across documents, and
// punctuation is set differently by the two engines.
func keyOf(words []string, furniture map[string]bool) string {
	var kept []string
	for i, w := range words {
		if i < skipWords || !ordinaryWord(w) || furniture[w] {
			continue
		}
		kept = append(kept, w)
		if len(kept) == keyWords {
			return strings.Join(kept, " ")
		}
	}
	return "" // a page with too few ordinary words cannot be keyed, and says so
}

// joinWithout is the haystack a key is sought in: the page's ordinary words, minus
// the furniture, in order and single-spaced — the same shape keyOf produces.
func joinWithout(words []string, furniture map[string]bool) string {
	kept := make([]string, 0, len(words))
	for _, w := range words {
		if furniture[w] {
			continue
		}
		kept = append(kept, w)
	}
	return strings.Join(kept, " ")
}

func ordinaryWord(w string) bool {
	if len([]rune(w)) < 3 {
		return false
	}
	for _, r := range w {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// page is one page reduced to what an alignment needs.
//
// ⛔ There is ONE word list, and the haystack is built from it at search time by the
// same rule as the key. The first version built the key from filtered words and
// searched the unfiltered text, so a key like "case study analytic protocol analysis
// max" could never match "a case study in analytic protocol analysis in acl2 max":
// the short words the key dropped were still between them in the haystack. Every
// page of a real paper came back "not found". Keeping a second, pre-joined string on
// this struct is how that happened, so there is no longer one.
type page struct {
	words []string // the ordinary words, in order
	lines int
}

// pagesOf reads every page of a PDF and reduces it. It reads through the same
// go-pdfkit path the rest of this command uses — no poppler, no MuPDF — so the line
// count here is the one the side-by-side report shows.
func pagesOf(path string) ([]page, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := openPDF(b)
	if err != nil {
		return nil, err
	}
	// PageCount, not "read until it errors": a page that fails to decode is a
	// damaged page, not the end of the document, and treating the two alike would
	// silently truncate the alignment at the first bad page.
	count := doc.PageCount()
	if count == 0 {
		return nil, fmt.Errorf("aucune page lisible")
	}
	out := make([]page, 0, count)
	for n := 1; n <= count; n++ {
		runs, err := PageRuns(doc, n)
		if err != nil {
			out = append(out, page{}) // unreadable: empty, and it will not match a key
			continue
		}
		lines := groupLines(runs, pageHeight(doc, n), reportOptions{})
		var words []string
		for _, l := range lines {
			for _, w := range strings.Fields(normaliseText(l.Text)) {
				if ordinaryWord(w) {
					words = append(words, w)
				}
			}
		}
		out = append(out, page{words: words, lines: len(lines)})
	}
	return out, nil
}

// normaliseText lowercases and strips what the two engines set differently —
// ligatures the text layer may or may not carry, and the hyphen a line break adds.
func normaliseText(s string) string {
	r := strings.NewReplacer(
		"ﬁ", "fi", "ﬂ", "fl", "ﬀ", "ff", "ﬃ", "ffi", "ﬄ", "ffl",
		"­", "", "‐", "-", "‑", "-",
	)
	return strings.ToLower(r.Replace(s))
}
