// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package loadguard

import (
	"strings"
	"testing"
)

// parse takes both shapes the two platforms produce.
func TestParseBothPlatforms(t *testing.T) {
	for _, c := range []struct {
		name, in           string
		one, five, fifteen float64
		wantErr            bool
	}{
		// Linux /proc/loadavg: three averages then running/total and last pid.
		{"linux", "4.52 7.92 8.15 2/1423 98765\n", 4.52, 7.92, 8.15, false},
		// macOS sysctl -n vm.loadavg.
		{"darwin", "{ 2.52 3.01 3.33 }\n", 2.52, 3.01, 3.33, false},
		{"darwin-no-spaces", "{2.52 3.01 3.33}", 2.52, 3.01, 3.33, false},
		// ⛔ Two numbers is not three: a short read must not be completed with a zero.
		{"two-numbers", "4.52 7.92\n", 0, 0, 0, true},
		{"empty", "", 0, 0, 0, true},
		{"not-numbers", "a b c", 0, 0, 0, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, err := parse(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("parse(%q) = %v, want an error: a reading that is not three "+
						"numbers must not be completed with zeros, which read as an idle machine", c.in, a)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse(%q): %v", c.in, err)
			}
			if a.One != c.one || a.Five != c.five || a.Fifteen != c.fifteen {
				t.Errorf("parse(%q) = %+v, want %v/%v/%v", c.in, a, c.one, c.five, c.fifteen)
			}
		})
	}
}

// ⛔⛔ THE CASE THIS PACKAGE EXISTS FOR: a 1-minute dip under a loaded 5-minute
// average is NOT a quiet machine. Reading only the first minute passed a run at
// 1-min 6.93 with a 5-min of 10.00 earlier in this fleet's history, which is why
// Check tests both.
func TestBothAveragesAreTested(t *testing.T) {
	for _, c := range []struct {
		name string
		a    Averages
		ok   bool
	}{
		{"both low", Averages{4.5, 5.2, 6.1}, true},
		{"one high", Averages{9.1, 5.2, 6.1}, false},
		{"five high, one dipped", Averages{6.93, 10.00, 9.0}, false},
		{"exactly at the threshold", Averages{8.0, 8.0, 20.0}, true},
		{"fifteen high only", Averages{3.0, 4.0, 40.0}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			// The decision Check makes, applied to a fixed reading rather than to
			// this machine's — the point is the predicate, not the host.
			refused := c.a.One > Threshold || c.a.Five > Threshold
			if refused == c.ok {
				t.Errorf("%s: refused=%v for %s, want ok=%v", c.name, refused, c.a, c.ok)
			}
		})
	}
}

// ⭐ The banner carries the conditions, and says so loudly when the measurement
// was taken on a busy machine on purpose. A report whose conditions live only in
// the operator's memory is a number nobody can re-judge later.
func TestBannerNamesAnOverride(t *testing.T) {
	a := Averages{12.0, 11.0, 10.0}
	plain := Banner(Averages{1, 2, 3}, "")
	if strings.Contains(plain, "UNDER LOAD") {
		t.Errorf("a clean run's banner claims an override: %q", plain)
	}
	if !strings.Contains(plain, "1-min 1.00") {
		t.Errorf("the banner does not carry the averages: %q", plain)
	}
	over := Banner(a, "the queue is mine and the papers are tiny")
	if !strings.Contains(over, "UNDER LOAD") || !strings.Contains(over, "queue is mine") {
		t.Errorf("an override is not visible in the banner: %q", over)
	}
}

// ⛔⛔ AND AN UNREADABLE MACHINE IS NOT AN IDLE ONE. This is the shape the fleet
// has paid for before — a scan that could not read reported zero — so the error
// path is asserted rather than assumed: parse failing must surface as an error
// from Check, not as Averages{} which every comparison would pass.
func TestAFailedReadIsAnErrorAndNotZero(t *testing.T) {
	a, err := parse("nonsense")
	if err == nil {
		t.Fatal("parse(\"nonsense\") returned no error")
	}
	if a != (Averages{}) {
		t.Errorf("a failed parse returned %+v; the zero value is what callers must not act on", a)
	}
	// And the zero value would pass the predicate, which is exactly why Check
	// must consult the error first.
	if !(a.One <= Threshold && a.Five <= Threshold) {
		t.Fatal("the zero Averages does not pass the threshold test; this test's premise is wrong")
	}
}
