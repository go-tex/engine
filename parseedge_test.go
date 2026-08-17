package engine

import "testing"

// The guards on the ends of the new readers: input that stops in the middle of
// what they are reading must return what they have, not run off the buffer.
func TestReadersAtTheEndOfInput(t *testing.T) {
	// rawAt is asked for a position past the end (its callers check first, so this
	// is the guard that keeps it honest if one ever does not).
	e := New()
	if r, next := e.rawAt(len(e.base) + 5); r != 0 || next != len(e.base)+6 {
		t.Errorf("rawAt past the end = (%q, %d), want (0, %d)", r, next, len(e.base)+6)
	}
	// ^^ before a character it cannot shift (anything outside ASCII) leaves the
	// caret alone rather than inventing a code point.
	if got := runExpr(t, `\message{[^^é]}`); got != "[^^é]" {
		t.Errorf("^^ before a non-ASCII character = %q, want %q", got, "[^^é]")
	}
	// A "#{" argument that never meets its brace ends with the input, and the body
	// still runs on what it did read.
	if got := runExpr(t, `\def\a#1#{\message{[#1]}}\message{before}\a yz`); got != "before [yz]" {
		t.Errorf("unterminated #{ argument = %q, want %q", got, "before [yz]")
	}
	// A parameter text that ends with the file.
	if got := runExpr(t, `\message{first}\def\a#`); got != "first" {
		t.Errorf("truncated parameter text = %q, want %q", got, "first")
	}
	// A file that ends on a bare escape character makes the EMPTY control sequence,
	// which is undefined — the same complaint real TeX makes, not a crash.
	if _, err := New().Run("\\message{first}\\"); err == nil {
		t.Error("a trailing escape character must be reported as an undefined control sequence")
	}
}

// \PassOptionsToPackage with no braced list leaves the input alone instead of
// eating what follows.
func TestPassOptionsWithoutAGroup(t *testing.T) {
	if got := ckRun(t, `\PassOptionsToPackage\message{AFTER}`); got != "AFTER" {
		t.Errorf("\\PassOptionsToPackage with no group = %q, want %q", got, "AFTER")
	}
	// An option list the input never closes ends with the input.
	if got := ckRun(t, `\message{BEFORE}\PassOptionsToPackage{a,b`); got != "BEFORE" {
		t.Errorf("unterminated option list = %q, want %q", got, "BEFORE")
	}
}
