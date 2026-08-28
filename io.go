// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// This file gives the engine two file-facing primitives so it can run a real
// document: \font loads a font file and defines a font-switching control
// sequence, and \input splices another source file into the input stream. Both
// read a filename with TeX's rules (space-terminated, or braced to allow spaces).

// fileEndMarkers are the control sequences the splicer appends at the end of a
// loaded file: \gotexendinput after an \input (io.go), and the end-of-package /
// end-of-class hooks after a \usepackage / \documentclass (packages.go).
// \endinput skips forward to the nearest one to stop reading the current file
// (see classprims.go). They must match the tokens the splicers emit.
var fileEndMarkers = [][]rune{
	[]rune(`\gotexendinput`),
	[]rune(`\@endofpackagehook`),
	[]rune(`\@endofclasshook`),
}

// runeIndexFrom returns the index of the first occurrence of needle in s at or
// after from, or -1. Used by \endinput to find its file's end marker.
func runeIndexFrom(s []rune, from int, needle []rune) int {
	if from < 0 {
		from = 0
	}
	for i := from; i+len(needle) <= len(s); i++ {
		match := true
		for j := range needle {
			if s[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

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
	e.spliceInputFile(data)
}

// mouthLevel is one level of TeX's input stack. tex.web §537 start_input does
// begin_file_reading — "set up cur_file and new level of input" — and the file is
// read to its end before the level underneath resumes; §329 end_file_reading pops
// back. A level holds the character buffer being read, where the mouth stands in
// it, and the state of the level it interrupted.
type mouthLevel struct {
	base       []rune
	bpos       int
	lineStarts []int
	lists      [][]tok
	progBpos   int
	srcPos     int
	srcLine    int
	srcCol     int
}

// pushInputLevel makes text the current input, to be read to its end before
// anything that was pending resumes.
//
// The pending token lists go WITH the interrupted level: this mouth reads lists
// before the character buffer, so a file merged into the buffer while a macro was
// still expanding would arrive after the rest of that macro's body. That is not a
// corner case — it is how beamer renders a [fragile] frame:
//
//	\frame<*>[…][{…,fragile=false}]{\begingroup\input{\jobname.vrb}\endgroup}
//
// The frame's body is written out verbatim and read back here; arriving late, it
// was typeset AFTER the frame had been contributed to the page, into a box nothing
// ever placed. Measured, the frame lost its body and its page.
//
// The no-progress guard watches e.bpos, which starts again at 0 in the new buffer,
// so its baseline is saved and reset with the level — otherwise every level below
// the first would look like an expansion loop making no headway.
func (e *Engine) pushInputLevel(text string) {
	e.levels = append(e.levels, mouthLevel{
		base: e.base, bpos: e.bpos, lineStarts: e.lineStarts, lists: e.lists,
		progBpos: e.progBpos, srcPos: e.srcPos, srcLine: e.curSrcLine, srcCol: e.curSrcCol,
	})
	e.base, e.bpos, e.lists = []rune(text), 0, nil
	e.lineStarts, e.progBpos, e.noProgSteps = nil, 0, 0
	e.buildLineStarts()
}

// popInputLevel returns to the level under the current one, and reports whether
// there was one. Anything the finished level left pending stays on top of what the
// level underneath had pending, so an unbalanced file cannot strand it.
func (e *Engine) popInputLevel() bool {
	n := len(e.levels)
	if n == 0 {
		return false
	}
	l := e.levels[n-1]
	e.levels = e.levels[:n-1]
	e.base, e.bpos, e.lineStarts = l.base, l.bpos, l.lineStarts
	e.lists = append(l.lists, e.lists...)
	e.progBpos, e.noProgSteps = l.progBpos, 0
	e.srcPos, e.curSrcLine, e.curSrcCol = l.srcPos, l.srcLine, l.srcCol
	return true
}

// spliceInputFile reads a file in as a new input level, followed by a marker that
// ends it. EVERY way of reading a file in must use this: the marker is where
// \endinput stops (it means "stop reading THIS file"), and TeX appends a space at
// the end of a file. The marker's name has no @ in it, since a file may be read in
// wherever the catcode of @ happens to be.
func (e *Engine) spliceInputFile(data []byte) {
	e.pushInputLevel(normalizeEOL(string(data)) + " \\gotexendinput ")
}

// endInput is what the marker at the end of an \input file runs. The level pops on
// its own when the buffer is exhausted, one token later, so there is nothing left
// to do here — it stays a primitive because \endinput jumps to it.
func (e *Engine) endInput() {}

// scanFileName reads a filename: optional spaces, then either a {braced} name
// (spaces allowed) or characters up to the next space / control sequence.
//
// The name is EXPANDED as it is read, which is what TeX's file-name scanner does
// and what lets a package build the name of the file it wants: pgf loads each of
// its modules with \input{pgfmodule\pgf@temp.code.tex}, so reading the name
// literally finds no file at all and the module silently never loads.
func (e *Engine) scanFileName() string {
	// A file name is read with expansion (so \input{pgfmodule\pgf@temp.code.tex}
	// works), but an active character in the name is part of the name, not the
	// \nobreakspace tie: Windows 8.3 short paths embed a ~ ("C:/Users/RUNNER~1/…").
	// Suppress active-char expansion for the length of the scan.
	defer func(prev bool) { e.literalActive = prev }(e.literalActive)
	e.literalActive = true
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return ""
	}
	if t.cat == catBegin && !t.cs_ { // {name with spaces}
		var b []rune
		for {
			u, ok := e.getXToken()
			if !ok || (u.cat == catEnd && !u.cs_) {
				break
			}
			if u.cs_ {
				continue // a control sequence that expands to nothing nameable
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
		u, ok := e.getXToken()
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

// readInput reads an \input file, applying the candidate-name search over the
// same places \usepackage looks (see findTeXFile): the document's directory, the
// TEXINPUTS/GOTEX_TEXMF path, then the embedded base set. A package splits itself
// across files and pulls them in with \input, so \input must search the path as
// well — a package found on TEXINPUTS whose own parts were not would load only
// its first file. It returns the first candidate that reads, or the last error.
func (e *Engine) readInput(file string) ([]byte, error) {
	var err error = fs.ErrNotExist
	for _, c := range inputCandidates(file) {
		// A file the document wrote itself comes first: \input of \jobname.vrb must
		// read what this run wrote, not a copy an earlier run left on disk. This is
		// the path a beamer [fragile] frame takes (see writestreams.go).
		if data, ok := e.writtenTeXFile(c); ok {
			return data, nil
		}
		if filepath.IsAbs(c) {
			data, e2 := os.ReadFile(c)
			if e2 == nil {
				return data, nil
			}
			err = e2
			continue
		}
		for _, d := range e.texInputDirs() {
			data, e2 := os.ReadFile(filepath.Join(d, c))
			if e2 == nil {
				return data, nil
			}
			err = e2
		}
		if data, _, ok := e.hostTeXFile(c); ok {
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
