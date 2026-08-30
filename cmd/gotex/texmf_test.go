// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-tex/engine"
)

// names is the bundle list as names, which is what the tests below assert on.
func names(src string) []string {
	var out []string
	for _, b := range bundlesFor([]byte(src)) {
		out = append(out, b.Name)
	}
	return out
}

func eq(a []string, b ...string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Deciding WHETHER to fetch is the whole risk here: a document that does not ask
// for a support tree must never reach the network, and one that does must be
// recognised through the spellings LaTeX actually accepts.
func TestBundlesForRecognisesTheClass(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      []string
	}{
		{"nu", `\documentclass{beamer}`, []string{"translator", "beamer"}},
		{"avec options", `\documentclass[handout,11pt]{beamer}`, []string{"translator", "beamer"}},
		{"espaces", "\\documentclass  [t] \t{beamer}", []string{"translator", "beamer"}},
		{"indenté", "  \\documentclass{beamer}", []string{"translator", "beamer"}},
		{"autre classe", `\documentclass{article}`, nil},
		{"aucune classe", `\input{quelquechose}`, nil},
		{"commenté", "% \\documentclass{beamer}\n", nil},
		{"commenté en fin de ligne", "\\message{a} % \\documentclass{beamer}\n", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := names(c.src); !eq(got, c.want...) {
				t.Errorf("bundlesFor(%q) = %v, attendu %v", c.src, got, c.want)
			}
		})
	}
}

// Reading \usepackage as well as \documentclass is what reaches pgf and
// pgfplots: no document names either as its class.
func TestBundlesForRecognisesPackages(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      []string
	}{
		{"tikz tire pgf", `\documentclass{article}\usepackage{tikz}`, []string{"pgf"}},
		{"pgf", `\documentclass{article}\usepackage{pgf}`, []string{"pgf"}},
		{"pgfplots tire pgf", `\documentclass{article}\usepackage{pgfplots}`, []string{"pgf", "pgfplots"}},
		{"RequirePackage", `\RequirePackage{pgf}`, []string{"pgf"}},
		{"avec options", `\usepackage[compat=1.18]{pgfplots}`, []string{"pgf", "pgfplots"}},
		{"liste séparée par des virgules", `\usepackage{amsmath, pgf ,xcolor}`, []string{"pgf"}},
		{"les deux, sans répétition", `\documentclass{beamer}\usepackage{pgfplots}\usepackage{pgf}`,
			[]string{"translator", "beamer", "pgf", "pgfplots"}},
		{"commenté", "% \\usepackage{pgfplots}\n", nil},
		{"paquet inconnu", `\usepackage{fancyhdr}`, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := names(c.src); !eq(got, c.want...) {
				t.Errorf("bundlesFor(%q) = %v, attendu %v", c.src, got, c.want)
			}
		})
	}
}

// A dependency answers before the package that required it, which is the order a
// TDS search path would have.
func TestDependencyComesFirst(t *testing.T) {
	got := names(`\usepackage{pgfplots}`)
	if len(got) != 2 || got[0] != "pgf" {
		t.Errorf("obtenu %v, attendu pgf en premier", got)
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
	if msg := errb.String(); !strings.Contains(msg, "indisponible") {
		t.Errorf("le message n'explique pas le repli: %q", msg)
	}
}

// One bundle failing must not lose the others: a document that asks for two
// trees and gets one renders better than one that gets none.
func TestResolverAsksEveryTreeInOrder(t *testing.T) {
	first := func(name string) ([]byte, bool) {
		if name == "a.tex" {
			return []byte("depuis le premier"), true
		}
		return nil, false
	}
	second := func(name string) ([]byte, bool) {
		if name == "a.tex" {
			return []byte("depuis le second"), true
		}
		if name == "b.tex" {
			return []byte("b"), true
		}
		return nil, false
	}
	r := resolverOverFuncs([]func(string) ([]byte, bool){first, second})
	if got, ok := r("a.tex"); !ok || string(got) != "depuis le premier" {
		t.Errorf("a.tex résolu en %q (%v), attendu le premier arbre", got, ok)
	}
	if got, ok := r("b.tex"); !ok || string(got) != "b" {
		t.Errorf("b.tex résolu en %q (%v), attendu le second arbre", got, ok)
	}
	if _, ok := r("absent.tex"); ok {
		t.Error("un nom absent des deux arbres ne devrait pas se résoudre")
	}
}

// trimSpace handles what a package list can hold around a name.
func TestTrimSpace(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"pgf", "pgf"},
		{"  pgf  ", "pgf"},
		{"\tpgf\n", "pgf"},
		{"\r\npgf\r\n", "pgf"},
		{"   ", ""},
		{"", ""},
	} {
		if got := trimSpace(c.in); got != c.want {
			t.Errorf("trimSpace(%q) = %q, attendu %q", c.in, got, c.want)
		}
	}
}

// Comments are blanked before matching, which is what lets the patterns go
// unanchored — and unanchored is what finds a second \usepackage on the same
// line. Anchored to the line start, a greedy prefix swallowed everything up to
// the LAST one and the earlier packages were never seen.
func TestSeveralPackagesOnOneLine(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      []string
	}{
		{"deux sur une ligne", `\usepackage{pgf}\usepackage{beamer}`, []string{"pgf", "translator", "beamer"}},
		{"trois, dont un inconnu", `\usepackage{amsmath}\usepackage{pgfplots}\usepackage{pgf}`,
			[]string{"pgf", "pgfplots"}},
		{"un commenté au milieu", `\usepackage{pgf}% \usepackage{beamer}`, []string{"pgf"}},
		{"pourcent échappé, pas un commentaire", `\def\x{100\%}\usepackage{pgf}`, []string{"pgf"}},
		{"commentaire sur une ligne, paquet sur la suivante", "% rien\n\\usepackage{pgf}", []string{"pgf"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := names(c.src); !eq(got, c.want...) {
				t.Errorf("bundlesFor(%q) = %v, attendu %v", c.src, got, c.want)
			}
		})
	}
}

// stripComments leaves the text's length alone so the patterns still see the
// same offsets, and stops at the end of the line rather than the file.
func TestStripComments(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a % b\nc", "a    \nc"},
		{"a \\% b", "a \\% b"},
		{"% tout", "      "},
		{"sans commentaire", "sans commentaire"},
	} {
		if got := string(stripComments([]byte(c.in))); got != c.want {
			t.Errorf("stripComments(%q) = %q, attendu %q", c.in, got, c.want)
		}
	}
}

// tikz is the name nearly every document uses to ask for pgf, and no archive is
// called tikz. texmf.Lookup answers it through the bundle's Provides list, which
// is what makes \usepackage{tikz} reach anything at all.
func TestTikzReachesPGF(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      []string
	}{
		{"tikz seul", `\usepackage{tikz}`, []string{"pgf"}},
		{"tikz et pgfplots, sans doublon", `\usepackage{tikz}\usepackage{pgfplots}`, []string{"pgf", "pgfplots"}},
		{"pgfkeys", `\usepackage{pgfkeys}`, []string{"pgf"}},
		{"pgfplotstable", `\usepackage{pgfplotstable}`, []string{"pgf", "pgfplots"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := names(c.src); !eq(got, c.want...) {
				t.Errorf("bundlesFor(%q) = %v, attendu %v", c.src, got, c.want)
			}
		})
	}
}
