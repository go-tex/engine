// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// amsart is gated to the built-in emulation for \documentclass{amsart}: its own
// \newtheorem…[section] machinery loops on the engine (the runaway guard is
// expansion-only and does not catch it), so a real math paper would hang. The real
// amsart.cls is still embedded and remains loadable via \LoadClass, and the class
// kernel additions it drove — token registers (toks_test.go), the ## parameter-char
// fix, and the plain-TeX substrate (amssubstrate_test.go) — stay under test and
// benefit every real class/package. Routing \documentclass{amsart} to the real
// class waits on the \newtheorem fix.
func TestAmsartGatedButEmbedded(t *testing.T) {
	if !emulatedClasses["amsart"] {
		t.Fatal("amsart should be gated to the emulation (its \\newtheorem loops)")
	}
	if _, ok := embeddedTeXFile("amsart.cls"); !ok {
		t.Fatal("amsart.cls is not embedded in texmf/")
	}
}
