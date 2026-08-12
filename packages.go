// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
)

// This file lets the engine LOAD real LaTeX class and package files (.cls/.sty)
// instead of only emulating them. \documentclass and \usepackage resolve the
// named file (from the document's directory, a TEXINPUTS-style search path, or an
// embedded base set), make @ a letter as LaTeX does, and splice the file into the
// input so its own \newcommand/\def/\DeclareOption... run on the engine. A file is
// loaded *tolerantly*: a command the engine does not yet implement is skipped
// (as in lenient mode) rather than aborting the compile, so a real third-party
// class contributes what it can. The LaTeX2e option mechanism
// (\DeclareOption/\ProcessOptions/\ExecuteOptions/\CurrentOption/\PassOptionsTo*)
// lives here too, since it is driven by the per-file load state.

// loadFrame is one entry on the class/package load stack: what to restore when the
// file finishes, plus its option-processing context.
type loadFrame struct {
	atcat    cat              // catcode of @ to restore when the file ends
	name     string           // package/class base name (for the loaded registry + \CurrentOption)
	endHook  string           // \@endofpackagehook / \@endofclasshook to reset after the file
	passed   []string         // options requested for this file
	declared map[string][]tok // \DeclareOption{name}{code}
	star     []tok            // \DeclareOption*{code}
	hasStar  bool
}

// texInputDirs is the ordered search path for \usepackage/\documentclass/\input
// files: the current directory (the document's dir — the CLI chdirs there), then
// any colon-separated dirs in TEXINPUTS or GOTEX_TEXMF.
func (e *Engine) texInputDirs() []string {
	dirs := []string{"."}
	for _, env := range []string{"TEXINPUTS", "GOTEX_TEXMF"} {
		for _, d := range filepath.SplitList(os.Getenv(env)) {
			if d = strings.TrimSpace(d); d != "" && d != "." {
				dirs = append(dirs, d)
			}
		}
	}
	return dirs
}

// findTeXFile resolves name (with one of exts appended when it has no extension)
// against the search path and the embedded base set. It returns the file bytes and
// a display path.
func (e *Engine) findTeXFile(name string, exts []string) ([]byte, string, bool) {
	var candidates []string
	if hasExtension(name) {
		candidates = []string{name}
	} else {
		for _, x := range exts {
			candidates = append(candidates, name+x)
		}
	}
	for _, c := range candidates {
		for _, d := range e.texInputDirs() {
			if data, err := os.ReadFile(filepath.Join(d, c)); err == nil {
				return data, filepath.Join(d, c), true
			}
		}
		if data, ok := embeddedTeXFile(c); ok {
			return data, "<embedded>/" + c, true
		}
	}
	return nil, "", false
}

// neverLoadReal lists packages the engine emulates natively (geometry) or whose
// real implementation is too distribution-heavy to run and is better served by the
// existing stubs (drawing/font/encoding packages). A file with one of these names
// is not loaded from disk even when present.
var neverLoadReal = map[string]bool{
	"geometry": true, "tikz": true, "pgf": true, "pgfplots": true,
	"hyperref": true, "fontspec": true, "inputenc": true, "fontenc": true,
	"babel": true, "biblatex": true, "listings": true, "pstricks": true,
	"xcolor": true, "color": true, "graphicx": true, "graphics": true,
}

// loadTeXFile splices a resolved class/package file into the input with @ made a
// letter, pushing a load frame so the catcode and tolerance are restored when the
// file's tokens are exhausted (via the \@gotex@endload marker appended after it).
// name is the base name and ext its extension (".sty"/".cls"); passed are its
// options.
func (e *Engine) loadTeXFile(data []byte, name, ext string, passed []string) {
	endHook := "@endofpackagehook"
	if ext == ".cls" {
		endHook = "@endofclasshook"
	}
	fr := loadFrame{
		atcat:    e.catcode['@'],
		name:     name,
		endHook:  endHook,
		passed:   passed,
		declared: map[string][]tok{},
	}
	e.loadStack = append(e.loadStack, fr)
	e.loadDepth++
	e.catcode['@'] = catLetter
	if e.loadedPackages == nil {
		e.loadedPackages = map[string]bool{}
	}
	e.loadedPackages[name] = true
	// Record the file the way \ProvidesPackage/\ProvidesClass would, so the kernel's
	// \@ifpackageloaded{name}/\@ifpackagewith{name}{opt} (which consult ver@<file>
	// and opt@<file>) see it as loaded with its options.
	e.define("ver@"+name+ext, &meaning{kind: mMacro, body: stringToToks("gotex")}, true)
	e.define("opt@"+name+ext, &meaning{kind: mMacro, body: stringToToks(strings.Join(passed, ","))}, true)
	// \CurrentOption starts empty for this file.
	e.define("CurrentOption", &meaning{kind: mMacro}, true)
	// Splice: file body, then the end-of-file hook (\AtEndOfPackage/Class code) and
	// a marker control sequence that pops the frame. The marker tokenizes with @
	// still a letter, so its name is valid. Line endings are normalised to LF first:
	// the engine treats only \n as end-of-line, so a CRLF file (e.g. a .cls checked
	// out on Windows) would otherwise typeset stray \r characters.
	insert := []rune(normalizeEOL(string(data)) + "\\" + endHook + "\\@gotex@endload ")
	tail := append(insert, e.base[e.bpos:]...)
	e.base = append(e.base[:e.bpos:e.bpos], tail...)
	e.buildLineStarts()
}

// normalizeEOL converts CRLF and lone CR line endings to LF. The engine's mouth
// treats only \n as end-of-line, so a file with Windows/classic-Mac line endings
// would otherwise leave stray \r characters that get typeset.
func normalizeEOL(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// endLoad pops the top load frame: it restores @'s catcode and the load tolerance,
// and resets the end-of-file hook so it does not leak into the next load.
func (e *Engine) endLoad() {
	if len(e.loadStack) == 0 {
		return
	}
	fr := e.loadStack[len(e.loadStack)-1]
	e.loadStack = e.loadStack[:len(e.loadStack)-1]
	e.catcode['@'] = fr.atcat
	if fr.endHook != "" {
		e.define(fr.endHook, &meaning{kind: mMacro}, true) // \let\@endof…hook\@empty
	}
	if e.loadDepth > 0 {
		e.loadDepth--
	}
}

// curFrame returns the top load frame (where \DeclareOption etc. record), or nil
// outside any file load.
func (e *Engine) curFrame() *loadFrame {
	if len(e.loadStack) == 0 {
		return nil
	}
	return &e.loadStack[len(e.loadStack)-1]
}

// ── \documentclass / \usepackage / \RequirePackage / \LoadClass ─────────────

// emulatedClasses are the standard classes the engine models directly and well
// (deterministically, under the strict-mode conformance gate). Routing them to the
// real .cls would make a strict compile depend on the entire LaTeX kernel being
// present (a real \documentclass{article} calls \@startsection, \list, NFSS … at
// document time); until that kernel is complete the built-in emulation is the
// faithful path, so these are not loaded from the embedded/real .cls. The embedded
// article.cls is still available to \LoadClass (a custom class building on it).
var emulatedClasses = map[string]bool{
	"article": true, "report": true, "book": true, "amsart": true,
	"letter": true, "proc": true, "slides": true, "minimal": true,
}

// doDocumentClass implements \documentclass[options]{class}: for a non-emulated
// class it loads class.cls when it can be resolved; for a standard class (and when
// the file cannot be found) it falls back to the built-in emulation.
func (e *Engine) doDocumentClass() {
	opts := e.scanBracketList()
	name := e.readBraceName()
	if name == "" {
		return
	}
	e.setPtsize(opts) // record 10pt/11pt/12pt for \@ptsize even without the .cls
	if emulatedClasses[name] || neverLoadReal[name] {
		return // use the built-in emulation for a standard class
	}
	if data, _, ok := e.findTeXFile(name, []string{".cls"}); ok {
		e.loadTeXFile(data, name, ".cls", append(opts, e.takePassed(name)...))
	}
}

// doUsepackageLoad implements \usepackage[options]{name,name,...}: geometry keeps
// its native handler; every other package is loaded from a resolved .sty when one
// exists (and is not on the never-load list), else left to the stubs.
func (e *Engine) doUsepackageLoad() {
	opts := e.scanBracketList()
	names := e.readBraceName()
	for _, raw := range strings.Split(names, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if name == "geometry" {
			e.applyGeometry(strings.Join(opts, ","))
			continue
		}
		if neverLoadReal[name] {
			continue
		}
		if data, _, ok := e.findTeXFile(name, []string{".sty"}); ok {
			e.loadTeXFile(data, name, ".sty", append(append([]string{}, opts...), e.takePassed(name)...))
		}
	}
}

// doLoadClass implements \LoadClass[options]{class} (used by a class to build on a
// base class). \LoadClassWithOptions passes the current class's options through.
func (e *Engine) doLoadClass(withOptions bool) {
	opts := e.scanBracketList()
	if withOptions {
		if fr := e.curFrame(); fr != nil {
			opts = append(opts, fr.passed...)
		}
	}
	name := e.readBraceName()
	if name == "" {
		return
	}
	if data, _, ok := e.findTeXFile(name, []string{".cls"}); ok && !neverLoadReal[name] {
		e.loadTeXFile(data, name, ".cls", append(opts, e.takePassed(name)...))
	}
}

// ── LaTeX2e option processing ────────────────────────────────────────────────

// doDeclareOption implements \DeclareOption{name}{code} and \DeclareOption*{code}:
// it records the option's code on the current load frame.
func (e *Engine) doDeclareOption() {
	fr := e.curFrame()
	if e.peekStar() {
		code := e.readBraceToksRaw()
		if fr != nil {
			fr.star, fr.hasStar = code, true
		}
		return
	}
	name := e.readBraceName()
	code := e.readBraceToksRaw()
	if fr != nil {
		fr.declared[name] = code
	}
}

// doProcessOptions implements \ProcessOptions (and \ProcessOptions*): for each
// requested option it runs the matching \DeclareOption code (or \DeclareOption*'s
// code, or nothing), with \CurrentOption bound to the option, by pushing the
// assembled token list so it executes before the rest of the file.
func (e *Engine) doProcessOptions() {
	e.peekStar() // \ProcessOptions* is accepted; order differences are not modelled
	fr := e.curFrame()
	if fr == nil {
		return
	}
	var run []tok
	for _, opt := range fr.passed {
		code, ok := fr.declared[opt]
		if !ok {
			if !fr.hasStar {
				continue // unknown option: ignore (LaTeX would warn)
			}
			code = fr.star
		}
		run = append(run, setCurrentOptionToks(opt)...)
		run = append(run, code...)
	}
	if len(run) > 0 {
		e.push(run)
	}
}

// doExecuteOptions implements \ExecuteOptions{opt,opt,...}: it runs the code of
// each named option that this file declared (used to set defaults before
// \ProcessOptions), with \CurrentOption bound.
func (e *Engine) doExecuteOptions() {
	list := e.readBraceName()
	fr := e.curFrame()
	if fr == nil {
		return
	}
	var run []tok
	for _, raw := range strings.Split(list, ",") {
		opt := strings.TrimSpace(raw)
		if code, ok := fr.declared[opt]; ok {
			run = append(run, setCurrentOptionToks(opt)...)
			run = append(run, code...)
		}
	}
	if len(run) > 0 {
		e.push(run)
	}
}

// doPassOptionsTo implements \PassOptionsToPackage{opts}{pkg} and
// \PassOptionsToClass: it stashes options to be merged when pkg/class is loaded.
func (e *Engine) doPassOptionsTo() {
	opts := e.readBraceName()
	target := e.readBraceName()
	if e.passedOptions == nil {
		e.passedOptions = map[string][]string{}
	}
	for _, raw := range strings.Split(opts, ",") {
		if o := strings.TrimSpace(raw); o != "" {
			e.passedOptions[target] = append(e.passedOptions[target], o)
		}
	}
}

// takePassed returns and clears the options queued for name by \PassOptionsTo*.
func (e *Engine) takePassed(name string) []string {
	if e.passedOptions == nil {
		return nil
	}
	p := e.passedOptions[name]
	delete(e.passedOptions, name)
	return p
}

// setCurrentOptionToks builds the tokens for \def\CurrentOption{opt}.
func setCurrentOptionToks(opt string) []tok {
	toks := []tok{csTok("def"), csTok("CurrentOption"), chTok('{', catBegin)}
	toks = append(toks, stringToToks(opt)...)
	return append(toks, chTok('}', catEnd))
}

// setPtsize records the base type size selected by a class option so \@ptsize
// (used by size1x.clo names) reflects 10/11/12pt; it defaults to 10pt.
func (e *Engine) setPtsize(opts []string) {
	pt := "0" // \@ptsize is (size-10): 0/1/2 for 10/11/12pt
	for _, o := range opts {
		switch strings.TrimSpace(o) {
		case "11pt":
			pt = "1"
		case "12pt":
			pt = "2"
		}
	}
	e.define("@ptsize", &meaning{kind: mMacro, body: stringToToks(pt)}, true)
}

// ── scanning helpers ─────────────────────────────────────────────────────────

// scanBracketList reads an optional [a,b,c] and returns the trimmed items (nil
// when absent).
func (e *Engine) scanBracketList() []string {
	toks, ok := e.scanOptBracketToks()
	if !ok {
		return nil
	}
	var out []string
	for _, raw := range strings.Split(e.toksToString(toks), ",") {
		if s := strings.TrimSpace(raw); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// peekStar consumes and reports a leading * (after optional spaces).
func (e *Engine) peekStar() bool {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok {
		return false
	}
	if !t.cs_ && t.ch == '*' {
		return true
	}
	e.back(t)
	return false
}

// readBraceToksRaw reads a {..} group without expansion, returning its tokens (the
// replacement text of an option or hook); nil when the next thing is not a group.
func (e *Engine) readBraceToksRaw() []tok {
	for {
		t, ok := e.getNext()
		if !ok {
			return nil
		}
		if t.cat == catSpace {
			continue
		}
		if t.cat == catBegin && !t.cs_ {
			return e.grabGroup()
		}
		e.back(t)
		return nil
	}
}

// ── \IfFileExists / \InputIfFileExists ───────────────────────────────────────

// doIfFileExists implements \IfFileExists{name}{then}{else}: it runs the then-code
// when name resolves on the search path, else the else-code.
func (e *Engine) doIfFileExists() {
	name := e.readBraceName()
	then := e.readBraceToksRaw()
	els := e.readBraceToksRaw()
	if _, _, ok := e.findTeXFile(name, []string{"", ".tex"}); ok {
		e.push(then)
	} else {
		e.push(els)
	}
}

// doInputIfFileExists implements \InputIfFileExists{name}{then}{else}: when name
// resolves it splices the file (with the then-code queued to run after it), else it
// runs the else-code.
func (e *Engine) doInputIfFileExists() {
	name := e.readBraceName()
	then := e.readBraceToksRaw()
	els := e.readBraceToksRaw()
	data, _, ok := e.findTeXFile(name, []string{"", ".tex"})
	if !ok {
		e.push(els)
		return
	}
	if len(then) > 0 {
		e.push(then)
	}
	insert := []rune(normalizeEOL(string(data)) + " ")
	tail := append(insert, e.base[e.bpos:]...)
	e.base = append(e.base[:e.bpos:e.bpos], tail...)
	e.buildLineStarts()
}
