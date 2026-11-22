// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// acmart asks \IfFileExists for libertine.sty, zi4.sty and newtxmath.sty and, on
// any NO, sets all its fonts aside. A package the engine provides is answered YES
// with or without the extension; an unknown one stays NO.
func TestIfFileExistsAnswersYesForAProvidedPackage(t *testing.T) {
	withTempDir(t, map[string]string{}, func() {
		out, _ := runLaTeX(t, `\documentclass{article}`+
			`\IfFileExists{libertine.sty}{\message{HAVE-LIB}}{\message{NO-LIB}}`+
			`\IfFileExists{zi4.sty}{\message{HAVE-ZI4}}{\message{NO-ZI4}}`+
			`\IfFileExists{newtxmath.sty}{\message{HAVE-NTX}}{\message{NO-NTX}}`+
			`\IfFileExists{amsmath}{\message{HAVE-AMS}}{\message{NO-AMS}}`+
			`\IfFileExists{nosuchpackage.sty}{\message{HAVE-GHOST}}{\message{NO-GHOST}}`+
			`\IfFileExists{nosuchpackage}{\message{HAVE-GHOST2}}{\message{NO-GHOST2}}`+
			`\IfFileExists{libertine.cls}{\message{HAVE-CLS}}{\message{NO-CLS}}`+
			`\IfFileExists{tikz.sty}{\message{HAVE-TIKZ}}{\message{NO-TIKZ}}`)
		for _, want := range []string{"HAVE-LIB", "HAVE-ZI4", "HAVE-NTX", "HAVE-AMS", "NO-GHOST", "NO-GHOST2", "NO-CLS", "NO-TIKZ"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in %q", want, out)
			}
		}
		for _, bad := range []string{"NO-LIB", "NO-ZI4", "NO-NTX", "NO-AMS", "HAVE-GHOST", "HAVE-CLS", "HAVE-TIKZ"} {
			if strings.Contains(out, bad) {
				t.Errorf("unexpected %q in %q", bad, out)
			}
		}
	})
}

func TestEngineProvidesStyTable(t *testing.T) {
	for name, want := range map[string]bool{
		"libertine": true, "libertine.sty": true, "zi4": true, "newtxmath.sty": true,
		"hyperref": true, "geometry.sty": true, "tgtermes": true,
		"tikz": false, "pgfplots.sty": false, "unknown": false, "libertine.cls": false, "": false,
	} {
		if got := engineProvidesSty(name); got != want {
			t.Errorf("engineProvidesSty(%q) = %v, want %v", name, got, want)
		}
	}
}
