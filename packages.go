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
// against the files the document has written, the search path, and the embedded base
// set, in that order. It returns the file bytes and
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
		// A file the document wrote itself wins over anything on disk: beamer reads
		// back \jobname.vrb, and a stale copy left by an earlier run would render
		// the previous document's frame (see writestreams.go).
		if data, ok := e.writtenTeXFile(c); ok {
			return data, c, true
		}
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

// twoColumnOptIn reports whether the \documentclass[twocolumn] option activates the
// two-column output routine (twocolumn.go). It is opt-in through GOTEX_TWOCOLUMN while
// the routine's page-fill is paired with each class's real \textheight: the routine
// places words in the right COLUMN positions (proven on revtex 2605.12538, where the
// median displacement fell from 5.4 to 0.6), but two \vsize-tall columns on the current
// article-shaped page height hold ~2x the text, so the page count drifts (a document
// loses or gains pages) until the paired geometry lands. Off by default, the corpus is
// untouched; the mechanism and its tests ship ready for that next step.
func twoColumnOptIn() bool { return os.Getenv("GOTEX_TWOCOLUMN") != "" }

// realBeamer reports whether \documentclass{beamer} loads the REAL beamer.cls
// rather than the built-in emulation in beamer.go.
//
// It is decided by whether the class file can be RESOLVED — from the document's
// directory, TEXINPUTS/GOTEX_TEXMF, the host's Options.Resolve, or the embedded
// set. There is nothing to opt into: a caller that supplies beamer gets beamer,
// and one that does not gets the emulation, which is what the fallback in
// doDocumentClass already promised.
//
// It used to be gated on GOTEX_BEAMER because the real class loaded but was
// believed not to typeset — "a talk still comes out nearly blank". That was true
// when it was written and is not any more; the grabUndelimited fix in v0.160.0
// is what changed it. Measured over 10025 real talks:
//
//	real beamer.cls   99.9% rendered   87209 pages   (~8.7 per talk)
//	emulation         92.3% rendered   14162 pages   (~1.4 per talk)
//
// GOTEX_BEAMER=0 still forces the emulation, for comparing the two paths.
func (e *Engine) realBeamer() bool {
	if os.Getenv("GOTEX_BEAMER") == "0" {
		return false
	}
	_, _, ok := e.findTeXFile("beamer", []string{".cls"})
	return ok
}

// emulateOnly reports whether a package must use the built-in stubs rather than
// its real file.
func emulateOnly(name string) bool {
	if isPGFFamily(name) {
		return !realPGF()
	}
	return neverLoadReal[name]
}

// isPGFFamily reports whether a package name belongs to pgf. The whole family
// has to be recognised, not the three headline names: pgf ships pgfcore,
// pgfmath, pgfpages, pgffor, pgfkeys, pgfsys, pgfrcs and more, and BEAMER ITSELF
// requires pgfpages, pgfmath and pgfcore directly.
//
// Guarding only {tikz, pgf, pgfplots} meant that the moment pgf's sources were
// findable — which is now easy, since a host can hand the engine a texmf tree —
// beamer pulled the REAL pgfcore through its own \RequirePackage, whatever
// GOTEX_PGF said. Measured over 500 real talks with beamer.cls present: 4286
// pages with pgf out of reach, 1487 with its sources on the search path and
// GOTEX_PGF UNSET. The flag was not the switch it looked like; reachability was.
func isPGFFamily(name string) bool {
	return name == "tikz" || strings.HasPrefix(name, "pgf")
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
	if name == "beamer" && ext == ".cls" {
		// This engine renders no speaker notes, and beamer's note machinery emits
		// PAGES. \setbeameroption{show notes on second screen} sets \beamer@notes and
		// makes every frame produce a note page beside the slide, which LaTeX hands to
		// pgfpages to merge onto ONE physical page. With no merging each frame came out
		// three pages instead of one: two frames became six where LaTeX makes two, and
		// the worst talk in the reference set rendered 21 pages against tectonic's 7.
		//
		// Switched off at \begin{document}, after the preamble's \setbeameroption has
		// had its say. Appended HERE, inside the class load, because that is where @ is
		// a letter.
		post += "\\AtBeginDocument{\\beamer@notesfalse}"
	}
	// \gotexeatdate consumes the OPTIONAL DATE a caller may write after the file
	// name — \RequirePackage{keyval}[1997/11/10] states the oldest acceptable
	// release. The engine loads whatever it finds, but an unread date is typeset,
	// and beamer's title page carried a stray "[1997/11/10]". It runs AFTER the
	// file (and after the frame is popped), where the date is the next thing in the
	// input: reading it BEFORE splicing the file would put the token that is not a
	// "[" back on the token stack, ahead of the file about to be spliced into the
	// character buffer — which silently swallowed everything after the \usepackage.
	e.pushInputLevel(pre + body + "\\" + endHook + post + "\\@gotex@endload \\gotexeatdate ")
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
// amsart is NOT here any more: it joins article/report/book on the real embedded
// class. It used to be gated to the emulation because its \newtheorem…[section]
// machinery looped on the engine and its theorem numbers came out as raw macro
// text; both are fixed (the runaway guard + delimited-arg brace-strip, and the
// \@thmcountersep/\@thmcounter hooks in AMSClassSubstrate), so \documentclass{amsart}
// now loads texmf/amsart.cls and typesets its own title/section/theorem heads.
// GOTEX_AMSART=0 forces the old emulation back, for comparing the two paths.
// letter/proc/slides/minimal are not embedded, so they fall back to the emulation
// regardless.
var emulatedClasses = map[string]bool{
	"letter": true, "proc": true, "slides": true, "minimal": true,
}

// realAmsart reports whether \documentclass{amsart} loads the REAL embedded
// amsart.cls rather than the built-in article-shaped emulation. Like realBeamer it
// is decided by whether the class file can be resolved (the embedded set always
// provides it); GOTEX_AMSART=0 forces the emulation, for A/B comparison. acmart
// papers already reach the real amsart through their bundled acmart.cls's
// \LoadClass{amsart} — this gate governs only the direct \documentclass{amsart}.
func (e *Engine) realAmsart() bool {
	if os.Getenv("GOTEX_AMSART") == "0" {
		return false
	}
	_, _, ok := e.findTeXFile("amsart", []string{".cls"})
	return ok
}

// setClassOptionList records \documentclass's options in \@classoptionslist, and the
// same list in \@raw@classoptionslist, as ltclass.dtx does on the first class load:
//
//	\ifx\@classoptionslist\relax
//	  \protected@xdef\@classoptionslist{\zap@space#2 \@empty}%
//	  \gdef\@raw@classoptionslist{#2}%
//
// Both start out \relax (see the class kernel), so a second \documentclass — or a
// \LoadClass from inside a class — leaves the first one's list alone, which is the
// test the reference makes.
//
// A package reads the list back to pick up class options it recognises. Leaving it
// undefined was not merely a blank: \@for over an undefined control sequence does not
// iterate over nothing, it SWALLOWS what follows. beamer runs that loop unguarded in
// \beamer@filterclassoptions and again in \ProcessOptionsBeamer, so any theme built on
// \ProcessOptionsBeamer — beamerthemesplit, and the fifteen themes that share its shape
// — took the rest of the document with it.
//
// The engine's option scanner has already trimmed each item, which is what \zap@space
// is there for.
func (e *Engine) setClassOptionList(opts []string) {
	if m := e.eq["@classoptionslist"]; m != nil && !(m.kind == mPrim && m.name == "relax") {
		return // a class list is already recorded: the first one wins
	}
	body := charToks(strings.Join(opts, ","))
	e.define("@classoptionslist", &meaning{kind: mMacro, body: body}, true)
	e.define("@raw@classoptionslist", &meaning{kind: mMacro, body: body}, true)
}

// doDocumentClass implements \documentclass[options]{class}: for a non-emulated
// class it loads class.cls when it can be resolved; for a standard class (and when
// the file cannot be found) it falls back to the built-in emulation.
func (e *Engine) doDocumentClass() {
	opts := e.scanBracketList()
	name := e.readBraceNameX()
	if name == "" {
		return
	}
	e.setPtsize(opts)          // record 10pt/11pt/12pt for \@ptsize even without the .cls
	e.setClassOptionList(opts) // \@classoptionslist, for packages that read them back
	// Record a paper-size class option (a4paper/letterpaper/…). The geometry package
	// inherits the class's paper size when its own options name none — without this,
	// a European \documentclass[a4paper] + \usepackage[margin]{geometry} document was
	// laid out on US letter, shrinking the text height and overflowing pages.
	for _, o := range opts {
		if _, ok := paperSizes[strings.TrimSpace(o)]; ok {
			e.classPaperSize = strings.TrimSpace(o)
		}
	}
	if twoColumnOptIn() && hasOption(opts, "twocolumn") && !classManagesOwnColumns(name) {
		e.twoColumn = true // \documentclass[twocolumn]{…}: two-column page layout (twocolumn.go)
	}
	if name == "beamer" && !e.realBeamer() {
		e.loadBeamer()
		return
	}
	if name == "amsart" {
		// Give the page builder amsart's real text block (360pt × 584pt, or 632pt
		// high on a4paper) as a PERSISTENT floor, in both paths. The emulation never
		// runs amsart.cls's \textwidth/\textheight and would keep the plain-TeX
		// default (6.5in × 8.9in), under-paginating; the real class DOES set them,
		// but through TeX assignments that the enclosing document group restores
		// after the page builder has run, so e.hsize/e.vsize revert post-render.
		// Setting the engine dimens here directly makes the budget stick either way
		// (the class's own \hsize=30pc then saves and restores this same value); a
		// later \usepackage{geometry} still overrides it, running after this.
		e.applyAmsartGeometry(opts)
		if !e.realAmsart() {
			return // GOTEX_AMSART=0 forces the built-in emulation
		}
	}
	if (name == "acmart" || name == "IEEEtran") && !e.classFileResolvable(name) {
		// acmart and IEEEtran are not embedded, and when the paper does not bundle
		// the .cls they fall to the article-shaped emulation, which sizes their page
		// wrong (the plain-TeX text block and the size-default leading). Give the page
		// builder the real class's single-column-equivalent text block and base
		// leading as a persistent floor, the way the amsart branch above does, so the
		// page count is right even though the two-column formats render single-column.
		// When the class IS resolvable it is loaded below and sizes its own page.
		if name == "acmart" {
			e.applyAcmartGeometry(opts)
			e.loadAcmartMetadata() // gobble acmart's top-matter metadata + CCSXML block
		} else {
			e.applyIEEEtranGeometry(opts)
		}
		return
	}
	if isRevtexClass(name) && !e.classFileResolvable(name) {
		// revtex is not embedded; the article-shaped fallback leaves \affiliation
		// and the author-list commands undefined — the corpus's single most common
		// undefined control sequence, which drops the whole author block. Load a
		// title-block emulation that keeps that content (revtex.go), then use the
		// article-shaped page (its geometry is already close). A bundled revtex .cls
		// resolves above and defines these itself.
		//
		// NOTE: revtex's journal styles (aps/prl) set the body in two columns, and the
		// two-column output routine (twocolumn.go) renders them correctly in position —
		// but only once paired with revtex's real (smaller) text height. Left on the
		// article-shaped page height, two full-height columns hold ~2x the text and the
		// document loses pages. So two-column stays OFF for revtex until that geometry is
		// wired; the measure proved the mechanism (2605.12538 medDisp 5.4->0.6) but the
		// page count regressed without the paired \textheight.
		e.loadRevtexEmulation()
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
	names := e.readBraceNameX()
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
	name := e.readBraceNameX()
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
// (used by size1x.clo names) reflects 10/11/12pt; it defaults to 10pt. It also
// wires the base size into the engine's font scale and leading: a [11pt]/[12pt]
// class sets \normalsize at 110%/120% of the 10pt design with the size clo's
// baselineskip (13.6/14.5pt), so body text is set at 11/12pt and wraps like real
// LaTeX. 10pt is the 100% default — byte-identical to the pre-existing behaviour.
func (e *Engine) setPtsize(opts []string) {
	pt := "0"                           // \@ptsize is (size-10): 0/1/2 for 10/11/12pt
	permille, leading := 1000, 12*unity // class base size and \normalsize leading
	for _, o := range opts {
		switch strings.TrimSpace(o) {
		case "10pt":
			pt, permille, leading = "0", 1000, 12*unity
		case "11pt":
			pt, permille, leading = "1", 1100, ptToSP(13.6)
		case "12pt":
			pt, permille, leading = "2", 1200, ptToSP(14.5)
		}
	}
	e.define("@ptsize", &meaning{kind: mMacro, body: stringToToks(pt)}, true)
	// Wire the class base size into the font system and the leading. 10pt is the
	// 100% default, so both are no-ops and 10pt documents stay byte-identical.
	e.baselineskip, e.baseBaselineskip = leading, leading
	e.scaleClassFontsToBase(permille)
}

// classManagesOwnColumns reports whether a class runs its own multi-column output
// routine rather than LaTeX's standard \twocolumn one, so the engine's two-column page
// builder (twocolumn.go) must NOT be switched on for it by a bare twocolumn option.
// revtex is the clearest case — it \let\twocolumn\@undefined and lays its columns
// through the ltxgrid grid engine — and the two-column journal classes (acmart,
// IEEEtran) size and fill their columns their own way; driving them through the generic
// routine mis-paginates (measured: revtex 2601.22272 loses a third of its pages).
func classManagesOwnColumns(name string) bool {
	if isRevtexClass(name) {
		return true
	}
	switch name {
	case "acmart", "IEEEtran", "IEEEconf", "aastex", "aastex6", "aastex61", "aastex62", "aastex631", "elsarticle":
		return true
	}
	return false
}

// hasOption reports whether the trimmed option list contains want (used to read
// class options such as twocolumn / onecolumn back from \documentclass).
func hasOption(opts []string, want string) bool {
	for _, o := range opts {
		if strings.TrimSpace(o) == want {
			return true
		}
	}
	return false
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
// resolves it runs the then-code and then reads the file, else it runs the
// else-code.
//
// The order is LaTeX's, and it is that way round. ltfiles.dtx:
//
//	\long\def\InputIfFileExists#1#2{%
//	  \IfFileExists{#1}%
//	   {#2\@addtofilelist{#1}\@@input \@filef@und}}
//
// so #2 runs BEFORE the file is read — a package announces itself, or sets what the
// file it is about to read expects to find. The then-code is pushed onto the file's
// own level, where it is read before the file's text and cannot outlive it.
func (e *Engine) doInputIfFileExists() {
	name := e.readBraceNameX()
	then := e.readBraceToksRaw()
	els := e.readBraceToksRaw()
	data, _, ok := e.findTeXFile(name, []string{"", ".tex"})
	if !ok {
		e.push(els)
		return
	}
	e.spliceInputFile(data)
	e.push(then)
}
