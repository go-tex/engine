// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \showthe — print what \the would give, and carry on.
//
// tex.web:23721-23726, the \showthe branch of show_whatever:
//
//	@ @<Show the current value of some parameter...@>=
//	begin p:=the_toks;
//	if interaction=error_stop_mode then wake_up_terminal;
//	print_nl("> "); token_show(temp_head);
//	flush_list(link(temp_head)); goto common_ending;
//	end
//
// So: expand the quantity exactly as \the does, and print it on a line of its own
// behind "> ". What follows in the reference (:23676-23692) is the interaction
// machinery — a help message and a stop for input under error_stop_mode. A batch
// engine has no terminal to stop at, and TeX itself does not stop under \batchmode or
// \nonstopmode: it prints and continues. That is the behaviour implemented here.
//
// ⛔⛔ IT WAS UNDEFINED, AND THAT BLOCKED OUR OWN MEASUREMENT TOOLING. measure dimens
// (go-tex/measure, harness/probe.go) writes one \showthe per dimension it wants and
// parses the answers; against this engine it read ZERO of them and refused —
// correctly, rather than returning zeros:
//
//	⛔ probe: asked 5 dimensions, read 0 — the correspondence is lost, so no value
//	   is returned (output: 5247 bytes)
//
// Every dimension compared with the reference during go-tex/engine#683 therefore had
// to go through \message{[X=\the…]} by hand, on both engines. (go-tex/engine#686)

// doShowThe implements \showthe: it reads the same operand \the reads and writes the
// result to the message stream, preceded by "> " on its own line.
//
// ⛔ THE OPERAND IS READ BY \the ITSELF, not re-parsed here. \the accepts a long list
// of things — a count, a dimen, a skip, a token register, a font, a catcode — and
// duplicating that reader would be a second grammar to keep in step with the first.
// Pushing \the back into the input and letting it run is what the reference does
// (the_toks is literally \the's own routine) and keeps one reader.
func (e *Engine) doShowThe() {
	// \the leaves its answer on the input as a pushed token list (pushString), so the
	// lists above this mark are exactly what it produced. Taking everything above the
	// mark — rather than assuming a single push — keeps this correct for the operands
	// that reach \the by more than one route.
	mark := len(e.lists)
	e.doThe()
	var val []tok
	for len(e.lists) > mark {
		val = append(e.lists[len(e.lists)-1], val...)
		e.lists = e.lists[:len(e.lists)-1]
	}
	if e.shows.Len() > 0 {
		e.shows.WriteByte('\n')
	}
	e.shows.WriteString("> " + e.toksToString(val) + ".")
}
