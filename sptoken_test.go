// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \@sptoken is a SPACE token (latex.ltx:1619, \def\:{\let\@sptoken= } \:), and
// LaTeX's own look-ahead tests against it with \ifx. Left undefined, every such test
// compares a space against an undefined control sequence and takes the wrong branch
// in silence.
//
// keyval's space trimmer is built on that test (keyval.sty:47-52):
//
//	\def\KV@@sp@d{\ifx\KV@tempa\@sptoken \expandafter\KV@@sp@b
//	  \else\expandafter\KV@@sp@b\expandafter#1\fi}
//
// so \setkeys{fam}{ key = value } looked the key up as "\KV@fam@ key" and dropped
// the call — and a preamble writes its options with spaces. tectonic 0.17.0 renders
// both forms; before this, the spaced one was lost with nothing reported.
//
// The key body defines a macro GLOBALLY (\gdef) and the test reads the engine's
// table instead of counting glyph paths in the SVG. Global because \end{document}
// pops the document group: a \def there is gone by the time compile returns, which
// made this test fail on a fix the binary plainly applied.
// The old note, still true: the table rather than the
// render, because a repeated glyph is not one path per character and counting them
// has already produced a wrong answer about this very witness.
func TestSptokenIsASpaceSoKeyvalTrims(t *testing.T) {
	const src = `\documentclass{article}\usepackage{keyval}\makeatletter` +
		`\define@key{fam}{name}{\expandafter\gdef\csname ran#1\endcsname{}}` +
		`\makeatother\begin{document}` +
		`\setkeys{fam}{name=TIGHT}\setkeys{fam}{ name = SPACED }` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if m := e.meaningOf(csTok("@sptoken")); m == nil {
		t.Error("\\@sptoken is undefined; keyval's space trimming cannot work")
	} else if m.kind != mLetChar || m.ch != ' ' || m.cat != catSpace {
		t.Errorf("\\@sptoken = kind %v ch %q cat %v, want mLetChar ' ' catSpace",
			m.kind, m.ch, m.cat)
	}
	for _, name := range []string{"ranTIGHT", "ranSPACED"} {
		if e.meaningOf(csTok(name)) == nil {
			t.Errorf("\\%s undefined: \\setkeys never ran that key", name)
		}
	}
}
