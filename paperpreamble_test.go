package engine

import "testing"

// A document asks which engine it is running on before it chooses how to include
// a graphic, which font machinery to load, or which encoding package to use. An
// unanswered question is a hard error, not a wrong branch: \ifpdf…\else…\fi has
// no conditional to match, and the document stops on its sixth line — which is
// exactly where a real arXiv paper stopped.
func TestEngineConditionals(t *testing.T) {
	cases := []struct{ src, want string }{
		// This engine writes PDF directly and reads UTF-8, so it answers the
		// pdfTeX-shaped questions yes: that is the branch whose packages it emulates.
		{`\ifpdf O\else N\fi`, "O"},
		{`\ifPDFTeX O\else N\fi`, "O"},
		{`\ifpdftex O\else N\fi`, "O"},
		{`\ifetex O\else N\fi`, "O"},
		{`\ifeTeX O\else N\fi`, "O"},
		// It is not XeTeX or LuaTeX, and must not be sent down those branches.
		{`\ifxetex O\else N\fi`, "N"},
		{`\ifXeTeX O\else N\fi`, "N"},
		{`\ifluatex O\else N\fi`, "N"},
		{`\ifLuaTeX O\else N\fi`, "N"},
		{`\ifvtex O\else N\fi`, "N"},
		{`\ifptex O\else N\fi`, "N"},
		// They are ordinary switches, so a document may set them itself.

	}
	for _, c := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		out, err := e.Run(`\message{` + c.src + `}`)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := trimNL(out); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// A maths alphabet declaration defines the command it names. A family that names
// a blackboard-bold font gets the blackboard alphabet the maths layer really has,
// so \mathbbm{N} still reads as the set of naturals; any other family typesets
// its argument rather than losing it.
func TestDeclareMathAlphabet(t *testing.T) {
	cases := []struct{ decl, use, want string }{
		{`\DeclareMathAlphabet{\mathbbm}{U}{bbm}{m}{n}`, `\meaning\mathbbm`, `macro:#1->\mathbb {#1}`},
		{`\DeclareMathAlphabet{\mathds}{U}{dsrom}{m}{n}`, `\meaning\mathds`, `macro:#1->\mathbb {#1}`},
		{`\DeclareMathAlphabet{\mathbbold}{U}{bbold}{m}{n}`, `\meaning\mathbbold`, `macro:#1->\mathbb {#1}`},
		{`\DeclareMathAlphabet{\mathpxr}{OT1}{pxr}{m}{n}`, `\mathpxr{X}`, `X`},
	}
	for _, c := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		out, err := e.Run(c.decl + `\message{` + c.use + `}`)
		if err != nil {
			t.Errorf("%s: %v", c.decl, err)
			continue
		}
		if got := trimNL(out); got != c.want {
			t.Errorf("%s → %s = %q, want %q", c.decl, c.use, got, c.want)
		}
	}
}

// The rest of NFSS's declarations, the line-numbering switches a conference class
// turns on and off, and \protected@write are accepted and consume their
// arguments: this engine cannot install what they ask for, but a real paper's
// preamble is full of them and an undefined one stops the document.
func TestPreambleDeclarationsAreAccepted(t *testing.T) {
	for _, src := range []string{
		`\DeclareSymbolFont{letters}{OML}{cmm}{m}{it}`,
		`\SetSymbolFont{letters}{bold}{OML}{cmm}{b}{it}`,
		`\SetMathAlphabet{\mathsf}{bold}{OT1}{cmss}{bx}{n}`,
		`\DeclareSymbolFontAlphabet{\mathnormal}{letters}`,
		`\DeclareFontFamily{U}{bbm}{}`,
		`\DeclareFontShape{U}{bbm}{m}{n}{<->bbm10}{}`,
		`\DeclareFontEncoding{T1}{}{}`,
		`\DeclareFontSubstitution{T1}{cmr}{m}{n}`,
		`\mathversion{bold}`,
		`\DeclareMathVersion{bold}`,
		`\fontfamily{cmr}\fontseries{bx}\fontshape{it}\fontsize{10}{12}`,
		`\usefont{OT1}{cmr}{m}{n}`,
		`\linenumbers `,
		`\nolinenumbers `,
		`\modulolinenumbers{5}`,
		`\protected@write\@auxout{}{\string\newlabel{a}{{1}{1}}}`,
	} {
		e := New()
		e.SetFont(spMock{})
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Run(`\makeatletter` + src + `X`); err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got := glyphString(e.mvl); got != "X" {
			t.Errorf("%s left %q on the page, want X alone", src, got)
		}
	}
}

// \DeclareTextFontCommand really defines what it names, since a document then
// uses it to set text. (\DeclareOldFontCommand is deliberately inert elsewhere in
// the kernel — it is how a class rebinds \rm and friends, which this engine binds
// to real faces itself.)
func TestTextFontCommandDeclarations(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\makeatletter\DeclareTextFontCommand{\textnormalish}{\relax}` +
		`\textnormalish{AB}C`); err != nil {
		t.Fatal(err)
	}
	if got := glyphString(e.mvl); got != "ABC" {
		t.Errorf("typeset %q, want ABC", got)
	}
}
