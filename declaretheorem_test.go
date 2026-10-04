// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \declaretheorem is thmtools' key-value front end onto \newtheorem. Skipped, the
// environments it declares arrive undefined, which costs the heading and its number —
// the body of an undefined environment still reaches the page. 219 corpus papers use it,
// 1735 times.
//
// Every assertion below comes from thm-kv.sty rather than from the package's prose,
// because three details are not guessable and two of them are silent when wrong.

func declThmGlyphs(t *testing.T, decl string) int {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsthm,thmtools}`+decl+
		`\begin{document}\begin{thmx}BODYWORD visible.\end{thmx}\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(strings.Join(e.RenderPages(e.renderMargin(0)), ""), "<path")
}

// With the SAME heading text on both sides the two declarations must draw the same
// glyphs. Comparing \declaretheorem's default heading against \newtheorem{thmx}{Theorem}
// would compare "Thmx 1." with "Theorem 1." and read the difference as a defect.
func TestDeclaretheoremDrawsTheSameHeadingAsNewtheorem(t *testing.T) {
	want := declThmGlyphs(t, `\newtheorem{thmx}{Theorem}`)
	got := declThmGlyphs(t, `\declaretheorem[name=Theorem]{thmx}`)
	if got != want {
		t.Errorf("\\declaretheorem[name=Theorem] drew %d glyph paths, \\newtheorem drew %d", got, want)
	}
}

// ⛔ The two counter keys map to the two DIFFERENT optional arguments of \newtheorem
// (thm-kv.sty:362). Swapping them gives numbering that looks plausible and is wrong, so
// the test reads the meanings back rather than counting glyphs.
func TestDeclaretheoremCounterKeysMapToTheRightOptionalArgument(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsthm,thmtools}\makeatletter`+
		`\newtheorem{theorem}{Theorem}[section]`+
		`\declaretheorem[sibling=theorem]{corx}`+
		`\declaretheorem[parent=section]{indx}`+
		`\message{[corx:\meaning\thecorx][indx:\meaning\theindx]}`+
		`\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := e.Diagnostics().Messages
	// sibling SHARES, so it delegates to the other environment's \the…
	if !strings.Contains(got, `[corx:macro:->\thetheorem `) {
		t.Errorf("sibling= did not share the counter: %s", got)
	}
	// parent NESTS, so the number carries the parent's formatted number in front.
	if !strings.Contains(got, `\thesection .\the \c@indx`) {
		t.Errorf("parent= did not nest within the counter: %s", got)
	}
}

// {<names>} is a comma list (thm-kv.sty:337). Declaring one of two would leave the other
// an undefined environment, which loses only its heading — silent in a page count.
func TestDeclaretheoremDeclaresEveryNameInTheList(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsthm,thmtools}\makeatletter`+
		`\declaretheorem{alpha,beta}`+
		`\message{[a:\meaning\thealpha][b:\meaning\thebeta]}`+
		`\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := e.Diagnostics().Messages
	for _, want := range []string{`[a:macro:->\the `, `[b:macro:->\the `} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}

// The default heading is the environment name with its FIRST LETTER uppercased, not the
// whole word: thm-kv.sty:354 is \thmt@setthmname{\thmt@modifycase #1} with NO braces, and
// \thmt@modifycase defaults to \MakeUppercase (:42), which takes one token.
func TestDeclaretheoremDefaultHeadingUppercasesOnlyTheFirstLetter(t *testing.T) {
	if got, want := upperFirstRune("theorem"), "Theorem"; got != want {
		t.Errorf("upperFirstRune(theorem) = %q, want %q", got, want)
	}
	if got, want := upperFirstRune("édition"), "Édition"; got != want {
		t.Errorf("upperFirstRune of a multi-byte rune = %q, want %q", got, want)
	}
}

// numbered=no is the starred form (thm-kv.sty:362), which draws a heading and no number.
func TestDeclaretheoremNumberedNoDrawsNoNumber(t *testing.T) {
	numbered := declThmGlyphs(t, `\declaretheorem[name=Theorem]{thmx}`)
	plain := declThmGlyphs(t, `\declaretheorem[name=Theorem,numbered=no]{thmx}`)
	if plain >= numbered {
		t.Errorf("numbered=no drew %d glyph paths, numbered drew %d — the number is still there",
			plain, numbered)
	}
}
