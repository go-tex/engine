// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-tex/texmf"
)

// The other half of TestAMinipageWhoseEndNeverComesDoesNotSwallowTheDocument
// (minipagerestore_test.go): that one covers a scan that stays on ONE input
// level, where the mouth can be rewound to a mark. This one covers the scan
// that crosses a file boundary, where it cannot — and where the body was
// SURRENDERED to the caller instead (go-tex/engine#786).
//
// A tikz node with `text width` is typeset by pgf inside a minipage. Put that
// node in an \input'ed figure file and the minipage's \end never arrives
// through this path, so the scan ran to the end of ALL input and handed back
// everything it had reached: measured, 134 tokens holding the node's text,
// \end{tikzpicture}, the document's remaining text and \end{document} itself.
// doMinipage typeset the lot into a 5cm vbox, so the page lost everything after
// the \input — silently, the \end{document} that would have ended the run
// having gone into the box too.
//
// ⛔ THE TEST IS A TRIPLE, NOT A SINGLE CASE. The same node typeset INLINE, and
// an \input'ed picture WITHOUT `text width`, are the controls: without them a
// red line says only "something here is broken" and cannot tell a defect in the
// node from one in the file seam. The discriminator is the pair that stays
// GREEN while the third goes red — measured, it did.
func TestATextWidthNodeInAnInputFileDoesNotSwallowTheRestOfTheDocument(t *testing.T) {
	const (
		// ⛔ THE ASSERTION IS ON RENDERED PAGE CONTENT, NOT ON THE ABSENCE OF AN
		// ERROR, and that is what makes it a witness for the MECHANISM and not only
		// for the symptom: tokens handed back when the input has already run out
		// could in principle be dropped instead of re-read, which would stop the
		// truncation without processing anything. A marker that is typeset onto the
		// page cannot have been dropped.
		markerAfter = "AFTERTHEINPUT"
		nodeWide    = `\node [text width=5cm, align=center, draw] (n) at (0,0) {X has a Taylor polymorphism} ;`
		nodePlain   = `\node [draw] (n) at (0,0) {X has a Taylor polymorphism} ;`
	)
	for _, c := range []struct {
		name     string
		body     string
		viaInput bool
	}{
		{"control: the wide node INLINE keeps the text after it", nodeWide, false},
		{"control: a plain node via \\input keeps the text after it", nodePlain, true},
		{"the wide node via \\input", nodeWide, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			withTeXMF(t, texmf.PGF)
			t.Setenv("GOTEX_PGF", "1")

			picture := `\begin{tikzpicture}` + "\n" + c.body + "\n" + `\end{tikzpicture}` + "\n"
			inner := picture
			if c.viaInput {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "figpart.tex"), []byte(picture), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Setenv("TEXINPUTS", dir)
				inner = `\input{figpart}`
			}

			e, err := buildEngine(Options{Lenient: true, NoProgressLimit: NoProgressLimitHeavy}, true)
			if err != nil {
				t.Fatal(err)
			}
			src := `\documentclass{article}\usepackage{tikz}\begin{document}` +
				`BEFORETHEINPUT ` + inner + ` ` + markerAfter + `\end{document}`
			if _, err := e.Run(src); err != nil {
				t.Fatalf("viaInput=%v: %v", c.viaInput, err)
			}
			out := strings.Join(e.RenderPages(72), "")

			if !strings.Contains(out, "BEFORETHEINPUT") {
				t.Errorf("the text BEFORE the picture is gone too, which is not this defect's shape:\n%s", out)
			}
			if !strings.Contains(out, markerAfter) {
				t.Errorf("%q never reached the page: everything after the picture was swallowed", markerAfter)
			}
			// go-tex/engine#816: keeping the document was half of it. The node's OWN
			// text must reach the page too. Before, the scan's exit closed the
			// minipage's group as it handed the tokens back, so the \end{minipage}
			// that pgf queues with \aftergroup closed nothing of its own and the
			// text ran outside its box: the node rendered EMPTY. The inline control
			// shows that this text can render at all.
			if !strings.Contains(out, "Taylor") {
				t.Errorf("the node's own text never reached the page: the node rendered empty (go-tex/engine#816)")
			}
		})
	}
}
