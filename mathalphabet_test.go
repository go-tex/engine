// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// firstMath and mathGeom live in mathleftindex_test.go: both were added there first,
// and duplicating them here is what broke the build after #470 merged — the branch
// predated it, so the redeclaration only appeared in CI.

// mathSrc compiles one formula and returns the SOURCE handed to go-tex/math, which is
// where the face was being lost — the rendered box barely moves, since an identity
// wrapper keeps the glyph count.
func mathSrc(t *testing.T, preamble, body string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}`+preamble+
		`\begin{document}`+body+`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	m, ok := firstMath(e.mvl)
	if !ok {
		if m, ok = firstMath(e.parList); !ok {
			t.Fatalf("%s: no math box on the page", body)
		}
	}
	return m.src
}

// A \math… alphabet inside a document macro must reach the maths layer. The engine
// defines these as identity wrappers for TEXT mode (classkernel.go:427-439,
// \def\mathtt#1{#1}), which is right there and wrong in a formula: flattenMathBody
// expanded them, so
//
//	\newcommand{\code}[1]{\mathtt{#1}}   then   $\code{A}$
//
// handed the maths layer a bare "A". Written directly, $\mathtt{A}$ was always fine,
// which is what made this invisible: the defect needed a macro.
//
// Nothing in the campaign's instruments could see it. The formula renders, in the
// wrong face, with the same glyph count — so neither the dropped-equation census nor
// the page barometer nor the ink ratio moves. The source string is the witness.
func TestMathAlphabetSurvivesAMacroBody(t *testing.T) {
	for _, name := range []string{"mathrm", "mathbf", "mathit", "mathsf",
		"mathtt", "mathcal", "mathfrak", "mathscr"} {
		pre := `\newcommand{\q}[1]{\` + name + `{#1}}`
		if got := mathSrc(t, pre, `$\q{A}$`); !strings.Contains(got, `\`+name) {
			t.Errorf(`\%s through a macro: source %q does not carry it`, name, got)
		}
		// And the same alphabet not at the head of the body, where a macro wraps it
		// around other material.
		pre2 := `\newcommand{\q}[1]{X\` + name + `{#1}}`
		if got := mathSrc(t, pre2, `$\q{A}$`); !strings.Contains(got, `\`+name) {
			t.Errorf(`\%s mid-body: source %q does not carry it`, name, got)
		}
	}
}

// mathAlphabet is a LIST, so it can drift from what go-tex/math actually renders. This
// checks every entry against the maths layer itself: \name{A} must not render like a
// bare A. If go-tex/math stops honouring one, the entry becomes a face silently
// dropped at a DIFFERENT point, and this fails instead of the defect returning.
//
// It is the guard that makes hard-coding the list defensible.
func TestMathAlphabetsAreAllHonouredByTheMathLayer(t *testing.T) {
	// Each entry gets a subject where its effect is OBSERVABLE, and every entry must
	// have one: a name whose face changes nothing cannot be shown to be honoured.
	//
	// \mathit needs the nested form, and finding that out is what this table is for.
	// Maths sets letters, digits and words in italic ALREADY, so \mathit{A} is
	// byte-identical to A and a flat subject proves nothing. Inside another alphabet it
	// switches back, which is observable: \mathbf{a\mathit{b}} is 655360 wide against
	// \mathbf{ab}'s 720896.
	subjects := map[string][2]string{
		"mathit": {`$\mathbf{a\mathit{b}}$`, `$\mathbf{ab}$`},
	}
	for name := range mathAlphabet {
		pair, ok := subjects[name]
		if !ok {
			pair = [2]string{`$\` + name + `{A}$`, `$A$`}
		}
		got, plain := mathGeom(t, pair[0]), mathGeom(t, pair[1])
		if got.width == plain.width && got.height == plain.height && got.depth == plain.depth {
			t.Errorf(`\%s: %s renders exactly like %s (%d/%d/%d) — go-tex/math no longer `+
				`distinguishes it, so keeping it in mathAlphabet hides a dropped face`,
				name, pair[0], pair[1], got.width, got.height, got.depth)
		}
	}
}

// The names deliberately left OUT, each for its own reason, asserted so a later edit
// does not add them by symmetry.
func TestTheAlphabetsLeftOutOfTheListStayOut(t *testing.T) {
	// \mathds is an ALIAS: classkernel.go defines \def\mathds#1{\mathbb{#1}} because
	// dsfont's \DeclareMathAlphabet cannot run here. Expanding it is the point.
	if mathAlphabet["mathds"] {
		t.Error(`\mathds must keep expanding — it is an alias to \mathbb, not an identity`)
	}
	if got := mathSrc(t, `\newcommand{\q}[1]{\mathds{#1}}`, `$\q{A}$`); !strings.Contains(got, `\mathbb`) {
		t.Errorf(`\mathds through a macro: source %q, want it expanded to \mathbb`, got)
	}
	// \mathnormal has no mapping in go-tex/math, so the identity is the right fallback:
	// keeping the name would turn a rendering formula into an unknown command.
	if mathAlphabet["mathnormal"] {
		t.Error(`\mathnormal must keep expanding — go-tex/math has no mapping for it`)
	}
}

// TeX's maths spacing commands lose the same way, for the same reason. format.go:47-56
// defines them as ordinary horizontal space — \let\,\thinspace,
// \def\;{\hskip.27778em}, \def\quad{\hskip1em} — under a comment that says so: "what a
// real LaTeX produces in text". In a formula the maths layer measures them in MATH
// UNITS and renders them itself, so flattening a macro body turned a\;b into "a b",
// the \hskip stripped and a plain space left behind. \quad and \qquad vanished.
func TestMathSpacingSurvivesAMacroBody(t *testing.T) {
	for _, n := range []string{`\,`, `\;`, `\:`, `\!`, `\quad`, `\qquad`} {
		pre := `\newcommand{\w}[1]{a` + n + `#1}`
		got := mathSrc(t, pre, `$\w{b}$`)
		if !strings.Contains(got, n) {
			t.Errorf(`%s through a macro: source %q does not carry it`, n, got)
		}
	}
}

// \! is the one that was not merely lost. It is a NEGATIVE thin space, −3mu, and it
// came out as a positive space: the sign flipped, so a formula written to pull two
// atoms together pushed them apart.
//
// Asserted as an ORDER, not a width: a\!b must be narrower than ab, which is narrower
// than a\,b. A width would pin today's font; the order is what −3mu, 0 and +3mu mean.
func TestNegativeThinSpaceKeepsItsSignThroughAMacro(t *testing.T) {
	w := func(pre, body string) int {
		return mathGeom2(t, pre, body).width
	}
	const negMacro = `\newcommand{\w}[1]{a\!#1}`
	const posMacro = `\newcommand{\w}[1]{a\,#1}`
	const noMacro = `\newcommand{\w}[1]{a#1}`
	neg, none, pos := w(negMacro, `$\w{b}$`), w(noMacro, `$\w{b}$`), w(posMacro, `$\w{b}$`)
	if !(neg < none && none < pos) {
		t.Errorf(`widths a\!b=%d, ab=%d, a\,b=%d — want strictly increasing: `+
			`-3mu, 0, +3mu`, neg, none, pos)
	}
}

// mathGeom2 is mathGeom with a preamble, for the spacing tests.
func mathGeom2(t *testing.T, preamble, body string) mathNode {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}`+preamble+
		`\begin{document}`+body+`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	if len(e.mathDropped) != 0 {
		t.Fatalf("%s: the math layer refused it: %v", body, e.mathDropped)
	}
	m, ok := firstMath(e.mvl)
	if !ok {
		if m, ok = firstMath(e.parList); !ok {
			t.Fatalf("%s: no math box on the page", body)
		}
	}
	return m
}

// Every entry of mathSpace must be honoured by the maths layer, the same drift guard
// mathAlphabet gets: a\Xb must differ from ab, in the direction the unit says.
func TestMathSpacingIsAllHonouredByTheMathLayer(t *testing.T) {
	plain := mathGeom(t, `$a b$`).width
	for n := range mathSpace {
		// A space after the name, required for \quad and \qquad — without it the test
		// asked for \qquadb and measured an unknown command instead of a space. The
		// control has the same space so the comparison is of the SPACING command alone.
		got := mathGeom(t, `$a\`+n+` b$`).width
		if n == "!" {
			if got >= plain {
				t.Errorf(`$a\! b$ is %d wide, not less than $a b$'s %d — a negative space `+
					`must NARROW`, got, plain)
			}
			continue
		}
		if got <= plain {
			t.Errorf(`$a\%s b$ is %d wide, not more than $a b$'s %d`, n, got, plain)
		}
	}
}
