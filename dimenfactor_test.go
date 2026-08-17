package engine

import "testing"

// TeX's <dimen> is <factor><unit of measure>, and the factor may be an internal
// integer — a count register, a \chardef'd constant — with the unit following
// it. \dimen0=\count0\dimen1 is count0 times dimen1, and
// \setlength\textheight{\@tempcnta\baselineskip} is how a class states a height
// in lines. The engine read the factor as nothing, so every such dimension came
// out zero.
//
// That was not merely a wrong number. pgfmath's long division subtracts
// \c@pgfmath@counta\pgfmath@y from its running remainder each round; with the
// term always zero the remainder never shrank and the division never ended,
// appending digits to \pgfmathresult until it held over a hundred thousand
// tokens. A TikZ node with a drawn circular border took 45 seconds; it now takes
// under half a second.
//
// The expected values below are a real TeX's (tectonic).
func TestDimenFactorFromAnInteger(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\newcount\N\N=5 \newdimen\Y\Y=3pt\newdimen\Z\Z=\N\Y\message{\the\Z}`, "15.0pt"},
		{`\count0=5 \newdimen\Y\Y=3pt\newdimen\Z\Z=\count0\Y\message{\the\Z}`, "15.0pt"},
		{`\chardef\C=5 \newdimen\Y\Y=3pt\newdimen\Z\Z=\C\Y\message{\the\Z}`, "15.0pt"},
		{`\newcount\N\N=-2 \newdimen\Y\Y=3pt\newdimen\Z\Z=\N\Y\message{\the\Z}`, "-6.0pt"},
		{`\newcount\N\N=0 \newdimen\Y\Y=3pt\newdimen\Z\Z=\N\Y\message{\the\Z}`, "0.0pt"},
		// The engine's own dimension parameters serve as the unit too.
		{`\newcount\N\N=3 \baselineskip=4pt\newdimen\Z\Z=\N\baselineskip\message{\the\Z}`, "12.0pt"},
		// A literal factor still works, and so does a plain dimension.
		{`\newdimen\Y\Y=3pt\newdimen\Z\Z=5\Y\message{\the\Z}`, "15.0pt"},
		{`\newdimen\Y\Y=3pt\newdimen\Z\Z=\Y\message{\the\Z}`, "3.0pt"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}

// An integer that is NOT followed by a unit is left exactly where it was: the
// factor reading is only taken when a dimension really follows, so every other
// reading of an integer behaves as before. (A class computes with the same
// scratch counters a few lines apart, and consuming one of them eagerly left an
// assignment's '=' to be typeset.)
func TestIntegerWithoutAUnitIsUntouched(t *testing.T) {
	// \count0=\count1 is an integer assignment, not a dimension.
	if got := runExpr(t, `\count1=7 \count0=\count1 \message{\the\count0}`); got != "7" {
		t.Errorf("integer assignment = %q, want 7", got)
	}
	// A count register read where a dimension is wanted, with no unit after it,
	// leaves the input alone rather than eating what follows.
	// The unit must be a dimension for the factor to be read. TeX also accepts a
	// spelled-out unit here (\dimen0=\count0 pt is 5.0pt in a real TeX, checked),
	// which this engine does not yet: taking it disturbs a class file's own parse
	// in a way not yet understood. The reading is simply not taken, and nothing is
	// consumed — which is what keeps every other use of an integer intact.
	if got := runExpr(t, `\newcount\N\N=5 \newdimen\Z\Z=\N pt\message{\the\Z}`); got != "0.0pt" {
		t.Errorf("integer with a spelled-out unit = %q, want 0.0pt for now", got)
	}
	// The same through \setlength, which a class uses constantly.
	got := runExpr(t, `\newcount\N\N=3 \baselineskip=4pt\newlength\L`+
		`\setlength\L{\N\baselineskip}\message{\the\L}`)
	if got != "12.0pt" {
		t.Errorf("\\setlength with an integer factor = %q, want 12.0pt", got)
	}
}

// markInput/restoreInput put the input back where it was, both in the base text
// and in the lists stacked above it — what lets a scanner read ahead and change
// its mind.
func TestInputMarkAndRestore(t *testing.T) {
	e := New()
	e.push(stringToToks("bc"))
	mark := e.markInput()
	a, _ := e.getNext()
	b, _ := e.getNext()
	if a.ch != 'b' || b.ch != 'c' {
		t.Fatalf("read %q%q, want bc", a.ch, b.ch)
	}
	e.restoreInput(mark)
	a2, ok := e.getNext()
	if !ok || a2.ch != 'b' {
		t.Errorf("after restoring, read %q (ok=%v), want b", a2.ch, ok)
	}
	// It restores the base position too.
	e2 := New()
	e2.SetFont(spMock{})
	e2.base = []rune("XY")
	m2 := e2.markInput()
	e2.getNext()
	e2.restoreInput(m2)
	if t2, _ := e2.getNext(); t2.ch != 'X' {
		t.Errorf("base position not restored: read %q", t2.ch)
	}
}

// isInternalInteger and upcomingInternalDimen recognise what may stand on each
// side of the factor form.
func TestFactorFormRecognisers(t *testing.T) {
	e := New()
	e.LoadPlain()
	e.Run(`\newcount\N\newdimen\D\newskip\S\chardef\C=3 \def\mac{x}`)
	for _, name := range []string{"N", "C", "count", "numexpr", "catcode"} {
		if !e.isInternalInteger(csTok(name)) {
			t.Errorf("\\%s should count as an integer", name)
		}
	}
	for _, name := range []string{"D", "S", "mac", "relax", "pasdefini", "hsize"} {
		if e.isInternalInteger(csTok(name)) {
			t.Errorf("\\%s should not count as an integer", name)
		}
	}
	// The unit side is exercised through real scanning: a factor followed by a
	// dimension multiplies, one followed by anything else does not.
	for _, c := range []struct{ src, want string }{
		{`\newcount\N\N=3 \newdimen\D\D=2pt\newdimen\Z\Z=\N\D\message{\the\Z}`, "6.0pt"},
		{`\newcount\N\N=3 \newskip\S\S=2pt\newdimen\Z\Z=\N\S\message{\the\Z}`, "6.0pt"},
		{`\newcount\N\N=3 \baselineskip=2pt\newdimen\Z\Z=\N\baselineskip\message{\the\Z}`, "6.0pt"},
		{`\newcount\N\N=3 \hsize=2pt\newdimen\Z\Z=\N\hsize\message{\the\Z}`, "6.0pt"},
	} {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// The end of the input is not a unit, and a factor sitting at the very end
// assigns nothing rather than hanging.
func TestFactorAtEndOfInput(t *testing.T) {
	e := New()
	e.noBase = true
	if e.upcomingInternalDimen() {
		t.Error("the end of the input is not a unit")
	}
	e2 := New()
	if _, err := e2.Run(`\newcount\N\N=5 \newdimen\Z\Z=\N`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e2.out.String()); got != "" {
		t.Errorf("unexpected output %q", got)
	}
}
