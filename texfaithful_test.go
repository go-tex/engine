package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// The behaviours checked here were each measured against a real TeX (tectonic)
// before being implemented; the expected values are that engine's, not a guess.
// They are the primitives and rules a package written for TeX takes for granted,
// and every one of them was found by running the real pgf/TikZ sources.

// \ifx compares meanings, and a character token has one: its category and its
// character — the same meaning a control sequence \let to that character has. So
// \ifx\next/ is true when \next was \let to a slash. Every one-token-lookahead
// scanner tests what it peeked at this way (LaTeX's \@ifnextchar family,
// pgfkeys' path splitting, which silently mis-parsed every key without it).
func TestIfxCharacterAgainstLetCharacter(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\def\key{/a/b}\def\c{\message{\ifx\p/Y\else N\fi}}\expandafter\futurelet\expandafter\p\expandafter\c\key\relax`, "Y"},
		{`\let\p=/\message{\ifx\p/Y\else N\fi}`, "Y"},
		{`\let\p=x\message{\ifx\p/Y\else N\fi}`, "N"},
		{`\let\p=/\message{\ifx/\p Y\else N\fi}`, "Y"}, // either way round: \p is the slash
		{`\message{\ifx//Y\else N\fi}`, "Y"},           // two characters
		{`\message{\ifx/xY\else N\fi}`, "N"},           //
		{`\def\p{/}\message{\ifx\p/Y\else N\fi}`, "N"}, // a macro is not a character
		{`\message{\ifx\undef/Y\else N\fi}`, "N"},      // undefined is not a character
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// TeX's alphabetic constant: `<character> or `<single-character control
// sequence> is that character's code. Without it a source cannot name the
// characters whose categories it changes — \catcode`\%=14, \lccode`\a=`\A — and
// every such assignment silently addressed character 0 instead.
func TestAlphabeticConstant(t *testing.T) {
	cases := []struct{ src, want string }{
		{"\\message{\\number`a}", "97"},
		{"\\message{\\number`\\a}", "97"},
		{"\\message{\\number`A}", "65"},
		{"\\message{\\number`\\\\}", "92"}, // a backslash, as a control symbol
		{"\\message{\\number`\\ }", "32"},  // a space
		{"\\message{\\number`0}", "48"},
		{"\\count0=`\\A \\message{\\the\\count0}", "65"},
		{"\\message{\\number`\\relax}", "0"}, // not a single character: no constant
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// A category code can be read as well as set, which is how a file that changes
// one puts it back: \edef\saved{\the\catcode`\@} … \catcode`\@=\saved. pgf's own
// files do exactly this, and restoring from an unreadable value left @ as an
// escape character, breaking every \pgf@… name that followed.
func TestCatcodeIsReadable(t *testing.T) {
	got := runExpr(t, "\\catcode`\\@=11 \\edef\\saved{\\the\\catcode`\\@}"+
		"\\catcode`\\@=12 \\message{[\\the\\catcode`\\@]}"+
		"\\catcode`\\@=\\saved \\message{[\\the\\catcode`\\@]}")
	if got != "[12] [11]" {
		t.Errorf("catcode round trip = %q, want [12] [11]", got)
	}
	if got := runExpr(t, "\\message{[\\the\\catcode`\\A][\\the\\catcode`\\ ][\\the\\catcode`\\{]}"); got != "[11][10][1]" {
		t.Errorf("initial catcodes = %q, want [11][10][1]", got)
	}
	// It is an internal integer, so it can be compared and computed with.
	if got := runExpr(t, "\\message{\\ifnum\\catcode`\\A=11 lettre\\else autre\\fi}"); got != "lettre" {
		t.Errorf("\\ifnum over a catcode = %q", got)
	}
}

// \afterassignment saves one token to be inserted once the next assignment has
// been carried out — after it, so the macro sees the value that was just
// assigned. A scanner resumes itself this way (pgf: \afterassignment\resume\let\t=).
func TestAfterassignment(t *testing.T) {
	got := runExpr(t, `\def\after{\message{[apres:\the\count0]}}`+
		`\afterassignment\after\count0=42 \message{[fin]}`)
	if got != "[apres:42] [fin]" {
		t.Errorf("= %q, want [apres:42] [fin]", got)
	}
	// It waits for an assignment, not for the next token.
	got = runExpr(t, `\def\after{\message{[apres]}}\afterassignment\after\relax\count0=1 \message{[fin]}`)
	if got != "[apres] [fin]" {
		t.Errorf("= %q, want the token after the assignment", got)
	}
	// Only one token is held: a second \afterassignment replaces the first.
	got = runExpr(t, `\def\a{\message{[a]}}\def\b{\message{[b]}}`+
		`\afterassignment\a\afterassignment\b\count0=1 \message{[fin]}`)
	if got != "[b] [fin]" {
		t.Errorf("= %q, want only the last token", got)
	}
	// A \def is an assignment too.
	got = runExpr(t, `\def\after{\message{[apres]}}\afterassignment\after\def\x{}\message{[fin]}`)
	if got != "[apres] [fin]" {
		t.Errorf("= %q", got)
	}
	if _, err := New().Run(`\afterassignment`); err != nil { // truncated input
		t.Fatal(err)
	}
}

// An internal dimension coerces to an integer — its value in scaled points —
// wherever a number is wanted. A package that computes with lengths relies on it
// on every value (pgf converts each coordinate with \number\pgf@x).
func TestDimenCoercesToInteger(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\newdimen\Z\Z=1pt\message{\number\Z}`, "65536"},
		{`\newdimen\Z\Z=1pt\message{\the\numexpr\Z*2\relax}`, "131072"},
		{`\hsize=1pt\message{\number\hsize}`, "65536"},
		{`\newdimen\Z\Z=2pt\count0=\Z\message{\the\count0}`, "131072"},
		{`\newdimen\Z\Z=1pt\message{\ifnum\Z>0 positif\else nul\fi}`, "positif"},
		{`\newskip\S\S=1pt plus 2pt\message{\number\S}`, "65536"}, // glue → its width
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// A box register allocated by \newbox is a register *number* (TeX allocates it
// with \chardef), so every box primitive reads the handle as the integer it
// stands for. Without it a package's own boxes were all box 0, and the material
// put in them vanished.
func TestNewboxHandleIsARegisterNumber(t *testing.T) {
	e := New()
	if _, err := e.Run(`\newbox\mybox\setbox\mybox=\hbox{\kern5pt}` +
		`\setbox1=\hbox{\copy\mybox\kern1pt}\message{[\the\wd\mybox][\the\wd1]}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[5.0pt][6.0pt]" {
		t.Errorf("= %q, want [5.0pt][6.0pt]", got)
	}
	// \box empties the register, as it does for a numbered one.
	got := runExpr(t, `\newbox\b\setbox\b=\hbox{\kern5pt}\setbox1=\hbox{\box\b}`+
		`\message{[\the\wd1][\the\wd\b]}`)
	if got != "[5.0pt][0.0pt]" {
		t.Errorf("= %q, want [5.0pt][0.0pt]", got)
	}
}

// A global assignment must outlive every group that is open, so a local
// assignment made earlier in one of them is no longer restored over it at the
// closing brace. The idiom that carries a computed result out of a group — pgf's
// \pgf@process is exactly {…\global\pgf@x=\pgf@x} — depends on it entirely.
func TestGlobalSurvivesAnEarlierLocalAssignment(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\newdimen\Z{\Z=10pt\global\Z=\Z}\message{\the\Z}`, "10.0pt"},
		{`\newdimen\Z{\Z=10pt}\message{\the\Z}`, "0.0pt"}, // still local without \global
		{`\newcount\N{\N=1 \global\N=7 }\message{\the\N}`, "7"},
		{`\newskip\S{\S=1pt\global\S=3pt}\message{\the\S}`, "3.0pt"},
		{`{\def\x{a}\gdef\x{b}}\message{\x}`, "b"},
		{`\def\x{a}{\def\x{b}}\message{\x}`, "a"},                        // an ordinary group still restores
		{`\newdimen\Z{{\Z=10pt\global\Z=\Z}}\message{\the\Z}`, "10.0pt"}, // through two groups
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// A file name is expanded before it is looked up, since a package names the file
// to load through a macro (pgf loads its driver as
// \pgfutil@InputIfFileExists{\pgfsysdriver}).
func TestInputIfFileExistsExpandsTheName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "named.tex"), []byte(`\message{[charge]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\def\thefile{named.tex}` +
		`\InputIfFileExists{\thefile}{\message{[trouve]}}{\message{[absent]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[trouve] [charge]" {
		t.Errorf("= %q, want the file found through the macro and loaded", got)
	}
	// A name that resolves to nothing still takes the else branch.
	e2 := New()
	e2.LoadLaTeX()
	out2, _ := e2.Run(`\def\thefile{nulle-part.tex}` +
		`\InputIfFileExists{\thefile}{\message{[trouve]}}{\message{[absent]}}`)
	if got := trimNL(out2); got != "[absent]" {
		t.Errorf("= %q, want [absent]", got)
	}
}

// The engine's named colours are published in the form a colour-reading package
// expects, so a drawing package can ask which model and values a colour has
// instead of reporting the model as unsupported.
func TestColorIsPublishedForReaders(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\makeatletter\definecolor{mine}{RGB}{255,128,0}` +
		`\def\read#1#2#3#4#5{\message{[modele=#4][valeurs=#5]}}` +
		`\expandafter\expandafter\expandafter\read\csname\string\color@mine\endcsname`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[modele=rgb][valeurs=1,0.50196,0]" {
		t.Errorf("= %q, want the model and its values", got)
	}
	// \colorlet publishes its result too, which is how a package names a colour
	// before reading it back.
	e2 := New()
	e2.LoadLaTeX()
	out2, _ := e2.Run(`\makeatletter\colorlet{copie}{red}` +
		`\def\read#1#2#3#4#5{\message{[#4:#5]}}` +
		`\expandafter\expandafter\expandafter\read\csname\string\color@copie\endcsname`)
	if got := trimNL(out2); got != "[rgb:1,0,0]" {
		t.Errorf("\\colorlet = %q, want [rgb:1,0,0]", got)
	}
}

// colorComponent spells a component the way a colour specification does.
func TestColorComponent(t *testing.T) {
	for _, c := range []struct {
		in   float64
		want string
	}{{0, "0"}, {1, "1"}, {0.5, "0.5"}, {128.0 / 255, "0.50196"}, {1.0 / 3, "0.33333"}} {
		if got := colorComponent(c.in); got != c.want {
			t.Errorf("colorComponent(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A malformed use of each of these is read as far as it makes sense rather than
// derailing the run.
func TestFaithfulPrimitivesOnMalformedInput(t *testing.T) {
	for _, src := range []string{
		"\\count0=`",                         // the backtick is the very last thing in the input
		"\\message{\\number`}",               // ` at end of input
		"\\definecolor{}{rgb}{1,0,0}",        // a colour with no name
		"\\InputIfFileExists x{}{}",          // no braced name
		"\\InputIfFileExists",                // nothing at all
		"\\InputIfFileExists{\\undefme}{}{}", // a name that expands to nothing useful
	} {
		if _, err := New().Run(src); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
}

// A colour name with no colour behind it publishes nothing, and an unreadable
// specification still yields a readable (black) value rather than nonsense.
func TestPublishColorEdges(t *testing.T) {
	e := New()
	e.publishColor("", 0xffffff) // ignored: no name
	if e.eq[`\color@`] != nil {
		t.Error("a colour with no name was published")
	}
	e.publishColor("noir", 0)
	m := e.eq[`\color@noir`]
	if m == nil {
		t.Fatal("colour not published")
	}
	if got := e.toksToString(m.body); got != `\xcolor@{}{}{rgb}{0,0,0}` {
		t.Errorf("black = %q", got)
	}
}

// scanCharCode covers each way the character after a ` can end: a character, a
// single-character control sequence, a multi-letter one (which is not a
// character constant), and nothing at all.
func TestScanCharCodeBranches(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"\\message{\\number`a}", "97"},
		{"\\message{\\number`\\a}", "97"},
		{"\\message{\\number`\\relax}", "0"},
		{"\\message{[\\number`}", "[0"}, // input ends right after the backtick
	} {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}
