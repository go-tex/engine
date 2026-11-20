// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The line straight after a list, centre, quotation or verse is NOT indented
// (\@doendpe, ltlists.dtx in the 2021-11-15 kernel tectonic runs), unless a
// blank line comes between or the list was closed inside {…}. Each golden is
// tectonic's: gotex-linediff on testdata/parity/<name>.tex, x0 minus the
// 133.77 margin.
//
//   - doendpe: before \@doendpe was wired, Charlie, India, Lima and Uniform all
//     started 14.94 bp in. Foxtrot (blank line), Oscar (closed in braces) and
//     Romeo (\noindent) must NOT move.
//   - doendpereset: a raw \trivlist…\endtrivlist raises \@endpetrue that no
//     \end consumes, and an environment opened after it must not inherit it —
//     \begin's \@endpefalse, done in doCheckEnv. Without that line Echo was
//     set flush where the reference indents it; nothing in the first witness
//     can see it, because re-running \@doendpe is otherwise idempotent.
//
// Only the asserted lines are compared: the bullets, the centred line, the
// trivlist item and the quote margins differ from the reference for other
// reasons (#587), and a golden over every line would pin those too.
func TestParityLineAfterAListIsNotIndented(t *testing.T) {
	for _, w := range []struct {
		name  string
		lines int            // text lines on the page, the page number included
		at    map[string]int // where each asserted line sits among them
	}{
		{"doendpe", 22, map[string]int{"Charlie": 2, "Foxtrot": 5, "India": 8, "Lima": 11, "Oscar": 14, "Romeo": 17, "Uniform": 20}},
		{"doendpereset", 6, map[string]int{"Delta": 3, "Echo": 4}},
	} {
		ref, err := os.ReadFile(filepath.Join("testdata", "parity", w.name+".tectonic.indents.txt"))
		if err != nil {
			t.Fatal(err)
		}
		// The count is checked first, so an index can never silently name a
		// different line.
		got := bodyLineIndents(t, w.name)
		if len(got) != w.lines {
			t.Fatalf("%s: read %d lines, want %d: %v", w.name, len(got), w.lines, got)
		}
		want := strings.Split(strings.TrimSpace(string(ref)), "\n")
		if len(want) != len(w.at) {
			t.Fatalf("%s: golden has %d lines, the table %d", w.name, len(want), len(w.at))
		}
		for _, row := range want {
			name, x, ok := strings.Cut(row, " ")
			i, known := w.at[name]
			if !ok || !known {
				t.Fatalf("%s: golden row %q names no line of the table", w.name, row)
			}
			if got[i] != x {
				t.Errorf("%s: %s starts %s bp in, tectonic's starts %s", w.name, name, got[i], x)
			}
		}
	}
}
