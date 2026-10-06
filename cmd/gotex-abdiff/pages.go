// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"

	"github.com/go-pdfkit/reader"
)

// pageCount reports how many pages a PDF has, by PARSING it.
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
func pageCount(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, fmt.Errorf("%s is empty", path)
	}
	doc, err := reader.Open(b)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	n := doc.PageCount()
	if n == 0 {
		return 0, fmt.Errorf("%s: parsed, but it has no pages", path)
	}
	return n, nil
}
