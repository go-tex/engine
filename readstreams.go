// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// TeX reads a file a line at a time through a numbered stream: \openin<n>=<file>
// opens one, \ifeof<n> asks whether it has run out (or was never opened at all),
// \read<n> to \cs takes the next line, and \closein<n> closes it.
//
// The reason every package needs them is not reading data files — it is asking
// whether a file EXISTS. There is no other way to put the question to TeX:
//
//	\openin\myread=fancyhdr.sty
//	\ifeof\myread <not installed>\else <installed>\fi
//	\closein\myread
//
// While these were missing, that block ran with \openin and \ifeof undefined. In
// strict mode the file stopped; in lenient mode the skipping heuristic ate the
// tokens that followed, which is how loading pgfplots — it probes for its own
// revision file this way — took the \input after it down as well and left tikz
// half-loaded. Pictures that drew without pgfplots on the path drew nothing with
// it.
//
// TeX has sixteen streams. A stream that was never opened, or whose file was not
// found, reads as ended, which is exactly what makes the idiom above work.

// maxReadStreams is TeX's \openin/\read limit.
const maxReadStreams = 16

// readStream is one open input file, held as the lines still to be read.
type readStream struct {
	lines []string
	pos   int
	open  bool
}

// atEOF reports what \ifeof reports: a stream that is closed, was never opened,
// or has no line left.
func (r *readStream) atEOF() bool { return !r.open || r.pos >= len(r.lines) }

// installReadStreams registers the four primitives.
func (e *Engine) installReadStreams() {
	e.prim("openin", func(e *Engine) { e.doOpenin() })
	e.prim("closein", func(e *Engine) { e.doClosein() })
	e.prim("read", func(e *Engine) { e.doRead() })
	e.prim("ifeof", func(e *Engine) { e.doIf(e.evalIfeof()) })
	expandableSet["ifeof"] = true
	etexIfPrims["ifeof"] = true
}

// streamIndex reads a stream number and reports whether it is one the engine
// keeps. TeX treats a number outside 0–15 as the terminal; here it simply names
// no stream, so \ifeof on it is true and \read from it yields nothing.
func (e *Engine) streamIndex() (int, bool) {
	n := e.scanInt()
	return n, n >= 0 && n < maxReadStreams
}

// doOpenin handles \openin<n>=<file>. A file that cannot be found leaves the
// stream closed rather than failing: that IS the answer the caller is asking for.
func (e *Engine) doOpenin() {
	n, ok := e.streamIndex()
	e.scanEquals()
	file := e.scanFileName()
	if !ok {
		return
	}
	e.readStreams[n] = readStream{}
	if file == "" {
		return
	}
	data, err := e.readInput(file)
	if err != nil {
		return
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	e.readStreams[n] = readStream{lines: strings.Split(text, "\n"), open: true}
}

// doClosein handles \closein<n>.
func (e *Engine) doClosein() {
	if n, ok := e.streamIndex(); ok {
		e.readStreams[n] = readStream{}
	}
}

// evalIfeof is \ifeof<n>.
func (e *Engine) evalIfeof() bool {
	n, ok := e.streamIndex()
	if !ok {
		return true
	}
	return e.readStreams[n].atEOF()
}

// doRead handles \read<n> to \cs: the next line of the stream becomes the body of
// \cs, tokenized under the category codes in force now. A stream that has ended
// gives an empty macro, which is what a file that reads past the end expects.
func (e *Engine) doRead() {
	n, ok := e.streamIndex()
	e.scanKeyword("to")
	e.skipOptSpace() // \read0 to \line — TeX allows space before the name
	name := e.scanCSName()
	line := ""
	if ok && !e.readStreams[n].atEOF() {
		s := &e.readStreams[n]
		line = s.lines[s.pos]
		s.pos++
	}
	if name == "" {
		return
	}
	e.define(name, &meaning{kind: mMacro, body: charToks(line)}, false)
}

// charToks turns a line into character tokens, spaces kept as spaces.
func charToks(s string) []tok {
	ts := make([]tok, 0, len(s))
	for _, r := range s {
		c := catOther
		if r == ' ' {
			c = catSpace
		}
		ts = append(ts, chTok(r, c))
	}
	return ts
}
