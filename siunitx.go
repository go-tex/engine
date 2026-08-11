// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"errors"
	"strings"
)

// This file implements a pragmatic subset of the LaTeX siunitx package for
// typesetting numbers and physical units: \num, \si, \unit, \SI/\qty and \ang.
//
// Supported subset (documented limitations):
//
//   - \num{...} formats a number: it groups the integer part in threes with a
//     thin space (\thinspace), keeps a leading sign, a '.' decimal separator and
//     a fractional part, and renders scientific notation "1.23e4" as inline math
//     "1.23×10^{4}" (a bare mantissa "1e4" becomes "10^{4}"). A leading ".5" is
//     normalised to "0.5". The integer part is grouped when it has more than three
//     digits (siunitx's default threshold is five — documented difference). Only
//     the integer part is grouped, not the fractional part. \pm / ± are not
//     supported inside \num.
//
//   - \si{...} / \unit{...} compose unit symbols from unit macros: unit names map
//     to upright symbols (\meter→m, \second→s, ...), SI prefixes attach with no
//     space (\kilo\meter→km), \squared/\cubed append ²/³, \per emits "/" between
//     the surrounding unit groups, and consecutive multiplied units are separated
//     by a thin space (\newton\meter→"N m"). Unknown unit macros pass their name
//     through unchanged rather than erroring.
//
//   - \SI{value}{unit} / \qty{value}{unit} typeset \num of the value, a thin
//     space, then \si of the unit (\SI{9.81}{\meter\per\second\squared} →
//     "9.81 m/s²").
//
//   - \ang{30} → "30°"; \ang{30;15;45} → "30°15′45″" (degree/arcminute/arcsecond).
//     Angle fields are passed through literally (no \num grouping is applied).
//
// Malformed or empty arguments are handled gracefully: the primitive typesets
// nothing rather than aborting the run.

// errEmptyNumber and errMalformedNumber report unusable \num arguments.
var (
	errEmptyNumber     = errors.New("siunitx: empty number")
	errMalformedNumber = errors.New("siunitx: malformed number")
)

// siUnits maps siunitx unit-name macros to their upright symbols.
var siUnits = map[string]string{
	"meter":         "m",
	"metre":         "m",
	"second":        "s",
	"kilogram":      "kg",
	"gram":          "g",
	"kelvin":        "K",
	"ampere":        "A",
	"mole":          "mol",
	"candela":       "cd",
	"newton":        "N",
	"pascal":        "Pa",
	"joule":         "J",
	"watt":          "W",
	"volt":          "V",
	"ohm":           "Ω", // Ω
	"hertz":         "Hz",
	"percent":       "%",
	"hour":          "h",
	"minute":        "min",
	"day":           "d",
	"liter":         "L",
	"litre":         "L",
	"coulomb":       "C",
	"farad":         "F",
	"tesla":         "T",
	"weber":         "Wb",
	"henry":         "H",
	"siemens":       "S",
	"radian":        "rad",
	"steradian":     "sr",
	"becquerel":     "Bq",
	"gray":          "Gy",
	"sievert":       "Sv",
	"lumen":         "lm",
	"lux":           "lx",
	"degreeCelsius": "°C", // °C
}

// siPrefixes maps SI prefix macros to their symbols. A prefix attaches to the
// following unit with no intervening space.
var siPrefixes = map[string]string{
	"yotta": "Y",
	"zetta": "Z",
	"exa":   "E",
	"peta":  "P",
	"tera":  "T",
	"giga":  "G",
	"mega":  "M",
	"kilo":  "k",
	"hecto": "h",
	"deca":  "da",
	"deka":  "da",
	"deci":  "d",
	"centi": "c",
	"milli": "m",
	"micro": "µ", // µ (micro sign)
	"nano":  "n",
	"pico":  "p",
	"femto": "f",
	"atto":  "a",
	"zepto": "z",
	"yocto": "y",
}

// siPowers maps power macros to their postfix superscript symbols.
var siPowers = map[string]string{
	"squared": "²", // ²
	"cubed":   "³", // ³
}

// isASCIILetter reports whether r is an ASCII letter (a control word constituent).
func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// tokenizeTeX turns an internally generated TeX-source string into a token list
// honouring the default catcodes: '\' starts a control sequence (a control word
// swallows one trailing space, as in TeX), and {, }, $, ^, _ and space take
// their usual categories. It is only ever fed strings this package produces, so
// fixed catcodes are sufficient.
func tokenizeTeX(s string) []tok {
	rs := []rune(s)
	ts := make([]tok, 0, len(rs))
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\\':
			j := i + 1
			if j < len(rs) && isASCIILetter(rs[j]) {
				k := j
				for k < len(rs) && isASCIILetter(rs[k]) {
					k++
				}
				ts = append(ts, csTok(string(rs[j:k])))
				if k < len(rs) && rs[k] == ' ' { // control-word delimiter space
					k++
				}
				i = k - 1
			} else if j < len(rs) {
				ts = append(ts, csTok(string(rs[j])))
				i = j
			}
			// a trailing backslash at end of string is dropped
		case '{':
			ts = append(ts, chTok(r, catBegin))
		case '}':
			ts = append(ts, chTok(r, catEnd))
		case '$':
			ts = append(ts, chTok(r, catMath))
		case '^':
			ts = append(ts, chTok(r, catSup))
		case '_':
			ts = append(ts, chTok(r, catSub))
		case ' ':
			ts = append(ts, chTok(r, catSpace))
		default:
			ts = append(ts, chTok(r, catOther))
		}
	}
	return ts
}

// pushTeX retokenises s and pushes it back onto the input to be typeset.
func (e *Engine) pushTeX(s string) {
	if s == "" {
		return
	}
	e.push(tokenizeTeX(s))
}

// groupInteger groups a run of decimal digits in threes from the right, joined by
// a thin space, e.g. "12345" → "12\thinspace 345". Runs of three digits or fewer
// are returned unchanged.
func groupInteger(digits string) string {
	n := len(digits)
	if n <= 3 {
		return digits
	}
	first := n % 3
	if first == 0 {
		first = 3
	}
	var b strings.Builder
	b.WriteString(digits[:first])
	for i := first; i < n; i += 3 {
		b.WriteString(`\thinspace `)
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

// parseMantissa splits a mantissa "±ddd.ddd" into its sign ("" or "-"), integer
// digits and fractional digits. A leading '+' is dropped. It errors on any stray
// character or when no digit is present at all.
func parseMantissa(m string) (sign, intPart, frac string, err error) {
	i := 0
	if i < len(m) && (m[i] == '+' || m[i] == '-') {
		if m[i] == '-' {
			sign = "-"
		}
		i++
	}
	j := i
	for j < len(m) && m[j] >= '0' && m[j] <= '9' {
		j++
	}
	intPart = m[i:j]
	i = j
	if i < len(m) && m[i] == '.' {
		i++
		k := i
		for k < len(m) && m[k] >= '0' && m[k] <= '9' {
			k++
		}
		frac = m[i:k]
		i = k
	}
	if i != len(m) {
		return "", "", "", errMalformedNumber
	}
	if intPart == "" && frac == "" {
		return "", "", "", errMalformedNumber
	}
	return sign, intPart, frac, nil
}

// normalizeExponent parses an exponent "±ddd" into a canonical signed integer
// string with leading zeros stripped ("+04" → "4", "-03" → "-3").
func normalizeExponent(e string) (string, error) {
	i := 0
	neg := false
	if i < len(e) && (e[i] == '+' || e[i] == '-') {
		neg = e[i] == '-'
		i++
	}
	digits := e[i:]
	if digits == "" {
		return "", errMalformedNumber
	}
	for k := 0; k < len(digits); k++ {
		if digits[k] < '0' || digits[k] > '9' {
			return "", errMalformedNumber
		}
	}
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		digits = "0"
	}
	if neg && digits != "0" {
		return "-" + digits, nil
	}
	return digits, nil
}

// formatNumber renders a \num argument to a TeX-source string. See the file
// header for the supported subset. It errors on empty or malformed input.
func formatNumber(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errEmptyNumber
	}
	mant := s
	exp := ""
	hasExp := false
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		hasExp = true
		mant = s[:i]
		exp = s[i+1:]
	}
	sign, intPart, frac, err := parseMantissa(mant)
	if err != nil {
		return "", err
	}
	// parseMantissa guarantees a non-empty integer or fractional part, so when the
	// integer part is empty a fractional part is present: ".5" → "0.5".
	if intPart == "" {
		intPart = "0"
	}
	var mb strings.Builder
	mb.WriteString(sign)
	mb.WriteString(groupInteger(intPart))
	if frac != "" {
		mb.WriteByte('.')
		mb.WriteString(frac)
	}
	mantText := mb.String()
	if !hasExp {
		return mantText, nil
	}
	en, err := normalizeExponent(exp)
	if err != nil {
		return "", err
	}
	if sign == "" && intPart == "1" && frac == "" { // "1e4" → "10^{4}"
		return "$10^{" + en + "}$", nil
	}
	return mantText + `$\times 10^{` + en + `}$`, nil
}

// formatUnitTokens composes a unit expression from the raw tokens of a \si/\unit
// argument. See the file header for the supported subset.
func formatUnitTokens(toks []tok) string {
	var groups []string
	var conns []string // conns[i] is the connector printed before groups[i]; conns[0] == ""
	var b strings.Builder
	open := false      // a group is currently being built
	hasUnit := false   // the current group already carries a base unit
	conn := ""         // the connector chosen for the current group
	nextIsPer := false // a \per was seen; the next opened group is "/"-connected

	closeGroup := func() {
		if !open {
			return
		}
		groups = append(groups, b.String())
		conns = append(conns, conn)
		b.Reset()
		open = false
		hasUnit = false
	}
	openGroup := func() {
		switch {
		case nextIsPer:
			conn = "/"
		case len(groups) == 0:
			conn = ""
		default:
			conn = `\thinspace `
		}
		nextIsPer = false
		open = true
		hasUnit = false
	}

	for _, t := range toks {
		if t.cs_ {
			name := t.cs
			if sym, ok := siPrefixes[name]; ok {
				if open && hasUnit {
					closeGroup()
				}
				if !open {
					openGroup()
				}
				b.WriteString(sym)
				continue
			}
			if sym, ok := siPowers[name]; ok {
				if !open {
					openGroup()
				}
				b.WriteString(sym)
				continue
			}
			if name == "per" {
				closeGroup()
				nextIsPer = true
				continue
			}
			sym, ok := siUnits[name]
			if !ok {
				sym = name // unknown unit: pass its name through
			}
			if open && hasUnit {
				closeGroup()
			}
			if !open {
				openGroup()
			}
			b.WriteString(sym)
			hasUnit = true
			continue
		}
		if t.cat == catSpace {
			continue // spacing is controlled by the unit macros
		}
		// A literal character is treated as unit content.
		if open && hasUnit {
			closeGroup()
		}
		if !open {
			openGroup()
		}
		b.WriteRune(t.ch)
		hasUnit = true
	}
	closeGroup()

	var out strings.Builder
	for i, g := range groups {
		out.WriteString(conns[i])
		out.WriteString(g)
	}
	return out.String()
}

// formatAngle renders an \ang argument. A single field N becomes "N°"; a
// semicolon-separated "d;m;s" becomes "d°m′s″". Fields are passed through
// literally (trimmed), without \num grouping.
func formatAngle(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ";")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) == 1 {
		return parts[0] + "°" // °
	}
	syms := []string{"°", "′", "″"} // ° ′ ″
	var b strings.Builder
	for i := 0; i < len(parts) && i < len(syms); i++ {
		b.WriteString(parts[i])
		b.WriteString(syms[i])
	}
	return b.String()
}

// grabRawArg reads a braced (or single-token) argument and returns its literal
// character text, dropping control sequences and spaces. Used for the numeric
// arguments of \num, \SI/\qty and \ang.
func (e *Engine) grabRawArg() string {
	toks := e.grabUndelimited()
	var b strings.Builder
	for _, t := range toks {
		if t.cs_ || t.cat == catSpace {
			continue
		}
		b.WriteRune(t.ch)
	}
	return b.String()
}

// loadSIUnitx installs the siunitx primitives. Called at the end of
// loadPrimitives so the control sequences exist as soon as an engine is created.
func (e *Engine) loadSIUnitx() {
	e.prim("num", func(e *Engine) {
		out, err := formatNumber(e.grabRawArg())
		if err != nil {
			return // malformed/empty: typeset nothing rather than abort
		}
		e.pushTeX(out)
	})
	e.prim("ang", func(e *Engine) {
		e.pushTeX(formatAngle(e.grabRawArg()))
	})
	unitPrim := func(e *Engine) {
		e.pushTeX(formatUnitTokens(e.grabUndelimited()))
	}
	e.prim("si", unitPrim)
	e.prim("unit", unitPrim)
	quantityPrim := func(e *Engine) {
		numText, err := formatNumber(e.grabRawArg())
		unitText := formatUnitTokens(e.grabUndelimited())
		var b strings.Builder
		if err == nil && numText != "" {
			b.WriteString(numText)
			if unitText != "" {
				b.WriteString(`\thinspace `)
			}
		}
		b.WriteString(unitText)
		e.pushTeX(b.String())
	}
	e.prim("SI", quantityPrim)
	e.prim("qty", quantityPrim)
}
