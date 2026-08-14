// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// TestSkipSweep_GobbleArgs checks the engine-level definitions that gobble a
// required group (and any optional [.]) do not leak their body: an undefined
// marker command placed inside the gobbled group must never be executed, and the
// command itself must not be reported skipped, while text after it still runs.
func TestSkipSweep_GobbleArgs(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"newcolumntype-opt", `\newcolumntype{C}[1]{\zzbody} \zzafter`},
		{"newcolumntype-plain", `\newcolumntype{L}{\zzbody} \zzafter`},
		{"footnotetext-plain", `\footnotetext{\zzbody} \zzafter`},
		{"footnotetext-opt", `\footnotetext[3]{\zzbody} \zzafter`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			skip := runSkip(t, c.body)
			if skip["zzbody"] != 0 {
				t.Errorf("gobbled body leaked: \\zzbody executed (%d)", skip["zzbody"])
			}
			for _, cmd := range []string{"newcolumntype", "footnotetext"} {
				if skip[cmd] != 0 {
					t.Errorf("%s reported skipped (should be defined): %d", cmd, skip[cmd])
				}
			}
			if skip["zzafter"] != 1 {
				t.Errorf("text after the command not processed: zzafter=%d (want 1)", skip["zzafter"])
			}
		})
	}
}

// TestSkipSweep_NoOps checks the switch-like commands are accepted as no-ops (not
// skipped), including the break hints that gobble an optional [priority], while
// the surrounding text keeps flowing.
func TestSkipSweep_NoOps(t *testing.T) {
	cases := map[string]string{
		"qedhere":       `\qedhere \zzafter`,
		"boldmath":      `\boldmath \unboldmath \zzafter`,
		"spacing":       `\frenchspacing \nonfrenchspacing \zzafter`,
		"break-plain":   `\pagebreak \nopagebreak \linebreak \nolinebreak \zzafter`,
		"break-opt":     `\pagebreak[2] \nopagebreak[1] \linebreak[3] \nolinebreak[4] \zzafter`,
		"boldmath-text": `A \boldmath B \unboldmath C \zzafter`,
	}
	commands := []string{
		"qedhere", "boldmath", "unboldmath", "frenchspacing", "nonfrenchspacing",
		"pagebreak", "nopagebreak", "linebreak", "nolinebreak",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			skip := runSkip(t, body)
			for _, cmd := range commands {
				if skip[cmd] != 0 {
					t.Errorf("%s reported skipped (should be a defined no-op): %d", cmd, skip[cmd])
				}
			}
			if skip["zzafter"] != 1 {
				t.Errorf("text after the no-ops not processed: zzafter=%d (want 1)", skip["zzafter"])
			}
		})
	}
}

// TestSkipSweep_AlreadyHandledBoxes guards that the box transforms delegated to Go
// (\scalebox, \rotatebox, \raisebox) still render their content, i.e. neither the
// factor nor the content is dropped.
func TestSkipSweep_AlreadyHandledBoxes(t *testing.T) {
	for _, body := range []string{
		`\scalebox{2}{\zzc}`, `\rotatebox{90}{\zzc}`, `\raisebox{2pt}{\zzc}`, `\reflectbox{\zzc}`,
	} {
		skip := runSkip(t, body)
		// The content is rendered, so the undefined \zzc IS seen (skipped) — proving
		// the box did not swallow its content the way a gobble would.
		if skip["zzc"] != 1 {
			t.Errorf("%s: content not rendered (zzc=%d, want 1)", body, skip["zzc"])
		}
		for _, cmd := range []string{"scalebox", "rotatebox", "raisebox", "reflectbox"} {
			if skip[cmd] != 0 {
				t.Errorf("%s: %s reported skipped", body, cmd)
			}
		}
	}
}
