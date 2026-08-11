// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's cross-reference mechanism (\label / \ref and its
// variants). LaTeX resolves references through an auxiliary file written on one
// run and read on the next, so a forward \ref (used before its \label) resolves
// on the second pass. The engine mirrors this exactly: CompileToPDF /
// CompileToSVGPages compile the source twice when it contains \label, carrying the
// label table from the first pass into the second (see compileLabels).
//
// A \label records the current \@currentlabel — the reference text the last
// counter-stepping command (\section, \subsection, an equation, an enumerate item)
// left behind, fully expanded to a string. A \ref pushes that string back into the
// input so it typesets in place; an unresolved key yields "??" (as in LaTeX).

// setLabel stores key → the fully-expanded \@currentlabel.
func (e *Engine) doLabel() {
	key := e.readBraceName()
	if key == "" {
		return
	}
	val := e.toksToString(e.expandList([]tok{csTok("@currentlabel")}))
	if e.labels == nil {
		e.labels = map[string]string{}
	}
	e.labels[key] = val
}

// doRef pushes the recorded reference text for a key back into the input. An
// unknown or empty key yields "??", matching LaTeX's unresolved-reference marker.
func (e *Engine) doRef() {
	key := e.readBraceName()
	e.pushString(e.refText(key))
}

// doEqref is \eqref: like \ref but parenthesised, for equation numbers.
func (e *Engine) doEqref() {
	key := e.readBraceName()
	e.pushString("(" + e.refText(key) + ")")
}

// refText returns the stored reference text for key, or "??" when unresolved.
func (e *Engine) refText(key string) string {
	if v, ok := e.labels[key]; ok && v != "" {
		return v
	}
	return "??"
}

// doCite implements \cite{k1,k2,…}: the bracketed, comma-joined reference numbers
// of one or more \bibitem entries, e.g. "[1, 3]". Each \bibitem stores its number
// in the label table (via \label), so a \cite before the bibliography resolves on
// the second pass. \nocite is a no-op here (it only affects a real .bib run).
func (e *Engine) doCite() {
	keys := splitComma(e.readBraceName())
	for i, k := range keys {
		keys[i] = e.refText(k)
	}
	e.pushString("[" + joinComma(keys) + "]")
}

// needsTwoPass reports whether the source must be compiled twice: it defines a
// \label or a \bibitem whose number a \ref or \cite may use before the
// definition is seen, or it requests a \tableofcontents / \listoffigures /
// \listoftables, whose entries are only known once the whole document has run
// (LaTeX's .toc/.lof/.lot mechanism — see toc.go).
func needsTwoPass(src []byte) bool {
	return indexOf(src, `\label`) >= 0 ||
		indexOf(src, `\bibitem`) >= 0 ||
		indexOf(src, `\tableofcontents`) >= 0 ||
		indexOf(src, `\listoffigures`) >= 0 ||
		indexOf(src, `\listoftables`) >= 0
}

// splitComma splits a comma-separated key list, trimming spaces around each key.
func splitComma(s string) []string {
	var out []string
	start := 0
	flush := func(end int) {
		k := trimSpaces(s[start:end])
		if k != "" {
			out = append(out, k)
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			flush(i)
			start = i + 1
		}
	}
	flush(len(s))
	return out
}

// joinComma joins keys with ", ".
func joinComma(keys []string) string {
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
}

// trimSpaces drops leading and trailing ASCII spaces.
func trimSpaces(s string) string {
	i, j := 0, len(s)
	for i < j && s[i] == ' ' {
		i++
	}
	for j > i && s[j-1] == ' ' {
		j--
	}
	return s[i:j]
}

// indexOf is a tiny substring search (avoids importing bytes just for this).
func indexOf(hay []byte, needle string) int {
	n := len(needle)
	for i := 0; i+n <= len(hay); i++ {
		if string(hay[i:i+n]) == needle {
			return i
		}
	}
	return -1
}
