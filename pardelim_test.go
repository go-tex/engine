// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A macro whose parameter text ends in " \par" (a space then \par) relies on TeX
// emitting the interword space that a MID-LINE line-ending leaves before the
// blank-line \par: content, then end-of-line (state M → space), then a blank line
// (→ \par). This is a real LaTeX idiom — sn-jnl's \abstract#1 \par is the one that
// motivated the fix; every Springer-Nature paper rendered 0 pages because the
// scanner collapsed the two line-endings into a bare \par, the delimiter never
// matched, and \abstract swallowed the keywords, \maketitle and the whole body.
func TestParDelimiterKeepsInterwordSpace(t *testing.T) {
	// \abstractlike grabs up to " \par"; the blank line after the abstract must
	// present that delimiter so the body that follows is NOT part of the argument.
	src := `\long\def\abstractlike#1 \par{\def\stored{#1}}` +
		"\\abstractlike Short abstract text.\n\nBODYWORD survives here.\n"
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "BODYWORD") {
		t.Errorf("body swallowed by the \\par-delimited macro (missing space before \\par); want BODYWORD, got %q", got)
	}
}
