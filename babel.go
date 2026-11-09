// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// babel, french only.
//
// babel was a stub: \usepackage[french]{babel} printed "Abstract" and "Contents"
// where tectonic prints "Résumé" and "Table des matières", and set "Slurm: x" where
// tectonic sets "Slurm : x". The two things babel-french does that show on a page are
// done here, from french.ldf (babel-french 3.x, the file tectonic 0.17.0 loads):
//
//  1. THE CAPTIONS. french.ldf lines 923-979 are `\StartBabelCommands*{\BabelLanguages}
//     {captions}` blocks of \SetString; babel runs them at \begin{document}, which
//     is why they are installed from \AtBeginDocument and not at \usepackage.
//  2. THE HIGH PUNCTUATION. Before ; ! ? a thin space (french.ldf:170,
//     \FBthinspace = \hskip .5\fontdimen2\font) and before : a full one (:169,
//     \FBcolonspace = \space), each preceded by \penalty\@M so the mark never starts
//     a line, and a space the author typed is removed first (the \unskip in 378,
//     395, 438, 449, 460, 471). tectonic runs XeTeX, where french.ldf does this with
//     \XeTeXinterchartoks (lines 337-418): between a character and the punctuation
//     class, or between a space and it. Measured on its PDF with pdftotext -bbox,
//     a typed "Slurm :" and a typed "Slurm:" both leave 3.318pt before the colon,
//     the width of the interword space, and "Vraiment!" 1.644pt before the mark,
//     half of it. The spacing is off in \ttfamily (FB@spacing@off, :512-517), in
//     maths, and after "(" "[" or another mark (class punctnul, :411-413).
//
// Not done, and said so: \today (the engine has no clock, Options.Date is the
// document date and is left alone), the part ordinals above ten, the guillemet spacing
// of \og \fg, the " – " caption separator, \selectlanguage's captions (it moves the
// punctuation spacing only), and every language but french.

// frenchCaptions is the \SetString set of french.ldf:938-979, in UTF-8.
const frenchCaptions = `\def\refname{Références}\def\abstractname{Résumé}` +
	`\def\bibname{Bibliographie}\def\prefacename{Préface}\def\chaptername{Chapitre}` +
	`\def\appendixname{Annexe}\def\contentsname{Table des matières}` +
	`\def\listfigurename{Table des figures}\def\listtablename{Liste des tableaux}` +
	`\def\indexname{Index}\def\figurename{Figure}\def\tablename{Table}` +
	`\def\pagename{page}\def\seename{voir}\def\alsoname{voir aussi}` +
	`\def\proofname{Démonstration}` + frenchPartName

// frenchPartName is \partname of french.ldf:961-978 with \ifFBPartNameFull true, the
// package's default: the part's ordinal, then "partie" (\partnameord), and \thepart
// emptied (\FB@emptypart) so the heading reads "Première partie" and not "… I".
// tectonic prints " partie" for \partname outside a \part, where the counter is 0
// and \csname ordinal\endcsname is \relax; this does the same.
const frenchPartName = `\def\partname{\csname gotexfrordinal\romannumeral\value{part}\endcsname` +
	`\space partie\def\thepart{\unskip}}` +
	`\expandafter\def\csname gotexfrordinali\endcsname{Première}` +
	`\expandafter\def\csname gotexfrordinalii\endcsname{Deuxième}` +
	`\expandafter\def\csname gotexfrordinaliii\endcsname{Troisième}` +
	`\expandafter\def\csname gotexfrordinaliv\endcsname{Quatrième}` +
	`\expandafter\def\csname gotexfrordinalv\endcsname{Cinquième}` +
	`\expandafter\def\csname gotexfrordinalvi\endcsname{Sixième}` +
	`\expandafter\def\csname gotexfrordinalvii\endcsname{Septième}` +
	`\expandafter\def\csname gotexfrordinalviii\endcsname{Huitième}` +
	`\expandafter\def\csname gotexfrordinalix\endcsname{Neuvième}` +
	`\expandafter\def\csname gotexfrordinalx\endcsname{Dixième}`

// babelLanguages are the language names a [..] option list can carry; the last one
// is the main language, as in babel.
var babelLanguages = map[string]bool{
	"english": true, "american": true, "british": true, "french": true, "german": true,
	"ngerman": true, "spanish": true, "italian": true, "portuguese": true, "dutch": true,
	"russian": true, "polish": true, "greek": true, "latin": true, "swedish": true,
	"norsk": true, "danish": true, "finnish": true, "czech": true, "turkish": true,
	"catalan": true,
}

// babelMainLanguage is the language babel makes the main one: main=<lang> if given,
// else the last language named in the options.
func babelMainLanguage(opts []string) string {
	main := ""
	for _, o := range opts {
		o = strings.TrimSpace(o)
		if v, ok := strings.CutPrefix(o, "main="); ok {
			return strings.TrimSpace(v)
		}
		if babelLanguages[o] {
			main = o
		}
	}
	return main
}

// applyBabel is \usepackage[opts]{babel}: it turns the french behaviour on when
// french is the main language and leaves every other language as the stub it was.
func (e *Engine) applyBabel(opts []string) {
	if babelMainLanguage(opts) != "french" {
		return
	}
	e.frenchPunct = true
	// \gotexfrenchoff is french.ldf's \FB@spacing switched off (:229-240) by
	// \ttfamily and back on by \rmfamily and \sffamily (:512-514). It is a count so
	// that the group a font switch lives in restores it, as \ifFB@spacing is.
	e.pushPackageLevel(`\newcount\gotexfrenchoff` +
		`\let\gotexttORI\ttfamily\def\ttfamily{\gotexfrenchoff=1 \gotexttORI}` +
		`\let\gotexrmORI\rmfamily\def\rmfamily{\gotexfrenchoff=0 \gotexrmORI}` +
		`\let\gotexsfORI\sffamily\def\sffamily{\gotexfrenchoff=0 \gotexsfORI}` +
		`\AtBeginDocument{` + frenchCaptions + `}` +
		`\def\selectlanguage#1{\gotexselectlanguage{#1}}`)
}

// doSelectLanguage is \selectlanguage{name} once french is the main language: it
// moves the high-punctuation spacing, nothing else (see the list above).
func (e *Engine) doSelectLanguage() {
	if name := e.grabEnvNameArg(); name != "" {
		e.frenchPunct = name == "french"
	}
}

// inTypewriter reports whether \ttfamily switched the spacing off in this group.
func (e *Engine) inTypewriter() bool { return e.namedInt("gotexfrenchoff") != 0 }

// frenchHighPunct is french.ldf's behaviour before : ; ! ? in horizontal mode, run
// just before the mark is set. See the header for where each number comes from.
func (e *Engine) frenchHighPunct(ch rune) {
	if !e.frenchPunct || !e.inPar || e.curFont == nil {
		return
	}
	switch ch {
	case ':', ';', '!', '?':
	default:
		return
	}
	if e.inTypewriter() {
		return
	}
	// \ifdim\lastskip>1sp: a space the author typed is replaced, not doubled.
	list, typed := e.parList, false
	if n := len(list); n > 0 {
		if g, ok := list[n-1].(glueNode); ok && g.spec.width > 1 {
			list, typed = list[:n-1], true
		}
	}
	// "(" "[" and the marks themselves are the punctnul class: nothing before them,
	// and a space the author typed there stays.
	if c, ok := lastChar(list); ok {
		switch c {
		case '(', '[', ':', ';', '!', '?', 0xA0, 0x202F:
			return
		}
	}
	if typed {
		e.parList = list
	}
	sp := e.spaceGlueOf(e.curFont)
	if ch != ':' {
		sp = glueSpec{width: sp.width / 2} // \hskip .5\fontdimen2\font: no stretch, no shrink
	}
	e.parList = append(e.parList, penaltyNode{penalty: 10000}, glueNode{spec: sp})
}
