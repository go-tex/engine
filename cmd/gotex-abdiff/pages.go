// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"

	"github.com/go-tex/engine/cmd/internal/pdfruns"
)

// measurement is what one side's PDF yields: its pagination AND its ink.
//
// ⛔ WHY THE INK IS HERE AND NOT IN A SECOND TOOL. A page count cannot tell
// "the same text, broken differently" from "text that appeared or vanished",
// and those are opposite findings. Σ pages moving while the glyph total holds
// still is a BREAKING change — the ink was re-dealt across pages, which is what
// a penalty or a measure change does. The glyph total moving is a CONTENT
// change: a macro that now expands, or output that got dropped.
//
// That question cost a whole second protocol earlier in this corpus's history,
// because the only code here that reads glyphs back out of a PDF lived inside
// gotex-linediff as package main. It is cmd/internal/pdfruns now, and one
// A/B run answers both halves.
//
// ⚠ The ink is a WITNESS, NOT A SCORE. The corpus's reference file carries page
// counts and nothing else, so there is no reference glyph total to be near or
// far from: this says whether a change moved ink, never whether the ink is
// right. The report says so in as many words, because a column that looks like
// the page columns will be read like them.
type measurement struct {
	Pages  int
	Glyphs int
	// BadPages are the pages that parsed as pages but whose text could not be
	// decoded. Glyphs is SHORT by whatever they held, so the number travels
	// with it rather than being folded in as zero — an unreadable page is not an
	// empty one, and a total that absorbed it would make a damaged document look
	// like a thin one.
	BadPages int
}

// measure parses a PDF and reports its pagination and its ink.
//
// ⛔ It returns an error rather than a count of zero, and that distinction is the
// point. The measurements this tool replaces counted `/Type /Page` with a regular
// expression over the raw bytes, which has two failure modes that both look like
// a number:
//
//   - a PDF whose objects live in an object stream (which is what the reference
//     engine writes) has no such text at all, so the regex reported ZERO pages
//     for a perfectly good 334-page document;
//   - a file that was never written — a compile that timed out, a path that was
//     wrong — also reports zero, so "no output" and "an empty document" and "I
//     could not read it" all arrived as the same number.
//
// A reader that parses the cross-reference table reads both engines' output, and
// anything it cannot read is an ERROR that the report counts separately and
// refuses to score.
func measure(path string) (measurement, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return measurement{}, err
	}
	if len(b) == 0 {
		return measurement{}, fmt.Errorf("%s is empty", path)
	}
	doc, err := pdfruns.Open(b)
	if err != nil {
		return measurement{}, fmt.Errorf("%s: %w", path, err)
	}
	n := doc.PageCount()
	if n == 0 {
		return measurement{}, fmt.Errorf("%s: parsed, but it has no pages", path)
	}
	glyphs, bad := pdfruns.Glyphs(doc)
	return measurement{Pages: n, Glyphs: glyphs, BadPages: bad}, nil
}
