// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A bundled bm.sty must NOT be loaded: bm is on neverLoadReal because its real
// implementation builds bold-math commands from low-level math-alphabet machinery
// the engine does not run, and its \protected@edef\bm#1{\bm{#1}} re-dispatch
// expands the robust \bm against the engine's non-protecting \protected@edef and
// swallows the whole document. The kernel's \bm (= \boldsymbol) stands instead. A
// bundled bm.sty here defines \bm to a marker; loading it would surface the marker.
func TestBundledBmIsNotLoaded(t *testing.T) {
	withTempDir(t, map[string]string{
		"bm.sty": `\ProvidesPackage{bm}\def\bm#1{ZZWRONGBM}`,
	}, func() {
		out, _ := runLaTeX(t, `\usepackage{bm}\message{[\meaning\bm]}`)
		if strings.Contains(out, "ZZWRONGBM") {
			t.Errorf("bundled bm.sty was loaded (neverLoadReal did not cover bm); \\bm = %q", out)
		}
	})
}
