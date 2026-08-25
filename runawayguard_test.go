// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"testing"
	"time"
)

// The no-progress ceiling has two jobs and both are load-bearing: stop a genuine
// loop, and let a heavy but terminating expansion through when the caller has
// asked for the headroom.
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
			// all, and both -race and a slow runner stretch this by an order of
			// magnitude. Without -race these abort in about two seconds.
			if d := time.Since(start); d > 60*time.Second {
				t.Errorf("%s : arrêtée en %v, ce qui est trop long", c.src, d)
			}
		})
	}
}

// The same expansion, stopped by default and let through with the headroom. That
// is the whole point of Options.NoProgressLimit: a caller rendering documents
// wants the ceiling generous — pgfplots needs three to four million steps inside
// token lists before it reads further — and a caller exercising the guard wants
// it tight. Neither default serves both.
//
// 750000 rounds is just over what the default allows (it stops this loop at
// 666667) and no more: the test has to discriminate, and every round costs time
// on a slow runner.
func TestNoProgressLimitIsWhatMakesTheDifference(t *testing.T) {
	const src = `\count0=0 \def\zloop{\advance\count0 by1 ` +
		`\ifnum\count0<750000 \expandafter\zloop\fi}\zloop`
	for _, c := range []struct {
		name  string
		limit int
		fires bool
	}{
		{"par défaut", 0, true},
		{"avec la marge", NoProgressLimitHeavy, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := buildEngine(Options{NoProgressLimit: c.limit}, false)
			if err != nil {
				t.Fatal(err)
			}
			e.SetFont(spMock{})
			// In strict mode a runaway IS the error, so only the case that must
			// get through is held to returning none.
			if _, err := e.Run(src); err != nil && !c.fires {
				t.Fatal(err)
			}
			if e.runaway != c.fires {
				t.Errorf("garde-fou déclenché = %v, attendu %v (le compteur s'est arrêté à %d)",
					e.runaway, c.fires, e.count[0])
			}
			if !c.fires && e.count[0] != 750000 {
				t.Errorf("avec la marge, la boucle s'est arrêtée à %d au lieu de 750000", e.count[0])
			}
		})
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
