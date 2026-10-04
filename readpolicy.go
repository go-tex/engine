// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
)

// A .tex is a program and this package is its interpreter, run over UNTRUSTED
// input: the fidelity work compiles arXiv sources in bulk and the playground
// compiles whatever a visitor pastes. A document must therefore not be able to
// read a file the person running it did not offer.
//
// It could. Measured on the engine before this file existed, each with a positive
// control in the same run so "nothing appeared" could not be mistaken for
// "refused":
//
//	\input{/tmp/probe/data}                    the file's text reached the PAGE
//	\input{../../../../../../tmp/probe/data}   the same
//	\bibliography{/tmp/probe/real}             its fields reached the page
//	\includegraphics{/etc/hosts}               the file was OPENED
//
// tectonic halts on the first of those and writes no PDF at all: TeX Live ships
// openin_any=p, which refuses a read outside the document tree. This is that
// policy (go-tex/engine#551).
//
// WHAT COUNTS AS INSIDE is not a new notion — it is the search path the engine
// already looks in, texInputDirs(): the working directory (the document's, since
// the CLI compiles in place) plus every TEXINPUTS and GOTEX_TEXMF entry. A read
// that resolves under one of those is a read of something the caller put there.
//
// It refuses only paths that are ALREADY DEAD in practice. Across the 154-paper
// fidelity corpus there are 3 absolute paths and 5 containing "..", over 5
// papers, and every one is a leftover from its author's own machine —
// /Users/D.Brueckner/Zotero/my_library, /figures/figure1.eps,
// ../usr/home/smith/myfiles/macros.tex. None resolves anywhere else, so none of
// them reads today either.
//
// ⛔ Dotfiles are NOT refused, although paranoid mode refuses those too. Inside
// the document's own tree a dotfile is the document's own business, and outside
// it the root check already refuses the path; the extra rule would be a
// restriction with nothing measured behind it.
//
// ⛔⛔ THE GATE COVERS ONLY THE READS THAT SPLICE CONTENT INTO THE DOCUMENT:
// \input / \include (io.go), \usepackage and \documentclass (packages.go), and
// \bibliography (bibtex.go). Those three put the file's text on the page, which
// is the disclosure.
//
// \font and \includegraphics are deliberately NOT gated. Each DECODES its file —
// as OpenType, as an image — so a file that is neither produces an error and no
// content, which is what the measurement showed: \includegraphics{/etc/hosts}
// reported "unreadable or unsupported format" and put nothing on the page.
// Gating them has a real cost and no measured benefit: the engine loads SYSTEM
// fonts by absolute path (/System/Library/Fonts/…), and a first version of this
// file broke \href's font and four figure tests for nothing.
//
// What they still leak is a path's EXISTENCE, through the shape of the error.
// That is a weak oracle and it is the price of letting a document name a font
// outside the tree, which is a thing real documents do.
//
// ⛔ A refusal is NOT reported through a channel of its own. It arrives at every
// caller as a path error, which each one already handles — "input file not
// found", "unreadable or unsupported format", the FilesMissing tally — so the
// person running the batch sees it where they already look. An API for it would
// be surface added on speculation.

// readAnyOptIn restores the old, unrestricted behaviour for the one legitimate
// case: a macro tree deliberately kept outside the document. It is a NAMED
// opt-out rather than a silent fallback, so a batch that needs it says so.
func readAnyOptIn() bool { return os.Getenv("GOTEX_READ_ANY") == "1" }

// readAllowed reports whether path may be read. It takes no engine state because
// texInputDirs() takes none either — the roots come from the environment and the
// working directory.
func readAllowed(path string) bool {
	if readAnyOptIn() {
		return true
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	abs = filepath.Clean(abs)
	for _, d := range texInputDirs() {
		root, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		root = filepath.Clean(root)
		if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// readDocFile reads a file a DOCUMENT named, refusing one outside the search
// roots. It is the single gate for the five content-splicing read sites, so that
// adding a sixth is a one-line decision at the call rather than a policy copied
// by hand. It is NOT a gate on the package's every read: \font and
// \includegraphics call os.ReadFile directly, on purpose, for the reason above.
//
// The error is a path error, the same shape a missing file gives: a document has
// no business learning whether a path it may not read exists.
func readDocFile(path string) ([]byte, error) {
	if !readAllowed(path) {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrPermission}
	}
	return os.ReadFile(path)
}
