// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
)

// A .tex is a program and this is its interpreter, so a document names the files it
// reads. \input{/etc/hostname} typesets that file, ../ walks out of the directory the
// document lives in, and \openin/\read brings the contents back as TOKENS the document
// can branch on. That is ordinary TeX — the reference engine does the same, and tectonic
// reads an absolute path with nothing but a warning — but TeX Live restricts it by
// configuration (openin_any = p forbids absolute paths, .. and dotfiles) and this engine
// has no equivalent knob. See go-tex/engine#553 for the audit.
//
// Confining reads would change what existing callers get, which is a decision about the
// API rather than a bug to fix. REPORTING them changes nothing and lets a host decide:
// a service compiling third-party uploads can look at Diagnostics.ReadsOutsideTree and
// refuse the result, while a CLI run on your own documents carries on as before.
//
// The tree is the working directory the engine started in, captured once: the CLI changes
// into the document's own directory, so that is the document's tree, and a later \chdir by
// anything else cannot move the baseline under the measurement.
func (e *Engine) noteRead(path string) {
	if path == "" {
		return
	}
	root := e.treeRoot()
	if root == "" {
		return // no baseline to judge against; recording every read would say nothing
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	// ⛔ The tree is the working directory AND the search directories the HOST configured
	// through TEXINPUTS/GOTEX_TEXMF. Judging against the working directory alone reported
	// /…/texmf/size10.clo — a class file the host pointed the engine at — and a signal that
	// fires on the host's own configuration is one a host learns to ignore, which is worse
	// than no signal. What this records is the DOCUMENT reaching outside what it was given.
	// (texInputDirs is a package function since readpolicy.go: the roots come from the
	// environment and the working directory, so it needs no engine — and the refusal
	// and this report then judge against ONE definition of "inside".)
	for _, d := range texInputDirs() {
		if ad, err := filepath.Abs(d); err == nil && pathWithin(ad, abs) {
			return
		}
	}
	if !pathWithin(root, abs) {
		if e.readsOutside == nil {
			e.readsOutside = map[string]int{}
		}
		e.readsOutside[filepath.Clean(path)]++
	}
}

// pathWithin reports whether abs is inside root (root itself counts).
func pathWithin(root, abs string) bool {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// treeRoot is the working directory as it was when the engine first needed it.
func (e *Engine) treeRoot() string {
	if e.treeRootOnce == "" {
		if wd, err := os.Getwd(); err == nil {
			e.treeRootOnce = wd
		} else {
			e.treeRootOnce = "\x00" // remember the failure so Getwd is not retried per read
		}
	}
	if e.treeRootOnce == "\x00" {
		return ""
	}
	return e.treeRootOnce
}
