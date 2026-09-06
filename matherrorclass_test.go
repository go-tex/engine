// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A math refusal that names no unknown command used to be tallied under the single
// opaque key "$math$", so a corpus census could not tell one reason from another:
// 19 documents and 52 equations, all indistinguishable. mathErrorClass gives each
// reason a bounded name.
func TestMathErrorClassNamesTheReason(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`texmath: unknown environment "pmatrix*"`, `texmath:␣unknown␣environment␣"pmatrix*"`},
		{"texmath: unexpected \"}\"", "texmath:␣unexpected␣\"}\""},
		// Digits are folded so a position or a size does not make every message its
		// own class.
		{"bad index 12 at offset 345", "bad␣index␣#␣at␣offset␣#"},
		// A blank is written visibly: the failing token is often a control space, and
		// collapsing whitespace printed "unknown command \" — a reason naming nothing.
		{`texmath: unknown command \ `, `texmath:␣unknown␣command␣\`},
		{"   ", "(sans message)"},
	} {
		if got := mathErrorClass(c.in); got != c.want {
			t.Errorf("mathErrorClass(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The key stays bounded: a very long message is truncated.
func TestMathErrorClassIsBounded(t *testing.T) {
	long := ""
	for i := 0; i < 50; i++ {
		long += "abcdefghij"
	}
	if got := mathErrorClass(long); len(got) > 60 {
		t.Errorf("key is %d bytes, want at most 60", len(got))
	}
}
