// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// This file implements the import package's \import and \subimport, which are how a
// paper split across directories pulls its parts in:
//
//	\import{sections/}{intro}     reads sections/intro.tex
//	\subimport{proofs/}{lemma}    reads <current import path>proofs/lemma.tex
//
// Undefined, they were SKIPPED — and a skipped \import does not lose a command, it
// loses a FILE. One elsarticle paper in the corpus imports seven: it rendered 5
// pages against a reference of 28, drawing 28% of the reference's characters. The
// engine had no way to see that as anything but a missing command.
//
// The path is kept on a stack so a file pulled in by \import can itself \subimport
// relative to where it lives; the pop is appended to the file's own text, so it runs
// exactly when that file ends, whatever it does in between. The starred forms
// (\import*, \subimport*) differ only in how the graphics path is treated, which
// this engine does not model, so the star is read and ignored.

// doImport implements \import{dir}{file} and \subimport{dir}{file}. sub selects the
// \subimport form, whose directory is relative to the importing file's own.
func (e *Engine) doImport(sub bool) {
	if t, ok := e.getNext(); ok { // the starred forms differ only in graphics paths
		if t.cs_ || t.ch != '*' {
			e.back(t)
		}
	}
	dir := e.readBraceNameX()
	file := e.readBraceNameX()
	if file == "" {
		return
	}
	path := withTrailingSlash(dir)
	if sub {
		path = e.importPath + path
	}
	data, err := e.readInput(path + file)
	if err != nil {
		// ⛔ \import RESETS \import@path, but import.sty keeps the enclosing directories
		// in the SEARCH path at the same time:
		//
		//\tprotected@edef\input@path{{\import@path}#2}   % #2 is \input@path's CURRENT value
		//
		// so the list accumulates while the path itself is reset. Honouring only the reset
		// is why a nested \import{./}{x} failed: inside chapters/01-intro/main.tex it looked
		// for ./x relative to the working directory, not to the file doing the importing.
		// Measured on 2601.22691, which imports five chapters that each import their own
		// sections: 9 of its 14 \import calls were recorded as a skipped command, and the
		// paper came out at 2 pages of 132KB.
		for i := len(e.importStack) - 1; i >= 0 && err != nil; i-- {
			if d := e.importStack[i]; d != "" {
				data, err = e.readInput(d + path + file)
			}
		}
	}
	if err != nil && e.importPath != "" {
		data, err = e.readInput(e.importPath + path + file)
	}
	if err != nil {
		// Fall back to the bare name: a paper that ships its parts flat still reads,
		// and a genuinely missing file is recorded like any other skipped input.
		if data, err = e.readInput(file); err != nil {
			if e.skippedCS == nil {
				e.skippedCS = map[string]int{}
			}
			e.skippedCS["import"]++
			return
		}
		path = e.importPath
	}
	e.importStack = append(e.importStack, e.importPath)
	e.importPath = path
	// The pop is part of the file's own text, so it runs when this file ends. Its
	// name carries no @: the marker is read in the DOCUMENT's catcode regime, where @
	// is not a letter, so \gotex@importpop would have parsed as \gotex followed by
	// four letters and halted the run.
	e.pushInputLevel(normalizeEOL(string(data)) + " \\gotexendimport \\gotexendinput ")
}

// importPop restores the path the enclosing file was importing from.
func (e *Engine) importPop() {
	if n := len(e.importStack); n > 0 {
		e.importPath = e.importStack[n-1]
		e.importStack = e.importStack[:n-1]
	} else {
		e.importPath = ""
	}
}

// withTrailingSlash makes a directory argument usable as a prefix: import.sty's
// documented usage writes the slash ("\import{sections/}{intro}"), but papers write
// it both ways.
func withTrailingSlash(dir string) string {
	if dir == "" || strings.HasSuffix(dir, "/") {
		return dir
	}
	return dir + "/"
}
