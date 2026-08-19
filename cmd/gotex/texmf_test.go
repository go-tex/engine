// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-tex/engine"
)

// Deciding WHETHER to fetch is the whole risk here: a document that does not ask
// for a support tree must never reach the network, and one that does must be
// recognised through the spellings LaTeX actually accepts.
func TestBundleForRecognisesTheClass(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      string
	}{
		{"nu", `\documentclass{beamer}`, "beamer"},
		{"avec options", `\documentclass[handout,11pt]{beamer}`, "beamer"},
		{"espaces", "\\documentclass  [t] \t{beamer}", "beamer"},
		{"indenté", "  \\documentclass{beamer}", "beamer"},
		{"précédé de texte", "\\RequirePackage{x}\\documentclass{beamer}", "beamer"},
		{"autre classe", `\documentclass{article}`, ""},
		{"aucune classe", `\input{quelquechose}`, ""},
		{"commenté", "% \\documentclass{beamer}\n", ""},
		{"commenté en fin de ligne", "\\message{a} % \\documentclass{beamer}\n", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, ok := bundleFor([]byte(c.src))
			if c.want == "" {
				if ok {
					t.Errorf("bundleFor(%q) = %s, attendu aucun", c.src, b.Name)
				}
				return
			}
			if !ok || b.Name != c.want {
				t.Errorf("bundleFor(%q) = (%v, %v), attendu %s", c.src, b.Name, ok, c.want)
			}
		})
	}
}

// A document naming no known bundle leaves Options.Resolve nil and says nothing:
// no network, no noise.
func TestAttachIsSilentWithoutABundle(t *testing.T) {
	var opt engine.Options
	var errb bytes.Buffer
	attachTeXMF(&opt, []byte(`\documentclass{article}\begin{document}A\end{document}`), false, &errb)
	if opt.Resolve != nil {
		t.Error("Resolve devrait rester nil")
	}
	if errb.Len() != 0 {
		t.Errorf("sortie inattendue: %q", errb.String())
	}
}

// Offline with nothing cached must not fail the compile: the engine still has
// its emulation, and a talk rendered by the emulation beats a talk not rendered.
func TestAttachOfflineReportsAndCarriesOn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())           // a cache directory of its own…
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // …on both conventions
	var opt engine.Options
	var errb bytes.Buffer
	attachTeXMF(&opt, []byte(`\documentclass{beamer}`), true, &errb)
	if opt.Resolve != nil {
		t.Error("hors ligne sans cache: Resolve devrait rester nil")
	}
	msg := errb.String()
	if !strings.Contains(msg, "indisponible") || !strings.Contains(msg, "émulation") {
		t.Errorf("le message n'explique pas le repli: %q", msg)
	}
}
