// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"os"
)

// GOTEX_FLTRACE=1 makes the float placer report every decision and the quantities it
// took it on, the way LaTeX's `fltrace` package does
// (ltoutput.dtx:4127-4323, 6010-6049).
//
// ⭐ WHY IT EXISTS. A PDF shows where a float LANDED and never why. LaTeX can be asked:
// `\usepackage{fltrace}\tracefloats` prints \@fpstype, \@textmin, \@reqcolroom,
// \@colroom, \@toproom, \@colnum and the verdict for each float, so the reference will
// report its own decisions. Without the same trace on this side there is nothing to
// compare them against, and a divergence can only be guessed at.
//
// ⛔ It was built after a guess failed. The reference's trace showed a 224.06pt float
// refused at the top because \@toproom had fallen to 220.94pt; the same arithmetic was
// written here, predicted to move the float to the next page, and did not. The
// prediction was about OUR numbers, which nothing had measured.
//
// The format deliberately echoes fltrace's wording — "Space OK", "Fail---no room",
// "toproom", "float size" — so the two logs can be read side by side without a
// translation table.
func flTracing() bool { return os.Getenv("GOTEX_FLTRACE") == "1" }

func flTrace(format string, a ...any) {
	if !flTracing() {
		return
	}
	fmt.Fprintf(os.Stderr, "gotex-fl: "+format+"\n", a...)
}

// spToStr prints a scaled dimension the way TeX's \the does, so a trace line can be
// compared with fltrace's character for character.
func spToStr(sp int) string { return fmt.Sprintf("%.5fpt", spToPt(sp)) }
