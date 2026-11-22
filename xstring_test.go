// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⭐ \IfInteger and \IfDecimal pick ONE branch. Undefined they were skipped and both
// braced groups were typeset, so a class testing an option with them printed the
// "true" and the "false" text side by side on the first page.
//
// Every expectation below is xstring's own answer, read from xstring.tex:533-584 and
// checked against tectonic for the first two (\IfInteger{42}{int}{notint} sets "int";
// \IfInteger{4x}{int}{notint} sets "notint"). The cases that are not obvious carry the
// reason: "5." is not a decimal because xstring.tex:578-581 turns an empty decimal
// part into "0X", and "2.0" IS an integer — xstring.tex:1152 is the bug fix that made
// it one.
func TestIfIntegerAndIfDecimalPickOneBranch(t *testing.T) {
	cases := []struct {
		call string
		want string
	}{
		{`\IfInteger{42}{I}{N}`, "I"},
		{`\IfInteger{4x}{I}{N}`, "N"},
		{`\IfInteger{-7}{I}{N}`, "I"},
		{`\IfInteger{+7}{I}{N}`, "I"},
		{`\IfInteger{2.0}{I}{N}`, "I"},
		{`\IfInteger{2.50}{I}{N}`, "N"},
		{`\IfInteger{}{I}{N}`, "N"},
		{`\IfInteger{-}{I}{N}`, "N"},
		{`\IfDecimal{2.5}{D}{N}`, "D"},
		{`\IfDecimal{.5}{D}{N}`, "D"},
		{`\IfDecimal{3,25}{D}{N}`, "D"},
		{`\IfDecimal{5.}{D}{N}`, "N"},
		{`\IfDecimal{1.2.3}{D}{N}`, "N"},
		// A separator other than "." or "," after the integer part is no number, even
		// when a digit follows it.
		{`\IfDecimal{5x1}{D}{N}`, "N"},
		// The number is EXPANDED first, as xstring does (\xs_expand_and_assign).
		{`\def\num{12}\IfInteger{\num}{I}{N}`, "I"},
	}
	for _, c := range cases {
		lines := lineTextOf(t, `A`+c.call+`B`)
		got := strings.Join(lines, "")
		if got != "A"+c.want+"B" {
			t.Errorf("%s set %q, want %q", c.call, got, "A"+c.want+"B")
		}
	}
}
