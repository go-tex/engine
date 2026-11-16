// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package loadguard reads the machine's load averages and refuses a measurement
// taken on a busy machine.
//
// ⛔ WHY IT IS A PACKAGE AND NOT A LINE IN EACH TOOL. gotex-abdiff and
// gotex-refdiff both produce numbers that go into pull requests, and both are
// worthless above a load their caller has to remember to check. Until this
// existed the check was a human running `uptime` before and after each run and
// pasting the result into the body by hand — which happened a dozen times in one
// day, and one run still went out at a 1-minute average of 8.46 with a caveat
// attached instead of being refused.
//
// ⛔⛔ AND "I COULD NOT READ THE LOAD" IS NOT "THE LOAD IS FINE". Every path that
// cannot answer returns an error and the caller refuses, because a guard that
// passes when blind is worse than no guard: it converts an unmeasurable machine
// into a clean bill of health. This fleet has paid for that shape before — a scan
// that could not read reported zero.
//
// ⚠ Why the measurement cares at all: page counts are deterministic, so load does
// not perturb them directly. It perturbs them THROUGH THE PER-PAPER TIMEOUT —
// gotex-refdiff's own doc says a compile that merely exhausts -timeout "is
// timed-out and leaves the score on EITHER side", so a slow paper, or a loaded
// machine, moves the mean one way only.
package loadguard

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Threshold is the 1-minute load average above which this fleet does not
// measure. It is the number the standing rule names ("never measure at 1-min
// load > ~8"), kept here so both tools cite one value.
const Threshold = 8.0

// Averages are the three load averages, as uptime reports them.
type Averages struct {
	One, Five, Fifteen float64
}

func (a Averages) String() string {
	return fmt.Sprintf("1-min %.2f / 5-min %.2f / 15-min %.2f", a.One, a.Five, a.Fifteen)
}

// Read returns the machine's load averages.
//
// Linux exposes them in /proc/loadavg; macOS does not, so sysctl is asked for
// vm.loadavg. ⛔ A platform where neither works returns an ERROR rather than a
// zero Averages: zero would read as an idle machine.
func Read() (Averages, error) {
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		return parse(string(b))
	}
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return Averages{}, fmt.Errorf("no /proc/loadavg and sysctl vm.loadavg failed: %w", err)
	}
	// sysctl prints "{ 2.52 3.01 3.33 }".
	return parse(strings.Trim(strings.TrimSpace(string(out)), "{} "))
}

// parse takes the first three whitespace-separated floats of s.
func parse(s string) (Averages, error) {
	f := strings.Fields(strings.Trim(strings.TrimSpace(s), "{} "))
	if len(f) < 3 {
		return Averages{}, fmt.Errorf("load averages: want three numbers, got %q", s)
	}
	var v [3]float64
	for i := range v {
		x, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			return Averages{}, fmt.Errorf("load average %d of %q: %w", i+1, s, err)
		}
		v[i] = x
	}
	return Averages{One: v[0], Five: v[1], Fifteen: v[2]}, nil
}

// Check reads the load and reports whether a measurement may proceed.
//
// It returns the averages so the caller can print them WITH its result — the
// conditions belong in the report, not in the operator's memory. When it refuses,
// the error names the reason and the override.
//
// ⚠ Both the 1-minute AND the 5-minute average must be under the threshold. A
// 1-minute dip under a loaded 5-minute average is not a quiet machine, and
// reading only the first minute passed a run at 1-min 6.93 with a 5-min of 10.00
// earlier in this fleet's history.
func Check(override string) (Averages, error) {
	a, err := Read()
	if err != nil {
		if override != "" {
			return a, nil
		}
		return a, fmt.Errorf("refusing to measure: the load averages could not be read (%v). "+
			"An unreadable machine is not an idle one; pass -under-load '<why>' to measure anyway", err)
	}
	if override != "" {
		return a, nil
	}
	if a.One > Threshold || a.Five > Threshold {
		return a, fmt.Errorf("refusing to measure at %s: this fleet does not measure above "+
			"%.0f on the 1-minute or the 5-minute average, because a paper that merely "+
			"exhausts its timeout is scored on one side only. Wait, or pass "+
			"-under-load '<why>' which is stamped into the report", a, Threshold)
	}
	return a, nil
}

// Banner is the line a tool prints above its result so the conditions travel with
// the number. An override is named in it, so a reader can see the measurement was
// taken deliberately on a busy machine.
func Banner(a Averages, override string) string {
	if override != "" {
		return fmt.Sprintf("load %s  ⚠ MEASURED UNDER LOAD ON PURPOSE: %s", a, override)
	}
	return fmt.Sprintf("load %s  (under the %.0f threshold on both the 1- and 5-minute averages)",
		a, Threshold)
}
