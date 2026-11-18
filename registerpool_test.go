// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strconv"
	"strings"
	"testing"
)

// go-tex/engine#788's other half: the pools are eTeX's 32768, not TeX82's 256.
//
// ⛔⛔ THE OLD CEILING WAS A CRASH, NOT A CEILING. Measured at 39da8fe, each of
// these one-line documents took the process down:
//
//	\the\count300  →  panic: index out of range [300] with length 256
//	\the\dimen300  →  panic: index out of range [300] with length 256
//	\the\skip300   →  panic: index out of range [300] with length 256
//
// Legal eTeX documents, reachable from any .tex a host hands this engine. The
// bound lived as the literal 256 at eight write sites and was simply absent from
// the reads — \count300=5 was refused while \the\count300 was not checked at
// all, and \advance\dimen300 read the register before setDimen could refuse the
// write. Boxes, which had accessors, never panicked. That asymmetry is why the
// bound now lives in one place (registers.go).
func TestARegisterPastTeX82sCeilingIsAnOrdinaryRegister(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want string
	}{
		// ⭐ THESE FOUR ARE THE CONTROL FOR THE WHOLE CHANGE, and they are the
		// reason the rows below are not enough on their own. A bound that simply
		// answered zero for every high index would pass every "does not panic"
		// row here while the pool stayed at 256. Only a VALUE READ BACK says the
		// register exists, so each writes a distinctive value and reads it.
		{"count 300", `\count300=7 \message{\the\count300}`, "7"},
		{"count 32767", `\count32767=9 \message{\the\count32767}`, "9"},
		{"dimen 300", `\dimen300=5pt \message{\the\dimen300}`, "5.0pt"},
		{"skip 300", `\skip300=5pt \message{\the\skip300}`, "5.0pt"},

		// A register inside the pool that nothing has written reads as its zero
		// value, exactly as \count5 does. This is the row that distinguishes
		// "grown on write" from "absent".
		{"count 300 unwritten", `\message{\the\count300}`, "0"},
		{"dimen 300 unwritten", `\message{\the\dimen300}`, "0.0pt"},

		// Arithmetic reads the register before it writes it, which is the path
		// that had no guard at all on dimensions and glue.
		{"advance count 300", `\count300=4 \advance\count300 by 3 \message{\the\count300}`, "7"},
		{"advance dimen 300", `\dimen300=4pt \advance\dimen300 by 3pt \message{\the\dimen300}`, "7.0pt"},
		{"multiply skip 300", `\skip300=4pt \multiply\skip300 by 3 \message{\the\skip300}`, "12.0pt"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := runMsg(t, c.src); got != c.want {
				t.Errorf("%s: got %q, want %q", c.src, got, c.want)
			}
		})
	}
}

// \setbox and \box reach past 256 too, and a box is the one kind that had
// accessors already — so this row says the accessors grew with the rest rather
// than keeping their old literal.
func TestABoxRegisterPastTeX82sCeilingHoldsItsBox(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\setbox300=\hbox{A}\message{\the\wd300}`); err != nil {
		t.Fatal(err)
	}
	if e.getBox(300) == nil {
		t.Error("box 300 is void after \\setbox300")
	}
	if got := e.Diagnostics().RegistersOutOfRange; len(got) != 0 {
		t.Errorf("RegistersOutOfRange = %v for register 300, which is inside the pool", got)
	}
}

// An index no pool can hold is refused and REPORTED, where it used to panic.
//
// ⛔ The engine's doctrine is that a recovery with no voice is the defect
// (go-tex/engine#841), so "it no longer crashes" is not the whole fix: TeX
// answers these with "Bad register code" and refuses, and a reader has to be
// able to tell that the refusal happened.
func TestAnImpossibleRegisterIndexIsRefusedAndReported(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		kind string
	}{
		{"count far past the pool", `\count99999=5 X`, `\count`},
		{"count read far past the pool", `\message{\the\count99999}`, `\count`},
		{"dimen read far past the pool", `\message{\the\dimen99999}`, `\dimen`},
		{"skip read far past the pool", `\message{\the\skip99999}`, `\skip`},
		{"count just past the pool", `\count32768=5 X`, `\count`},
		{"negative count", `\count-3=5 X`, `\count`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := buildEngine(Options{Lenient: true}, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := e.Run(c.src); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := e.Diagnostics().RegistersOutOfRange; got[c.kind] == 0 {
				t.Errorf("an impossible %s index was not reported: %v", c.kind, got)
			}
		})
	}
}

// ⛔ AND IT MUST NOT REPORT A REGISTER THAT IS MERELY UNWRITTEN. This is the
// control that separates the two facts the tally could confuse: index 300 is
// legal and empty, index 99999 cannot exist. Without this row, a report fired on
// every read above len() would pass the test above and make `\the\count300` —
// now an ordinary read — look like a malformed document.
func TestAnUnwrittenRegisterInsideThePoolIsNotReported(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\message{\the\count300 \the\dimen4000 \the\skip32767}`); err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().RegistersOutOfRange; len(got) != 0 {
		t.Errorf("RegistersOutOfRange = %v for three unwritten but legal registers, want empty", got)
	}
}

// The pools grow on write, so a document that touches a high register must not
// lose the low ones in the process — the copy is the part a growth scheme gets
// wrong.
// ⛔ IT READS BOTH ENDS BACK, BECAUSE ONE END IS NOT A WITNESS. A first version
// asserted only that the LOW registers survived — and that passed with the pool
// forced back to 256, where the high writes are refused and so cannot disturb
// anything. A test for growth has to show the growth happened.
func TestGrowingAPoolKeepsWhatWasAlreadyInIt(t *testing.T) {
	got := runMsg(t, `\count7=11 \dimen7=3pt \skip7=3pt `+
		`\count9000=22 \dimen9000=1pt \skip9000=1pt `+
		`\message{\the\count7 /\the\dimen7 /\the\skip7 /\the\count9000}`)
	// The space after `\the\count7` is eaten by the register number's own scan,
	// which is TeX's rule and not this change's business.
	if want := "11/3.0pt/3.0pt/22"; got != want {
		t.Errorf("after growing the pools: got %q, want %q", got, want)
	}
}

// A group restores a high register exactly as it restores a low one: the save
// stack holds an index, and growth must not leave an entry pointing at storage
// that moved.
func TestAGroupRestoresARegisterPastTheOldCeiling(t *testing.T) {
	got := runMsg(t, `\count400=1 {\count400=2 }\message{\the\count400}`)
	if got != "1" {
		t.Errorf("after the group closed, \\count400 = %q, want 1", got)
	}
}

// ⛔ THE POOL IS NOT PAID FOR UP FRONT. A dense [32768] of each kind is about
// 1.5 MB per Engine — a glueSpec is five ints — charged to every New() whether
// the document names a high register or not, and this package builds thousands
// of engines per run. A fresh engine must therefore hold TeX82's width and no
// more, which is what makes the raise free for documents that do not use it.
func TestAFreshEngineDoesNotPayForTheWholePool(t *testing.T) {
	e := New()
	for _, c := range []struct {
		name string
		got  int
	}{
		{`\count`, len(e.count)},
		{`\dimen`, len(e.dimen)},
		{`\skip`, len(e.skip)},
		{`\box`, len(e.box)},
	} {
		if c.got != registerBaseSize {
			t.Errorf("a fresh engine holds %d %s registers, want %d: the pool is being allocated up front",
				c.got, c.name, registerBaseSize)
		}
	}
}

// ⛔ AND THE VALUE REACHES THE PAGE, NOT ONLY \message. Every row above reads
// the register back through \the into a \message, which proves the expansion saw
// it; none of them proves a reader would. This one compiles a real document and
// asserts the digits are SET — the assertion the #817 witness taught, where an
// engine can be perfectly correct about a value it then fails to typeset.
func TestARegisterPastTheOldCeilingReachesThePage(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\count300=1234 \dimen300=12.5pt ` +
		`count=\the\count300 dim=\the\dimen300` +
		`\end{document}`); err != nil {
		t.Fatal(err)
	}
	got := pageText(e)
	for _, want := range []string{"count=1234", "dim=12.5pt"} {
		if !strings.Contains(got, want) {
			t.Errorf("the page does not carry %q; it carries %q", want, got)
		}
	}
}

// ⛔⛔ THE SAME UNSTORED REGISTER MUST NOT READ TWO WAYS DEPENDING ON ITS NUMBER.
//
// Growing the pools on write gives `len(e.skip)` a second meaning — "how much of
// the pool has been reached" — and eight guards across the engine were asking it
// as if it still meant "legal". A register bound to a parameter name and never
// assigned is worth its zero value whether the allocator handed out 20 or 4000;
// asked with len, index 20 read 0pt through the array and index 4000 fell out of
// the guard into a DEFAULT. Those are different answers to one question.
//
// ⚠ `topskipSpec`'s caller at topskip.go:50 is `spec, _ :=` — it discards the
// bool — so there "absent" became a silent \topskip=0pt. That file's own comment
// measures the cost: about 3pt on every page, never a whole line, so Σ pages does
// not move and no page-count metric can see it.
//
// Found by the go-tex session while working on \topskip for #834; the grep is
// theirs, these rows and the fix are measured here.
func TestAnUnstoredRegisterReadsTheSameHighAndLow(t *testing.T) {
	// ⛔ 200, NOT 20. The first version of this test used 20 and the namedInt row
	// reported 901 against 0 — because loadTeXParams writes every \count parameter
	// its default at allocation, starting at index 10, so index 20 is an OCCUPIED
	// register and not an unstored one. The test was measuring its own bad premise.
	// 200 is inside registerBaseSize and past everything the format allocates.
	const lowCode, highCode = 200, 4000
	for _, c := range []struct {
		name string
		read func(e *Engine, code int) (string, bool)
	}{
		{"topskipSpec", func(e *Engine, code int) (string, bool) {
			e.eq["topskip"] = &meaning{kind: mSkipRef, code: code}
			g, ok := e.topskipSpec()
			return formatGlue(g), ok
		}},
		{"namedSkip", func(e *Engine, code int) (string, bool) {
			e.eq["abovedisplayskip"] = &meaning{kind: mSkipRef, code: code}
			return formatGlue(e.namedSkip("abovedisplayskip")), true
		}},
		{"namedInt", func(e *Engine, code int) (string, bool) {
			e.eq["lefthyphenmin"] = &meaning{kind: mCountRef, code: code}
			return strconv.Itoa(e.namedInt("lefthyphenmin")), true
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			lowEngine, highEngine := New(), New()
			low, lowOK := c.read(lowEngine, lowCode)
			high, highOK := c.read(highEngine, highCode)
			if low != high || lowOK != highOK {
				t.Errorf("an unstored register reads %q/%v at index %d and %q/%v at index %d — "+
					"the answer depends on the register NUMBER, not on its state",
					low, lowOK, lowCode, high, highOK, highCode)
			}
		})
	}
}

// cmd/gotex prints the refusals; this is the engine-side half of that line, so
// the two cannot drift apart silently.
func TestTheOutOfRangeTallyNamesTheKind(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\count99999=1 \message{\the\dimen99999}`); err != nil {
		t.Fatal(err)
	}
	got := e.Diagnostics().RegistersOutOfRange
	for _, kind := range []string{`\count`, `\dimen`} {
		if got[kind] == 0 {
			t.Errorf("RegistersOutOfRange = %v, missing %s", got, kind)
		}
		if !strings.HasPrefix(kind, `\`) {
			t.Errorf("kind %q is not a primitive name", kind)
		}
	}
}
