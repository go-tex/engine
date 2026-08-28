// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// Output streams: \openout, \write, \closeout, and the in-memory files they leave
// behind for \input to read back.
//
// A document that writes a file and reads it again is not an exotic case — it is how
// beamer renders a [fragile] frame. beamerbaseframe.sty copies the frame's body out
// verbatim, line by line, and reads it back with the catcodes restored:
//
//	\immediate\openout\beamer@verbatimfileout=\beamer@verbfilename
//	  … \immediate\write\beamer@verbatimfileout{<one source line>} …
//	\immediate\closeout\beamer@verbatimfileout
//	\frame<…>[…,fragile=false]{\begingroup\input{\beamer@verbfilename}\endgroup}
//
// With \write consuming its argument and writing nowhere, the \input found nothing
// and the frame's body was simply gone. Measured over 200 real talks, giving the
// files back is worth 26 pages and 7875 glyphs: ten talks gain (one goes from 63
// glyphs to 1163 — a talk that was almost entirely verbatim frames), two lose.
//
// The two that lose are the known limitation. tex.web §537 start_input does
// begin_file_reading, "set up cur_file and new level of input": the file becomes the
// CURRENT input level and is read to its end before whatever was already being read
// resumes. This mouth reads pending token lists (macro expansions) before the base
// text, so a file spliced in while a macro is still expanding arrives AFTER the rest
// of that macro's body — for a [fragile] frame, after the frame has closed. Holding
// the pending lists aside for the length of the file does put them in TeX's order,
// and measured on the same 200 talks it costs 63 pages: it moves the body back
// inside the frame, where a further defect loses it altogether (the frame's
// \global\setbox\beamer@framebox=\vbox\bgroup comes out empty). The order is worth
// fixing WITH that defect, not before it.
//
// Nothing reaches the filesystem: the text is kept in memory and the file readers
// are taught to look there first. The engine has no business creating files beside a
// document it was asked to render, and beamer only ever reads back what it wrote.
//
// What \write does with its token list is tex.web §1369-1370: "To write a token list,
// we must run it through TeX's scanner, expanding macros and \the and \number", then
// token_show(def_ref); print_ln — the EXPANDED list, followed by exactly one newline.
// So one \write is one line.
//
// Only \immediate is modelled. TeX makes a plain \write a whatsit that fires during
// \shipout (§1370 is called from hlist_out/vlist_out); beamer's verbatim writer is
// \immediate throughout, and the engine's \immediate is a no-op prefix, so both forms
// take effect at once here. A document that relies on a deferred \write seeing page
// state would differ — none in the corpora does.

// doOpenout implements \openout<n>=<filename>: it opens stream n on a fresh buffer.
// The number is scanned as any integer (as \closeout does) rather than with TeX's
// scan_four_bit_int, and the file name with the engine's scanFileName (tex.web §526).
func (e *Engine) doOpenout() {
	n := e.scanInt()
	e.scanEquals()
	name := e.scanFileName()
	if name == "" {
		return
	}
	if e.outStream == nil {
		e.outStream = map[int]*strings.Builder{}
		e.outName = map[int]string{}
	}
	e.outStream[n] = &strings.Builder{}
	e.outName[n] = name
}

// doWrite implements \write<n>{token list}: the list is expanded and appended to
// stream n as one line. A write to a stream that is not open is discarded — TeX sends
// it to the log, which is not something this engine keeps.
func (e *Engine) doWrite() {
	n := e.scanInt()
	e.skipOptSpace()
	var toks []tok
	if t, ok := e.getNext(); ok {
		if t.cat == catBegin && !t.cs_ {
			toks = e.grabGroup()
		} else {
			e.back(t)
		}
	}
	b, open := e.outStream[n]
	if !open {
		return
	}
	b.WriteString(e.toksToString(e.expandList(toks)))
	b.WriteByte('\n')
}

// doCloseout implements \closeout<n>: the stream's text becomes a file that \input
// can read.
func (e *Engine) doCloseout() {
	n := e.scanInt()
	b, open := e.outStream[n]
	if !open {
		return
	}
	if name := e.outName[n]; name != "" {
		if e.writtenFile == nil {
			e.writtenFile = map[string]string{}
		}
		e.writtenFile[name] = b.String()
	}
	delete(e.outStream, n)
	delete(e.outName, n)
}

// writtenTeXFile returns a file the document itself wrote, if one matches. It is
// consulted BEFORE the search path: a document that writes \jobname.vrb and reads it
// back must get its own text, never a stale file of the same name left on disk by an
// earlier run.
func (e *Engine) writtenTeXFile(name string) ([]byte, bool) {
	if e.writtenFile == nil {
		return nil, false
	}
	if s, ok := e.writtenFile[name]; ok {
		return []byte(s), true
	}
	return nil, false
}
