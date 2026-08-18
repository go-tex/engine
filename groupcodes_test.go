package engine

import "testing"

// A closer that arrives at a level its partner did not open is a mismatch, and
// TeX's two mismatches are NOT symmetric. Ground truth read off tectonic
// (2026-08-18), messages and rendered values both:
//
//	}                                     ! Too many }'s.
//	\endgroup                             ! Extra \endgroup.
//	\begingroup}                          ! Extra }, or forgotten \endgroup.
//	\setbox0=\hbox{\begingroup A}         ! Extra }, or forgotten \endgroup.
//	{\endgroup}                           ! Missing } inserted.
//	\begingroup\setbox1=\hbox{B\endgroup} ! Missing } inserted.
//
// "Missing } inserted" INSERTS the brace the box or simple group is waiting for
// and re-reads the \endgroup, so both levels unwind — that is what offSave does.
// "Extra }" takes the other road: its help text says "I've deleted a
// group-closing symbol", and TeX leaves both groups open.

// TestGroupMismatchInsertsTheMissingBrace covers the half this engine reproduces:
// an \endgroup meeting a box or simple group.
func TestGroupMismatchInsertsTheMissingBrace(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{{
		// } is inserted, the box closes, then the \endgroup closes the semi simple
		// group and restores \x — tectonic prints [OUT] here too.
		name: "\\endgroup dans une hbox",
		src:  "\\def\\x{OUT}\\begingroup\\def\\x{IN}\\setbox0=\\hbox{B\\endgroup}\n\\message{[\\x]}\n",
		want: "[OUT]",
	}, {
		name: "\\endgroup dans un groupe simple",
		src:  "\\def\\w{OUT}{\\def\\w{IN}\\endgroup}\n\\message{[\\w]}\n",
		want: "[OUT]",
	}} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.lenient = true
			out, err := e.Run(c.src)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if out != c.want {
				t.Errorf("sortie = %q, attendu %q", out, c.want)
			}
			if len(e.groups) != 0 {
				t.Errorf("groupes restants = %d, attendu 0", len(e.groups))
			}
		})
	}
}

// TestExtraBraceStillClosesTheGroup pins the KNOWN divergence, so that closing
// it is a test change rather than a surprise. Real TeX deletes the brace and
// leaves both groups open, which renders [IN]; this engine reports the mismatch
// but still closes the group, which renders [OUT].
//
// The deletion was implemented and MEASURED over the 10025-document beamer
// corpus: it cost pages in two talks, because there the brace lands on
// \end{document} with four groups open that TeX never opened. Faithful recovery
// from a state TeX would never be in is not fidelity. When the engine stops
// leaking those groups, delete the brace here and these expectations become
// [IN].
func TestExtraBraceStillClosesTheGroup(t *testing.T) {
	for _, c := range []struct{ src, want, tex string }{
		{"\\def\\y{OUT}\\setbox0=\\hbox{\\begingroup\\def\\y{IN}A}\n\\message{[\\y]}\n", "[OUT]", "[IN]"},
		{"\\def\\z{OUT}\\begingroup\\def\\z{IN}}\n\\message{[\\z]}\n", "[OUT]", "[IN]"},
	} {
		e := New()
		e.lenient = true
		out, err := e.Run(c.src)
		if err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		if out != c.want {
			t.Errorf("%q → %q, attendu %q (tectonic rend %s)", c.src, out, c.want, c.tex)
		}
	}
}

// Balanced grouping is untouched by the kind checks: each closer meets its own
// opener, so nothing is inserted and nothing is reported.
func TestBalancedGroupingUnaffected(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"\\def\\a{OUT}{\\def\\a{IN}\\message{[\\a]}}\\message{[\\a]}", "[IN] [OUT]"},
		{"\\def\\b{OUT}\\begingroup\\def\\b{IN}\\message{[\\b]}\\endgroup\\message{[\\b]}", "[IN] [OUT]"},
		{"\\def\\c{OUT}\\setbox0=\\hbox{\\def\\c{IN}}\\message{[\\c]}", "[OUT]"},
		{"\\def\\d{OUT}\\begingroup{\\def\\d{IN}}\\message{[\\d]}\\endgroup", "[OUT]"},
	} {
		e := New()
		e.lenient = true
		out, err := e.Run(c.src)
		if err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		if out != c.want {
			t.Errorf("%q → %q, attendu %q", c.src, out, c.want)
		}
		if len(e.groups) != 0 {
			t.Errorf("%q laisse %d groupe(s) ouvert(s)", c.src, len(e.groups))
		}
	}
}

// The mismatch is reported with TeX's own wording. This is the measurable gain
// of the group codes: before them the engine popped whichever group was on top
// and said nothing at all.
func TestGroupMismatchReportedInStrictMode(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"\\def\\z{}\\begingroup}", "Extra }, or forgotten \\endgroup."},
		{"\\def\\z{}{\\endgroup}", "Missing } inserted."},
		{"\\def\\z{}\\setbox0=\\hbox{\\begingroup A}", "Extra }, or forgotten \\endgroup."},
	} {
		e := New()
		_, err := e.Run(c.src)
		if err == nil {
			t.Errorf("%q: aucune erreur, attendu %q", c.src, c.want)
			continue
		}
		se, ok := err.(SourceError)
		if !ok || se.Msg != c.want {
			t.Errorf("%q → %v, attendu %q", c.src, err, c.want)
		}
	}
}

// A closer with no group at all open is dropped in silence. Real TeX says
// "Too many }'s." / "Extra \endgroup.", but several of this engine's own
// recovery paths push a stray closer on purpose, so reporting it would blame the
// author for the engine's bookkeeping.
func TestClosersWithNoGroupOpenAreSilent(t *testing.T) {
	for _, src := range []string{"\\message{[a]}}", "\\message{[a]}\\endgroup"} {
		e := New()
		out, err := e.Run(src)
		if err != nil {
			t.Errorf("%q: %v", src, err)
		}
		if out != "[a]" {
			t.Errorf("%q → %q", src, out)
		}
	}
}

func TestCurGroupKindWithNoGroup(t *testing.T) {
	e := New()
	if k, open := e.curGroupKind(); open || k != simpleGroup {
		t.Errorf("curGroupKind() = (%v, %v), attendu (simple, false)", k, open)
	}
	e.beginGroupKind(boxGroup)
	if k, open := e.curGroupKind(); !open || k != boxGroup {
		t.Errorf("curGroupKind() = (%v, %v), attendu (box, true)", k, open)
	}
}
