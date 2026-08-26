package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each behaviour below was found by running the real pgf/TikZ sources and, where
// TeX's own answer was in question, measured against a real TeX (tectonic).

// runPlain runs a source on an engine with the Plain format loaded (where TeX's
// constants such as \active live) and returns what it printed.
func runPlain(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadPlain(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return trimNL(out)
}

// \active is plain TeX's name for category 13. A package makes a character
// active by name — \catcode`\;=\active — and asks whether one already is. When
// the name is missing that whole block fails, and everything defined after it in
// the file is lost with it (tikz lost its scope environment and every command it
// installs, so \draw was never defined).
func TestActiveConstant(t *testing.T) {
	if got := runPlain(t, `\message{[\number\active]}`); got != "[13]" {
		t.Errorf("\\active = %q, want [13]", got)
	}
	// The tikz idiom: define a macro delimited by an active character.
	got := runPlain(t, "{\\catcode`\\;=\\active \\gdef\\coll#1;{[#1]}}\\message{\\meaning\\coll}")
	if got != `macro:#1;->[#1]` {
		t.Errorf("delimited by an active character = %q", got)
	}
	// And the test that asks whether a character is active.
	got = runPlain(t, "{\\catcode`\\;=\\active \\message{\\ifnum\\catcode`\\;=\\active oui\\else non\\fi}}")
	if got != "oui" {
		t.Errorf("\\ifnum over \\active = %q, want oui", got)
	}
}

// A file name is expanded as it is read, which is what lets a package build the
// name of the file it wants: pgf loads each of its modules with
// \input{pgfmodule\pgf@temp.code.tex}, and a literal reading finds no file at
// all — the module then silently never loads.
func TestInputExpandsItsFileName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "partie-deux.tex"), []byte(`\message{[deux]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	got := runExpr(t, `\def\n{deux}\input{partie-\n.tex}`)
	if got != "[deux]" {
		t.Errorf("= %q, want the file named through the macro", got)
	}
	// The unbraced form expands too.
	got = runExpr(t, `\def\n{deux}\input partie-\n.tex `)
	if got != "[deux]" {
		t.Errorf("unbraced = %q", got)
	}
}

// The lines of an \input file are discounted from the document's own numbering,
// exactly as a class or package's are: otherwise an error or an editor's source
// map points hundreds of lines past the real place.
func TestInputDoesNotShiftSourceLines(t *testing.T) {
	dir := t.TempDir()
	body := strings.Repeat("% une ligne de commentaire\n", 40) + `\def\rien{}`
	if err := os.WriteFile(filepath.Join(dir, "gros.tex"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run("\\input{gros.tex}\nTrouve"); err != nil {
		t.Fatal(err)
	}
	line := 0
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch v := n.(type) {
			case charNode:
				if v.ch == 'T' && line == 0 {
					line = v.srcLine
				}
			case *boxNode:
				walk(v.list)
			}
		}
	}
	walk(e.mvl)
	if line != 2 {
		t.Errorf("glyph after a 41-line \\input reports line %d, want 2", line)
	}
}

// A name \csname brings into existence means \relax, and that definition is
// LOCAL. A package asks whether a control sequence exists by expanding \csname
// inside a group it then closes, so the question leaves no trace; defining it
// globally makes every such test answer "yes" from then on. pgf asks that way
// whether it is running under LuaTeX, and a stale answer made it emit its Lua
// branch — hundreds of lines of Lua onto the page.
func TestCsnameDefinitionIsLocal(t *testing.T) {
	got := runExpr(t, `\begingroup\expandafter\expandafter\expandafter\endgroup`+
		`\expandafter\ifx\csname pasdefini\endcsname\relax\message{[absent]}\else\message{[present]}\fi`+
		`\message{[\ifdefined\pasdefini encore\else parti\fi]}`)
	if got != "[absent] [parti]" {
		t.Errorf("= %q, want the name gone after the group", got)
	}
	// Outside a group it does persist, as TeX's does.
	got = runExpr(t, `\expandafter\relax\csname horsgroupe\endcsname`+
		`\message{[\ifdefined\horsgroupe reste\else parti\fi]}`)
	if got != "[reste]" {
		t.Errorf("= %q, want the name to persist outside a group", got)
	}
}

// \lccode/\uccode are real tables, and \lowercase/\uppercase map through them.
// A package sets an entry to make \lowercase substitute one character for
// another — pgfmath builds its parser's catcode block exactly that way.
func TestCaseCodeTables(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	// The pgfmath idiom: ~ lowercases to ", so \lowercase rewrites the body.
	out, err := e.Run("{\\lccode`\\~=`\\\" \\lowercase{\\message{[~]}}}")
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != `["]` {
		t.Errorf("\\lowercase through \\lccode = %q, want [\"]", got)
	}
	// Letters keep their ordinary case behaviour when the table says nothing.
	if got := runExpr(t, `\uppercase{\message{abC}}\lowercase{\message{DeF}}`); got != "ABC def" {
		t.Errorf("ordinary case mapping = %q", got)
	}
	// The tables are readable, and an entry of 0 means "leave it alone".
	if got := runExpr(t, "\\message{[\\the\\lccode`\\A][\\the\\uccode`\\a]}"); got != "[97][65]" {
		t.Errorf("reading the tables = %q, want [97][65]", got)
	}
	if got := runExpr(t, "{\\uccode`\\x=0 \\uppercase{\\message{x}}}"); got != "x" {
		t.Errorf("a zero entry must leave the character alone, got %q", got)
	}
}

// \colorlet takes xcolor's optional colour-model argument. This engine keeps
// colours as RGB so the hint changes nothing, but it must be consumed: pgf calls
// \colorlet[named]{…}{…} whenever the source colour is a named one, and the
// unread [named] lands on the page.
func TestColorletOptionalModel(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\colorlet[named]{copie}{red}X`); err != nil {
		t.Fatal(err)
	}
	if got := glyphString(e.mvl); got != "X" {
		t.Errorf("the optional argument leaked: %q", got)
	}
	if e.colors["copie"] != 0xFF0000 {
		t.Errorf("colour not defined through the optional form: %06x", e.colors["copie"])
	}
}

// The standard colour names are published in the readable form too, not just
// what \definecolor added: a drawing package treats an unrecognised option as a
// colour name and asks whether it is one (\draw[red] works only because
// \color@red exists).
func TestNamedColorsArePublished(t *testing.T) {
	e := New()
	for _, name := range []string{"red", "blue", "black", "orange"} {
		if e.eq[`\color@`+name] == nil {
			t.Errorf("the built-in colour %s is not readable", name)
		}
	}
	if got := e.toksToString(e.eq[`\color@red`].body); got != `\xcolor@{}{}{rgb}{1,0,0}` {
		t.Errorf("red = %q", got)
	}
}

// \setlength on a register reference — a class sets an insertion's skip as
// \setlength{\skip\footins}{9pt plus 4pt minus 2pt} — assigns that register, and
// in any case never leaves its value on the page.
func TestSetlengthOnARegisterReference(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\setlength{\skip4}{9pt plus 4pt minus 2pt}X\message{[\the\skip4]}`); err != nil {
		t.Fatal(err)
	}
	if got := glyphString(e.mvl); got != "X" {
		t.Errorf("the value leaked onto the page: %q", got)
	}
	if got := trimNL(e.out.String()); got != "[9.0pt plus 4.0pt minus 2.0pt]" {
		t.Errorf("register not assigned: %q", got)
	}
	// A target the engine has no register for is reported in strict mode, and in
	// lenient mode dropped whole — the value never reaches the page either way.
	e2 := New()
	e2.SetFont(spMock{})
	e2.LoadLaTeX()
	if _, err := e2.Run(`\setlength{\muskip\quelquechose}{3pt}Y`); err == nil {
		t.Error("an unknown length target must be reported in strict mode")
	}
	e3, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\setlength{\muskip\quelquechose}{3pt}Y\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e3); strings.ContainsAny(got, "0123456789") {
		t.Errorf("an unknown target leaked its value: %q", got)
	}
}

// \@settopoint rounds a length down to whole points; the size option files apply
// it to the lengths they compute, so without it a class drops those calls.
func TestSettopoint(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\makeatletter\newdimen\d\d=10.7pt\@settopoint\d\message{[\the\d]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[10.0pt]" {
		t.Errorf("= %q, want [10.0pt]", got)
	}
}

// registerHandle only recognises the register primitives it can assign to, and
// only a register number that exists.
func TestSetlengthRegisterTargetEdges(t *testing.T) {
	for _, c := range []struct {
		src  string
		want string // what \the of the target should read, "" if nothing is assigned
	}{
		{`\setlength{\dimen7}{3pt}\message{[\the\dimen7]}`, "[3.0pt]"},
		{`\setlength{\skip7}{3pt}\message{[\the\skip7]}`, "[3.0pt]"},
		{`\setlength{\count7}{3pt}\message{[\the\count7]}`, "[0]"},       // not a length register
		{`\setlength{\dimen999}{3pt}\message{[\the\dimen7]}`, "[0.0pt]"}, // out of range
		{`\setlength{\relax x}{3pt}\message{[\the\dimen7]}`, "[0.0pt]"},  // not a register at all
	} {
		e, err := compile([]byte(`\documentclass{article}\begin{document}`+c.src+`\end{document}`),
			Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got := trimNL(e.out.String()); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// Arithmetic works on the dimension parameters the engine models as primitives
// of its own, not only on registers: a class computes its page geometry that way
// (LaTeX's size option files round every length with \divide…\multiply), and a
// parameter that ignored the operation ended up holding nonsense — a text width
// of one point, which truncated every paragraph on the page.
func TestArithmeticOnEngineParameters(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\hsize=100pt\advance\hsize by 5pt \message{\the\hsize}`, "105.0pt"},
		{`\hsize=100pt\multiply\hsize by 3 \message{\the\hsize}`, "300.0pt"},
		{`\hsize=300pt\divide\hsize by 4 \message{\the\hsize}`, "75.0pt"},
		{`\hsize=300pt\divide\hsize by 0 \message{\the\hsize}`, "300.0pt"}, // no division by zero
		{`\vsize=100pt\advance\vsize by-10pt \message{\the\vsize}`, "90.0pt"},
		{`\parindent=10pt\multiply\parindent by 2 \message{\the\parindent}`, "20.0pt"},
		{`\baselineskip=12pt\advance\baselineskip by 2pt \message{\the\baselineskip}`, "14.0pt"},
		// The rounding idiom the size files use: down to a whole point.
		{`\hsize=345.7pt\divide\hsize\p@\multiply\hsize\p@\message{\the\hsize}`, "345.0pt"},
		// A register still behaves as before.
		{`\newdimen\d\d=345.7pt\divide\d\p@\multiply\d\p@\message{\the\d}`, "345.0pt"},
	}
	for _, c := range cases {
		e := New()
		if err := e.LoadPlain(); err != nil {
			t.Fatal(err)
		}
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		out, err := e.Run(`\makeatletter` + c.src)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := trimNL(out); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// The class's size option file really loads now (its name is built from a macro),
// so the page geometry a document gets is the one the class computed.
func TestClassLoadsItsSizeFile(t *testing.T) {
	e := New()
	if err := e.LoadPlain(); err != nil {
		t.Fatal(err)
	}
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\documentclass{article}\message{[\the\hsize][\the\paperwidth]}`)
	if err != nil {
		t.Fatal(err)
	}
	// 345pt is article's text width for a 10pt document on letter paper, rounded
	// to a whole point by the size file; the paper is 8.5in.
	if got := trimNL(out); got != "[345.0pt][614.295pt]" {
		t.Errorf("page geometry = %q, want [345.0pt][614.295pt]", got)
	}
}

// The parameter-arithmetic path only claims the parameters it can act on, and
// leaves anything else to the register paths.
func TestEngineParameterMatching(t *testing.T) {
	e := New()
	if err := e.LoadPlain(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hsize", "vsize", "parindent", "baselineskip"} {
		if _, _, ok := e.engineDimenParam(csTok(name), false); !ok {
			t.Errorf("\\%s should be an engine dimension parameter", name)
		}
	}
	for _, name := range []string{"relax", "count", "undefinedname", "leftskip"} {
		if _, _, ok := e.engineDimenParam(csTok(name), false); ok {
			t.Errorf("\\%s should not be one", name)
		}
	}
	if _, _, ok := e.engineDimenParam(chTok('x', catLetter), false); ok {
		t.Error("a character is not a parameter")
	}
}

// A file name that reads as nothing, and an unbraced name at the end of the
// input, leave the input where the caller expects it.
func TestScanFileNameEdges(t *testing.T) {
	for _, src := range []string{
		`\input`,             // nothing at all
		`\input\relax`,       // a control sequence, not a name
		`\input{}`,           // an empty name
		`\input{\relax}`,     // a name that is only a control sequence
		`\input{nulle-part`,  // an unterminated braced name
		`\input nulle\relax`, // unbraced, ended by a control sequence
		`\input nulle`,       // unbraced, ended by the end of the input
	} {
		e := New()
		e.LoadPlain()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err == nil {
			continue // reported or skipped: either is fine, it must not hang
		}
	}
}

// \colorlet without the optional argument still works, and an empty target name
// is ignored.
func TestColorletPlainForm(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\colorlet{copie}{blue}\colorlet{}{red}`); err != nil {
		t.Fatal(err)
	}
	if e.colors["copie"] != 0x0000FF {
		t.Errorf("plain \\colorlet = %06x", e.colors["copie"])
	}
}
