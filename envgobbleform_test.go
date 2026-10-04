// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ An environment has TWO spellings of its own boundary, and gobbleEnvBody saw one.
//
// \begin{X} and \end{X} EXPAND to \X and \endX. So an \edef over a macro that contains an
// environment leaves the ONE-TOKEN form — measured directly:
//
//	\def\zzs{Author\begin{tikzpicture}\end{tikzpicture}}
//	\edef\zzx{\zzs}   ->   macro:->Author\tikzpicture \endtikzpicture
//
// Executing that runs \tikzpicture, which lands in the gobbler looking for a \end followed
// by a braced name, finds none, and reads to the end of the document.
//
// \MakeUppercase is exactly that shape — \edef\@MakeCase@a{#1} then
// \uppercase\expandafter{…} (classkernel.go) — so \MakeUppercase of a macro holding an
// environment swallowed everything after it (#535). It is also what truncated 2603.18955 to
// ONE page out of 139KB of source through lmcs's \maketitle: with both forms recognised it
// comes out at 49 pages with no group left open (#517).
//
// Two sub-hypotheses were refuted on the way, and the tests below pin what actually matters
// rather than either of them: the case shift is NOT the cause (\MakeLowercase failed
// identically), and \protected@edef does not help (\begin carries no \protect).

func envGobbleRun(t *testing.T, src string) (int, int) {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\usepackage{tikz}`+src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	return strings.Count(svg, "<path"), e.Diagnostics().OpenGroups
}

// The one-token form must terminate the gobbler. Z after the \MakeUppercase is the witness:
// unterminated, the gobbler ate it along with the rest of the document.
func TestTheOneTokenEnvironmentEndTerminatesTheGobbler(t *testing.T) {
	const src = `\def\zzs{\begin{tikzpicture}\end{tikzpicture}}` +
		`\begin{document}\MakeUppercase{\zzs}Z\end{document}`
	paths, groups := envGobbleRun(t, src)
	if groups != 0 {
		t.Errorf("%d group(s) left open, want 0: the gobbler did not find \\endtikzpicture", groups)
	}
	// Z and the page number. Swallowed, only the page number is drawn.
	if paths != 2 {
		t.Errorf("%d glyph path(s), want 2 (Z and the page number): the text after the "+
			"environment was swallowed", paths)
	}
}

// ⛔ \MakeLowercase fails the same way without the fix, which is what says the case shift is
// not the mechanism. Both are asserted so a future change that only handles uppercasing
// cannot pass.
func TestBothCaseFoldersSurviveAnEnvironmentInTheirArgument(t *testing.T) {
	for _, cmd := range []string{"MakeUppercase", "MakeLowercase"} {
		src := `\def\zzs{\begin{tikzpicture}\end{tikzpicture}}` +
			`\begin{document}\` + cmd + `{\zzs}Z\end{document}`
		paths, groups := envGobbleRun(t, src)
		if groups != 0 || paths != 2 {
			t.Errorf("\\%s: %d glyph path(s) and %d group(s) open, want 2 and 0",
				cmd, paths, groups)
		}
	}
}

// Nesting still counts, and it counts in the one-token form too: an inner \tikzpicture must
// raise the depth so the first \endtikzpicture does not close the outer one.
func TestNestingCountsInTheOneTokenForm(t *testing.T) {
	const src = `\def\zzs{\begin{tikzpicture}\begin{tikzpicture}\end{tikzpicture}` +
		`\end{tikzpicture}}` + `\begin{document}\MakeUppercase{\zzs}Z\end{document}`
	paths, groups := envGobbleRun(t, src)
	if groups != 0 {
		t.Errorf("%d group(s) left open, want 0", groups)
	}
	if paths != 2 {
		t.Errorf("%d glyph path(s), want 2: a nested environment closed the outer one early "+
			"or not at all", paths)
	}
}

// The braced form must keep working — it is how a document writes an environment, and the
// one-token form only ever arrives through expansion.
func TestTheBracedEnvironmentEndStillTerminatesTheGobbler(t *testing.T) {
	const src = `\begin{document}\begin{tikzpicture}\end{tikzpicture}Z\end{document}`
	paths, groups := envGobbleRun(t, src)
	if groups != 0 {
		t.Errorf("%d group(s) left open, want 0", groups)
	}
	if paths != 2 {
		t.Errorf("%d glyph path(s), want 2", paths)
	}
}
