// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \end<name> can reach the terminator a body scanner is hunting through ONE more
// parameterless macro. beamer's columns are built that way
// (beamerbaseframecomponents.sty):
//
//	\newcommand<>\beamer@columncom[2][\beamer@colmode]{%
//	  \beamer@colclose
//	  \def\beamer@colclose{\end{minipage}\hfill\end{actionenv}\ignorespaces}%
//	  \begin{actionenv}#3\begin{minipage}…}
//
// so \column opens a minipage and STORES its \end{minipage} in \beamer@colclose, run at
// the next \column or at \end{columns}. \endcolumns therefore begins with
// \beamer@colclose, and a scanner reading raw stores the \end{columns} and carries on.
//
// The deeper rule is consulted ONLY where the scan is about to leave the file it began
// in — applied from the start it runs terminators early and costs pages.
func TestABodyRewindsToAnEndThatLeadsFurther(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	// The environment lives in a FILE of its own, as a [fragile] frame's body does:
	// \endenveloppe leads to \end{minipage} through \ferme, so the narrow rule stores
	// the \end{enveloppe} raw and the scan runs to the end of that file.
	out, err := e.Run(`\hsize=300pt` +
		`\def\ferme{\end{minipage}}` +
		`\newenvironment{enveloppe}{\begin{minipage}{100pt}}{\ferme}` +
		`\immediate\openout7=zz-bord.tex` +
		`\immediate\write7{\noexpand\begin{enveloppe}X\noexpand\end{enveloppe}}` +
		`\immediate\closeout7` +
		`\setbox0=\hbox{\input{zz-bord.tex}}\message{[largeur \the\wd0]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Without the rewind the scan runs to the end of the file and the box measures
	// nothing at all.
	if got := trimNL(out); strings.Contains(got, "[largeur 0.0pt]") {
		t.Errorf("= %q, want the minipage closed where \\endenveloppe leads", got)
	}
}

// The deeper rule must NOT fire while the scan is still inside its own file: running an
// environment's terminator early cost 28 pages over 200 talks when it did.
func TestTheDeeperEndRuleDoesNotFireEarly(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	// \endinterne leads to \end{minipage} through \ferme, but the minipage's own
	// \end{minipage} is right there in the input: the narrow rule finds it, and the
	// body must be exactly "X\begin{interne}Y\end{interne}".
	out, err := e.Run(`\hsize=300pt` +
		`\def\ferme{\end{minipage}}` +
		`\newenvironment{interne}{}{\ferme}` +
		`\setbox0=\hbox{\begin{minipage}{100pt}X\end{minipage}}\message{[largeur \the\wd0]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := trimNL(out); !strings.Contains(got, "[largeur 100.0pt]") {
		t.Errorf("= %q, want the minipage to end at its own \\end (100.0pt)", got)
	}
}
