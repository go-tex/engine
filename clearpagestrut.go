// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// doClearpageStrut is LaTeX's \clearpage opening move: contribute an empty
// \hbox when the page is still empty.
//
// ⛔ IT IS NOT DECORATION. latex.ltx:15061-15069 reads
//
//	\def\clearpage{%
//	  \ifvmode \ifnum \@dbltopnum =\m@ne \ifdim \pagetotal <\topskip
//	        \hbox{}%
//	  \fi \fi \fi
//	  \newpage …
//
// and that \hbox{} is why \prevdepth is 0.0pt at the start of a LaTeX document
// body in the reference and was -1000pt (ignore_depth) here — #699. Nothing
// assigns \prevdepth: appending a BOX to the vertical list sets it to that box's
// depth, and an empty hbox has depth 0.
//
// The chain, bisected against tectonic 0.17.0 rather than reasoned about:
//
//	article.cls:640   \onecolumn   ->   \clearpage   ->   \hbox{}
//
// and the probe that separates the mechanism from everything else:
//
//	a deferred \write (a whatsit)  -> \prevdepth unchanged, -1000.0pt
//	a \penalty                     -> unchanged
//	a \kern                        -> unchanged
//	an \hbox{}                     -> 0.0pt
//
// ⛔ THE GUARD IS NOT PORTED LITERALLY, AND PORTING IT WOULD HAVE BEEN WORSE
// THAN NOT PORTING IT AT ALL. LaTeX asks \ifdim\pagetotal<\topskip. In this
// engine \pagetotal is an accept-and-ignore register (texparams.go): a document
// may set it and read it back, and nothing maintains it, so it is 0 for ever.
// The condition would read 0pt < 10pt — ALWAYS TRUE — and every \clearpage in
// every document would contribute a box, which is not the reference's behaviour
// anywhere except on an empty page. A guard whose input is dead is worse than a
// missing guard: it looks like fidelity and is a change of behaviour everywhere.
// (\@dbltopnum does not exist here either.)
//
// What is used instead is the state this engine really holds: the main vertical
// list is empty. That is narrower than LaTeX's condition — LaTeX asks whether
// the CURRENT PAGE is empty, and this engine gathers the whole document into one
// list and splits it at the end, so "a second \clearpage on an already-empty
// page" is a case we cannot see and do not reproduce. It is exact in the case
// this is for, which is the one the reference exercises at class-load time, and
// it cannot fire in the middle of a document, which is where a literal port
// would have done damage.
//
// ⚠ THE TEST IS TWO-SIDED ON PURPOSE. \documentclass{article} must read 0.0pt
// AND \documentclass{minimal} must still read -1000.0pt, both measured from
// tectonic. A change that makes both read 0 has not ported anything — it has
// hard-coded the answer to one of them, which is the fallback #699 offered and
// which would stop matching the moment a class does not call \onecolumn.
// ⛔⛔ AND THE BOX IS NOT CONTRIBUTED, ONLY ITS EFFECT. The first version of
// this appended a real empty hbox, which is what the reference's source says.
// It cost EIGHTY-ONE tests: an extra box at the head of the main vertical list
// takes the \topskip of the first page, and every document's first line then
// sits a \baselineskip lower.
//
// The reference does not keep it either, and that is measured rather than
// assumed: a document whose whole body is the word "Text." comes out of
// tectonic as ONE page, not two. The \hbox{} is contributed and then DISCARDED
// at the forced break that immediately follows it (latex.ltx:15069-15072 —
// \newpage, \write, \vbox{}, \penalty -\@Mi). Nothing of it reaches a page.
//
// What survives a page break in TeX is \prevdepth, not the box. So the port is
// the box's one surviving effect: \prevdepth takes the depth an empty hbox
// would have had, which is zero.
//
// ⚠ This is NOT the fallback #699 offered — "set \prevdepth to 0 in the class
// lead". The difference is that this still runs from \onecolumn, so a class
// that never calls it (minimal) keeps ignore_depth exactly as the reference
// does, and the value is derived from the box rather than chosen to match a
// reading. The two-sided test is what holds that distinction, and it is the
// test, not this comment, that will catch a future change that loses it.
func (e *Engine) doClearpageStrut() {
	if len(e.mvl) != 0 {
		return
	}
	e.prevDepth = 0
}
