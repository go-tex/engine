package engine

import (
	"strings"
	"testing"
)

// runExpr runs a source that prints with \message on a bare engine (no macro
// layer) and returns what it printed.
func runExpr(t *testing.T, src string) string {
	t.Helper()
	e := New()
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return out
}

// \numexpr evaluates an integer expression with e-TeX's precedence and rounding.
func TestNumexpr(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\the\numexpr 1+2\relax`, "3"},
		{`\the\numexpr 10-3-2\relax`, "5"},               // left associative
		{`\the\numexpr 2*3+4\relax`, "10"},               // * binds tighter than +
		{`\the\numexpr 4+2*3\relax`, "10"},               //
		{`\the\numexpr (2+3)*4\relax`, "20"},             // parentheses
		{`\the\numexpr ((1+1))*3\relax`, "6"},            // nested
		{`\the\numexpr 7/2\relax`, "4"},                  // rounds, not truncates
		{`\the\numexpr 5/2\relax`, "3"},                  // ties away from zero
		{`\the\numexpr -5/2\relax`, "-3"},                //
		{`\the\numexpr -7/2\relax`, "-4"},                //
		{`\the\numexpr 1/0\relax`, "0"},                  // division by zero yields zero
		{`\the\numexpr -3*-4\relax`, "12"},               // signed factors
		{`\the\numexpr +5\relax`, "5"},                   //
		{`\the\numexpr 1000000*400/803\relax`, "498132"}, // the product keeps full precision
		{`\the\numexpr\numexpr 2+3\relax*2\relax`, "10"}, // an expression as a factor
	}
	for _, c := range cases {
		if got := runExpr(t, `\message{`+c.src+`}`); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// \dimexpr evaluates a dimension expression; a unitless factor is a coefficient.
func TestDimexpr(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\the\dimexpr 1pt+2pt\relax`, "3.0pt"},
		{`\the\dimexpr 5pt-1pt-1pt\relax`, "3.0pt"},
		{`\the\dimexpr 2pt*3\relax`, "6.0pt"},  // dimension then coefficient
		{`\the\dimexpr 3*2pt\relax`, "6.0pt"},  // coefficient then dimension
		{`\the\dimexpr 10pt/4\relax`, "2.5pt"}, //
		{`\the\dimexpr (1pt+1pt)*2\relax`, "4.0pt"},
		{`\the\dimexpr 65536sp\relax`, "1.0pt"},
		{`\the\dimexpr 1in\relax`, "72.26999pt"},
		{`\the\dimexpr -2pt\relax`, "-2.0pt"},
		{`\the\dimexpr 1truept\relax`, "1.0pt"}, // a "true" unit (no \mag here)
	}
	for _, c := range cases {
		if got := runExpr(t, `\message{`+c.src+`}`); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// A register or other internal dimension is both an operand and a valid unit for
// a preceding factor (TeX's <factor><internal dimen>).
func TestDimexprInternalDimen(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\hsize=100pt \message{\the\dimexpr .5\hsize\relax}`, "50.0pt"},
		{`\hsize=100pt \message{\the\dimexpr \hsize/4\relax}`, "25.0pt"},
		{`\dimen3=8pt \message{\the\dimexpr \dimen3*2\relax}`, "16.0pt"},
		{`\newdimen\z \z=3pt \message{\the\dimexpr \z+1pt\relax}`, "4.0pt"},
		{`\newdimen\z \z=3pt \message{\the\dimexpr 2\z\relax}`, "6.0pt"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// The result of either primitive is an internal quantity: it can be assigned to a
// register of its own type, and coerces between them the way TeX's do.
func TestExprAsInternalQuantity(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\count0=\numexpr 3*4\relax \message{\the\count0}`, "12"},
		{`\dimen0=\dimexpr 1in-1pt\relax \message{\the\dimen0}`, "71.26999pt"},
		{`\hsize=\dimexpr 50pt*2\relax \message{\the\hsize}`, "100.0pt"},
		{`\count0=\dimexpr 1pt\relax \message{\the\count0}`, "65536"},   // dimen → sp
		{`\dimen0=\numexpr 65536\relax \message{\the\dimen0}`, "1.0pt"}, // integer → sp
		{`\count0=-\numexpr 5\relax \message{\the\count0}`, "-5"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

// The terminating \relax is consumed, and only that: text after the expression is
// typeset normally, and an expression with no \relax stops at the first token
// that cannot continue it.
func TestExprTermination(t *testing.T) {
	if got := runExpr(t, `\message{\the\numexpr 1+1\relax x}`); got != "2x" {
		t.Errorf("\\relax was not absorbed: %q", got)
	}
	if got := runExpr(t, `\message{\the\numexpr 1+1 x}`); got != "2x" {
		t.Errorf("expression did not stop at a non-operator: %q", got)
	}
	if got := runExpr(t, `\message{[\the\numexpr 2\relax]}`); got != "[2]" {
		t.Errorf("terminator handling: %q", got)
	}
}

// A malformed expression is read as far as it makes sense rather than derailing
// the run: an unbalanced parenthesis leaves the offending token for the caller.
func TestExprMalformed(t *testing.T) {
	if got := runExpr(t, `\message{\the\numexpr (1+2\relax}`); got != "3" {
		t.Errorf("unclosed parenthesis = %q, want 3", got)
	}
	if got := runExpr(t, `\message{\the\numexpr\relax}`); got != "0" {
		t.Errorf("empty expression = %q, want 0", got)
	}
}

// divRound implements e-TeX's rounding directly, including the zero divisor.
func TestDivRound(t *testing.T) {
	cases := []struct{ a, b, want int64 }{
		{7, 2, 4}, {5, 2, 3}, {4, 2, 2}, {1, 2, 1}, {0, 5, 0},
		{-7, 2, -4}, {7, -2, -4}, {-7, -2, 4}, {3, 0, 0},
	}
	for _, c := range cases {
		if got := divRound(c.a, c.b); got != c.want {
			t.Errorf("divRound(%d,%d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// isInternalDimen recognises what may serve as a unit of measure.
func TestIsInternalDimen(t *testing.T) {
	e := New()
	e.Run(`\newdimen\len \newcount\num \newskip\sk \def\mac{x}`)
	yes := []string{"len", "sk", "hsize", "vsize", "dimen", "wd", "dimexpr", "parindent"}
	no := []string{"num", "mac", "relax", "undefinedname"}
	for _, cs := range yes {
		if !e.isInternalDimen(tok{cs: cs, cs_: true}) {
			t.Errorf("\\%s should count as a dimension", cs)
		}
	}
	for _, cs := range no {
		if e.isInternalDimen(tok{cs: cs, cs_: true}) {
			t.Errorf("\\%s should not count as a dimension", cs)
		}
	}
}

// Both tokens survive \futurelet, in their original order.
func TestFutureletPutsBothBack(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\futurelet\next AB`); err != nil {
		t.Fatal(err)
	}
	if got := glyphString(e.mvl); got != "AB" {
		t.Errorf("typeset %q, want AB", got)
	}
}

// \futurelet of a control sequence copies its meaning; of an undefined one it
// makes \next undefined (so \ifx against \relax is false).
func TestFutureletMeaning(t *testing.T) {
	e := New()
	out, err := e.Run(`\def\b{B}\futurelet\next\relax\b\ifx\next\b\message{same}\else\message{diff}\fi`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "same" {
		t.Errorf("meaning not copied: %q", out)
	}
}

// A truncated \futurelet (no tokens left to peek at) leaves the input alone
// instead of failing.
func TestFutureletAtEnd(t *testing.T) {
	e := New()
	if _, err := e.Run(`\futurelet\next`); err != nil {
		t.Fatal(err)
	}
	if _, err := New().Run(`\futurelet\next A`); err != nil {
		t.Fatal(err)
	}
}

// \newread allocates a stream so a package that opens one still loads.
func TestNewread(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	if _, err := e.Run(`\newread\myinput\message{ok}`); err != nil {
		t.Fatal(err)
	}
}

// An expression cut short by the end of the input yields what it had read rather
// than hanging or failing.
func TestExprInputEnds(t *testing.T) {
	for _, src := range []string{
		`\count0=\numexpr 1+`,
		`\count0=\numexpr 2*`,
		`\count0=\numexpr`,
		`\dimen0=\dimexpr 1pt+`,
		`\dimen0=\dimexpr 3`,
		`\dimen0=\dimexpr 1pt*`,
		`\dimen0=\dimexpr (1pt`,
	} {
		if _, err := New().Run(src); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
}

// A factor whose unit keyword is only half-present is no unit at all: rather than
// erroring as e-TeX does, the number stands as a bare coefficient (a count of
// scaled points) and the stray letter is left in the input for the caller.
func TestExprPartialUnit(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	out, err := e.Run(`\message{\the\dimexpr 2p}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "0.00003pt") {
		t.Errorf("a lone 'p' must not read as a unit: %q", out)
	}
}
