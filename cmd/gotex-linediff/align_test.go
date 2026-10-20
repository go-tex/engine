// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// multiPage writes a document of n pages whose pages are distinguishable: each one
// opens with words found nowhere else, so an alignment has something to key on.
func multiPage(t *testing.T, n int, extraFirst string) []byte {
	t.Helper()
	var b strings.Builder
	b.WriteString("\\documentclass{article}\n\\begin{document}\n")
	b.WriteString(extraFirst)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString("\\newpage\n")
		}
		// Ordinary words, three letters or more, unique to this page.
		for _, w := range pageWords(i) {
			b.WriteString(w + " ")
		}
		b.WriteString("\n")
	}
	b.WriteString("\\end{document}\n")
	return pdfOf(t, b.String())
}

// ⛔ Every line here carries SHORT words (a, in, of, to) between the distinctive
// ones, because real prose does. The first version of this fixture used only words
// of three letters or more, so the key's vocabulary and the haystack's happened to
// coincide and the control passed on a document where they could not disagree. On a
// real paper every page came back "not found".
func pageWords(i int) []string {
	base := []string{
		"a alpha in bravo of charlie to delta on echo by foxtrot at golf and hotel for igloo",
		"a india in juliett of kilo to lima on mike by november at oscar and papa for romeo",
		"a quebec in sierra of tango to uniform on victor by whiskey at xenon and yacht for zebra",
		"a yankee in zulu of abacus to bandage on candle by dagger at ember and fathom for girder",
		"a garnet in harbour of ingot to jasmine on kernel by lantern at marble and nectar for orchid",
		"a opal in parcel of quarry to ribbon on saddle by timber at umber and vellum for walnut",
	}
	return strings.Fields(base[i%len(base)])
}

func write(t *testing.T, dir, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// ⛔ THE CONTROL. A document aligned against ITSELF must come out at offset 0 on
// every page. An alignment that cannot do that cannot be trusted to say anything
// about two different documents — and the key is a heuristic (six ordinary words
// from near the top), so this is not a formality.
func TestAlignIsExactAgainstItself(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "a.pdf", multiPage(t, 5, ""))
	var buf bytes.Buffer
	if err := align(p, p, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "restent en phase") {
		t.Errorf("un document contre lui-même doit rester en phase partout:\n%s", out)
	}
	if strings.Contains(out, "introuv.") {
		t.Errorf("une clé de ce document est introuvable DANS LUI-MÊME:\n%s", out)
	}
	for _, slip := range []string{"+1", "-1", "+2"} {
		if strings.Contains(out, " "+slip+" ") {
			t.Errorf("décalage %s contre lui-même:\n%s", slip, out)
		}
	}
}

// And it must NAME the page where an extra page appears, not merely notice that the
// counts differ. A page inserted at the front shifts everything from page 1.
func TestAlignNamesTheFirstPageThatSlips(t *testing.T) {
	dir := t.TempDir()
	ref := write(t, dir, "ref.pdf", multiPage(t, 4, ""))
	// The same four pages, preceded by a page of their own: every reference page
	// now sits one later in ours.
	ours := write(t, dir, "ours.pdf", multiPage(t, 4, "Preliminary matter that nobody asked for.\\newpage\n"))
	var buf bytes.Buffer
	if err := align(ref, ours, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "décrochage à la page 1") {
		t.Errorf("une page ajoutée au DÉBUT doit faire décrocher dès la page 1:\n%s", out)
	}
}

// A key that is nowhere in the other document is reported as NOT FOUND. Reporting
// it as offset 0 would read as "aligned", which is the opposite of what it means.
func TestAlignSaysNotFoundRatherThanZero(t *testing.T) {
	dir := t.TempDir()
	ref := write(t, dir, "ref.pdf", multiPage(t, 3, ""))
	other := write(t, dir, "other.pdf", pdfOf(t, `\documentclass{article}
\begin{document}
Nothing here resembles the other document whatsoever today.
\end{document}`))
	var buf bytes.Buffer
	if err := align(ref, other, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "introuv.") {
		t.Errorf("une clé absente doit se lire « introuvable », pas 0:\n%s", out)
	}
	// ⛔ And the SUMMARY must not read as success. The first version left firstSlip
	// at 0 when nothing was found and printed "les deux documents restent en phase",
	// which is what a perfect alignment prints.
	if strings.Contains(out, "restent en phase") {
		t.Errorf("un alignement qui n'a RIEN trouvé se déclare en phase:\n%s", out)
	}
	if !strings.Contains(out, "AUCUNE clé retrouvée") {
		t.Errorf("le résumé ne dit pas que rien n'a été trouvé:\n%s", out)
	}
}

// Short words sit between the distinctive ones in real prose. The key is built
// from the ordinary words only, so the text it is sought in must be too — or the
// key can never match and every page reads "not found".
func TestKeyAndHaystackShareOneVocabulary(t *testing.T) {
	words := strings.Fields("a case study in analytic protocol analysis in acl2 max von hippel")
	var ordinary []string
	for _, w := range words {
		if ordinaryWord(w) {
			ordinary = append(ordinary, w)
		}
	}
	key := keyOf(ordinary, nil)
	if key == "" {
		t.Fatal("aucune clé bâtie")
	}
	if strings.Contains(strings.Join(words, " "), key) {
		t.Skip("ce témoin n'oppose pas les deux vocabulaires")
	}
	if !strings.Contains(strings.Join(ordinary, " "), key) {
		t.Errorf("la clé %q ne se retrouve pas dans le vocabulaire qui l'a produite", key)
	}
}

// The search never goes back BEFORE a page already matched. Text repeated across
// pages — a running head, a reused phrase — would otherwise pull a later reference
// page's match backwards and report an alignment that walks the wrong way.
//
// ⚠ It does not forbid the SAME page twice, and must not: when we pack two of the
// reference's pages into one of ours, both of its keys are genuinely on that page,
// and the −1 that follows is the measurement, not an artefact.
func TestAlignNeverMatchesBeforeAPageAlreadyMatched(t *testing.T) {
	ref := []page{
		{words: strings.Fields("skip me alpha bravo charlie delta echo foxtrot")},
		{words: strings.Fields("skip me india juliett kilo lima mike november")},
	}
	// Our page 1 carries the SECOND reference page's words, and our page 2 carries
	// both. Reference page 1 therefore matches our page 2 — and reference page 2,
	// whose key is also on our page 1, must not be dragged back there.
	our := []page{
		{words: strings.Fields("india juliett kilo lima mike november"), lines: 3},
		{words: strings.Fields("alpha bravo charlie delta echo foxtrot india juliett kilo lima mike november"), lines: 4},
	}
	rows := alignPages(ref, our)
	if len(rows) != 2 {
		t.Fatalf("%d lignes, attendu 2", len(rows))
	}
	if rows[0].OurPage != 2 {
		t.Errorf("page 1 appariée à %d, attendu 2", rows[0].OurPage)
	}
	if rows[1].OurPage != 2 {
		t.Errorf("page 2 appariée à %d: la recherche est repartie AVANT une page déjà appariée", rows[1].OurPage)
	}
}

// Two reference pages landing on ONE of ours is a real measurement — we packed
// them together — and must come out as such rather than as a refusal.
func TestAlignReportsTwoReferencePagesOnOneOfOurs(t *testing.T) {
	ref := []page{
		{words: strings.Fields("skip me alpha bravo charlie delta echo foxtrot")},
		{words: strings.Fields("skip me india juliett kilo lima mike november")},
	}
	our := []page{{words: strings.Fields("alpha bravo charlie delta echo foxtrot india juliett kilo lima mike november"), lines: 9}}
	rows := alignPages(ref, our)
	if len(rows) != 2 || rows[0].OurPage != 1 || rows[1].OurPage != 1 {
		t.Fatalf("appariements = %+v, attendu les deux sur notre page 1", rows)
	}
}

// A page with too few ordinary words cannot be keyed, and gets no row rather than a
// row keyed on a folio.
func TestKeyOfRefusesAPageWithoutOrdinaryWords(t *testing.T) {
	for _, c := range []struct {
		name  string
		words []string
	}{
		{"folio seul", strings.Fields("12")},
		{"chiffres", strings.Fields("1 2 3 4 5 6 7 8 9")},
		{"mots trop courts", strings.Fields("a b c de fg hi jk lm no")},
	} {
		if k := keyOf(c.words, nil); k != "" {
			t.Errorf("%s: clé %q bâtie sur une page sans mots ordinaires", c.name, k)
		}
	}
	if k := keyOf(strings.Fields("skip me alpha bravo charlie delta echo foxtrot golf"), nil); k != "alpha bravo charlie delta echo foxtrot" {
		t.Errorf("clé = %q", k)
	}
}

// ⛔ A running head repeats on every page, and no fixed number of skipped words
// gets past one. On 2311.08855 the reference prints "Real Analysis Using …" atop
// every odd page while this engine prints no head at all, so nine of eighteen keys
// were built out of the head and could not be found in a document that never
// carries it.
func TestFurnitureKeepsARunningHeadOutOfEveryKey(t *testing.T) {
	head := "real analysis using concurrent systems"
	var pages []page
	for i := 0; i < 6; i++ {
		body := strings.Fields(pageWords(i)[0] + " " + strings.Join(pageWords(i), " "))
		words := append(strings.Fields(head), body...)
		var ordinary []string
		for _, w := range words {
			if ordinaryWord(w) {
				ordinary = append(ordinary, w)
			}
		}
		pages = append(pages, page{words: ordinary})
	}
	f := furnitureOf(pages)
	for _, w := range strings.Fields(head) {
		if ordinaryWord(w) && !f[w] {
			t.Errorf("%q revient sur chaque page et n'est pas reconnu comme mobilier", w)
		}
	}
	for i := range pages {
		if k := keyOf(pages[i].words, f); strings.Contains(k, "analysis") || strings.Contains(k, "concurrent") {
			t.Errorf("page %d: la clé %q est bâtie sur le titre courant", i+1, k)
		}
	}
	// And a document too short for the rule to mean anything gets no furniture at
	// all: on two pages "more than half" is "on both", which is ordinary repetition.
	if f := furnitureOf(pages[:2]); len(f) != 0 {
		t.Errorf("du mobilier déduit de 2 pages: %v", f)
	}
}

// The haystack must be filtered by the same rule as the key, or the furniture is
// still sitting between the key's words in the text it is sought in.
func TestFurnitureIsRemovedFromTheHaystackToo(t *testing.T) {
	// Six pages, so the "more than half the pages" rule has something to work on,
	// each opening with the same running head.
	var ref, our []page
	for i := 0; i < 6; i++ {
		body := strings.Join(pageWords(i), " ")
		var ordinary []string
		for _, w := range strings.Fields("skip me running head " + body) {
			if ordinaryWord(w) {
				ordinary = append(ordinary, w)
			}
		}
		ref = append(ref, page{words: ordinary})
		// Ours carries the same body and the head in a DIFFERENT place, which is
		// what a head at the bottom, or no head at all, amounts to for a key.
		var ours []string
		for _, w := range strings.Fields("skip me " + body + " running head") {
			if ordinaryWord(w) {
				ours = append(ours, w)
			}
		}
		our = append(our, page{words: ours})
	}
	rows := alignPages(ref, our)
	if len(rows) != 6 {
		t.Fatalf("%d ligne(s), attendu 6", len(rows))
	}
	for i, r := range rows {
		if r.OurPage != i+1 {
			t.Errorf("page %d: appariée à %d (clé %q) — le mobilier reste dans la botte de foin",
				i+1, r.OurPage, r.Key)
		}
	}
}

// Ligatures and the soft hyphen are set differently by the two engines; the key
// must not depend on which one a text layer happened to carry.
func TestNormaliseTextFoldsWhatDiffersBetweenEngines(t *testing.T) {
	if got, want := normaliseText("Diﬀerent ﬁrst"), "different first"; got != want {
		t.Errorf("normaliseText = %q, attendu %q", got, want)
	}
	if got := normaliseText("sepa\u00adrate"); got != "separate" {
		t.Errorf("le trait d'union conditionnel survit: %q", got)
	}
}
