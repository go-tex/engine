// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// renderBody compiles a lenient document body and returns the engine plus the
// concatenated SVG of every page.
func renderBody(t *testing.T, body string) (*Engine, string) {
	t.Helper()
	src := `\documentclass{article}\begin{document}` + body + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile %q: %v", body, err)
	}
	return e, strings.Join(e.RenderPages(e.renderMargin(0)), "")
}

// The comment package's \begin{comment}…\end{comment} is discarded ENTIRELY (no
// placeholder — a comment is invisible), and the text after it still renders: the
// gobble scan stopped at the matching \end{comment}.
func TestCommentEnvGobbled(t *testing.T) {
	skip := runSkip(t, `Before \begin{comment} hidden \textbf{stuff} \item stray `+
		`\somecmd \end{comment} After \zzmark`)
	// Commands inside the comment body must not be seen at all.
	for _, c := range []string{"somecmd", "textbf", "item"} {
		if skip[c] != 0 {
			t.Errorf("command %q inside the comment leaked (skip=%d)", c, skip[c])
		}
	}
	// The marker after \end{comment} was reached, so the scan stopped correctly.
	if skip["zzmark"] != 1 {
		t.Errorf("content after \\end{comment} not processed: zzmark=%d (want 1)", skip["zzmark"])
	}
	// Unlike a picture gobble, a comment leaves NO placeholder.
	if skip["comment"] != 0 {
		t.Errorf("comment should leave no placeholder, got skip=%d", skip["comment"])
	}
}

// A comment whose body carries an unbalanced brace and a stray \item must NOT
// leave a group open at end of document — typesetting such a body (rather than
// gobbling it) left a group open and dropped the page it sat on, a whole-document
// swallow on real arXiv papers (e.g. 2608.13400: 0 → 2384 glyphs once gobbled).
func TestCommentEnvUnbalancedNoSwallow(t *testing.T) {
	e, svg := renderBody(t, `VISIBLEBODYTEXT `+
		`\begin{comment}`+
		`\item leaked {unbalanced brace and \textbf{bold never closed `+
		`\end{comment}`+
		` MOREVISIBLETEXT`)
	if d := e.Diagnostics(); d.OpenGroups != 0 {
		t.Errorf("comment body left %d group(s) open (should be gobbled whole)", d.OpenGroups)
	}
	if !strings.Contains(svg, "<path") {
		t.Error("the document rendered no glyphs — the comment swallowed the body")
	}
}

// \excludecomment{name} turns `name` into another silently-gobbled environment.
func TestExcludeComment(t *testing.T) {
	skip := runSkip(t, `\excludecomment{secret}`+
		`Keep \begin{secret} \hiddencmd buried \end{secret} Kept \zzafter`)
	if skip["hiddencmd"] != 0 {
		t.Errorf("\\excludecomment body leaked \\hiddencmd (skip=%d)", skip["hiddencmd"])
	}
	if skip["zzafter"] != 1 {
		t.Errorf("content after the excluded env not processed: zzafter=%d (want 1)", skip["zzafter"])
	}
}

// grabEnvNameArg returns "" (leaving the input untouched) when the next token is
// not an opening brace, and "" on exhausted input — the error branches
// \excludecomment/\includecomment fall through when their {name} is missing.
func TestGrabEnvNameArgErrorBranches(t *testing.T) {
	// A non-brace next token is pushed back and "" returned.
	e := &Engine{noBase: true}
	e.push([]tok{chTok('Z', catLetter)})
	if got := e.grabEnvNameArg(); got != "" {
		t.Errorf("no brace should yield %q, got %q", "", got)
	}
	if t2, ok := e.getNext(); !ok || t2.ch != 'Z' {
		t.Errorf("the non-brace token was consumed, not pushed back: %v ok=%v", t2, ok)
	}
	// Exhausted input yields "" without panicking.
	e2 := &Engine{noBase: true}
	if got := e2.grabEnvNameArg(); got != "" {
		t.Errorf("exhausted input should yield %q, got %q", "", got)
	}
}

// \excludecomment / \includecomment with no {name} argument are harmless no-ops
// (the following text still renders) — the missing-argument branch.
func TestCommentDeclarationsWithoutName(t *testing.T) {
	skip := runSkip(t, `\excludecomment \zzone \includecomment \zztwo`)
	if skip["zzone"] != 1 || skip["zztwo"] != 1 {
		t.Errorf("text after an argument-less comment declaration was lost: %v", skip)
	}
}

// \includecomment{name} makes `name`'s body typeset (begin/end become no-ops), so
// a command inside it is seen normally rather than gobbled.
func TestIncludeComment(t *testing.T) {
	skip := runSkip(t, `\includecomment{shown}`+
		`\begin{shown} \visiblecmd here \end{shown} \zzafter`)
	if skip["visiblecmd"] != 1 {
		t.Errorf("\\includecomment body should typeset \\visiblecmd (skip=%d, want 1)", skip["visiblecmd"])
	}
	if skip["zzafter"] != 1 {
		t.Errorf("content after the included env not processed: zzafter=%d (want 1)", skip["zzafter"])
	}
}
