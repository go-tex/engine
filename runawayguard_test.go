// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"testing"
	"time"
)

// The no-progress ceiling has two jobs and both are load-bearing: stop a genuine
// loop, and let a heavy but terminating package through. Raising it for pgfplots
// is only defensible while the first still holds.
func TestRunawayGuardStopsARealLoop(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"macro qui s'appelle elle-même", `\def\zx{\zx}\zx`},
		{"deux macros qui se renvoient", `\def\za{\zb}\def\zb{\za}\za`},
		{"boucle par \\csname", `\def\za{\csname za\endcsname}\za`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.SetFont(spMock{})
			start := time.Now()
			e.Run(c.src)
			if !e.runaway {
				t.Fatalf("%s : la boucle n'a pas été arrêtée", c.src)
			}
			// Generous on purpose: what is under test is that the guard fires at
			// all, and -race slows the engine by about an order of magnitude, so
			// a tight bound here would fail for a reason unrelated to the guard.
			// Without -race these abort in under two seconds.
			if d := time.Since(start); d > 60*time.Second {
				t.Errorf("%s : arrêtée en %v, ce qui est trop long", c.src, d)
			}
		})
	}
}

// The other half: expansion that never touches base input again, but terminates,
// must get through. This is pgfplots' shape — it spends three to four million
// steps inside token lists before reading further — at a scale the old ceiling
// of two million would have cut off.
func TestRunawayGuardLetsHeavyExpansionThrough(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	const rounds = 1500000
	src := `\count0=0 \def\zloop{\advance\count0 by1 ` +
		`\ifnum\count0<1500000 \expandafter\zloop\fi}\zloop`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("une expansion lourde mais finie a échoué : %v", err)
	}
	if e.runaway {
		t.Error("une expansion lourde mais finie a déclenché le garde-fou")
	}
	if e.count[0] != rounds {
		t.Errorf("la boucle s'est arrêtée à %d au lieu de %d", e.count[0], rounds)
	}
}

// A document that keeps reading its own source never approaches the ceiling,
// however long it is: consuming base input resets the counter.
func TestLongDocumentDoesNotTripTheGuard(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	var sb []byte
	for i := 0; i < 20000; i++ {
		sb = append(sb, "mot "...)
	}
	if _, err := e.Run(string(sb)); err != nil {
		t.Fatal(err)
	}
	if e.runaway {
		t.Error("un document long mais ordinaire a déclenché le garde-fou")
	}
}
