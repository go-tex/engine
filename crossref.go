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

// hasLabels reports whether a two-pass compile is needed (the source defines a
// \label whose value a \ref may need before it is seen).
func hasLabels(src []byte) bool {
	return indexOf(src, `\label`) >= 0
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
