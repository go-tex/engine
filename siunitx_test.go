// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// TestFormatNumber exercises the pure \num formatter with exact-string asserts,
// covering grouping, sign, decimal, leading-dot normalisation and scientific
// notation, plus the malformed/empty error branches.
func TestFormatNumber(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"12345.678", `12\thinspace 345.678`},
		{"1234", `1\thinspace 234`},
		{"123", "123"},
		{"1000000", `1\thinspace 000\thinspace 000`},
		{"100000", `100\thinspace 000`},
		{"-12345", `-12\thinspace 345`},
		{"+42", "42"},
		{".5", "0.5"},
		{"0.5", "0.5"},
		{"12.", "12"},
		{"1.23e4", `1.23$\times 10^{4}$`},
		{"6.022e23", `6.022$\times 10^{23}$`},
		{"1e4", `$10^{4}$`},
		{"1E4", `$10^{4}$`},
		{"1.5e-3", `1.5$\times 10^{-3}$`},
		{"1.5e+03", `1.5$\times 10^{3}$`},
		{"2e0", `2$\times 10^{0}$`},
		{"1e-0", `$10^{0}$`},
		{"12345", `12\thinspace 345`},
	}
	for _, c := range cases {
		got, err := formatNumber(c.in)
		if err != nil {
			t.Errorf("formatNumber(%q) unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("formatNumber(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestFormatNumberErrors checks every error branch of the number parser.
func TestFormatNumberErrors(t *testing.T) {
	bad := []string{
		"",      // empty
		"   ",   // blank
		"abc",   // no digits
		"1.2.3", // stray dot
		"1,234", // comma not accepted
		"-",     // sign only
		"+",     // sign only
		".",     // dot only
		"1e",    // empty exponent
		"1e+",   // exponent sign only
		"1eX",   // non-digit exponent
		"1.2x3", // trailing junk
		"12 34", // internal space (grabRawArg strips spaces, but the raw formatter must reject)
	}
	for _, in := range bad {
		if _, err := formatNumber(in); err == nil {
			t.Errorf("formatNumber(%q) = nil error, want error", in)
		}
	}
}

// TestGroupInteger asserts the digit-grouping helper directly.
func TestGroupInteger(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"1":       "1",
		"12":      "12",
		"123":     "123",
		"1234":    `1\thinspace 234`,
		"12345":   `12\thinspace 345`,
		"123456":  `123\thinspace 456`,
		"1234567": `1\thinspace 234\thinspace 567`,
	}
	for in, want := range cases {
		if got := groupInteger(in); got != want {
			t.Errorf("groupInteger(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestNormalizeExponent covers exponent canonicalisation and its error branch.
func TestNormalizeExponent(t *testing.T) {
	ok := map[string]string{
		"4":   "4",
		"+4":  "4",
		"-4":  "-4",
		"04":  "4",
		"+04": "4",
		"-03": "-3",
		"0":   "0",
		"000": "0",
		"-0":  "0",
	}
	for in, want := range ok {
		got, err := normalizeExponent(in)
		if err != nil || got != want {
			t.Errorf("normalizeExponent(%q) = %q, %v; want %q, nil", in, got, err, want)
		}
	}
	for _, in := range []string{"", "+", "-", "x", "1x"} {
		if _, err := normalizeExponent(in); err == nil {
			t.Errorf("normalizeExponent(%q) = nil error, want error", in)
		}
	}
}

// mkToks builds the raw token list \si would grab from a source fragment (control
// sequences and literal characters), so the unit formatter can be tested directly.
func mkToks(items ...string) []tok {
	var ts []tok
	for _, it := range items {
		if strings.HasPrefix(it, `\`) {
			ts = append(ts, csTok(it[1:]))
		} else {
			for _, r := range it {
				c := catOther
				if r == ' ' {
					c = catSpace
				}
				ts = append(ts, chTok(r, c))
			}
		}
	}
	return ts
}

// TestFormatUnitTokens covers unit composition: single units, prefixes, powers,
// \per division, multiplied units, unknown-macro passthrough and empty input.
func TestFormatUnitTokens(t *testing.T) {
	cases := []struct {
		name string
		toks []tok
		want string
	}{
		{"metre", mkToks(`\meter`), "m"},
		{"per", mkToks(`\meter`, `\per`, `\second`), "m/s"},
		{"prefix", mkToks(`\kilo`, `\meter`, `\per`, `\hour`), "km/h"},
		{"squared", mkToks(`\meter`, `\squared`), "m²"},
		{"cubed", mkToks(`\meter`, `\cubed`), "m³"},
		{"product", mkToks(`\newton`, `\meter`), `N\thinspace m`},
		{"accel", mkToks(`\meter`, `\per`, `\second`, `\squared`), "m/s²"},
		{"twoper", mkToks(`\kilogram`, `\per`, `\meter`, `\per`, `\second`), "kg/m/s"},
		{"prefixed-squared", mkToks(`\kilo`, `\meter`, `\squared`), "km²"},
		{"micro", mkToks(`\micro`, `\meter`), "µm"},
		{"unknown", mkToks(`\foobar`), "foobar"},
		{"unknown-product", mkToks(`\newton`, `\foobar`), `N\thinspace foobar`},
		{"empty", nil, ""},
		{"spaces-ignored", mkToks(`\meter`, " ", `\per`, " ", `\second`), "m/s"},
		{"ohm", mkToks(`\ohm`), "Ω"},
		{"percent", mkToks(`\percent`), "%"},
		{"literal-char", mkToks("m"), "m"},
		{"leading-per", mkToks(`\per`, `\second`), "/s"},
		{"prefix-after-unit", mkToks(`\meter`, `\kilo`, `\meter`), `m\thinspace km`},
		{"lone-power", mkToks(`\squared`), "²"},
		{"two-literal", mkToks("m", "s"), `m\thinspace s`},
	}
	for _, c := range cases {
		if got := formatUnitTokens(c.toks); got != c.want {
			t.Errorf("%s: formatUnitTokens = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestFormatAngle covers the single-field and d;m;s forms, plus empty input.
func TestFormatAngle(t *testing.T) {
	cases := map[string]string{
		"30":          "30°",
		"  30 ":       "30°",
		"30;15;45":    "30°15′45″",
		"30;15":       "30°15′",
		"1.5":         "1.5°",
		"":            "",
		"   ":         "",
		"30;15;45;99": "30°15′45″", // extra fields ignored
	}
	for in, want := range cases {
		if got := formatAngle(in); got != want {
			t.Errorf("formatAngle(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTokenizeTeX asserts the round-trip tokeniser: control words swallow one
// trailing space, control symbols do not, and category codes are assigned.
func TestTokenizeTeX(t *testing.T) {
	ts := tokenizeTeX(`ab\thinspace 3`)
	// expected: a, b, \thinspace, 3   (the delimiter space is consumed)
	if len(ts) != 4 {
		t.Fatalf("tokenizeTeX produced %d tokens, want 4: %+v", len(ts), ts)
	}
	if ts[0].ch != 'a' || ts[1].ch != 'b' {
		t.Errorf("first tokens = %q %q, want a b", ts[0].ch, ts[1].ch)
	}
	if !ts[2].cs_ || ts[2].cs != "thinspace" {
		t.Errorf("token[2] = %+v, want cs thinspace", ts[2])
	}
	if ts[3].cs_ || ts[3].ch != '3' {
		t.Errorf("token[3] = %+v, want char 3", ts[3])
	}
	// math, super, group and control-symbol categories
	ts = tokenizeTeX(`$x^{2}$\!`)
	if ts[0].cat != catMath || ts[2].cat != catSup || ts[3].cat != catBegin || ts[5].cat != catEnd {
		t.Errorf("categories not assigned: %+v", ts)
	}
	last := ts[len(ts)-1]
	if !last.cs_ || last.cs != "!" {
		t.Errorf("last token = %+v, want control symbol !", last)
	}
	// subscript and a surviving literal space (not a control-word delimiter)
	ts = tokenizeTeX(`a_b c`)
	if ts[1].cat != catSub || ts[3].cat != catSpace {
		t.Errorf("subscript/space categories not assigned: %+v", ts)
	}
	// a trailing backslash is dropped, empty input yields nothing
	if got := tokenizeTeX(`x\`); len(got) != 1 || got[0].ch != 'x' {
		t.Errorf("trailing backslash not dropped: %+v", got)
	}
	if got := tokenizeTeX(""); got != nil && len(got) != 0 {
		t.Errorf("tokenizeTeX(\"\") = %+v, want empty", got)
	}
}

// runSI typesets src through a LaTeX engine (with a mock font) and returns the
// literal characters read back from the main vertical list.
func runSI(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatalf("LoadLaTeX: %v", err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run("\\hsize=300pt\n" + src); err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	return b.String()
}

// TestNumTypeset drives the whole pipeline: \num groups digits (the thin space is
// glue, so the read-back chars are the digits and separator characters), and the
// scientific-notation mantissa is typeset (its exponent lives in a math box).
func TestNumTypeset(t *testing.T) {
	if got := runSI(t, `\num{12345.678}`); !strings.Contains(got, "12345.678") {
		t.Errorf("\\num{12345.678} typeset chars = %q, want to contain 12345.678", got)
	}
	if got := runSI(t, `\num{6.022e23}`); !strings.Contains(got, "6.022") {
		t.Errorf("\\num{6.022e23} typeset chars = %q, want to contain mantissa 6.022", got)
	}
	// a malformed \num must not abort the run nor panic
	if got := runSI(t, `ok\num{}\num{zzz}done`); !strings.Contains(got, "ok") || !strings.Contains(got, "done") {
		t.Errorf("malformed \\num broke the run; chars = %q", got)
	}
	// grabRawArg drops interior spaces and control sequences from the argument:
	// "1 234" groups to "1 234" and "\empty" is ignored, so both read back as digits.
	if got := runSI(t, `\num{1 234}`); !strings.Contains(got, "1234") {
		t.Errorf(`\num{1 234} chars = %q, want to contain 1234`, got)
	}
	if got := runSI(t, `\num{5\empty}`); !strings.Contains(got, "5") {
		t.Errorf(`\num{5\empty} chars = %q, want to contain 5`, got)
	}
}

// TestSITypeset composes units through the full pipeline and reads the glyphs
// back (thin spaces between multiplied units are glue and thus not captured).
func TestSITypeset(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\si{\meter\per\second}`, "m/s"},
		{`\si{\kilo\meter\per\hour}`, "km/h"},
		{`\si{\meter\squared}`, "m²"},
		{`\unit{\meter\per\second}`, "m/s"},
		{`\si{\newton\meter}`, "Nm"}, // thin space is glue → "Nm" in the char stream
	}
	for _, c := range cases {
		if got := runSI(t, c.src); !strings.Contains(got, c.want) {
			t.Errorf("%s typeset chars = %q, want to contain %q", c.src, got, c.want)
		}
	}
}

// TestQuantityTypeset checks \SI and its \qty alias join value and unit.
func TestQuantityTypeset(t *testing.T) {
	for _, src := range []string{
		`\SI{9.81}{\meter\per\second\squared}`,
		`\qty{9.81}{\meter\per\second\squared}`,
	} {
		got := runSI(t, src)
		if !strings.Contains(got, "9.81") || !strings.Contains(got, "m/s²") {
			t.Errorf("%s typeset chars = %q, want to contain 9.81 and m/s²", src, got)
		}
	}
	// a malformed value still typesets the unit and does not abort
	if got := runSI(t, `\SI{bad}{\meter}`); !strings.Contains(got, "m") {
		t.Errorf(`\SI{bad}{\meter} chars = %q, want to contain m`, got)
	}
}

// TestAngTypeset checks \ang renders the degree symbol and the d;m;s form.
func TestAngTypeset(t *testing.T) {
	if got := runSI(t, `\ang{30}`); !strings.Contains(got, "30°") {
		t.Errorf(`\ang{30} chars = %q, want to contain 30°`, got)
	}
	if got := runSI(t, `\ang{30;15;45}`); !strings.Contains(got, "30°15′45″") {
		t.Errorf(`\ang{30;15;45} chars = %q, want to contain 30°15′45″`, got)
	}
}

// TestSIUnitxNoPanicEmpty makes sure the empty-argument branches of every command
// typeset nothing and never abort the surrounding run.
func TestSIUnitxNoPanicEmpty(t *testing.T) {
	src := `A\num{}B\si{}C\unit{}D\ang{}E\SI{}{}F\qty{}{}G`
	got := runSI(t, src)
	for _, want := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		if !strings.Contains(got, want) {
			t.Errorf("empty-arg run lost %q; chars = %q", want, got)
		}
	}
}
