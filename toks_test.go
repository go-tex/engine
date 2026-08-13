// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A token register set with a braced list stores it verbatim and \the reads it back.
func TestToksStoreAndThe(t *testing.T) {
	out := mustRun(t, `\toks5{a b c}\message{[\the\toks5 ]}`)
	if !strings.Contains(out, "[a b c]") {
		t.Errorf("\\the\\toks5 = %q, want to contain [a b c]", out)
	}
}

// \newtoks allocates a distinct register; \the on an unset one is empty.
func TestNewtoksAllocatesEmpty(t *testing.T) {
	out := mustRun(t, `\newtoks\mytoks\message{[\the\mytoks]}\mytoks{X}\message{[\the\mytoks]}`)
	if !strings.Contains(out, "[]") || !strings.Contains(out, "[X]") {
		t.Errorf("newtoks store/read = %q, want [] then [X]", out)
	}
}

// The amsart idiom \reg\expandafter{\the\reg more} appends to a register: \expandafter
// expands \the\reg (its current contents) before the group is grabbed.
func TestToksExpandafterAppend(t *testing.T) {
	out := mustRun(t, `\newtoks\t\t{A}\t\expandafter{\the\t B}\t\expandafter{\the\t C}\message{[\the\t]}`)
	if !strings.Contains(out, "[ABC]") {
		t.Errorf("expandafter append = %q, want [ABC]", out)
	}
}

// One register can be copied from another (\a\b), including from an empty one.
func TestToksCopyRegister(t *testing.T) {
	out := mustRun(t, `\newtoks\a\newtoks\b\a{hi}\b\a\message{[\the\b]}`)
	if !strings.Contains(out, "[hi]") {
		t.Errorf("toks copy = %q, want [hi]", out)
	}
}

// \the\toks inside \edef captures the stored tokens (they expand into the edef body).
func TestToksInEdef(t *testing.T) {
	out := mustRun(t, `\newtoks\t\t{PQR}\edef\x{<\the\t>}\message{\x}`)
	if !strings.Contains(out, "<PQR>") {
		t.Errorf("\\the\\toks in \\edef = %q, want <PQR>", out)
	}
}
