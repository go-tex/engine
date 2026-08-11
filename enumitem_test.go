// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// runEI typesets src through a fresh LaTeX engine with the mock font and returns
// the main vertical list, failing the test on any engine error.
func runEI(t *testing.T, src string) *Engine {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run %q: %v", src, err)
	}
	return e
}

// itemLeadGlue returns, for each top-level line box in the main vertical list,
// the summed width of all vertical glue nodes preceding it since the previous
// box, paired with the line's text. Summing captures the item-separation glue
// enumitem inserts before each \item together with the ordinary interline glue.
func itemLeadGlue(nodes []node) []struct {
	glue int
	text string
} {
	var out []struct {
		glue int
		text string
	}
	sum := 0
	for _, n := range nodes {
		switch c := n.(type) {
		case glueNode:
			sum += c.spec.width
		case *boxNode:
			if c.kind != hbox {
				continue
			}
			out = append(out, struct {
				glue int
				text string
			}{sum, mvlText([]node{c})})
			sum = 0
		}
	}
	return out
}

// enumerate [label=\alph*)] numbers items "a)", "b)", … (the counter mapped to
// \@alph, the ")" kept as a literal).
func TestEnumitemLabelAlph(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label=\alph*)]\item one\item two\end{enumerate}`)
	if got, want := mvlText(e.mvl), "a)oneb)two"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// enumerate [label=(\roman*)] numbers items "(i)", "(ii)", … .
func TestEnumitemLabelRoman(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label=(\roman*)]\item a\item b\item c\end{enumerate}`)
	if got, want := mvlText(e.mvl), "(i)a(ii)b(iii)c"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// enumerate [label=\Alph*.] numbers items "A.", "B." (uppercase alpha).
func TestEnumitemLabelAlphUpper(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label=\Alph*.]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "A.aB.b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// enumerate [label=\arabic*)] numbers items "1)", "2)" (arabic mapped to \the).
func TestEnumitemLabelArabic(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label=\arabic*)]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "1)a2)b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// enumerate [start=5] begins numbering at 5 (counter set to n-1).
func TestEnumitemStart(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[start=5]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "5.a6.b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// start combines with a custom label.
func TestEnumitemStartWithLabel(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label=\arabic*),start=10]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "10)a11)b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// itemize [label=--] replaces the bullet with a literal "--" on every item.
func TestEnumitemItemizeLabel(t *testing.T) {
	e := runEI(t, `\begin{itemize}[label=--]\item x\item y\end{itemize}`)
	if got, want := mvlText(e.mvl), "--x--y"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// resume continues the enumerate counter across two separate top-level lists.
func TestEnumitemResume(t *testing.T) {
	src := `\begin{enumerate}\item a\item b\end{enumerate}` +
		`\begin{enumerate}[resume]\item c\item d\end{enumerate}`
	e := runEI(t, src)
	if got, want := mvlText(e.mvl), "1.a2.b3.c4.d"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// Without resume, the second list restarts at 1 (regression guard for the
// resume machinery: recording must not leak into a plain list).
func TestEnumitemNoResume(t *testing.T) {
	src := `\begin{enumerate}\item a\item b\end{enumerate}` +
		`\begin{enumerate}\item c\item d\end{enumerate}`
	e := runEI(t, src)
	if got, want := mvlText(e.mvl), "1.a2.b1.c2.d"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// nosep yields strictly smaller inter-item glue than an explicit itemsep, and
// cancels the list's leading \smallskip. All widths are compared against a plain
// list rendered with the same content, so the fixed interline glue cancels out.
func TestEnumitemSpacing(t *testing.T) {
	const pt = unity

	// Baseline: a plain list. Its leading (smallskip) and inter-item (interline)
	// glue are the reference the option keys move away from.
	gp := itemLeadGlue(runEI(t, `\begin{enumerate}\item a\item b\end{enumerate}`).mvl)
	if len(gp) != 2 {
		t.Fatalf("plain: expected 2 lines, got %+v", gp)
	}

	// itemsep=12pt adds exactly 12pt of glue before each item.
	g1 := itemLeadGlue(runEI(t, `\begin{enumerate}[itemsep=12pt]\item a\item b\end{enumerate}`).mvl)
	if len(g1) != 2 {
		t.Fatalf("itemsep: expected 2 lines, got %+v", g1)
	}
	if got, want := g1[1].glue, gp[1].glue+12*pt; got != want {
		t.Errorf("itemsep=12pt: inter-item glue = %d, want %d (plain %d + 12pt)", got, want, gp[1].glue)
	}

	// nosep adds no inter-item glue and cancels the leading smallskip.
	g2 := itemLeadGlue(runEI(t, `\begin{enumerate}[nosep]\item a\item b\end{enumerate}`).mvl)
	if len(g2) != 2 {
		t.Fatalf("nosep: expected 2 lines, got %+v", g2)
	}
	if g2[1].glue >= g1[1].glue {
		t.Errorf("nosep inter-item glue %d not smaller than itemsep %d", g2[1].glue, g1[1].glue)
	}
	if g2[0].glue != 0 {
		t.Errorf("nosep: leading glue before first item = %d, want 0 (smallskip cancelled)", g2[0].glue)
	}
	if g2[0].glue >= gp[0].glue {
		t.Errorf("nosep leading glue %d not smaller than plain leading %d", g2[0].glue, gp[0].glue)
	}
}

// noitemsep zeroes the inter-item glue (matching a plain list) but leaves the
// leading \smallskip intact (unlike nosep).
func TestEnumitemNoItemsep(t *testing.T) {
	gp := itemLeadGlue(runEI(t, `\begin{enumerate}\item a\item b\end{enumerate}`).mvl)
	g := itemLeadGlue(runEI(t, `\begin{enumerate}[noitemsep]\item a\item b\end{enumerate}`).mvl)
	if len(g) != 2 {
		t.Fatalf("expected 2 lines, got %+v", g)
	}
	// No extra inter-item glue beyond the plain baseline.
	if g[1].glue != gp[1].glue {
		t.Errorf("noitemsep: inter-item glue = %d, want %d (same as plain)", g[1].glue, gp[1].glue)
	}
	// Leading smallskip preserved (unlike nosep, which cancels it).
	if g[0].glue != gp[0].glue {
		t.Errorf("noitemsep: leading glue = %d, want %d (smallskip kept)", g[0].glue, gp[0].glue)
	}
}

// leftmargin sets the list indentation (\leftskip) to the given dimension.
func TestEnumitemLeftmargin(t *testing.T) {
	const pt = unity
	e := runEI(t, `\begin{enumerate}[leftmargin=40pt]\item a\item b\end{enumerate}`)
	lines := lineLeftskips(e.mvl)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}
	for i, l := range lines {
		if l.skip != 40*pt {
			t.Errorf("line %d leftskip = %d, want 40pt", i, l.skip)
		}
	}
}

// An unknown key (widest) is accepted and ignored: numbering is unchanged.
func TestEnumitemUnknownKey(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[widest=99]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "1.a2.b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// Several unknown keys mixed with a known one: the known key applies, the rest
// are ignored without error.
func TestEnumitemUnknownKeysMixed(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[align=left,start=3,font=\bf,widest=9]\item a\end{enumerate}`)
	if got, want := mvlText(e.mvl), "3.a"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// A plain \begin{enumerate} (no bracket) is completely unchanged, including its
// per-item optional-label handling.
func TestEnumitemPlainUnchanged(t *testing.T) {
	e := runEI(t, `\begin{enumerate}\item a\item[X] b\item c\end{enumerate}`)
	if got, want := mvlText(e.mvl), "1.aXb2.c"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// A plain \begin{itemize} keeps its default bullets and its \item[..] override.
func TestEnumitemPlainItemizeUnchanged(t *testing.T) {
	e := runEI(t, `\begin{itemize}\item a\item[!] b\end{itemize}`)
	if got, want := mvlText(e.mvl), "•a!b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// An empty option bracket is a no-op.
func TestEnumitemEmptyBracket(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "1.a2.b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// A custom label still honours a per-item \item[..] override: the override wins
// for that item and, as in standard LaTeX, does not step the counter, so the
// following item is "b)" (the second alphabetic value), not "c)".
func TestEnumitemLabelWithOverride(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label=\alph*)]\item a\item[Z] b\item c\end{enumerate}`)
	if got, want := mvlText(e.mvl), "a)aZbb)c"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// A bracketed nested enumerate applies its options only to its own level; the
// outer level keeps its default numbering and resumes afterwards.
func TestEnumitemNestedScope(t *testing.T) {
	src := `\begin{enumerate}\item a` +
		`\begin{enumerate}[label=\Alph*)]\item x\item y\end{enumerate}` +
		`\item b\end{enumerate}`
	e := runEI(t, src)
	if got, want := mvlText(e.mvl), "1.aA)xB)y2.b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// description accepts leftmargin (and ignores label), still emboldening terms.
func TestEnumitemDescription(t *testing.T) {
	const pt = unity
	e := runEI(t, `\begin{description}[leftmargin=30pt,label=\alph*]\item[Cat] felin\item[Dog] canin\end{description}`)
	if got, want := mvlText(e.mvl), "CatfelinDogcanin"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
	lines := lineLeftskips(e.mvl)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if l.skip != 30*pt {
			t.Errorf("line %d leftskip = %d, want 30pt", i, l.skip)
		}
	}
}

// resume with a start override: start wins over resume when both are present.
func TestEnumitemResumeOverriddenByStart(t *testing.T) {
	src := `\begin{enumerate}\item a\item b\end{enumerate}` +
		`\begin{enumerate}[resume,start=100]\item c\end{enumerate}`
	e := runEI(t, src)
	if got, want := mvlText(e.mvl), "1.a2.b100.c"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// enumerate [label=\Roman*)] numbers items "I)", "II)" (uppercase roman).
func TestEnumitemLabelRomanUpper(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label=\Roman*)]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "I)aII)b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// A braced label value keeps its braces balanced through the option parser (a
// comma or "=" inside the braces would not split); \arabic* still maps.
func TestEnumitemLabelBracedValue(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label={\arabic*}]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "1a2b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// A label key with no value is ignored (numbering stays default).
func TestEnumitemLabelNoValue(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label]\item a\item b\end{enumerate}`)
	if got, want := mvlText(e.mvl), "1.a2.b"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// start and resume are enumerate-only: on itemize they are ignored (bullets kept).
func TestEnumitemStartResumeIgnoredOnItemize(t *testing.T) {
	e := runEI(t, `\begin{itemize}[start=5,resume]\item x\item y\end{itemize}`)
	if got, want := mvlText(e.mvl), "•x•y"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// A fifth nesting level reuses the deepest (iv) counter/format; start applies to
// it (exercising the depth clamp). theenumiv is "\@Alph\c@enumiv.", so start=3
// makes the first innermost item "C.".
func TestEnumitemDeepDepthStart(t *testing.T) {
	src := `\begin{enumerate}\item l1` +
		`\begin{enumerate}\item l2` +
		`\begin{enumerate}\item l3` +
		`\begin{enumerate}\item l4` +
		`\begin{enumerate}[start=3]\item l5\end{enumerate}` +
		`\end{enumerate}\end{enumerate}\end{enumerate}\end{enumerate}`
	e := runEI(t, src)
	if got, want := mvlText(e.mvl), "1.l1(a)l2i.l3A.l4C.l5"; got != want {
		t.Fatalf("typeset %q, want %q", got, want)
	}
}

// White-box tests for the pure helpers, covering branches not reachable (or not
// worth reaching) through full-document input.
func TestEnumitemHelpers(t *testing.T) {
	if got := romanSuffix(0); got != "i" {
		t.Errorf("romanSuffix(0) = %q, want i", got)
	}
	if got := romanSuffix(4); got != "iv" {
		t.Errorf("romanSuffix(4) = %q, want iv", got)
	}
	if got := romanSuffix(9); got != "iv" {
		t.Errorf("romanSuffix(9) = %q, want iv", got)
	}
	e := New()
	if got := e.toksToString(intToks(-3)); got != "-3" {
		t.Errorf("intToks(-3) = %q, want -3", got)
	}
	if got := e.toksToString(intToks(42)); got != "42" {
		t.Errorf("intToks(42) = %q, want 42", got)
	}
	if got := e.counterValue("c@nosuchcounter"); got != 0 {
		t.Errorf("counterValue(missing) = %d, want 0", got)
	}
	// A value containing further "=" is rejoined into the value verbatim.
	toks := stringToToks("k=a=b")
	entries := parseEnumitemOpts(e, toks)
	if len(entries) != 1 || entries[0].key != "k" || e.toksToString(entries[0].val) != "a=b" {
		t.Errorf("parseEnumitemOpts(k=a=b) = %+v", entries)
	}
}

// readEnumitemBracket edge cases: end of input, a non-bracket token (pushed back),
// and an unterminated bracket (consume to end).
func TestEnumitemReadBracket(t *testing.T) {
	// End of input with no tokens: not a bracket.
	e := New()
	e.noBase = true
	if toks, ok := e.readEnumitemBracket(); ok || toks != nil {
		t.Errorf("empty input: got (%v,%v), want (nil,false)", toks, ok)
	}

	// A non-bracket token is left in place.
	e = New()
	e.noBase = true
	e.push([]tok{chTok('x', catLetter)})
	if _, ok := e.readEnumitemBracket(); ok {
		t.Errorf("non-bracket: got ok=true, want false")
	}
	if u, ok := e.getNext(); !ok || u.ch != 'x' {
		t.Errorf("non-bracket token not restored: %+v ok=%v", u, ok)
	}

	// An unterminated bracket (no closing ']') consumes to end of input, and a
	// balanced {…} inside is tracked without ending the option early.
	e = New()
	e.noBase = true
	e.push([]tok{
		chTok('[', catOther),
		chTok('{', catBegin), chTok('a', catLetter), chTok('}', catEnd),
		chTok('b', catLetter),
	})
	toks, ok := e.readEnumitemBracket()
	if !ok {
		t.Fatalf("unterminated bracket: ok=false")
	}
	if got := e.toksToString(toks); got != "{a}b" {
		t.Errorf("unterminated bracket content = %q, want {a}b", got)
	}
}
