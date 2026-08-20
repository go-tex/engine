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
// distribution: when a document asks for a class the engine cannot resolve, the
// support tree is fetched, verified against a pinned digest and cached, then
// handed to the engine through Options.Resolve.
//
// Nothing is fetched speculatively. A document that does not name beamer never
// triggers a download, a machine that already has beamer never does either
// (texmf.Open reads its cache, and the engine's own search path wins regardless),
// and -offline forbids it outright.

// classRe finds the document's class name. It is deliberately shallow: this is a
// hint used to decide whether to fetch, never the engine's own parsing, which
// runs afterwards and decides for itself what to load.
var classRe = regexp.MustCompile(`(?m)^[^%\n]*\\documentclass\s*(?:\[[^\]]*\])?\s*\{([^}]*)\}`)

// bundleFor returns the catalogue bundle a source needs, if any.
func bundleFor(src []byte) (texmf.Bundle, bool) {
	m := classRe.FindSubmatch(src)
	if m == nil {
		return texmf.Bundle{}, false
	}
	return texmf.Lookup(string(m[1]))
}

// attachTeXMF fills opt.Resolve when the document needs a support tree this
// machine does not have. A failure is reported and swallowed: the engine still
// has its own emulation, and a talk rendered by the emulation beats a talk not
// rendered at all.
func attachTeXMF(opt *engine.Options, src []byte, offline bool, stderr io.Writer) {
	b, ok := bundleFor(src)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	tree, err := texmf.Open(ctx, b, texmf.Options{
		Offline: offline,
		Log:     func(s string) { fmt.Fprintln(stderr, "gotex: "+s) },
	})
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %s@%s indisponible (%v) — rendu avec l'émulation intégrée\n",
			b.Name, b.Version, err)
		return
	}
	opt.Resolve = tree.Resolve
}
