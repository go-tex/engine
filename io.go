// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"strings"
)

// This file gives the engine two file-facing primitives so it can run a real
// document: \font loads a font file and defines a font-switching control
// sequence, and \input splices another source file into the input stream. Both
// read a filename with TeX's rules (space-terminated, or braced to allow spaces).

// doFont handles \font\cs=<file> [at <dimen> | scaled <int>]: it loads the font
// and defines \cs so that using it selects that font as the current one.
func (e *Engine) doFont() {
	name := e.scanCSName()
	e.scanEquals()
	file := e.scanFileName()
	sizePt := 10.0 // default design size
	switch {
	case e.scanKeyword("at"):
		sizePt = spToPt(e.scanDimen())
	case e.scanKeyword("scaled"):
		sizePt = 10.0 * float64(e.scanInt()) / 1000.0 // \scaled is in thousandths
	}
	data, err := os.ReadFile(file)
	if err != nil {
		if e.tolerant() {
			return // best-effort: skip a \font whose file isn't shipped
		}
		e.fail("font file not found: " + file)
		return
	}
	f, err := NewOpenTypeFont(data, int(sizePt+0.5))
	if err != nil {
		e.fail("cannot parse font: " + file)
		return
	}
	if name != "" {
		e.define(name, &meaning{kind: mFont, font: f, name: name}, false)
	}
}

// doInput handles \input <file>: it reads the file (adding ".tex" if the name has
// no extension) and splices its contents into the base input at the current
// position, so it is tokenized next with the live category codes.
func (e *Engine) doInput() {
	file := e.scanFileName()
	if file == "" {
		return
	}
	data, err := e.readInput(file)
	if err != nil {
		if e.tolerant() {
			if e.skippedCS == nil {
				e.skippedCS = map[string]int{}
			}
			e.skippedCS["input"]++
			return // best-effort: skip an \input whose file isn't shipped
		}
		e.fail("input file not found: " + file)
		return
	}
	insert := []rune(normalizeEOL(string(data)) + " ") // TeX appends a space at end of file
	tail := append(insert, e.base[e.bpos:]...)
	e.base = append(e.base[:e.bpos:e.bpos], tail...)
	e.buildLineStarts() // the splice shifted every offset; rebuild the line table
}

// scanFileName reads a filename: optional spaces, then either a {braced} name
// (spaces allowed) or characters up to the next space / control sequence.
func (e *Engine) scanFileName() string {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok {
		return ""
	}
	if t.cat == catBegin && !t.cs_ { // {name with spaces}
		var b []rune
		for {
			u, ok := e.getNext()
			if !ok || (u.cat == catEnd && !u.cs_) {
				break
			}
			b = append(b, u.ch)
		}
		return string(b)
	}
	if t.cs_ {
		e.back(t)
		return ""
	}
	b := []rune{t.ch}
	for {
		u, ok := e.getNext()
		if !ok {
			break
		}
		if u.cs_ || (u.cat == catSpace && !u.cs_) {
			if u.cs_ {
				e.back(u)
			}
			break
		}
		b = append(b, u.ch)
	}
	return string(b)
}

// inputCandidates returns the filenames \input tries, in order. TeX's default
// extension is .tex, but a dot in the basename does not reliably mean an
// extension is present: real section files are named "1.Introduction",
// "2.1.prelims", etc., whose real file is "1.Introduction.tex". So when the name
// has a dot we try it verbatim first (it may already be complete, e.g. foo.tex or
// data.csv) and fall back to name+.tex; when it has no dot we append .tex first
// (TeX's rule) and try the bare name second.
func inputCandidates(file string) []string {
	if hasExtension(file) {
		return []string{file, file + ".tex"}
	}
	return []string{file + ".tex", file}
}

// readInput reads an \input file, applying the candidate-name search. It returns
// the first candidate that reads successfully, or the last error otherwise.
func (e *Engine) readInput(file string) ([]byte, error) {
	var err error
	var data []byte
	for _, c := range inputCandidates(file) {
		if data, err = os.ReadFile(c); err == nil {
			return data, nil
		}
	}
	return nil, err
}

// hasExtension reports whether the filename's basename contains a dot.
func hasExtension(name string) bool {
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	return strings.Contains(name, ".")
}
