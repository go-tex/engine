package engine

import "testing"

// theLen runs src (with spMock loaded) and returns \the of the given length cs,
// captured through \message. It fails the test on any engine error.
func theLen(t *testing.T, src, cs string) string {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src + `\message{\the` + cs + `}`); err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return trimNL(e.out.String())
}

// \newlength allocates a rubber length; \setlength assigns it and \the reads it
// back. A rigid value shows only the pt width.
func TestNewSetLength(t *testing.T) {
	if got := theLen(t, `\newlength{\gap}\setlength{\gap}{7pt}`, `\gap`); got != "7.0pt" {
		t.Errorf("setlength 7pt: got %q want %q", got, "7.0pt")
	}
}

// \addtolength advances a length by a (possibly rubber) glue.
func TestAddToLength(t *testing.T) {
	if got := theLen(t, `\newlength{\gap}\setlength{\gap}{7pt}\addtolength{\gap}{3pt}`, `\gap`); got != "10.0pt" {
		t.Errorf("addtolength: got %q want %q", got, "10.0pt")
	}
	// Adding a rubber glue combines widths and accumulates same-order stretch.
	src := `\newlength{\gap}\setlength{\gap}{2pt plus 1pt}\addtolength{\gap}{1pt plus 2pt}`
	if got := theLen(t, src, `\gap`); got != "3.0pt plus 3.0pt" {
		t.Errorf("addtolength rubber: got %q want %q", got, "3.0pt plus 3.0pt")
	}
}

// A rubber length keeps its stretch/shrink through \setlength.
func TestRubberLengthPreserved(t *testing.T) {
	if got := theLen(t, `\newlength{\gap}\setlength{\gap}{2pt plus 3fil}`, `\gap`); got != "2.0pt plus 3.0fil" {
		t.Errorf("rubber setlength: got %q want %q", got, "2.0pt plus 3.0fil")
	}
	if got := theLen(t, `\newlength{\gap}\setlength{\gap}{4pt plus 2pt minus 1pt}`, `\gap`); got != "4.0pt plus 2.0pt minus 1.0pt" {
		t.Errorf("rubber plus/minus: got %q want %q", got, "4.0pt plus 2.0pt minus 1.0pt")
	}
}

// \stretch{n} is the rubber length 0pt plus n fil (order-1 infinite stretch).
func TestStretch(t *testing.T) {
	if got := theLen(t, `\newlength{\gap}\setlength{\gap}{\stretch{2}}`, `\gap`); got != "0.0pt plus 2.0fil" {
		t.Errorf("stretch{2}: got %q want %q", got, "0.0pt plus 2.0fil")
	}
	if got := theLen(t, `\newlength{\gap}\setlength{\gap}{\stretch{1}}`, `\gap`); got != "0.0pt plus 1.0fil" {
		t.Errorf("stretch{1}: got %q want %q", got, "0.0pt plus 1.0fil")
	}
}

// \settowidth/\settoheight/\settodepth set a length to the natural dimension of
// content typeset as an hbox. With spMock, "aaaa" is 4×5pt = 20pt wide, letters
// are 7pt tall and 2pt deep. Assertions are exact scaled-point values.
func TestSetToDimensions(t *testing.T) {
	// width: "aaaa" = 20pt = 20*unity sp
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\newlength{\w}\settowidth{\w}{aaaa}\message{\the\w}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "20.0pt" {
		t.Errorf("settowidth: got %q want %q", got, "20.0pt")
	}
	if got := theLen(t, `\newlength{\h}\settoheight{\h}{a}`, `\h`); got != "7.0pt" {
		t.Errorf("settoheight: got %q want %q", got, "7.0pt")
	}
	if got := theLen(t, `\newlength{\d}\settodepth{\d}{a}`, `\d`); got != "2.0pt" {
		t.Errorf("settodepth: got %q want %q", got, "2.0pt")
	}
}

// The exact sp value of a \settowidth result matches the packed hbox width.
func TestSetToWidthExactSP(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// "ab c" = 5 + 5 + 3(space) + 5 = 18pt.
	if _, err := e.Run(`\newlength{\w}\settowidth{\w}{ab c}\setbox0=\hbox{ab c}`); err != nil {
		t.Fatal(err)
	}
	m := e.eq["w"]
	if m == nil || m.kind != mSkipRef {
		t.Fatalf("\\w is not a length: %+v", m)
	}
	if e.skip[m.code].width != 18*unity {
		t.Errorf("settowidth width = %d sp, want %d", e.skip[m.code].width, 18*unity)
	}
	if e.skip[m.code].width != e.boxDim('w') {
		t.Errorf("settowidth %d != hbox width %d", e.skip[m.code].width, e.boxDim('w'))
	}
}

// \setlength is group-local by default: an assignment inside { … } reverts when
// the group closes.
func TestSetLengthGroupLocal(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\newlength{\g}\setlength{\g}{5pt}{\setlength{\g}{9pt}}\message{\the\g}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "5.0pt" {
		t.Errorf("group-local setlength: got %q want %q (should revert)", got, "5.0pt")
	}
}

// \setlength routes to a \newdimen register (width only, stretch dropped).
func TestSetLengthDimenRegister(t *testing.T) {
	if got := theLen(t, `\newdimen\d\setlength{\d}{6pt plus 4pt}`, `\d`); got != "6.0pt" {
		t.Errorf("setlength dimen: got %q want %q", got, "6.0pt")
	}
}

// \setlength writes through to the engine parameters \hsize and \parindent.
func TestSetLengthEngineParams(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setlength{\hsize}{300pt}\setlength{\parindent}{0pt}`); err != nil {
		t.Fatal(err)
	}
	if e.hsize != 300*unity {
		t.Errorf("hsize = %d, want %d", e.hsize, 300*unity)
	}
	if e.parindent != 0 {
		t.Errorf("parindent = %d, want 0", e.parindent)
	}
	// \addtolength advances an engine parameter too.
	if _, err := e.Run(`\addtolength{\hsize}{50pt}`); err != nil {
		t.Fatal(err)
	}
	if e.hsize != 350*unity {
		t.Errorf("hsize after addtolength = %d, want %d", e.hsize, 350*unity)
	}
}

// \setlength writes through to the group-scoped \leftskip parameter and reverts.
func TestSetLengthLeftskip(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`{\setlength{\leftskip}{12pt plus 2fil}\message{\the\leftskip}}\message{|\the\leftskip}`); err != nil {
		t.Fatal(err)
	}
	// Two \message calls are separated by a space; the second, after the group,
	// shows \leftskip reverted to its default 0pt.
	got := trimNL(e.out.String())
	if got != "12.0pt plus 2.0fil |0.0pt" {
		t.Errorf("leftskip group scope: got %q want %q", got, "12.0pt plus 2.0fil |0.0pt")
	}
}

// Error branches must not panic. An unknown length cs is an error; a malformed
// (empty) value assigns zero; missing target for \settowidth is an error.
func TestLengthErrors(t *testing.T) {
	cases := []struct {
		src     string
		wantErr bool
	}{
		{`\setlength{\nolen}{2pt}`, true},         // \nolen is not a length
		{`\addtolength{\nolen}{2pt}`, true},       // idem for \addtolength
		{`\newlength{\notacs}`, false},            // fine: allocates \notacs
		{`\newlength{\z}\setlength{\z}{}`, false}, // empty value → 0pt, no panic
		{`\settowidth{\nolen}{ab}`, true},         // unknown target for \settowidth
		{`\newlength{\z}\settowidth{\z}{ab}`, false},
	}
	for _, c := range cases {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		_, err := e.Run(c.src) // must never panic
		if (err != nil) != c.wantErr {
			t.Errorf("%q: err=%v, wantErr=%v", c.src, err, c.wantErr)
		}
	}
	// After an empty value, \z reads back as 0pt.
	if got := theLen(t, `\newlength{\z}\setlength{\z}{}`, `\z`); got != "0.0pt" {
		t.Errorf("empty value: got %q want %q", got, "0.0pt")
	}
}

// \newlength with no control sequence at all is an error (no panic).
func TestNewLengthNoCS(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\newlength 5`); err == nil {
		t.Error("newlength without a cs should error")
	}
}

// \setlength/\addtolength route to every engine parameter and register kind.
func TestSetLengthRouting(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\setlength{\vsize}{123pt}` +
		`\setlength{\baselineskip}{15pt}` +
		`\newdimen\dd\setlength{\dd}{4pt}\addtolength{\dd}{3pt}` + // dimen add path
		`{\setlength{\rightskip}{8pt plus 1fil}\message{\the\rightskip}}` // group-scoped rightskip
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if e.vsize != 123*unity {
		t.Errorf("vsize = %d, want %d", e.vsize, 123*unity)
	}
	if e.baselineskip != 15*unity {
		t.Errorf("baselineskip = %d, want %d", e.baselineskip, 15*unity)
	}
	if e.dimen[e.eq["dd"].code] != 7*unity {
		t.Errorf("dimen add = %d, want %d", e.dimen[e.eq["dd"].code], 7*unity)
	}
	if e.rightskip.width != 0 { // reverted after the group closed
		t.Errorf("rightskip not reverted: %d", e.rightskip.width)
	}
}

// A target that resolves to something other than a length (here a macro) is an
// error, not a panic.
func TestSetLengthNotALength(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\def\notalen{x}\setlength{\notalen}{2pt}`); err == nil {
		t.Error("setlength on a non-length cs should error")
	}
}

// Missing control sequence (no braces, no cs) for the length commands is an
// error and never panics.
func TestLengthMissingTarget(t *testing.T) {
	for _, src := range []string{
		`\setlength 5{2pt}`,   // value present, but target is a digit
		`\addtolength 5{2pt}`, // idem
		`\settowidth 5{ab}`,   // settoX with no cs target
		`\setlength{}{2pt}`,   // empty braces: no cs inside
		`\setlength`,          // truncated input: no tokens at all
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err == nil {
			t.Errorf("%q: expected an error", src)
		}
	}
}

// An unbraced value assigns zero (readBraceGlue requires a { … } group).
func TestSetLengthUnbracedValue(t *testing.T) {
	if got := theLen(t, `\newlength{\z}\setlength{\z}2pt`, `\z`); got != "0.0pt" {
		t.Errorf("unbraced value: got %q want %q", got, "0.0pt")
	}
}

// \newlength eventually exhausts the \skip register pool; the overflow is an
// error, not a panic. Allocators start at 10, so 246 allocations fill them.
func TestNewLengthExhaustion(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	var b []byte
	for i := 0; i < 250; i++ {
		b = append(b, []byte(`\newlength{\ln`)...)
		b = append(b, []byte{'a' + byte(i%26), 'a' + byte((i/26)%26)}...)
		b = append(b, '}')
	}
	if _, err := e.Run(string(b)); err == nil {
		t.Error("exhausting the skip pool should error")
	}
}

// The length commands also accept a bare (unbraced) control-sequence target,
// as TeX allows: \setlength\gap{7pt}.
func TestSetLengthUnbracedTarget(t *testing.T) {
	if got := theLen(t, `\newlength{\gap}\setlength\gap{7pt}`, `\gap`); got != "7.0pt" {
		t.Errorf("unbraced target: got %q want %q", got, "7.0pt")
	}
}

// \addtolength on the group-scoped glue parameters accumulates onto their
// current value (exercising the additive \leftskip/\rightskip paths).
func TestAddToLengthGlueParams(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\setlength{\leftskip}{4pt}\addtolength{\leftskip}{6pt}` +
		`\setlength{\rightskip}{5pt}\addtolength{\rightskip}{5pt}` +
		`\message{\the\leftskip|\the\rightskip}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "10.0pt|10.0pt" {
		t.Errorf("addtolength glue params: got %q want %q", got, "10.0pt|10.0pt")
	}
}

// Malformed brace groups (a non-brace token where the closing brace is expected)
// exercise the defensive re-synchronisation branches without panicking.
func TestLengthResyncBranches(t *testing.T) {
	for _, src := range []string{
		`\newlength{\gap}\setlength{\gap\relax}{7pt}`, // junk before target's closing brace
		`\newlength{\gap}\setlength{\gap}{7pt\relax}`, // junk before value's closing brace
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		_, _ = e.Run(src) // may or may not error; must not panic
	}
}

// A length (skip register) used where a rigid <dimen> is expected coerces to its
// natural width component, so \parbox{\len}, \framebox[\len] and \dimenN=\len all
// take the length's width. The same holds for a bare \skip register.
func TestLengthCoercesToDimen(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\newlength{\w}\setlength{\w}{30pt plus 5pt}\dimen0=\w` + // length → dimen (width only)
		`\skip5=7pt\dimen1=\skip5` + // skip register → dimen
		`\message{\the\dimen0|\the\dimen1}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "30.0pt|7.0pt" {
		t.Errorf("glue→dimen coercion: got %q want %q", got, "30.0pt|7.0pt")
	}
}

// A \newlength length drives a \parbox measure: the placed vbox has exactly the
// length's width, and content wider than it wraps.
func TestLengthDrivesParboxWidth(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\newlength{\w}\setlength{\w}{20pt}\noindent\parbox{\w}{aaaa aaaa aaaa}`); err != nil {
		t.Fatal(err)
	}
	pb, ok := firstVboxOfWidth(e.mvl, 20*unity)
	if !ok {
		t.Fatal("no parbox vbox of the length's 20pt width was placed")
	}
	lines := 0
	for _, n := range pb.list {
		if _, ok := n.(*boxNode); ok {
			lines++
		}
	}
	if lines < 2 {
		t.Errorf("parbox at length width wrapped into %d lines, want >=2", lines)
	}
}

// \hskip\stretch{1} contributes order-1 infinite glue (like \hfil), usable
// wherever a glue is scanned.
func TestStretchAsGlue(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\newskip\s\s=\stretch{3}\message{\the\s}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "0.0pt plus 3.0fil" {
		t.Errorf("stretch as glue: got %q want %q", got, "0.0pt plus 3.0fil")
	}
}
