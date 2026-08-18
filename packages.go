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
	nlCount  int              // newlines in the spliced file (added to loadedNL when it ends)
	endHook  string           // \@endofpackagehook / \@endofclasshook to reset after the file
	passed   []string         // options requested for this file
	declared map[string][]tok // \DeclareOption{name}{code}
	star     []tok            // \DeclareOption*{code}
	hasStar  bool
	// The \@currname / \@currext / \@currnamestack in force when this file was
	// entered, restored when it ends. LaTeX keeps them on a stack for the same
	// reason: a class that loads a package in the middle of its own option
	// declarations must still be "the current file" afterwards. Without the
	// restore, beamer declared its options into the family named by the LAST
	// package it happened to require, and its own \ExecuteOptionsBeamer{c} then
	// reported "key c undefined".
	prevName  *meaning
	prevExt   *meaning
	prevStack *meaning
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
		if data, path, ok := e.hostTeXFile(c); ok {
			return data, path, true
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
	// bm builds its bold-math commands from low-level math-alphabet machinery
	// (\install@mathalphabet, \getanddefine@fonts) the engine's font model does not
	// run, and its \protected@edef\bm#1{\bm{#1}} re-dispatch expands the robust \bm
	// against the engine's non-protecting \protected@edef and swallows the document.
	// The kernel already defines \bm as \boldsymbol (the math layer's bold path), so
	// the bundled real bm.sty is only ever worse.
	"bm": true,
}

// pgfPackages are the drawing packages the engine now has a system-layer driver
// for (texmf/pgfsys-gotex.def, see special.go for the seam it draws through).
// Loading their real sources — rather than gobbling a picture and standing a
// placeholder box in for it — is what makes the engine behave like a TeX
// distribution here. The path is still being brought up: the sources load a long
// way but do not yet complete, so it is opt-in through GOTEX_PGF while the
// remaining gaps are closed, and the stubs stay the default. With the variable
// set (and the pgf sources on TEXINPUTS/GOTEX_TEXMF) the real files load and the
// driver draws through the \special seam.
var pgfPackages = map[string]bool{"tikz": true, "pgf": true, "pgfplots": true}

// realPGF reports whether the real pgf/TikZ sources may be loaded.
func realPGF() bool { return os.Getenv("GOTEX_PGF") != "" }

// realBeamer reports whether \documentclass{beamer} may load the REAL beamer.cls
// (with the beamer sources, etoolbox and keyval on TEXINPUTS/GOTEX_TEXMF) instead
// of the built-in emulation in beamer.go.
//
// Where this stands, honestly: the real class now LOADS — the whole beamer base,
// its overlay decoder and its option machinery run with no undefined control
// sequence, which is what the kernel work in this file, hooks.go and
// kernelhelpers.go bought. It does not yet TYPESET a frame: beamer builds each
// slide into a box and ships it out through machinery this engine's page builder
// does not provide, so a talk still comes out nearly blank. Until that is done
// the emulation (frames→pages, which renders the content) stays the default, and
// this variable is how the real path is worked on.
func realBeamer() bool { return os.Getenv("GOTEX_BEAMER") != "" }

// emulateOnly reports whether a package must use the built-in stubs rather than
// its real file.
func emulateOnly(name string) bool {
	if pgfPackages[name] {
		return !realPGF()
	}
	return neverLoadReal[name]
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
	// \@currname / \@currext name the file being loaded, as \ProvidesClass/Package's
	// caller (\documentclass/\usepackage) sets them in real LaTeX; a class reads them
	// (amsart's opening \csname ver@\@currname.\@currext\endcsname and its
	// \@currnamestack scan). ext is stored without the leading dot ("cls"/"sty").
	extNoDot := strings.TrimPrefix(ext, ".")
	top := &e.loadStack[len(e.loadStack)-1]
	top.prevName, top.prevExt, top.prevStack = e.eq["@currname"], e.eq["@currext"], e.eq["@currnamestack"]
	e.define("@currname", &meaning{kind: mMacro, body: stringToToks(name)}, true)
	e.define("@currext", &meaning{kind: mMacro, body: stringToToks(extNoDot)}, true)
	// \@currnamestack is a flat brace-group list the class dissects with a delimited
	// macro (\@tempa#1#2\@nil); a single {name}{ext} frame satisfies that scan.
	stack := append([]tok{chTok('{', catBegin)}, stringToToks(name)...)
	stack = append(stack, chTok('}', catEnd), chTok('{', catBegin))
	stack = append(stack, stringToToks(extNoDot)...)
	stack = append(stack, chTok('}', catEnd))
	e.define("@currnamestack", &meaning{kind: mMacro, body: stack}, true)
	// Splice: file body, then the end-of-file hook (\AtEndOfPackage/Class code) and
	// a marker control sequence that pops the frame. The marker tokenizes with @
	// still a letter, so its name is valid. Line endings are normalised to LF first:
	// the engine treats only \n as end-of-line, so a CRLF file (e.g. a .cls checked
	// out on Windows) would otherwise typeset stray \r characters.
	body := normalizeEOL(string(data))
	e.loadStack[len(e.loadStack)-1].nlCount = strings.Count(body, "\n")
	// The 2020 format lets a package register code to run around ANOTHER file's
	// loading: \AddToHook{package/amsmath/after} (beamer's overlay layer does exactly
	// this) and \AddToHook{file/<name>.sty/before}. Fire those four hooks around the
	// body. \UseHook on a hook nobody registered is a no-op, so this costs nothing
	// when no one is listening.
	kind := "package"
	if ext == ".cls" {
		kind = "class"
	}
	pre := "\\UseHook{file/" + name + ext + "/before}\\UseHook{" + kind + "/" + name + "/before}"
	post := "\\UseHook{" + kind + "/" + name + "/after}\\UseHook{file/" + name + ext + "/after}"
	// \gotexeatdate consumes the OPTIONAL DATE a caller may write after the file
	// name — \RequirePackage{keyval}[1997/11/10] states the oldest acceptable
	// release. The engine loads whatever it finds, but an unread date is typeset,
	// and beamer's title page carried a stray "[1997/11/10]". It runs AFTER the
	// file (and after the frame is popped), where the date is the next thing in the
	// input: reading it BEFORE splicing the file would put the token that is not a
	// "[" back on the token stack, ahead of the file about to be spliced into the
	// character buffer — which silently swallowed everything after the \usepackage.
	insert := []rune(pre + body + "\\" + endHook + post + "\\@gotex@endload \\gotexeatdate ")
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
	// The file's lines are now behind the mouth: exclude them from the document's
	// source-line numbering (see setSrcPos), so loading a class does not shift the
	// lines the editor/error reporter attributes to the user's own document.
	e.loadedNL += fr.nlCount
	if fr.endHook != "" {
		e.define(fr.endHook, &meaning{kind: mMacro}, true) // \let\@endof…hook\@empty
	}
	// Hand \@currname / \@currext / \@currnamestack back to the file that was
	// being loaded when this one started (see loadFrame). Outside any file they
	// are EMPTY, not undefined — that is what LaTeX leaves behind, and what code
	// that interpolates them expects.
	restore := func(name string, prev *meaning) {
		if prev == nil {
			prev = &meaning{kind: mMacro}
		}
		e.eq[name] = prev
	}
	restore("@currname", fr.prevName)
	restore("@currext", fr.prevExt)
	restore("@currnamestack", fr.prevStack)
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

// emulatedClasses are the standard classes served by the built-in emulation rather
// than a real .cls. article/report/book are NOT here: the engine loads their real,
// embedded classes (the class kernel — \@startsection numbering, \secdef via
// \@dblarg for \chapter, \list, NFSS aliases, \@float, \@starttoc, rubber glue,
// source-line stability — is complete enough that they pass the conformance and
// fidelity gates).
//
// amsart IS here (gated to emulation) even though amsart.cls is embedded and loads:
// its own \newtheorem…[section] machinery loops on the engine (the runaway guard is
// expansion-only and doesn't catch it), so a real math paper hangs. The class kernel
// additions it drove — token registers, the ## parameter-char fix, the plain-TeX
// substrate — are kept (they help every real class/package); routing
// \documentclass{amsart} to the real class waits on the \newtheorem fix. The real
// amsart.cls stays available to \LoadClass. letter/proc/slides/minimal are not
// embedded, so they fall back to the emulation regardless.
var emulatedClasses = map[string]bool{
	"amsart": true,
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
	if name == "beamer" && !realBeamer() {
		e.loadBeamer()
		return
	}
	if emulatedClasses[name] || emulateOnly(name) {
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
		if emulateOnly(name) {
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
	if data, _, ok := e.findTeXFile(name, []string{".cls"}); ok && !emulateOnly(name) {
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
	opts := e.readBraceGroupText()
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

// readBraceGroupText reads a braced group and returns its text with the INNER
// braces kept and counted. An option list is not a name: beamer passes
//
//	\PassOptionsToPackage{pdfborder={0 0 0},linkbordercolor=[rgb]{.5,.5,.5}}{hyperref}
//
// and a reader that stops at the first closing brace ends the list inside
// "pdfborder={0 0 0" — the remainder was typeset onto the first page.
func (e *Engine) readBraceGroupText() string {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok || t.cs_ || t.cat != catBegin {
		if ok {
			e.back(t)
		}
		return ""
	}
	var b []rune
	for depth := 1; ; {
		u, ok := e.getNext()
		if !ok {
			break
		}
		if !u.cs_ && u.cat == catEnd {
			depth--
			if depth == 0 {
				break
			}
		}
		if !u.cs_ && u.cat == catBegin {
			depth++
		}
		if !u.cs_ {
			b = append(b, u.ch)
		}
	}
	return string(b)
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

// peekStar consumes and reports a leading * (after optional spaces). When no star
// follows, the input is restored EXACTLY as it stood — nothing is left pushed back.
// A plain back() of the peeked token would strand it in a pending token list, ahead
// of the base input it was read from. That is harmless until the caller then splices
// a file into the base: \ProcessOptions runs a class option's code, and an option
// that loads a companion file (svjour's [epj] \InputIfFileExists{svepj.clo}) splices
// that file at the mouth position — landing it BETWEEN the stranded token and the
// base tokens that followed it. svjour's next line, \ifx\journalopt\@empty, then has
// its already-peeked \ifx bind to the loaded file's first token (\ProvidesFile{…})
// instead of \journalopt, and the mismatched conditional skips to end-of-file,
// swallowing the whole document (0 pages). Restoring the mark keeps the peeked token
// attached to what genuinely follows it, so a later splice cannot slip in between.
func (e *Engine) peekStar() bool {
	m := e.markInput()
	e.skipOptSpace()
	if t, ok := e.getNext(); ok && !t.cs_ && t.ch == '*' {
		return true
	}
	e.restoreInput(m)
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

// readBraceNameX reads a braced file name, expanding it as TeX's file-name
// scanner does: a package names the file to load through a macro (pgf loads its
// driver as \pgfutil@InputIfFileExists{\pgfsysdriver}), so an unexpanded name
// would never resolve.
func (e *Engine) readBraceNameX() string {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || t.cs_ || t.cat != catBegin {
		if ok {
			e.back(t)
		}
		return ""
	}
	var b []rune
	for {
		u, ok := e.getXToken()
		if !ok || (!u.cs_ && u.cat == catEnd) {
			break
		}
		if u.cs_ {
			b = append(b, []rune(u.cs)...)
			continue
		}
		b = append(b, u.ch)
	}
	return strings.TrimSpace(string(b))
}

// doInputIfFileExists implements \InputIfFileExists{name}{then}{else}: when name
// resolves it splices the file (with the then-code queued to run after it), else it
// runs the else-code.
func (e *Engine) doInputIfFileExists() {
	name := e.readBraceNameX()
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
	e.spliceInputFile(data)
}
