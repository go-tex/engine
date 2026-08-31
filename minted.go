// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// This file implements the minted package's code facilities on top of the
// listings verbatim machinery (see listings.go). minted normally shells out to
// Pygments to syntax-highlight code (\usepackage{minted} + -shell-escape); with no
// Pygments available it typesets the code verbatim — which is exactly what minted's
// OWN draft mode does (\begin{minted}[draft]… -> \begin{Verbatim}). The engine has
// no shell escape, so it always takes that faithful verbatim fallback rather than
// leaving \begin{minted} an undefined environment whose head ([opts]{lang}) and
// body leak into the running text.
//
// Reference (minted.sty): the environment is \newenvironment{minted}[2][], i.e.
//
//	\begin{minted}[options]{language} … code … \end{minted}
//
// — an OPTIONAL [options] then a MANDATORY {language}; the body is read verbatim to
// \end{minted}. The inline forms are \mintinline[options]{language}<delim>code<delim>
// and \mint[options]{language}<delim>code<delim>, and \inputminted[options]{language}{file}
// sets a file's contents (that file is not read here — its two arguments are consumed
// so nothing leaks, and it is left as a follow-up).

// doMinted typesets \begin{minted}[options]{language} … \end{minted} as a verbatim
// code block. The options are honoured for line numbers (minted's linenos) and a
// frame; the language is consumed and ignored (no highlighting).
func (e *Engine) doMinted() {
	opts, _ := e.scanRawOptBracket() // optional [options]
	e.scanRawBraceArg()              // mandatory {language}, consumed and ignored
	content, line := e.readRawEnvBody(`\end{minted}`)
	e.renderVerbatimBlock(content, line, mintedOptions(opts))
}

// doMintinline typesets \mintinline[options]{language}<delim>code<delim> (and the
// equivalent \mint) inline in the tt font, reusing \lstinline's delimiter reader.
func (e *Engine) doMintinline() {
	e.scanRawOptBracket() // optional [options]; ignored inline
	e.scanRawBraceArg()   // mandatory {language}; ignored
	e.setInlineVerbatim()
}

// doInputminted consumes \inputminted[options]{language}{file}. The referenced file
// is not read here (a follow-up), but both mandatory arguments are consumed so the
// language and file name never leak into the text.
func (e *Engine) doInputminted() {
	e.scanRawOptBracket() // optional [options]
	e.scanRawBraceArg()   // {language}
	e.scanRawBraceArg()   // {file}
}

// scanRawBraceArg reads a required "{…}" argument straight from the raw base input
// at the cursor — the verbatim-environment counterpart of a tokenized brace read,
// used where the surrounding syntax (minted's {language}) sits in front of a
// verbatim body. Leading spaces, tabs and newlines are skipped; a matched
// (brace-depth-aware) group is returned and the cursor advanced past its closing
// brace. With no '{' present nothing is consumed and "" is returned.
func (e *Engine) scanRawBraceArg() string {
	i := e.bpos
	for i < len(e.base) && (e.base[i] == ' ' || e.base[i] == '\t' || e.base[i] == '\n' || e.base[i] == '\r') {
		i++
	}
	if i >= len(e.base) || e.base[i] != '{' {
		return ""
	}
	depth := 1
	j := i + 1
	for j < len(e.base) && depth > 0 {
		switch e.base[j] {
		case '{':
			depth++
		case '}':
			depth--
		}
		if depth == 0 {
			break
		}
		j++
	}
	arg := string(e.base[i+1 : j])
	if j < len(e.base) {
		j++ // consume the closing '}'
	}
	e.bpos = j
	return arg
}

// mintedOptions maps a minted "[key=value,…]" option string to the honoured
// verbatim-block options. minted spells line numbering "linenos" (a boolean key,
// on when present) rather than listings' "numbers", and shares "frame"; every other
// key (fontsize, bgcolor, breaklines, …) is accepted and ignored.
func mintedOptions(opts string) lstOptions {
	o := parseLstOptions(opts) // handles frame= (and numbers=, harmless if unused)
	for _, part := range strings.Split(opts, ",") {
		key, val, hasVal := strings.Cut(part, "=")
		if strings.TrimSpace(key) != "linenos" {
			continue
		}
		// A bare "linenos" turns numbering on; "linenos=false"/"none" turns it off.
		v := strings.TrimSpace(val)
		o.numbers = !hasVal || (v != "false" && v != "none")
	}
	return o
}
