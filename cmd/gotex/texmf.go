// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/go-tex/engine"
	"github.com/go-tex/texmf"
)

// This file is what makes `gotex talk.tex` work on a machine with no TeX
// distribution: when a document asks for a class or a package the engine cannot
// resolve, the support tree is fetched, verified against a pinned digest and
// cached, then handed to the engine through Options.Resolve.
//
// Nothing is fetched speculatively. A document that names none of the catalogue's
// bundles never triggers a download, a machine that already has them never does
// either (texmf.Open reads its cache, and the engine's own search path wins
// regardless), and -offline forbids it outright.

// classRe and packageRe find the names a document asks for. They are deliberately
// shallow: a hint used to decide whether to fetch, never the engine's own
// parsing, which runs afterwards and decides for itself what to load. Reading
// \usepackage as well as \documentclass is what reaches pgf and pgfplots, which
// no document names as its class.
var (
	classRe   = regexp.MustCompile(`\\documentclass\s*(?:\[[^\]]*\])?\s*\{([^}]*)\}`)
	packageRe = regexp.MustCompile(`\\(?:usepackage|RequirePackage)\s*(?:\[[^\]]*\])?\s*\{([^}]*)\}`)
	commaRe   = regexp.MustCompile(`\s*,\s*`)
)

// bundlesFor returns the catalogue bundles a source needs, dependencies first
// and without repeats. One \usepackage can name several packages at once, and a
// bundle can require another — pgfplots does not load without pgf — so this is
// a list rather than the single bundle a class used to imply.
func bundlesFor(src []byte) []texmf.Bundle {
	src = stripComments(src)
	var names []string
	if m := classRe.FindSubmatch(src); m != nil {
		names = append(names, string(m[1]))
	}
	for _, m := range packageRe.FindAllSubmatch(src, -1) {
		names = append(names, commaRe.Split(string(m[1]), -1)...)
	}
	var out []texmf.Bundle
	seen := map[string]bool{}
	for _, n := range names {
		b, ok := texmf.Lookup(trimSpace(n))
		if !ok {
			continue
		}
		for _, d := range texmf.WithDependencies(b) {
			if seen[d.Name] {
				continue
			}
			seen[d.Name] = true
			out = append(out, d)
		}
	}
	return out
}

// trimSpace trims ASCII whitespace, which is all a package list can hold.
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

// attachTeXMF fills opt.Resolve when the document needs support trees this
// machine does not have. A failure on one bundle is reported and swallowed: the
// engine still has its own emulation and whatever other trees did arrive, and a
// talk rendered by the emulation beats a talk not rendered at all.
func attachTeXMF(opt *engine.Options, src []byte, offline bool, stderr io.Writer) {
	bundles := bundlesFor(src)
	if len(bundles) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var resolvers []func(string) ([]byte, bool)
	for _, b := range bundles {
		tree, err := texmf.Open(ctx, b, texmf.Options{
			Offline: offline,
			Log:     func(s string) { fmt.Fprintln(stderr, "gotex: "+s) },
		})
		if err != nil {
			fmt.Fprintf(stderr, "gotex: %s@%s indisponible (%v) — rendu sans lui\n",
				b.Name, b.Version, err)
			continue
		}
		resolvers = append(resolvers, tree.Resolve)
	}
	if len(resolvers) == 0 {
		return
	}
	opt.Resolve = resolverOverFuncs(resolvers)
}

// resolverOverFuncs asks each tree in turn. The order is the one bundlesFor
// produced, so a dependency answers before the package that required it — the
// order a TDS search path would have too, and it matters because a package
// sometimes ships a copy of a file its dependency also ships.
func resolverOverFuncs(trees []func(string) ([]byte, bool)) func(string) ([]byte, bool) {
	return func(name string) ([]byte, bool) {
		for _, r := range trees {
			if data, ok := r(name); ok {
				return data, true
			}
		}
		return nil, false
	}
}

// stripComments blanks out what TeX would not read: everything from an unescaped
// % to the end of its line. Doing it before matching is what lets the patterns
// above go unanchored, and unanchored is what finds a second \usepackage on the
// same line — a generated preamble often puts them all on one.
func stripComments(src []byte) []byte {
	out := make([]byte, len(src))
	copy(out, src)
	for i := 0; i < len(out); i++ {
		if out[i] != '%' || (i > 0 && out[i-1] == '\\') {
			continue
		}
		for ; i < len(out) && out[i] != '\n'; i++ {
			out[i] = ' '
		}
	}
	return out
}
